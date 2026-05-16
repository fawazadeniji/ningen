package handlers

import (
	"context"
	"fmt"
	"net/http"
	"ningen/internal/llm"
	"ningen/internal/models"
	"ningen/internal/rag"
)

const (
	defaultRecommendLimit = 10
	coldStartThreshold    = 3
	candidatePoolSize     = 50
)

// RecommendHandler serves POST /recommend.
// Three-phase agentic workflow:
//  1. Cold-start gate: no history → return a humanized clarifying question.
//  2. Retrieve 50 candidates via vector search (falls back to full-text).
//  3. Synthesise a ranked narrative via LLM, then humanize through Nigerian cultural lens.
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

		// Cold-start gate: no conversation history means we have no intent signal.
		// Return a clarifying question rather than a blind recommendation.
		if len(req.History) == 0 {
			raw := "What kind of item are you looking for — a book, a product, or a place to eat? Tell me a little about what you need or what mood you're in."
			question, err := provider.Humanize(ctx, raw, req.UserPersona)
			if err != nil {
				question = raw
			}
			writeJSON(w, http.StatusOK, models.RecommendResponse{
				RequiresInput: true,
				Question:      question,
			})
			return
		}

		limit := req.Limit
		if limit <= 0 {
			limit = defaultRecommendLimit
		}

		// Retrieve a larger candidate pool for better ranking quality.
		candidates, err := retrieve(ctx, d, req, candidatePoolSize)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "retrieval failed: "+err.Error())
			return
		}

		raw, err := synthesise(ctx, provider, req, candidates)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "synthesis failed: "+err.Error())
			return
		}

		// Humanizer always runs; degrades gracefully to raw output on failure.
		humanised, err := provider.Humanize(ctx, raw, req.UserPersona)
		if err != nil {
			humanised = raw
		}

		// Trim candidates to requested limit for the response payload.
		if limit < len(candidates) {
			candidates = candidates[:limit]
		}

		writeJSON(w, http.StatusOK, buildResponse(candidates, humanised))
	}
}

func retrieve(ctx context.Context, d *Deps, req models.RecommendRequest, poolSize int) ([]rag.Result, error) {
	query := lastUserTurn(req)

	// 1. Vector search — primary path.
	vec, err := d.Embed.Embed(ctx, query)
	if err == nil {
		candidates, err := d.Vector.Search(ctx, vec, poolSize, nil)
		if err == nil && len(candidates) >= coldStartThreshold {
			return candidates, nil
		}
	}

	// 2. Full-text fallback on embedder failure or sparse results.
	candidates, err := d.Vector.SearchByText(ctx, query, poolSize)
	if err != nil {
		return nil, err
	}

	// 3. Last resort: broaden to full persona if query match is too narrow.
	if len(candidates) < coldStartThreshold {
		candidates, err = d.Vector.SearchByText(ctx, req.UserPersona, poolSize)
		if err != nil {
			return nil, err
		}
	}

	return candidates, nil
}

func synthesise(ctx context.Context, provider llm.LLMProvider, req models.RecommendRequest, candidates []rag.Result) (string, error) {
	if len(candidates) == 0 {
		return "No relevant items found for your context.", nil
	}

	crossDomainInstruction := "Feel free to recommend items across different categories (books, products, restaurants) if they serve the user's needs."
	if !req.CrossDomain {
		crossDomainInstruction = "Keep recommendations within the same category unless the user's context strongly suggests otherwise."
	}

	messages := make([]llm.Message, 0, len(req.History)+2)
	messages = append(messages, llm.Message{
		Role: "system",
		Content: fmt.Sprintf(`You are a world-class personalised recommendation engine.
Given the user's persona and retrieved candidate items, produce a concise ranked
recommendation list with a brief reasoning sentence per item. Be specific and
tailor your tone to the user's stated context.
%s`, crossDomainInstruction),
	})
	for _, turn := range req.History {
		messages = append(messages, llm.Message{Role: turn.Role, Content: turn.Content})
	}
	messages = append(messages, llm.Message{
		Role: "user",
		Content: fmt.Sprintf(
			"User persona: %s\n\nCandidate items:\n%s\n\nProvide your ranked recommendations.",
			req.UserPersona,
			formatCandidates(candidates),
		),
	})

	return provider.Complete(ctx, messages)
}

func buildResponse(candidates []rag.Result, reasoning string) models.RecommendResponse {
	items := make([]models.RecommendedItem, len(candidates))
	for i, c := range candidates {
		items[i] = models.RecommendedItem{
			ItemID:     c.ItemID,
			Domain:     c.Domain,
			SearchText: c.SearchText,
			Score:      c.Score,
		}
	}
	return models.RecommendResponse{Recommendations: items, Reasoning: reasoning}
}

func lastUserTurn(req models.RecommendRequest) string {
	for i := len(req.History) - 1; i >= 0; i-- {
		if req.History[i].Role == "user" {
			return req.History[i].Content
		}
	}
	return req.UserPersona
}

func formatCandidates(results []rag.Result) string {
	sb := ""
	for i, r := range results {
		sb += fmt.Sprintf("%d. [%s] %s (score: %.4f)\n", i+1, r.Domain, r.SearchText, r.Score)
	}
	return sb
}
