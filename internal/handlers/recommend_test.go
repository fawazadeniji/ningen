package handlers

import (
	"testing"

	"ningen/internal/agents"
	"ningen/internal/models"
	"ningen/internal/rag"
)

// ── deduplicateByText ─────────────────────────────────────────────────────────

func TestDeduplicateByText_RemovesDuplicates(t *testing.T) {
	results := []rag.Result{
		{ItemID: "a", SearchText: "hello world", Score: 0.1},
		{ItemID: "b", SearchText: "hello world", Score: 0.2}, // same text, worse score
		{ItemID: "c", SearchText: "different text", Score: 0.3},
	}
	got := deduplicateByText(results)
	if len(got) != 2 {
		t.Fatalf("expected 2 results after dedup, got %d", len(got))
	}
	if got[0].ItemID != "a" {
		t.Errorf("should keep first (best score) occurrence, got %q", got[0].ItemID)
	}
}

func TestDeduplicateByText_NoDuplicates(t *testing.T) {
	results := []rag.Result{
		{ItemID: "a", SearchText: "text one", Score: 0.1},
		{ItemID: "b", SearchText: "text two", Score: 0.2},
	}
	got := deduplicateByText(results)
	if len(got) != 2 {
		t.Errorf("expected 2 results, got %d", len(got))
	}
}

func TestDeduplicateByText_Empty(t *testing.T) {
	got := deduplicateByText(nil)
	if len(got) != 0 {
		t.Errorf("expected empty, got %d", len(got))
	}
}

// ── buildOrderedItems ─────────────────────────────────────────────────────────

func makeResult(id, text string, score float64) rag.Result {
	return rag.Result{ItemID: id, Domain: "books", SearchText: text, Score: score}
}

func TestBuildOrderedItems_RankedOrder(t *testing.T) {
	candidates := []rag.Result{
		makeResult("a", "text a", 0.1),
		makeResult("b", "text b", 0.2),
		makeResult("c", "text c", 0.3),
	}
	ranked := &agents.RerankResult{
		RankedIDs:     []string{"c", "a", "b"},
		ItemReasoning: map[string]string{"c": "best", "a": "ok", "b": "fine"},
	}
	got := buildOrderedItems(ranked, candidates, 3)
	if len(got) != 3 {
		t.Fatalf("expected 3 items, got %d", len(got))
	}
	if got[0].ItemID != "c" {
		t.Errorf("first item should be c (top ranked), got %q", got[0].ItemID)
	}
	if got[0].Reasoning != "best" {
		t.Errorf("reasoning should carry through, got %q", got[0].Reasoning)
	}
}

func TestBuildOrderedItems_LimitRespected(t *testing.T) {
	candidates := []rag.Result{
		makeResult("a", "text a", 0.1),
		makeResult("b", "text b", 0.2),
		makeResult("c", "text c", 0.3),
	}
	ranked := &agents.RerankResult{
		RankedIDs:     []string{"a", "b", "c"},
		ItemReasoning: map[string]string{},
	}
	got := buildOrderedItems(ranked, candidates, 2)
	if len(got) != 2 {
		t.Errorf("limit=2 should return 2 items, got %d", len(got))
	}
}

func TestBuildOrderedItems_MissingRankedIDs_FilledFromCandidates(t *testing.T) {
	candidates := []rag.Result{
		makeResult("a", "text a", 0.1),
		makeResult("b", "text b", 0.2),
		makeResult("c", "text c", 0.3),
	}
	// Reranker only returned 1 ID out of 3
	ranked := &agents.RerankResult{
		RankedIDs:     []string{"b"},
		ItemReasoning: map[string]string{"b": "ranked"},
	}
	got := buildOrderedItems(ranked, candidates, 3)
	if len(got) != 3 {
		t.Fatalf("should fill missing slots from candidates, got %d", len(got))
	}
	if got[0].ItemID != "b" {
		t.Errorf("ranked item should come first, got %q", got[0].ItemID)
	}
	// remaining two filled from retrieval order (a then c), b already seen
	ids := map[string]bool{got[1].ItemID: true, got[2].ItemID: true}
	if !ids["a"] || !ids["c"] {
		t.Errorf("remaining slots should be a and c, got %v", ids)
	}
}

func TestBuildOrderedItems_IDNotInCandidates_Skipped(t *testing.T) {
	candidates := []rag.Result{
		makeResult("a", "text a", 0.1),
	}
	ranked := &agents.RerankResult{
		RankedIDs:     []string{"ghost", "a"}, // "ghost" not in candidates
		ItemReasoning: map[string]string{},
	}
	got := buildOrderedItems(ranked, candidates, 3)
	if len(got) != 1 {
		t.Errorf("ghost ID should be skipped, got %d items", len(got))
	}
	if got[0].ItemID != "a" {
		t.Errorf("got %q want a", got[0].ItemID)
	}
}

func TestBuildOrderedItems_NoDuplicatesFromRankedIDs(t *testing.T) {
	candidates := []rag.Result{
		makeResult("a", "text a", 0.1),
	}
	ranked := &agents.RerankResult{
		RankedIDs:     []string{"a", "a", "a"}, // LLM repeated same ID
		ItemReasoning: map[string]string{},
	}
	got := buildOrderedItems(ranked, candidates, 5)
	if len(got) != 1 {
		t.Errorf("duplicate ranked IDs should produce 1 item, got %d", len(got))
	}
}

// ── limit validation ──────────────────────────────────────────────────────────

func TestRecommendedItemReasoningField(t *testing.T) {
	item := models.RecommendedItem{
		ItemID:    "x",
		Domain:    "books",
		Reasoning: "fits perfectly",
	}
	if item.Reasoning != "fits perfectly" {
		t.Errorf("reasoning field not set: %q", item.Reasoning)
	}
}
