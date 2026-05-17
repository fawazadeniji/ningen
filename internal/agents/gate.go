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

const gateSystem = `You are a retrieval quality gate for a recommendation engine.
Your sole output is a single JSON object — no markdown, no explanation, no text outside the JSON.

Evaluate whether the retrieved candidates match the user's intent signal well enough to recommend.
Produce exactly this JSON:
{
  "decision": "<ACCEPT | REFINE | ASK>",
  "refined_queries": ["<new query 1>", "<new query 2>"],
  "question": "<clarifying question for the user, only when decision=ASK>"
}

Decision rules:
- ACCEPT: candidates are on-domain and diverse enough for a good recommendation. This is the default.
- REFINE: candidates are clearly off-target (wrong domain, irrelevant topics); provide 2 improved
  search phrases in refined_queries that will retrieve better results.
- ASK: the intent is fundamentally ambiguous and no retrieval strategy will help without more context;
  write a single clarifying question in the question field.

refined_queries and question may be empty strings when not applicable.
When in doubt, choose ACCEPT — a slightly imperfect retrieval is better than interrupting the user.`

// GateDecision is the quality gate's verdict on retrieval quality.
type GateDecision string

const (
	GateAccept GateDecision = "ACCEPT"
	GateRefine GateDecision = "REFINE"
	GateAsk    GateDecision = "ASK"
)

// GateResult holds the quality gate's decision and any follow-up data.
type GateResult struct {
	Decision       GateDecision
	RefinedQueries []string
	Question       string
}

// Gate is Stage 3 of the SIGNAL pipeline.
// It validates retrieval quality and routes to ACCEPT, REFINE, or ASK.
type Gate struct {
	provider llm.LLMProvider
}

func NewGate(p llm.LLMProvider) *Gate {
	return &Gate{provider: p}
}

// Evaluate checks whether the candidate pool is fit for ranking.
func (g *Gate) Evaluate(ctx context.Context, signal *models.UserSignal, candidates []rag.Result) (*GateResult, error) {
	messages := []llm.Message{
		{Role: "system", Content: gateSystem},
		{Role: "user", Content: buildGatePrompt(signal, candidates)},
	}

	raw, err := g.provider.Complete(ctx, messages)
	if err != nil {
		return nil, fmt.Errorf("gate LLM: %w", err)
	}

	var resp struct {
		Decision       string   `json:"decision"`
		RefinedQueries []string `json:"refined_queries"`
		Question       string   `json:"question"`
	}
	if err := json.Unmarshal([]byte(cleanJSON(raw)), &resp); err != nil {
		// Parse failure is non-fatal: default to ACCEPT so we never block a response.
		return &GateResult{Decision: GateAccept}, nil
	}

	decision := GateDecision(resp.Decision)
	if decision != GateRefine && decision != GateAsk {
		decision = GateAccept
	}

	return &GateResult{
		Decision:       decision,
		RefinedQueries: resp.RefinedQueries,
		Question:       resp.Question,
	}, nil
}

func buildGatePrompt(signal *models.UserSignal, candidates []rag.Result) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "Intent: %s\nDomain: %s\nMood: %s\n", signal.Intent, signal.Domain, signal.Mood)
	if len(signal.Constraints) > 0 {
		fmt.Fprintf(&sb, "Constraints: %s\n", strings.Join(signal.Constraints, ", "))
	}
	sb.WriteString("\nTop retrieved candidates (first 10 shown):\n")
	for i, c := range candidates {
		if i >= 10 {
			break
		}
		text := c.SearchText
		if len(text) > 200 {
			text = text[:200]
		}
		fmt.Fprintf(&sb, "%d. [%s] %s\n", i+1, c.Domain, text)
	}
	return sb.String()
}
