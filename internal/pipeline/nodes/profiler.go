package nodes

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"ningen/internal/llm"
)



// Profiler creates a node function that extracts user behavioral patterns.
func Profiler(model llm.LLMProvider) func(context.Context, AgentState) (AgentState, error) {
	return func(ctx context.Context, state AgentState) (AgentState, error) {
		if len(state.UserHistory) == 0 {
			return state, fmt.Errorf("user history is empty")
		}

		// Calculate precise metrics in Go to prevent LLM hallucination
		var totalRating float64
		distribution := map[string]int{"1": 0, "2": 0, "3": 0, "4": 0, "5": 0}

		var totalLength int
		minLength := -1
		maxLength := 0

		for _, entry := range state.UserHistory {
			totalRating += entry.StarRating

			// Safely count ratings distribution
			ratingInt := int(entry.StarRating + 0.5) // Round to nearest star
			if ratingInt >= 1 && ratingInt <= 5 {
				distribution[fmt.Sprintf("%d", ratingInt)]++
			}

			// Calculate exact review lengths (character count)
			textLength := len(entry.ReviewText)
			totalLength += textLength

			if minLength == -1 || textLength < minLength {
				minLength = textLength
			}
			if textLength > maxLength {
				maxLength = textLength
			}
		}
		calculatedAverageRating := totalRating / float64(len(state.UserHistory))

		// Safely compute review lengths
		averageLength := 0
		if len(state.UserHistory) > 0 {
			averageLength = totalLength / len(state.UserHistory)
		}
		if minLength == -1 {
			minLength = 0
		}

		calculatedReviewLength := ReviewLengthProfile{
			AverageLength: averageLength,
			MinLength:     minLength,
			MaxLength:     maxLength,
		}

		// Dynamically compute thresholds based on their average
		lowThreshold := calculatedAverageRating - 1.0
		if lowThreshold < 1.0 {
			lowThreshold = 1.0
		}
		highThreshold := calculatedAverageRating + 1.0
		if highThreshold > 5.0 {
			highThreshold = 5.0
		}

		calculatedRatingPatterns := RatingPatterns{
			RatingsDistribution: distribution,
			RatingThresholds: RatingThresholds{
				Low:  lowThreshold,
				High: highThreshold,
			},
		}

		historyStr := buildHistoryContext(state.UserHistory)
		messages := buildProfilerPrompt(historyStr)

		response, err := model.Complete(ctx, messages, llm.WithJSONSchemaResponse("profiler_response", buildProfilerSchema()))
		if err != nil {
			return state, fmt.Errorf("profiler LLM call failed: %w", err)
		}

		var profile ProfilerResponse
		if err := json.Unmarshal([]byte(response), &profile); err != nil {
			return state, fmt.Errorf("failed to unmarshal structured output: %w", err)
		}

		state.UserProfile = &UserProfile{
			UserID:              profile.UserID,
			OverallTendency:     profile.OverallTendency,
			ConsumerPersona:     profile.ConsumerPersona,
			AverageRating:       calculatedAverageRating, // Calculated safely in Go
			RatingPatterns:      calculatedRatingPatterns, // Calculated safely in Go
			ReviewLength:        calculatedReviewLength, // Calculated safely in Go
			PreferredCategories: profile.PreferredCategories,
			FormattingQuirks:    profile.FormattingQuirks,
			ReviewStyle:         profile.ReviewStyle,
			BehavioralMarkers:   profile.BehavioralMarkers,
			ToneProfile:         profile.ToneProfile,
			TopicPreferences:    profile.TopicPreferences,
			CulturalHooks:       profile.CulturalHooks,
		}

		return state, nil
	}
}

func buildHistoryContext(history []HistoryEntry) string {
	var sb strings.Builder
	for i, entry := range history {
		fmt.Fprintf(&sb, "Review %d:\n", i+1)
		fmt.Fprintf(&sb, "  Product: %s (Category: %s)\n", entry.ProductName, entry.ProductCategory)
		fmt.Fprintf(&sb, "  Rating: %.1f stars\n", entry.StarRating)
		fmt.Fprintf(&sb, "  Source: %s\n", entry.Source)
		fmt.Fprintf(&sb, "  Text: %s\n", entry.ReviewText)
		fmt.Fprintf(&sb, "  Date: %s\n\n", entry.ReviewDate)
	}
	return sb.String()
}

