package nodes

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"ningen/internal/llm"
)

// RaterResponse represents the structured output from the Rater node.
type RaterResponse struct {
	Rationale       string  `json:"rationale"`
	PredictedRating float64 `json:"predicted_rating"`
}

// Rater creates a node function that predicts a star rating based on user profile and target product.
// It uses strict mathematical anchoring to minimize RMSE.
func Rater(model llm.LLMProvider) func(context.Context, AgentState) (AgentState, error) {
	return func(ctx context.Context, state AgentState) (AgentState, error) {
		if state.UserProfile == nil {
			return state, fmt.Errorf("user profile is missing")
		}

		// Optional: If you localized the product for the Drafter, do it here too so the Rater understands the context
		localizedProduct := LocalizeContext(&state.TargetProduct)

		messages := buildRaterPrompt(state.UserProfile, &localizedProduct)

		opts := []llm.CompletionOption{llm.WithJSONSchemaResponse("rater_response", buildRaterSchema())}
		if m := state.ModelFor("rater"); m != "" {
			opts = append(opts, llm.WithModel(m))
		}

		response, err := model.Complete(ctx, messages, opts...)
		if err != nil {
			return state, fmt.Errorf("rater LLM call failed: %w", err)
		}

		raterResp, err := parseRaterResponse(response)
		if err != nil {
			return state, fmt.Errorf("failed to parse rater response: %w", err)
		}

		// Ensure prediction doesn't wildly escape bounds (anti-hallucination)
		if raterResp.PredictedRating > 5.0 {
			raterResp.PredictedRating = 5.0
		} else if raterResp.PredictedRating < 1.0 {
			raterResp.PredictedRating = 1.0
		}

		state.PredictedRating = raterResp.PredictedRating
		state.RatingReasoning = raterResp.Rationale

		return state, nil
	}
}

// buildRaterPrompt constructs the prompt for predicting a star rating.
func buildRaterPrompt(profile *UserProfile, product *TargetProduct) []llm.Message {
	systemInstruction := `You are a strict behavioral data scientist. Your goal is to predict EXACTLY what rating (1.0 to 5.0) a specific user will give a new product. You must avoid generic AI rating bias. You must anchor your prediction on the user's historical mathematical averages.`

	userPrompt := fmt.Sprintf(`Predict the star rating for this user.

<USER_MATHEMATICAL_BASELINE>
- Historical Average Rating: %.2f stars
- Their "Low" Threshold (Anything below this is considered a failure by them): %.2f stars
- Their "High" Threshold (Requires exceptional alignment to achieve): %.2f stars
- Overall Tendency: %s
</USER_MATHEMATICAL_BASELINE>

<USER_PSYCHOLOGICAL_PROFILE>
- Consumer Persona: %s
- Preferred Categories: %v
- Behavioral Markers: %v
- Cultural Hooks: %v
</USER_PSYCHOLOGICAL_PROFILE>

<TARGET_PRODUCT>
%s
</TARGET_PRODUCT>

<PREDICTION_LOGIC>
Anchor on the user's Historical Average Rating, then adjust:
1. Does the product align with their Behavioral Markers or Cultural Hooks? If they hate slow delivery and it's mentioned, penalize.
2. Is it in their preferred categories?
3. Adjust the baseline up or down to reach the final Predicted Rating.
</PREDICTION_LOGIC>

Respond strictly with the provided JSON schema. Your "rationale" field must be a user-safe rationale — 3-4 bullet points a real user could read, explaining why this rating was predicted.`,
		profile.AverageRating,
		profile.RatingPatterns.RatingThresholds.Low,
		profile.RatingPatterns.RatingThresholds.High,
		profile.OverallTendency,
		profile.ConsumerPersona,
		strings.Join(profile.PreferredCategories, ", "),
		formatBehavioralMarkers(profile.BehavioralMarkers), // Assuming helper exists
		strings.Join(profile.CulturalHooks, ", "),
		formatStructuredProduct(product),
	)

	return []llm.Message{
		{Role: "system", Content: systemInstruction},
		{Role: "user", Content: userPrompt},
	}
}

