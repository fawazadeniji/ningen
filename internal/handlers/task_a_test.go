package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"math"
	"net/http"
	"net/http/httptest"
	"testing"

	"ningen/internal/llm"
)

type fakeReviewModel struct {
	responses []string
	calls     int
}

func (f *fakeReviewModel) Name() string { return "fake" }

func (f *fakeReviewModel) Complete(_ context.Context, _ []llm.Message, _ ...llm.CompletionOption) (string, error) {
	if f.calls >= len(f.responses) {
		return "", nil
	}
	response := f.responses[f.calls]
	f.calls++
	return response, nil
}

func (f *fakeReviewModel) Humanize(ctx context.Context, rawText string, userContext string) (string, error) {
	return f.Complete(ctx, []llm.Message{{Role: "user", Content: rawText + userContext}})
}

func TestGenerateReviewHandler_EndToEnd(t *testing.T) {
	model := &fakeReviewModel{
		responses: []string{
			`{"user_id":"u-1","overall_tendency":"balanced","average_rating":3.8,"preferred_categories":["electronics"],"review_style":{"detail_level":"moderate","use_emotional_lang":false,"use_tech_language":true,"comparison_frequency":"occasional"},"behavioral_markers":[{"marker":"price_conscious","frequency":"frequent","confidence":0.92,"description":"Watches price carefully"}],"tone_profile":{"cheerfulness":0.4,"sarcasm":0.1,"urgency":0.2,"formality":0.5},"rating_patterns":{"ratings_distribution":{"3":2,"4":3},"rating_thresholds":{"high_satisfaction":4.5,"acceptable":3.0}},"topic_preferences":[{"topic":"battery life","sentiment":"positive","frequency":4,"importance":"high"}],"review_length":{"average_length":72,"min_length":40,"max_length":120}}`,
			`{"rationale":"- Fits the user's preferences well.\n- The price and category are a reasonable match.","predicted_rating":4.2}`,
			"First draft review with a bit too much AI polish.",
			`{"verdict":"FAIL","feedback":"Make it sound more like a real person and remove AI phrasing."}`,
			"Revised review that sounds more natural and direct.",
			`{"verdict":"PASS","feedback":""}`,
		},
	}

	deps := &Deps{
		LLM: llm.Registry{
			"kimi": model,
		},
	}

	mux := http.NewServeMux()
	mux.HandleFunc("POST /generate-review", GenerateReviewHandler(deps))

	payload := GenerateReviewRequest{
		UserHistory: []ReviewHistoryEntry{
			{
				ProductID:       "h1",
				ProductName:     "Wireless Earbuds",
				ProductCategory: "electronics",
				StarRating:      4,
				ReviewText:      "Good sound for the price.",
				ReviewDate:      "2026-05-01",
				Source:          "amazon",
			},
			{
				ProductID:       "h2",
				ProductName:     "Laptop Stand",
				ProductCategory: "electronics",
				StarRating:      3.5,
				ReviewText:      "Useful, but a little overpriced.",
				ReviewDate:      "2026-05-10",
				Source:          "amazon",
			},
		},
		TargetProduct: ReviewTargetProduct{
			ProductID:       "t1",
			ProductName:     "Portable Speaker",
			ProductCategory: "electronics",
			Description:     "Compact Bluetooth speaker with deep bass.",
			Price:           25000,
			Currency:        "NGN",
			Source:          "amazon",
			Features:        []string{"bluetooth", "portable", "deep bass"},
			Rating:          4.4,
			ReviewCount:     152,
		},
		Provider: "kimi",
	}

	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/generate-review", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status: got %d, body=%s", rec.Code, rec.Body.String())
	}

	var resp ReviewGenerationResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}

	if resp.GeneratedReview != "Revised review that sounds more natural and direct." {
		t.Fatalf("unexpected review: %q", resp.GeneratedReview)
	}
	if math.Abs(resp.PredictedRating-4.2) > 1e-9 {
		t.Fatalf("unexpected rating: %v", resp.PredictedRating)
	}
	if resp.IterationsUsed != 2 {
		t.Fatalf("unexpected iterations: %d", resp.IterationsUsed)
	}
	if resp.CriticVerdict != "PASS" {
		t.Fatalf("unexpected verdict: %s", resp.CriticVerdict)
	}
	// ExecutionTiming is present (may be 0 or > 0 depending on system speed)
	if resp.ExecutionTiming.DrafterMs < 0 || resp.ExecutionTiming.CriticMs < 0 {
		t.Fatalf("unexpected timing: %+v", resp.ExecutionTiming)
	}

	if model.calls != 6 {
		t.Fatalf("unexpected model calls: got %d, want 6", model.calls)
	}
}

