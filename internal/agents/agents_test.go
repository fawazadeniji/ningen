package agents

import (
	"context"
	"testing"

	"ningen/internal/llm"
	"ningen/internal/models"
	"ningen/internal/rag"
)

// mockProvider returns a fixed response for every Complete call.
type mockProvider struct {
	response string
	err      error
}

func (m *mockProvider) Name() string { return "mock" }
func (m *mockProvider) Complete(_ context.Context, _ []llm.Message) (string, error) {
	return m.response, m.err
}
func (m *mockProvider) Humanize(_ context.Context, text, _ string) (string, error) {
	return text, nil
}

// ── cleanJSON ─────────────────────────────────────────────────────────────────

func TestCleanJSON_Plain(t *testing.T) {
	input := `{"key":"value"}`
	if got := cleanJSON(input); got != input {
		t.Errorf("got %q want %q", got, input)
	}
}

func TestCleanJSON_BacktickFence(t *testing.T) {
	input := "```json\n{\"key\":\"value\"}\n```"
	want := `{"key":"value"}`
	if got := cleanJSON(input); got != want {
		t.Errorf("got %q want %q", got, want)
	}
}

func TestCleanJSON_FenceNoLang(t *testing.T) {
	input := "```\n{\"key\":\"value\"}\n```"
	want := `{"key":"value"}`
	if got := cleanJSON(input); got != want {
		t.Errorf("got %q want %q", got, want)
	}
}

// ── Extractor ─────────────────────────────────────────────────────────────────

