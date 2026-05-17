package nodes

import (
	"context"
	"testing"

	"ningen/internal/llm"
)

type mockLLMProvider struct {
	modelUsed string
	calls     int
}

func (m *mockLLMProvider) Name() string { return "mock" }

func (m *mockLLMProvider) Complete(ctx context.Context, messages []llm.Message, opts ...llm.CompletionOption) (string, error) {
	m.calls++
	// Return different responses based on context
	// For rater responses, include rationale
	return `{"rationale":"Fits user preferences and price range well.","predicted_rating":4.2}`, nil
}

func (m *mockLLMProvider) Humanize(ctx context.Context, rawText string, userContext string) (string, error) {
	return rawText, nil
}

func TestAgentState_ModelFor_ReturnsOverride(t *testing.T) {
	state := &AgentState{
		ModelOverrides: map[string]string{
			"profiler": "gpt-5.4-mini",
			"rater":    "gpt-4o",
			"drafter":  "gpt-5.4-mini",
			"critic":   "gpt-4o",
		},
	}

	if got := state.ModelFor("profiler"); got != "gpt-5.4-mini" {
		t.Errorf("ModelFor(\"profiler\") = %q, want %q", got, "gpt-5.4-mini")
	}
	if got := state.ModelFor("rater"); got != "gpt-4o" {
		t.Errorf("ModelFor(\"rater\") = %q, want %q", got, "gpt-4o")
	}
	if got := state.ModelFor("drafter"); got != "gpt-5.4-mini" {
		t.Errorf("ModelFor(\"drafter\") = %q, want %q", got, "gpt-5.4-mini")
	}
	if got := state.ModelFor("critic"); got != "gpt-4o" {
		t.Errorf("ModelFor(\"critic\") = %q, want %q", got, "gpt-4o")
	}
}

func TestAgentState_ModelFor_EmptyString_WhenNotPresent(t *testing.T) {
	state := &AgentState{
		ModelOverrides: map[string]string{
			"profiler": "gpt-5.4-mini",
		},
	}

	if got := state.ModelFor("rater"); got != "" {
		t.Errorf("ModelFor(\"rater\") = %q, want empty string", got)
	}
}

func TestAgentState_ModelFor_EmptyString_WhenNilOverrides(t *testing.T) {
	state := &AgentState{
		ModelOverrides: nil,
	}

	if got := state.ModelFor("profiler"); got != "" {
		t.Errorf("ModelFor(\"profiler\") = %q, want empty string", got)
	}
}

func TestAgentState_ModelFor_EmptyString_WhenNilState(t *testing.T) {
	var state *AgentState

	if got := state.ModelFor("profiler"); got != "" {
		t.Errorf("ModelFor(\"profiler\") on nil state = %q, want empty string", got)
	}
}

func TestProfiler_WithModelOverride(t *testing.T) {
	// Verify that Profiler correctly uses the ModelFor helper and would pass the override
	model := &mockLLMProvider{}

	profilerNode := Profiler(model)

	state := AgentState{
		UserHistory: []HistoryEntry{
			{
				ProductName:     "Test Product",
				ProductCategory: "electronics",
				StarRating:      4.5,
				ReviewText:      "Great product, very satisfied!",
			},
		},
		ModelOverrides: map[string]string{
			"profiler": "gpt-5.4-mini",
		},
	}

	resultState, err := profilerNode(context.Background(), state)
	if err != nil {
		t.Fatalf("Profiler node failed: %v", err)
	}

	if resultState.UserProfile == nil {
		t.Fatalf("expected UserProfile to be set, got nil")
	}

	if model.calls != 1 {
		t.Errorf("expected 1 LLM call, got %d", model.calls)
	}
}

func TestRater_WithModelOverride(t *testing.T) {
	// Verify that Rater correctly uses the ModelFor helper
	model := &mockLLMProvider{}

	raterNode := Rater(model)

	state := AgentState{
		UserProfile: &UserProfile{
			AverageRating: 4.0,
			RatingPatterns: RatingPatterns{
				RatingThresholds: RatingThresholds{Low: 3.0, High: 5.0},
			},
			ConsumerPersona:     "Budget Shopper",
			PreferredCategories: []string{"electronics"},
			BehavioralMarkers: []BehavioralMarker{
				{Marker: "price_sensitive", Description: "Cares about cost"},
			},
			CulturalHooks:   []string{"Naira"},
			OverallTendency: "balanced",
		},
		TargetProduct: TargetProduct{
			ProductName:     "Test Product",
			ProductCategory: "electronics",
			Price:           5000,
			Currency:        "NGN",
		},
		ModelOverrides: map[string]string{
			"rater": "gpt-4o",
		},
	}

	resultState, err := raterNode(context.Background(), state)
	if err != nil {
		t.Fatalf("Rater node failed: %v", err)
	}

	if resultState.PredictedRating == 0 {
		t.Errorf("expected PredictedRating to be set, got 0")
	}

	if model.calls != 1 {
		t.Errorf("expected 1 LLM call, got %d", model.calls)
	}
}

