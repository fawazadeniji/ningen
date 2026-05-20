package handlers

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"ningen/internal/agents"
	"ningen/internal/llm"
	"ningen/internal/models"
	"ningen/internal/rag"
)

const neutralHumanizerPrompt = `You are a warm, professional advisor. Rewrite the provided text to be clear, friendly, and conversational — like a knowledgeable human speaking directly to the user. Keep every fact and recommendation intact. Output only the rewritten text, nothing else.`

// humanizeText rewrites text through the humanizer.
// When nigerian=true it uses the full Nigerian cultural voice (provider.Humanize).
// When nigerian=false it uses a neutral warm register via a direct Complete call.
// Falls back to the original text on any error.
func humanizeText(ctx context.Context, provider llm.LLMProvider, text, userContext string, nigerian bool) string {
	if nigerian {
		if h, err := provider.Humanize(ctx, text, userContext); err == nil {
			return h
		}
		return text
	}
	msgs := []llm.Message{
		{Role: "system", Content: neutralHumanizerPrompt},
		{Role: "user", Content: fmt.Sprintf("User context: %s\n\nText:\n%s", userContext, text)},
	}
	if h, err := provider.Complete(ctx, msgs); err == nil {
		return h
	}
	return text
}

const (
	defaultRecommendLimit = 10
	candidatePoolSize     = 50
	agentTimeout          = 25 * time.Second
	fallbackClarifyQ      = "Could you tell me more — are you looking for a book, a product, or a place to eat? What mood are you in?"
)

// RecommendHandler serves POST /recommend.
// It runs the four-stage SIGNAL pipeline:
//  1. Signal Extractor  — LLM distills persona + history into a structured UserSignal.
//  2. Multi-vector Retrieval — each search query is embedded and searched; results unioned.
//  3. Quality Gate      — LLM validates retrieval quality; can REFINE queries or ASK for input.
//  4. Psychographic Reranker — LLM ranks candidates by psychographic fit to the signal.
//
// All user-facing text passes through the Nigerian cultural humanizer before response.
func RecommendHandler(d *Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req models.RecommendRequest
		if !decode(w, r, &req) {
			return
		}
		if req.UserPersona == "" {
			writeError(w, http.StatusBadRequest, "user_persona is required")
			return
		}

		provider, err := d.LLM.Get(req.Provider)
		if err != nil {
			writeError(w, http.StatusBadRequest, "unknown or unavailable provider: "+req.Provider)
			return
		}

		ctx := r.Context()

		nigerian := req.NigerianFlavor == nil || *req.NigerianFlavor

		// Cold-start gate: no history means no intent signal at all.
		if len(req.History) == 0 {
			question := humanizeText(ctx, provider,
				"What kind of item are you looking for — a book, a product, or a place to eat? Tell me a little about what you need or what mood you're in.",
				req.UserPersona, nigerian)
			writeJSON(w, http.StatusOK, models.RecommendResponse{RequiresInput: true, Question: question})
			return
		}

		limit := req.Limit
		if limit <= 0 {
			limit = defaultRecommendLimit
		} else if limit > candidatePoolSize {
			limit = candidatePoolSize
		}

		// Pre-search: embed the raw last user turn to sample corpus examples.
		// These ground the Extractor in what actually exists in the DB before it
		// generates search queries, preventing queries for items that don't exist.
		sampleCtx, sampleCancel := context.WithTimeout(ctx, agentTimeout)
		corpusExamples := sampleCorpus(sampleCtx, d, req.History)
		sampleCancel()

		// Stage 1 — Signal Extraction (corpus-aware)
		extractCtx, extractCancel := context.WithTimeout(ctx, agentTimeout)
		signal, err := agents.NewExtractor(provider).Extract(extractCtx, req.UserPersona, req.History, corpusExamples)
		extractCancel()
		if err != nil {
			writeError(w, http.StatusInternalServerError, "signal extraction failed: "+err.Error())
			return
		}

		if signal.ClarifyNeeded {
			raw := strings.TrimSpace(signal.ClarifyReason)
			if raw == "" {
				raw = fallbackClarifyQ
			}
			question := humanizeText(ctx, provider, raw, req.UserPersona, nigerian)
			writeJSON(w, http.StatusOK, models.RecommendResponse{RequiresInput: true, Question: question})
			return
		}

		// Stage 2 — Multi-vector Retrieval
		candidates, err := retrieveBySignal(ctx, d, signal, candidatePoolSize)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "retrieval failed: "+err.Error())
			return
		}

		// Stage 3 — Quality Gate
		gateCtx, gateCancel := context.WithTimeout(ctx, agentTimeout)
		gateResult, err := agents.NewGate(provider).Evaluate(gateCtx, signal, candidates)
		gateCancel()
		if err == nil {
			switch gateResult.Decision {
			case agents.GateAsk:
				raw := gateResult.Question
				if raw == "" {
					raw = fallbackClarifyQ
				}
				question := humanizeText(ctx, provider, raw, req.UserPersona, nigerian)
				writeJSON(w, http.StatusOK, models.RecommendResponse{RequiresInput: true, Question: question})
				return
			case agents.GateRefine:
				if len(gateResult.RefinedQueries) > 0 {
					signal.SearchQueries = gateResult.RefinedQueries
					if refined, refineErr := retrieveBySignal(ctx, d, signal, candidatePoolSize); refineErr != nil || len(refined) == 0 {
						log.Printf("gate REFINE retrieval failed (keeping original): %v", refineErr)
					} else {
						candidates = refined
					}
				}
			}
		}

		// Stage 4 — Psychographic Reranking (cap pool at 20 to bound LLM output size)
		rerankerPool := candidates
		if len(rerankerPool) > 20 {
			rerankerPool = rerankerPool[:20]
		}
		rankCtx, rankCancel := context.WithTimeout(ctx, agentTimeout)
		ranked, err := agents.NewReranker(provider).Rank(rankCtx, signal, rerankerPool, req.CrossDomain)
		rankCancel()
		if err != nil {
			writeError(w, http.StatusInternalServerError, "reranking failed: "+err.Error())
			return
		}

		reasoning := humanizeText(ctx, provider, ranked.OverallReasoning, req.UserPersona, nigerian)

		writeJSON(w, http.StatusOK, models.RecommendResponse{
			Recommendations: buildOrderedItems(ranked, candidates, limit),
			Reasoning:       reasoning,
		})
	}
}