func TestExtractor_ValidSignal(t *testing.T) {
	p := &mockProvider{response: `{
		"intent": "science fiction novel",
		"domain": "books",
		"search_queries": ["hard sci-fi space opera", "dystopian future thriller"],
		"mood": "adventurous",
		"constraints": [],
		"clarify_needed": false,
		"clarify_reason": ""
	}`}
	signal, err := NewExtractor(p).Extract(context.Background(), "sci-fi fan", []models.ConversationTurn{
		{Role: "user", Content: "I want a sci-fi book"},
	}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if signal.Intent != "science fiction novel" {
		t.Errorf("intent: got %q want %q", signal.Intent, "science fiction novel")
	}
	if len(signal.SearchQueries) != 2 {
		t.Errorf("search_queries len: got %d want 2", len(signal.SearchQueries))
	}
	if signal.ClarifyNeeded {
		t.Error("clarify_needed should be false")
	}
}

func TestExtractor_ClarifyNeeded(t *testing.T) {
	p := &mockProvider{response: `{
		"intent": "",
		"domain": "mixed",
		"search_queries": [],
		"mood": "",
		"constraints": [],
		"clarify_needed": true,
		"clarify_reason": "What type of item are you looking for?"
	}`}
	signal, err := NewExtractor(p).Extract(context.Background(), "someone", []models.ConversationTurn{
		{Role: "user", Content: "something good"},
	}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !signal.ClarifyNeeded {
		t.Error("clarify_needed should be true")
	}
	if signal.ClarifyReason == "" {
		t.Error("clarify_reason should not be empty")
	}
}

func TestExtractor_EmptySearchQueriesFallsBackToIntent(t *testing.T) {
	p := &mockProvider{response: `{
		"intent": "action movie recommendation",
		"domain": "mixed",
		"search_queries": [],
		"mood": "excited",
		"constraints": [],
		"clarify_needed": false,
		"clarify_reason": ""
	}`}
	signal, err := NewExtractor(p).Extract(context.Background(), "p", []models.ConversationTurn{
		{Role: "user", Content: "recommend action movies"},
	}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(signal.SearchQueries) == 0 {
		t.Error("should fall back to intent when search_queries empty")
	}
	if signal.SearchQueries[0] != "action movie recommendation" {
		t.Errorf("fallback query: got %q", signal.SearchQueries[0])
	}
}

func TestExtractor_BadJSON_ReturnsError(t *testing.T) {
	p := &mockProvider{response: "sorry I cannot help with that"}
	_, err := NewExtractor(p).Extract(context.Background(), "p", []models.ConversationTurn{
		{Role: "user", Content: "help"},
	}, nil)
	if err == nil {
		t.Error("expected error for unparseable LLM response")
	}
}

// ── Gate ──────────────────────────────────────────────────────────────────────

var dummyCandidates = []rag.Result{
	{ItemID: "a", Domain: "books", SearchText: "great novel", Score: 0.2},
	{ItemID: "b", Domain: "books", SearchText: "sci-fi adventure", Score: 0.3},
}

func TestGate_Accept(t *testing.T) {
	p := &mockProvider{response: `{"decision":"ACCEPT","refined_queries":[],"question":""}`}
	result, err := NewGate(p).Evaluate(context.Background(), &models.UserSignal{Intent: "books"}, dummyCandidates)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Decision != GateAccept {
		t.Errorf("decision: got %q want ACCEPT", result.Decision)
	}
}

func TestGate_Refine(t *testing.T) {
	p := &mockProvider{response: `{"decision":"REFINE","refined_queries":["better query 1","better query 2"],"question":""}`}
	result, err := NewGate(p).Evaluate(context.Background(), &models.UserSignal{Intent: "books"}, dummyCandidates)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Decision != GateRefine {
		t.Errorf("decision: got %q want REFINE", result.Decision)
	}
	if len(result.RefinedQueries) != 2 {
		t.Errorf("refined_queries len: got %d want 2", len(result.RefinedQueries))
	}
}

func TestGate_Ask(t *testing.T) {
	p := &mockProvider{response: `{"decision":"ASK","refined_queries":[],"question":"What genre?"}`}
	result, err := NewGate(p).Evaluate(context.Background(), &models.UserSignal{Intent: "books"}, dummyCandidates)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Decision != GateAsk {
		t.Errorf("decision: got %q want ASK", result.Decision)
	}
	if result.Question != "What genre?" {
		t.Errorf("question: got %q", result.Question)
	}
}

func TestGate_BadJSON_DefaultsToAccept(t *testing.T) {
	p := &mockProvider{response: "not json at all"}
	result, err := NewGate(p).Evaluate(context.Background(), &models.UserSignal{}, dummyCandidates)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Decision != GateAccept {
		t.Errorf("bad JSON should default to ACCEPT, got %q", result.Decision)
	}
}

func TestGate_UnknownDecision_DefaultsToAccept(t *testing.T) {
	p := &mockProvider{response: `{"decision":"MAYBE","refined_queries":[],"question":""}`}
	result, _ := NewGate(p).Evaluate(context.Background(), &models.UserSignal{}, dummyCandidates)
	if result.Decision != GateAccept {
		t.Errorf("unknown decision should default to ACCEPT, got %q", result.Decision)
	}
}

// ── Reranker ──────────────────────────────────────────────────────────────────

func TestReranker_ValidRanking(t *testing.T) {
	p := &mockProvider{response: `{
		"ranked_ids": ["b", "a"],
		"item_reasoning": {"b": "better fit", "a": "also good"},
		"overall_reasoning": "Here is why these are ranked this way."
	}`}
	result, err := NewReranker(p).Rank(context.Background(), &models.UserSignal{Intent: "books"}, dummyCandidates, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.RankedIDs) != 2 {
		t.Errorf("ranked_ids len: got %d want 2", len(result.RankedIDs))
	}
	if result.RankedIDs[0] != "b" {
		t.Errorf("top rank: got %q want b", result.RankedIDs[0])
	}
	if result.OverallReasoning == "" {
		t.Error("overall_reasoning should not be empty")
	}
}

func TestReranker_BadJSON_FallsBackToRetrievalOrder(t *testing.T) {
	p := &mockProvider{response: "cannot rank these"}
	result, err := NewReranker(p).Rank(context.Background(), &models.UserSignal{}, dummyCandidates, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.RankedIDs) != len(dummyCandidates) {
		t.Errorf("fallback should return all candidates, got %d", len(result.RankedIDs))
	}
	if result.RankedIDs[0] != "a" {
		t.Errorf("fallback should preserve retrieval order, got %q", result.RankedIDs[0])
	}
}