// buildProfilerPrompt constructs the prompt for extracting user behavioral patterns.
func buildProfilerPrompt(historyContext string) []llm.Message {
	systemInstruction := `You are an expert behavioral psychologist and forensic linguist. Your task is to analyze a user's review history and extract a highly detailed psychological and stylistic profile. 
You MUST respond with valid JSON matching the exact schema provided. Do not include markdown formatting or explanations.`

	userInstruction := fmt.Sprintf(`Analyze the following user's review history and extract their behavioral and linguistic profile.

REVIEW HISTORY:
%s

Extract the following dimensions:
1. user_id: A unique identifier for this user (e.g., "user_123").
2. overall_tendency: "positive", "balanced", or "critical".
3. consumer_persona: A short descriptor of their shopping identity (e.g., "Bargain Hunter", "Quality Snob", "Impatient Buyer").
4. preferred_categories: Array of product categories this user reviews most.
5. formatting_quirks: Crucial for reproducing their exact writing style. Note their punctuation_habits (e.g., "uses excessive exclamation marks", "rarely uses periods"), capitalization_style ("proper", "all_lowercase", "random_caps"), and emoji_usage ("frequent", "none").
6. review_style: Detail their verbosity_level ("terse", "concise", "verbose", "rambling"), use_emotional_lang (boolean), and use_tech_language (boolean).
7. behavioral_markers: 2-3 specific behavioral patterns with a confidence score and description (e.g., "price_conscious", "focuses_on_delivery_speed").
8. tone_profile: Cheerfulness, sarcasm, urgency, formality (0.0 to 1.0).
9. topic_preferences: 2-3 specific topics they care about (e.g., "customer service", "durability") with sentiment and importance.
10. cultural_hooks: Keywords or concepts they focus on that can be localized (e.g., "complains about shipping delays", "mentions family size").

Ensure ALL fields are populated based on the text. Output ONLY the raw JSON object.`, historyContext)

	return buildMessages(systemInstruction, userInstruction)
}

// buildProfilerSchema constructs the strict response schema for the Profiler LLM call.
func buildProfilerSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"user_id": map[string]any{"type": "string"},
			"overall_tendency": map[string]any{
				"type": "string",
				"enum": []string{"positive", "balanced", "critical"},
			},
			"consumer_persona": map[string]any{"type": "string"},
			"preferred_categories": map[string]any{
				"type":  "array",
				"items": map[string]any{"type": "string"},
			},
			"formatting_quirks": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"punctuation_habits":  map[string]any{"type": "string"},
					"capitalization_style": map[string]any{
						"type": "string",
						"enum": []string{"proper", "mostly_lowercase", "excessive_caps", "inconsistent"},
					},
					"emoji_usage": map[string]any{
						"type": "string",
						"enum": []string{"none", "rare", "frequent"},
					},
				},
				"required":             []string{"punctuation_habits", "capitalization_style", "emoji_usage"},
				"additionalProperties": false,
			},
			"review_style": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"verbosity_level": map[string]any{
						"type": "string",
						"enum": []string{"terse", "concise", "verbose", "rambling"},
					},
					"use_emotional_lang":   map[string]any{"type": "boolean"},
					"use_tech_language":    map[string]any{"type": "boolean"},
				},
				"required":             []string{"verbosity_level", "use_emotional_lang", "use_tech_language"},
				"additionalProperties": false,
			},
			"behavioral_markers": map[string]any{
				"type": "array",
				"items": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"marker": map[string]any{"type": "string"},
						"confidence": map[string]any{
							"type":    "number",
							"minimum": 0.0,
							"maximum": 1.0,
						},
						"description": map[string]any{"type": "string"},
					},
					"required":             []string{"marker", "confidence", "description"},
					"additionalProperties": false,
				},
			},
			"tone_profile": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"cheerfulness": map[string]any{"type": "number", "minimum": 0.0, "maximum": 1.0},
					"sarcasm":      map[string]any{"type": "number", "minimum": 0.0, "maximum": 1.0},
					"urgency":      map[string]any{"type": "number", "minimum": 0.0, "maximum": 1.0},
					"formality":    map[string]any{"type": "number", "minimum": 0.0, "maximum": 1.0},
				},
				"required":             []string{"cheerfulness", "sarcasm", "urgency", "formality"},
				"additionalProperties": false,
			},
			"topic_preferences": map[string]any{
				"type": "array",
				"items": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"topic": map[string]any{"type": "string"},
						"sentiment": map[string]any{
							"type": "string",
							"enum": []string{"positive", "negative", "neutral"},
						},
						"importance": map[string]any{
							"type": "string",
							"enum": []string{"high", "medium", "low"},
						},
					},
					"required":             []string{"topic", "sentiment", "importance"},
					"additionalProperties": false,
				},
			},
			"cultural_hooks": map[string]any{
				"type":  "array",
				"items": map[string]any{"type": "string"},
				"description": "Concepts the user cares about that can be localized (e.g. 'values fast delivery', 'complains about high prices')",
			},
		},
		"required": []string{
			"user_id", "overall_tendency", "consumer_persona", "preferred_categories",
			"formatting_quirks", "review_style", "behavioral_markers", "tone_profile",
			"topic_preferences", "cultural_hooks",
		},
		"additionalProperties": false,
	}
}
