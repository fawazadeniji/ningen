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

		// 1. Calculate precise metrics in Go using the FULL history (Fast & Accurate)
		var totalRating float64
		distribution := map[string]int{"1": 0, "2": 0, "3": 0, "4": 0, "5": 0}

		var totalLength int
		minLength := -1
		maxLength := 0

		for _, entry := range state.UserHistory {
			totalRating += entry.StarRating

			// Safely count ratings distribution
			ratingInt := int(entry.StarRating + 0.5)
			if ratingInt >= 1 && ratingInt <= 5 {
				distribution[fmt.Sprintf("%d", ratingInt)]++
			}

			// Calculate exact review lengths
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

		lowThreshold := maxFloat(calculatedAverageRating-1.0, 1.0)
		highThreshold := minFloat(calculatedAverageRating+1.0, 5.0)

		calculatedRatingPatterns := RatingPatterns{
			RatingsDistribution: distribution,
			RatingThresholds: RatingThresholds{
				Low:  lowThreshold,
				High: highThreshold,
			},
		}

		// 2. SPEED OPTIMIZATION: Only pass the 7 most recent reviews to the LLM.
		// We don't need 50 reviews to determine someone's writing style.
		historyStr := buildHistoryContext(state.UserHistory, 7)
		messages := buildProfilerPrompt(historyStr)

		// 3. Ensure you are using a fast model here (e.g., gpt-4o-mini or claude-3-haiku)
		// Allow per-run override of the model used for the profiler node.
		opts := []llm.CompletionOption{llm.WithJSONSchemaResponse("profiler_response", buildProfilerSchema())}
		if m := state.ModelFor("profiler"); m != "" {
			opts = append(opts, llm.WithModel(m))
		}

		response, err := model.Complete(ctx, messages, opts...)
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
			AverageRating:       calculatedAverageRating,
			RatingPatterns:      calculatedRatingPatterns,
			ReviewLength:        calculatedReviewLength,
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

// SPEED OPTIMIZATION: Added maxReviews parameter to cap input context size
func buildHistoryContext(history []HistoryEntry, maxReviews int) string {
	var sb strings.Builder

	start := 0
	if len(history) > maxReviews {
		start = len(history) - maxReviews // Take the most recent ones
	}

	for i := start; i < len(history); i++ {
		entry := history[i]
		fmt.Fprintf(&sb, "Review %d:\n", i-start+1)
		fmt.Fprintf(&sb, "  Product: %s (%s)\n", entry.ProductName, entry.ProductCategory)
		fmt.Fprintf(&sb, "  Text: %s\n\n", entry.ReviewText)
		// Removed Rating, Date, and Source here to save input tokens,
		// since the LLM is focusing purely on stylistic and psychological profiling now.
	}
	return sb.String()
}

// buildProfilerPrompt constructs the prompt for extracting user behavioral patterns.
func buildProfilerPrompt(historyContext string) []llm.Message {
	systemInstruction := `You are an expert behavioral psychologist. Analyze the user's review history and extract a psychological profile. 
CRITICAL SPEED REQUIREMENT: Keep all text descriptions extremely concise (under 5 words). Output ONLY valid JSON.`

	// SPEED OPTIMIZATION: Explicitly constrained array lengths and description lengths
	userInstruction := fmt.Sprintf(`Analyze this user's recent review history:

REVIEW HISTORY:
%s

Extract the dimensions into JSON. Follow these strict limits to ensure fast processing:
1. consumer_persona: Max 3 words (e.g., "Impatient Bargain Hunter").
2. preferred_categories: Max 2 items.
3. behavioral_markers: EXACTLY 2 items. Keep 'description' under 5 words.
4. topic_preferences: EXACTLY 2 items.
5. cultural_hooks: Max 2 keywords based on Nigerian context.
6. formatting_quirks & review_style & tone_profile: Fill based on the text provided.

Do not write long sentences. Use short keywords.`, historyContext)

	return []llm.Message{
		{Role: "system", Content: systemInstruction},
		{Role: "user", Content: userInstruction},
	}
}

func buildProfilerSchema() map[string]any {
	// ... (Your existing schema remains exactly the same, no changes needed here
	// because the prompt now controls the length of the arrays and strings).
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
					"punctuation_habits": map[string]any{"type": "string"},
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
					"use_emotional_lang": map[string]any{"type": "boolean"},
					"use_tech_language":  map[string]any{"type": "boolean"},
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
				"type":        "array",
				"items":       map[string]any{"type": "string"},
				"description": "Concepts the user cares about that can be localized",
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

// Helpers for min/max
func maxFloat(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}

func minFloat(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}
