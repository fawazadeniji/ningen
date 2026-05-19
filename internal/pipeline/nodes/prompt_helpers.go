package nodes

import (
	"fmt"
	"strings"

	"ningen/internal/llm"
)

func buildMessages(systemPrompt, userPrompt string) []llm.Message {
	return []llm.Message{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: userPrompt},
	}
}

func extractJSON(text string) string {
	start := strings.Index(text, "{")
	end := strings.LastIndex(text, "}")
	if start >= 0 && end > start {
		return text[start : end+1]
	}
	return text
}

func formatStructuredProfile(profile *UserProfile) string {
	if profile == nil {
		return "No profile available."
	}
	return fmt.Sprintf(
		"UserID: %s\nOverall Tendency: %s\nAverage Rating: %.2f\nPreferred Categories: %v\nReview Style: %+v\nBehavioral Markers: %+v\nTone Profile: %+v\nRating Patterns: %+v\nTopic Preferences: %+v\nReview Length: %+v",
		profile.UserID,
		profile.OverallTendency,
		profile.AverageRating,
		profile.PreferredCategories,
		profile.ReviewStyle,
		profile.BehavioralMarkers,
		profile.ToneProfile,
		profile.RatingPatterns,
		profile.TopicPreferences,
		profile.ReviewLength,
	)
}

func formatStructuredProduct(product *TargetProduct) string {
	if product == nil {
		return "No product available."
	}
	return fmt.Sprintf(
		"ProductID: %s\nName: %s\nCategory: %s\nDescription: %s\nPrice: %.2f %s\nSource: %s\nFeatures: %v\nPlatform Rating: %.2f\nReview Count: %d",
		product.ProductID,
		product.ProductName,
		product.ProductCategory,
		product.Description,
		product.Price,
		product.Currency,
		product.Source,
		product.Features,
		product.Rating,
		product.ReviewCount,
	)
}
