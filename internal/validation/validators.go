package validation

import (
	"fmt"

	"ningen/internal/pipeline"
)

// ValidationError represents a validation failure with a detailed message.
type ValidationError struct {
	Field   string
	Message string
}

func (e ValidationError) Error() string {
	if e.Field != "" {
		return fmt.Sprintf("%s: %s", e.Field, e.Message)
	}
	return e.Message
}

// ValidateHistoryEntry checks a single review history entry for invalid data.
func ValidateHistoryEntry(entry pipeline.HistoryEntry, index int) error {
	prefix := fmt.Sprintf("user_history[%d]", index)

	if entry.ProductID == "" {
		return ValidationError{Field: prefix, Message: "product_id is required"}
	}
	if entry.StarRating < 1.0 || entry.StarRating > 5.0 {
		return ValidationError{Field: prefix, Message: fmt.Sprintf("star_rating must be between 1.0 and 5.0, got %.1f", entry.StarRating)}
	}
	if entry.ReviewText == "" {
		return ValidationError{Field: prefix, Message: "review_text is required"}
	}

	return nil
}

// ValidateUserHistory checks the entire user history array.
func ValidateUserHistory(history []pipeline.HistoryEntry) error {
	if len(history) < 2 {
		return ValidationError{Field: "user_history", Message: "at least 2 prior reviews are required for style inference"}
	}
	if len(history) > 50 {
		return ValidationError{Field: "user_history", Message: fmt.Sprintf("maximum 50 history items allowed, got %d", len(history))}
	}

	for i, entry := range history {
		if err := ValidateHistoryEntry(entry, i); err != nil {
			return err
		}
	}

	return nil
}

// ValidateTargetProduct checks the target product for invalid data.
func ValidateTargetProduct(product pipeline.TargetProduct) error {
	if product.ProductID == "" {
		return ValidationError{Field: "target_product.product_id", Message: "product_id is required"}
	}

	if product.Price < 0 {
		return ValidationError{Field: "target_product.price", Message: fmt.Sprintf("price cannot be negative, got %.2f", product.Price)}
	}

	if product.Rating < 0 || product.Rating > 5.0 {
		return ValidationError{Field: "target_product.rating", Message: fmt.Sprintf("rating must be between 0.0 and 5.0, got %.1f", product.Rating)}
	}

	if product.ReviewCount < 0 {
		return ValidationError{Field: "target_product.review_count", Message: fmt.Sprintf("review_count cannot be negative, got %d", product.ReviewCount)}
	}

	return nil
}