// retrieveBySignal embeds each search query from the signal and unions the results.
// Falls back to full-text search on the intent phrase if embedding fails.
// Results are deduplicated by search_text to remove corpus duplicates.
func retrieveBySignal(ctx context.Context, d *Deps, signal *models.UserSignal, poolSize int) ([]rag.Result, error) {
	vecs := make([][]float32, 0, len(signal.SearchQueries))
	for _, q := range signal.SearchQueries {
		vec, err := d.Embed.Embed(ctx, q)
		if err == nil && len(vec) > 0 {
			vecs = append(vecs, vec)
		}
	}

	var results []rag.Result
	if len(vecs) > 0 {
		r, err := d.Vector.SearchByVectors(ctx, vecs, poolSize)
		if err == nil && len(r) > 0 {
			results = r
		}
	}

	if len(results) == 0 {
		// Fallback: full-text on the distilled intent phrase
		r, err := d.Vector.SearchByText(ctx, signal.Intent, poolSize)
		if err != nil {
			return nil, err
		}
		results = r
	}

	return deduplicateByEntity(deduplicateByText(results)), nil
}

// sampleCorpus embeds the last user turn and retrieves 5 representative items from the DB.
// These are fed to the Extractor so it can calibrate search_queries to what actually exists.
// Failures are non-fatal — returns nil and the Extractor proceeds without corpus grounding.
func sampleCorpus(ctx context.Context, d *Deps, history []models.ConversationTurn) []agents.CorpusExample {
	query := ""
	for i := len(history) - 1; i >= 0; i-- {
		if history[i].Role == "user" {
			query = history[i].Content
			break
		}
	}
	if query == "" {
		return nil
	}
	vec, err := d.Embed.Embed(ctx, query)
	if err != nil || len(vec) == 0 {
		return nil
	}
	samples, err := d.Vector.Search(ctx, vec, 5, nil)
	if err != nil || len(samples) == 0 {
		return nil
	}
	examples := make([]agents.CorpusExample, len(samples))
	for i, s := range samples {
		examples[i] = agents.CorpusExample{Domain: s.Domain, SearchText: s.SearchText}
	}
	return examples
}