func TestGenerateReviewHandler_InvalidStarRating(t *testing.T) {
	model := &fakeReviewModel{
		responses: []string{},
	}

	deps := &Deps{
		LLM: llm.Registry{
			"kimi": model,
		},
	}

	mux := http.NewServeMux()
	mux.HandleFunc("POST /generate-review", GenerateReviewHandler(deps))

	payload := GenerateReviewRequest{
		UserHistory: []ReviewHistoryEntry{
			{
				ProductID:       "h1",
				ProductName:     "Product 1",
				ProductCategory: "category",
				StarRating:      6.0, // Invalid: > 5.0
				ReviewText:      "Some review",
				ReviewDate:      "2026-05-01",
				Source:          "amazon",
			},
			{
				ProductID:       "h2",
				ProductName:     "Product 2",
				ProductCategory: "category",
				StarRating:      4.0,
				ReviewText:      "Another review",
				ReviewDate:      "2026-05-02",
				Source:          "amazon",
			},
		},
		TargetProduct: ReviewTargetProduct{
			ProductID:       "t1",
			ProductName:     "Target",
			ProductCategory: "category",
		},
		Provider: "kimi",
	}

	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/generate-review", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}

	body = rec.Body.Bytes()
	if !bytes.Contains(body, []byte("star_rating must be between 1.0 and 5.0")) {
		t.Fatalf("unexpected error message: %s", string(body))
	}
}

func TestGenerateReviewHandler_InsufficientHistory(t *testing.T) {
	model := &fakeReviewModel{
		responses: []string{},
	}

	deps := &Deps{
		LLM: llm.Registry{
			"kimi": model,
		},
	}

	mux := http.NewServeMux()
	mux.HandleFunc("POST /generate-review", GenerateReviewHandler(deps))

	payload := GenerateReviewRequest{
		UserHistory: []ReviewHistoryEntry{
			{
				ProductID:       "h1",
				ProductName:     "Product 1",
				ProductCategory: "category",
				StarRating:      4.0,
				ReviewText:      "Some review",
				ReviewDate:      "2026-05-01",
				Source:          "amazon",
			},
			// Only 1 item, need at least 2
		},
		TargetProduct: ReviewTargetProduct{
			ProductID:       "t1",
			ProductName:     "Target",
			ProductCategory: "category",
		},
		Provider: "kimi",
	}

	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/generate-review", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}

	body = rec.Body.Bytes()
	if !bytes.Contains(body, []byte("at least 2 prior reviews")) {
		t.Fatalf("unexpected error message: %s", string(body))
	}
}

func TestGenerateReviewHandler_UnavailableProvider(t *testing.T) {
	model := &fakeReviewModel{
		responses: []string{},
	}

	deps := &Deps{
		LLM: llm.Registry{
			"kimi": model,
		},
	}

	mux := http.NewServeMux()
	mux.HandleFunc("POST /generate-review", GenerateReviewHandler(deps))

	payload := GenerateReviewRequest{
		UserHistory: []ReviewHistoryEntry{
			{
				ProductID:       "h1",
				ProductName:     "Product 1",
				ProductCategory: "category",
				StarRating:      4.0,
				ReviewText:      "Some review",
				ReviewDate:      "2026-05-01",
				Source:          "amazon",
			},
			{
				ProductID:       "h2",
				ProductName:     "Product 2",
				ProductCategory: "category",
				StarRating:      3.5,
				ReviewText:      "Another review",
				ReviewDate:      "2026-05-02",
				Source:          "amazon",
			},
		},
		TargetProduct: ReviewTargetProduct{
			ProductID:       "t1",
			ProductName:     "Target",
			ProductCategory: "category",
		},
		Provider: "nonexistent_provider",
	}

	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/generate-review", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}

	body = rec.Body.Bytes()
	if !bytes.Contains(body, []byte("requested provider not available")) {
		t.Fatalf("unexpected error message: %s", string(body))
	}
	if !bytes.Contains(body, []byte("available_providers")) {
		t.Fatalf("expected available_providers in error details: %s", string(body))
	}
}