// Helper to format behavioral markers nicely for the prompt
func formatBehavioralMarkers(markers []BehavioralMarker) string {
	var sb strings.Builder
	for _, m := range markers {
		fmt.Fprintf(&sb, "[%s: %s] ", m.Marker, m.Description)
	}
	return sb.String()
}

// parseRaterResponse extracts the rating and rationale from the LLM response.
func parseRaterResponse(responseText string) (*RaterResponse, error) {
	// Simple JSON extraction block if model outputs markdown around it
	jsonStr := responseText
	if strings.Contains(responseText, "```json") {
		parts := strings.SplitN(responseText, "```json\n", 2)
		if len(parts) == 2 {
			jsonStr = strings.SplitN(parts[1], "\n```", 2)[0]
		}
	} else if strings.Contains(responseText, "```") {
		parts := strings.SplitN(responseText, "```\n", 2)
		if len(parts) == 2 {
			jsonStr = strings.SplitN(parts[1], "\n```", 2)[0]
		}
	}

	var raterResp RaterResponse
	if err := json.Unmarshal([]byte(jsonStr), &raterResp); err != nil {
		rating, err := extractRatingFromText(responseText)
		if err != nil {
			return nil, fmt.Errorf("could not parse JSON or regex extract rating from response")
		}
		return &RaterResponse{
			Rationale:       responseText, // Fallback: just dump the text
			PredictedRating: rating,
		}, nil
	}

	if err := validateRaterResponse(&raterResp); err != nil {
		return nil, fmt.Errorf("rater response validation failed: %w", err)
	}

	return &raterResp, nil
}

// validateRaterResponse validates a RaterResponse against the structured schema.
func validateRaterResponse(raterResp *RaterResponse) error {
	if raterResp.Rationale == "" {
		return fmt.Errorf("rationale is required")
	}

	if raterResp.PredictedRating < 1.0 || raterResp.PredictedRating > 5.0 {
		return fmt.Errorf("predicted_rating must be between 1.0 and 5.0, got %.2f", raterResp.PredictedRating)
	}

	return nil
}

// extractRatingFromText attempts to extract a rating value from text using regex.
func extractRatingFromText(text string) (float64, error) {
	re := regexp.MustCompile(`"predicted_rating"\s*:\s*([\d.]+)`)
	matches := re.FindStringSubmatch(text)
	if len(matches) > 1 {
		return strconv.ParseFloat(matches[1], 64)
	}

	fallbackPatterns := []*regexp.Regexp{
		regexp.MustCompile(`(?i)\bpredicted[_\s]*rating\b\s*[:=]\s*([1-5](?:\.\d+)?)\b`),
		regexp.MustCompile(`(?i)\brating\b\s*[:=]\s*([1-5](?:\.\d+)?)\b`),
		regexp.MustCompile(`(?i)\brating\b\s+(?:is|was)\s+([1-5](?:\.\d+)?)\b`),
	}

	for _, re := range fallbackPatterns {
		matches = re.FindStringSubmatch(text)
		if len(matches) > 1 {
			return strconv.ParseFloat(matches[1], 64)
		}
	}

	return 0, fmt.Errorf("could not extract rating from text")
}

// buildRaterSchema constructs the strict response schema for the Rater LLM call.
func buildRaterSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"rationale": map[string]any{
				"type":        "string",
				"description": "Chain-of-thought bullet points showing how you adjusted from their baseline average to reach the final rating.",
			},
			"predicted_rating": map[string]any{
				"type":        "number",
				"minimum":     1.0,
				"maximum":     5.0,
				"description": "The final predicted star rating for this user and product.",
			},
		},
		"required":             []string{"rationale", "predicted_rating"},
		"additionalProperties": false,
	}
}
