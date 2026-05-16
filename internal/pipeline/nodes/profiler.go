package nodes

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"ningen/internal/llm"
)

// ProfilerResponse represents the structured output from the Profiler node.
type ProfilerResponse struct {
	UserID              string              `json:"user_id"`
	OverallTendency     string              `json:"overall_tendency"`
	AverageRating       float64             `json:"average_rating"`
	PreferredCategories []string            `json:"preferred_categories"`
	ReviewStyle         ReviewStyle         `json:"review_style"`
	BehavioralMarkers   []BehavioralMarker  `json:"behavioral_markers"`
	ToneProfile         ToneProfile         `json:"tone_profile"`
	RatingPatterns      RatingPatterns      `json:"rating_patterns"`
	TopicPreferences    []TopicPreference   `json:"topic_preferences"`
	ReviewLength        ReviewLengthProfile `json:"review_length"`
}

// Profiler creates a node function that extracts user behavioral patterns.
func Profiler(model llm.LLMProvider) func(context.Context, AgentState) (AgentState, error) {
	return func(ctx context.Context, state AgentState) (AgentState, error) {
		if len(state.UserHistory) == 0 {
			return state, fmt.Errorf("user history is empty")
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
			AverageRating:       profile.AverageRating,
			PreferredCategories: profile.PreferredCategories,
			ReviewStyle:         profile.ReviewStyle,
			BehavioralMarkers:   profile.BehavioralMarkers,
			ToneProfile:         profile.ToneProfile,
			RatingPatterns:      profile.RatingPatterns,
			TopicPreferences:    profile.TopicPreferences,
			ReviewLength:        profile.ReviewLength,
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
	return buildMessages(
		"You are an expert behavioral analyst. Extract a structured profile from review history.",
		fmt.Sprintf(`Analyze the following user's review history and extract their behavioral profile.

REVIEW HISTORY:
%s`, historyContext),
	)
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
			"average_rating": map[string]any{
				"type":    "number",
				"minimum": 1.0,
				"maximum": 5.0,
			},
			"preferred_categories": map[string]any{
				"type":  "array",
				"items": map[string]any{"type": "string"},
			},
			"review_style": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"detail_level": map[string]any{
						"type": "string",
						"enum": []string{"brief", "moderate", "detailed"},
					},
					"use_emotional_lang": map[string]any{"type": "boolean"},
					"use_tech_language":  map[string]any{"type": "boolean"},
					"comparison_frequency": map[string]any{
						"type": "string",
						"enum": []string{"rare", "occasional", "frequent"},
					},
				},
				"required":             []string{"detail_level", "use_emotional_lang", "use_tech_language", "comparison_frequency"},
				"additionalProperties": false,
			},
			"behavioral_markers": map[string]any{
				"type": "array",
				"items": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"marker": map[string]any{"type": "string"},
						"frequency": map[string]any{
							"type": "string",
							"enum": []string{"rare", "occasional", "frequent"},
						},
						"confidence": map[string]any{
							"type":    "number",
							"minimum": 0.0,
							"maximum": 1.0,
						},
						"description": map[string]any{"type": "string"},
					},
					"required":             []string{"marker", "frequency", "confidence", "description"},
					"additionalProperties": false,
				},
			},
			"tone_profile": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"cheerfulness": map[string]any{
						"type":    "number",
						"minimum": 0.0,
						"maximum": 1.0,
					},
					"sarcasm": map[string]any{
						"type":    "number",
						"minimum": 0.0,
						"maximum": 1.0,
					},
					"urgency": map[string]any{
						"type":    "number",
						"minimum": 0.0,
						"maximum": 1.0,
					},
					"formality": map[string]any{
						"type":    "number",
						"minimum": 0.0,
						"maximum": 1.0,
					},
				},
				"required":             []string{"cheerfulness", "sarcasm", "urgency", "formality"},
				"additionalProperties": false,
			},
			"rating_patterns": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"ratings_distribution": map[string]any{
						"type":                 "object",
						"additionalProperties": map[string]any{"type": "integer"},
					},
					"rating_thresholds": map[string]any{
						"type":                 "object",
						"additionalProperties": map[string]any{"type": "number"},
					},
				},
				"required":             []string{"ratings_distribution", "rating_thresholds"},
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
						"frequency": map[string]any{"type": "integer"},
						"importance": map[string]any{
							"type": "string",
							"enum": []string{"high", "medium", "low"},
						},
					},
					"required":             []string{"topic", "sentiment", "frequency", "importance"},
					"additionalProperties": false,
				},
			},
			"review_length": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"average_length": map[string]any{"type": "integer"},
					"min_length":     map[string]any{"type": "integer"},
					"max_length":     map[string]any{"type": "integer"},
				},
				"required":             []string{"average_length", "min_length", "max_length"},
				"additionalProperties": false,
			},
		},
		"required": []string{
			"user_id", "overall_tendency", "average_rating", "preferred_categories",
			"review_style", "behavioral_markers", "tone_profile", "rating_patterns",
			"topic_preferences", "review_length",
		},
		"additionalProperties": false,
	}
}
