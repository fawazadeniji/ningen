package nodes

import (
	"strings"
	"testing"
)

func TestBuildRaterPrompt_AsksForUserSafeRationale(t *testing.T) {
	prompt := buildRaterPrompt(&UserProfile{}, &TargetProduct{})
	if len(prompt) < 2 {
		t.Fatalf("expected system and user messages, got %d", len(prompt))
	}

	userPrompt := prompt[1].Content
	if strings.Contains(strings.ToLower(userPrompt), "chain of thought") {
		t.Fatal("prompt should not request chain-of-thought")
	}
	if !strings.Contains(userPrompt, "user-safe rationale") {
		t.Fatal("prompt should request a user-safe rationale")
	}
	if !strings.Contains(userPrompt, "rationale") {
		t.Fatal("prompt should include the rationale field")
	}
}

func TestValidateRaterResponse_RequiresRationale(t *testing.T) {
	err := validateRaterResponse(&RaterResponse{PredictedRating: 4.2})
	if err == nil {
		t.Fatal("expected validation error when rationale is empty")
	}
}

func TestParseRaterResponse_UsesRationale(t *testing.T) {
	resp, err := parseRaterResponse(`{"rationale":"- Fits the user's category\n- Price seems fair","predicted_rating":4.1}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Rationale == "" {
		t.Fatal("expected rationale to be populated")
	}
	if resp.PredictedRating != 4.1 {
		t.Fatalf("unexpected rating: %.1f", resp.PredictedRating)
	}
}