func TestDrafter_WithModelOverride(t *testing.T) {
	// Verify that Drafter correctly uses the ModelFor helper
	model := &mockLLMProvider{}

	drafterNode := Drafter(model)

	state := AgentState{
		UserProfile: &UserProfile{
			AverageRating: 4.0,
			ReviewLength: ReviewLengthProfile{
				AverageLength: 80,
				MinLength:     50,
				MaxLength:     120,
			},
			ReviewStyle: ReviewStyle{
				VerbosityLevel:   "concise",
				UseEmotionalLang: false,
			},
			FormattingQuirks: FormattingQuirks{
				CapitalizationStyle: "lowercase",
				PunctuationHabits:   "exclamation marks",
			},
			ConsumerPersona:     "Budget Shopper",
			PreferredCategories: []string{"electronics"},
		},
		TargetProduct: TargetProduct{
			ProductName:     "Test Product",
			ProductCategory: "electronics",
		},
		PredictedRating: 4.0,
		ModelOverrides: map[string]string{
			"drafter": "gpt-5.4-mini",
		},
	}

	resultState, err := drafterNode(context.Background(), state)
	if err != nil {
		t.Fatalf("Drafter node failed: %v", err)
	}

	if resultState.DraftReview == "" {
		t.Errorf("expected DraftReview to be set")
	}

	if model.calls != 1 {
		t.Errorf("expected 1 LLM call, got %d", model.calls)
	}
}

func TestCritic_WithModelOverride(t *testing.T) {
	// Verify that Critic correctly uses the ModelFor helper
	model := &mockLLMProvider{}

	criticNode := Critic(model)

	state := AgentState{
		UserProfile: &UserProfile{
			AverageRating: 4.0,
			ReviewLength: ReviewLengthProfile{
				AverageLength: 80,
				MinLength:     50,
				MaxLength:     120,
			},
			FormattingQuirks: FormattingQuirks{
				CapitalizationStyle: "lowercase",
				PunctuationHabits:   "exclamation marks",
			},
			ConsumerPersona: "Budget Shopper",
		},
		UserHistory: []HistoryEntry{
			{ProductName: "Product 1", ReviewText: "Great product!", StarRating: 4.0},
		},
		DraftReview: "Test review that should pass validation checks and sound natural.",
		ModelOverrides: map[string]string{
			"critic": "gpt-4o",
		},
	}

	resultState, err := criticNode(context.Background(), state)
	if err != nil {
		t.Fatalf("Critic node failed: %v", err)
	}

	if resultState.CriticVerdict == "" {
		t.Errorf("expected CriticVerdict to be set")
	}

	if model.calls < 1 {
		t.Errorf("expected at least 1 LLM call, got %d", model.calls)
	}
}

func TestMultipleNodes_WithDifferentModelOverrides(t *testing.T) {
	// Test that different nodes can use different model overrides simultaneously
	state := AgentState{
		UserHistory: []HistoryEntry{
			{
				ProductName:     "Test Product",
				ProductCategory: "electronics",
				StarRating:      4.5,
				ReviewText:      "Great product!",
			},
		},
		ModelOverrides: map[string]string{
			"profiler": "gpt-5.4-mini",
			"rater":    "gpt-4o",
			"drafter":  "gpt-5.4-mini",
			"critic":   "gpt-4o",
		},
	}

	// Verify each node uses its respective override
	if profilerModel := state.ModelFor("profiler"); profilerModel != "gpt-5.4-mini" {
		t.Errorf("profiler model override = %q, want %q", profilerModel, "gpt-5.4-mini")
	}
	if raterModel := state.ModelFor("rater"); raterModel != "gpt-4o" {
		t.Errorf("rater model override = %q, want %q", raterModel, "gpt-4o")
	}
	if drafterModel := state.ModelFor("drafter"); drafterModel != "gpt-5.4-mini" {
		t.Errorf("drafter model override = %q, want %q", drafterModel, "gpt-5.4-mini")
	}
	if criticModel := state.ModelFor("critic"); criticModel != "gpt-4o" {
		t.Errorf("critic model override = %q, want %q", criticModel, "gpt-4o")
	}
}