// deduplicateByText removes results with identical search_text, keeping the best-scored copy.
// Results must already be sorted ascending by score (best first).
func deduplicateByText(results []rag.Result) []rag.Result {
	seen := make(map[string]bool, len(results))
	deduped := make([]rag.Result, 0, len(results))
	for _, r := range results {
		if !seen[r.SearchText] {
			seen[r.SearchText] = true
			deduped = append(deduped, r)
		}
	}
	return deduped
}

// entityKey extracts a proper-noun fingerprint from text.
// Returns the first sequence of ≥2 consecutive capitalised words (& / "and" treated as joiners).
// Returns "" when no sequence is found; items with no key are never deduplicated by entity.
func entityKey(text string) string {
	words := strings.Fields(text)
	var run []string
	for _, w := range words {
		clean := strings.TrimFunc(w, func(r rune) bool {
			return r == '.' || r == ',' || r == '!' || r == '?' || r == '"' || r == '\'' || r == '(' || r == ')'
		})
		lc := strings.ToLower(clean)
		if lc == "&" || lc == "and" {
			if len(run) > 0 {
				continue // allow mid-entity connector; don't append to run
			}
			continue
		}
		if len(clean) > 0 && clean[0] >= 'A' && clean[0] <= 'Z' {
			run = append(run, lc)
		} else {
			if len(run) >= 2 {
				break
			}
			run = run[:0] // not a sequence yet, reset
		}
	}
	if len(run) < 2 {
		return ""
	}
	return strings.Join(run, " ")
}

// deduplicateByEntity removes candidates that appear to be reviews of the same named entity.
// Fingerprinting uses the first ≥2 consecutive capitalised words found in search_text.
// Domain is included in the composite key so same-named entities across domains are kept.
// Items with no detectable entity are never suppressed.
// Results must already be sorted ascending by score so the best review survives.
func deduplicateByEntity(results []rag.Result) []rag.Result {
	seen := make(map[string]bool, len(results))
	deduped := make([]rag.Result, 0, len(results))
	for _, r := range results {
		key := entityKey(r.SearchText)
		if key == "" {
			deduped = append(deduped, r)
			continue
		}
		composite := r.Domain + ":" + key
		if !seen[composite] {
			seen[composite] = true
			deduped = append(deduped, r)
		}
	}
	return deduped
}

// buildOrderedItems maps the reranker's ranked IDs back to full candidate data.
// Items absent from the ranked list are appended at the end to always return `limit` items.
func buildOrderedItems(ranked *agents.RerankResult, candidates []rag.Result, limit int) []models.RecommendedItem {
	byID := make(map[string]rag.Result, len(candidates))
	for _, c := range candidates {
		byID[c.ItemID] = c
	}

	items := make([]models.RecommendedItem, 0, limit)
	seen := make(map[string]bool, limit)

	for _, id := range ranked.RankedIDs {
		if len(items) >= limit {
			break
		}
		c, ok := byID[id]
		if !ok || seen[id] {
			continue
		}
		items = append(items, models.RecommendedItem{
			ItemID:     c.ItemID,
			Domain:     c.Domain,
			SearchText: c.SearchText,
			Score:      c.Score,
			Reasoning:  ranked.ItemReasoning[id],
		})
		seen[id] = true
	}

	// Fill any remaining slots with un-ranked candidates in retrieval order
	for _, c := range candidates {
		if len(items) >= limit {
			break
		}
		if !seen[c.ItemID] {
			items = append(items, models.RecommendedItem{
				ItemID:     c.ItemID,
				Domain:     c.Domain,
				SearchText: c.SearchText,
				Score:      c.Score,
			})
			seen[c.ItemID] = true
		}
	}

	return items
}
