package validation

import (
	"testing"

	"ningen/internal/pipeline"
)

func TestValidateHistoryEntry_Valid(t *testing.T) {
	entry := pipeline.HistoryEntry{
		ProductID:  "p1",
		StarRating: 3.5,
		ReviewText: "Good product",
	}

	err := ValidateHistoryEntry(entry, 0)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}

func TestValidateHistoryEntry_MissingProductID(t *testing.T) {
	entry := pipeline.HistoryEntry{
		StarRating: 3.5,
		ReviewText: "Good product",
	}

	err := ValidateHistoryEntry(entry, 0)
	if err == nil {
		t.Fatalf("expected error for missing product_id")
	}

	if err.Error() != "user_history[0]: product_id is required" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateHistoryEntry_InvalidStarRating(t *testing.T) {
	tests := []struct {
		name   string
		rating float64
	}{
		{"Below minimum", 0.5},
		{"Above maximum", 5.5},
		{"Negative", -1.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entry := pipeline.HistoryEntry{
				ProductID:  "p1",
				StarRating: tt.rating,
				ReviewText: "Good product",
			}

			err := ValidateHistoryEntry(entry, 0)
			if err == nil {
				t.Fatalf("expected error for rating %.1f", tt.rating)
			}

			if !contains(err.Error(), "star_rating must be between 1.0 and 5.0") {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestValidateHistoryEntry_MissingReviewText(t *testing.T) {
	entry := pipeline.HistoryEntry{
		ProductID:  "p1",
		StarRating: 3.5,
		ReviewText: "",
	}

	err := ValidateHistoryEntry(entry, 0)
	if err == nil {
		t.Fatalf("expected error for missing review_text")
	}

	if err.Error() != "user_history[0]: review_text is required" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateUserHistory_MinimumRequired(t *testing.T) {
	history := []pipeline.HistoryEntry{
		{
			ProductID:  "p1",
			StarRating: 3.5,
			ReviewText: "Good product",
		},
	}

	err := ValidateUserHistory(history)
	if err == nil {
		t.Fatalf("expected error for insufficient history")
	}

	if !contains(err.Error(), "at least 2 prior reviews") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateUserHistory_MaximumLimit(t *testing.T) {
	history := make([]pipeline.HistoryEntry, 51)
	for i := range history {
		history[i] = pipeline.HistoryEntry{
			ProductID:  "p" + string(rune('0'+i%10)),
			StarRating: 3.5,
			ReviewText: "Review text",
		}
	}

	err := ValidateUserHistory(history)
	if err == nil {
		t.Fatalf("expected error for too many items")
	}

	if !contains(err.Error(), "maximum 50 history items") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateUserHistory_Valid(t *testing.T) {
	history := []pipeline.HistoryEntry{
		{
			ProductID:  "p1",
			StarRating: 3.5,
			ReviewText: "Good product",
		},
		{
			ProductID:  "p2",
			StarRating: 4.0,
			ReviewText: "Great product",
		},
	}

	err := ValidateUserHistory(history)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}

func TestValidateTargetProduct_Valid(t *testing.T) {
	product := pipeline.TargetProduct{
		ProductID:   "t1",
		Price:       100.0,
		Rating:      4.5,
		ReviewCount: 10,
	}

	err := ValidateTargetProduct(product)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}

func TestValidateTargetProduct_MissingProductID(t *testing.T) {
	product := pipeline.TargetProduct{
		Price:       100.0,
		Rating:      4.5,
		ReviewCount: 10,
	}

	err := ValidateTargetProduct(product)
	if err == nil {
		t.Fatalf("expected error for missing product_id")
	}

	if err.Error() != "target_product.product_id: product_id is required" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateTargetProduct_NegativePrice(t *testing.T) {
	product := pipeline.TargetProduct{
		ProductID:   "t1",
		Price:       -10.0,
		Rating:      4.5,
		ReviewCount: 10,
	}

	err := ValidateTargetProduct(product)
	if err == nil {
		t.Fatalf("expected error for negative price")
	}

	if !contains(err.Error(), "price cannot be negative") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateTargetProduct_InvalidRating(t *testing.T) {
	tests := []struct {
		name   string
		rating float64
	}{
		{"Negative", -1.0},
		{"Above maximum", 5.5},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			product := pipeline.TargetProduct{
				ProductID:   "t1",
				Price:       100.0,
				Rating:      tt.rating,
				ReviewCount: 10,
			}

			err := ValidateTargetProduct(product)
			if err == nil {
				t.Fatalf("expected error for rating %.1f", tt.rating)
			}

			if !contains(err.Error(), "rating must be between 0.0 and 5.0") {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

// Helper function
func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
