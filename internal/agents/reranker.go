package agents

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"ningen/internal/llm"
	"ningen/internal/models"
	"ningen/internal/rag"
)

const rerankerSystem = `You are a psychographic recommendation reranker.
Your sole output is a single JSON object — no markdown, no explanation, no text outside the JSON.

Given a user's intent signal and a pool of candidate items, rank every candidate by how precisely
it matches the user's psychographic profile: stated intent, emotional mood, domain, and constraints.

Produce exactly this JSON:
{
  "ranked_ids": ["<item_id_1>", "<item_id_2>", ...],
  "item_reasoning": {
    "<item_id_1>": "<one sentence on why this is the best fit>",
    "<item_id_2>": "<one sentence on why this fits>"
  },
  "overall_reasoning": "<2-3 sentences narrating the recommendation strategy and why these items were chosen>"
}

Rules:
- ranked_ids must contain the item_ids of ALL provided candidates, ordered best to worst.
- item_reasoning must cover at least the top 10 items by rank.
- overall_reasoning is the narrative shown to the user — write it warmly and directly to them.
- Hard constraints are NON-NEGOTIABLE: any item that violates a stated constraint MUST be placed at the very bottom of ranked_ids, below every constraint-satisfying item. The top-N slots must be exclusively constraint-satisfying items.
- Cross-domain flag: if cross_domain=true, freely mix categories. If false, prefer same-domain items.`

// RerankResult holds the psychographic reranker's output.
type RerankResult struct {
	RankedIDs        []string
	ItemReasoning    map[string]string
	OverallReasoning string
}

// Reranker is Stage 4 of the SIGNAL pipeline.
// It psychographically ranks the candidate pool against the user's extracted signal.
type Reranker struct {
	provider llm.LLMProvider
}

func NewReranker(p llm.LLMProvider) *Reranker {
	return &Reranker{provider: p}
}

// Rank orders candidates by psychographic fit to the signal.
func (r *Reranker) Rank(ctx context.Context, signal *models.UserSignal, candidates []rag.Result, crossDomain bool) (*RerankResult, error) {
	messages := []llm.Message{
		{Role: "system", Content: rerankerSystem},
		{Role: "user", Content: buildRerankerPrompt(signal, candidates, crossDomain)},
	}

	raw, err := r.provider.Complete(ctx, messages)
	if err != nil {
		return nil, fmt.Errorf("reranker LLM: %w", err)
	}

	var resp struct {
		RankedIDs        []string          `json:"ranked_ids"`
		ItemReasoning    map[string]string `json:"item_reasoning"`
		OverallReasoning string            `json:"overall_reasoning"`
	}
	if err := json.Unmarshal([]byte(cleanJSON(raw)), &resp); err != nil {
		// Parse failure: return candidates in retrieval order with a generic narrative.
		ids := make([]string, len(candidates))
		for i, c := range candidates {
			ids[i] = c.ItemID
		}
		return &RerankResult{
			RankedIDs:        ids,
			ItemReasoning:    map[string]string{},
			OverallReasoning: "Here are the most relevant items found for your request.",
		}, nil
	}

	if resp.ItemReasoning == nil {
		resp.ItemReasoning = map[string]string{}
	}

	return &RerankResult{
		RankedIDs:        resp.RankedIDs,
		ItemReasoning:    resp.ItemReasoning,
		OverallReasoning: resp.OverallReasoning,
	}, nil
}

func buildRerankerPrompt(signal *models.UserSignal, candidates []rag.Result, crossDomain bool) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "Intent: %s\nDomain: %s\nMood: %s\ncross_domain: %v\n",
		signal.Intent, signal.Domain, signal.Mood, crossDomain)
	if len(signal.Constraints) > 0 {
		fmt.Fprintf(&sb, "Constraints: %s\n", strings.Join(signal.Constraints, ", "))
	}
	sb.WriteString("\nCandidate items:\n")
	for i, c := range candidates {
		fmt.Fprintf(&sb, "%d. item_id=%s domain=%s\n%s\n\n", i+1, c.ItemID, c.Domain, c.SearchText)
	}
	return sb.String()
}
