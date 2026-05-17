package nodes

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"ningen/internal/llm"
)

// CriticResponse represents the verdict and feedback from the Critic node.
type CriticResponse struct {
	Verdict  string `json:"verdict"`  // "PASS" or "FAIL"
	Feedback string `json:"feedback"` // Suggestions for improvement if FAIL
}

// Critic creates a node function that performs behavioral fidelity QA.
// It checks the draft review against the user's historical patterns and strict rules.
func Critic(model llm.LLMProvider) func(context.Context, AgentState) (AgentState, error) {
	return func(ctx context.Context, state AgentState) (AgentState, error) {
		if state.UserProfile == nil {
			return state, fmt.Errorf("user profile is missing")
		}
		if state.DraftReview == "" {
			return state, fmt.Errorf("draft review is missing")
		}

		// Calculate exact boundaries based on same logic as Drafter
		avgWords := state.UserProfile.ReviewLength.AverageLength / 5
		minWords := max(avgWords-10, 5)
		maxWords := avgWords + 15

		// Calculate actual draft length in Go to prevent LLM counting hallucination
		actualWordCount := len(strings.Fields(state.DraftReview))

		// PRE-CHECK: Save API calls by failing obvious violations locally in Go
		localVerdict, localFeedback := strictLocalValidation(state.DraftReview, actualWordCount, minWords, maxWords)
		if localVerdict == "FAIL" {
			state.Iterations++
			state.CriticVerdict = localVerdict
			state.CriticFeedback = localFeedback
			return state, nil
		}

		// If it passes local checks, let the LLM evaluate the behavioral/cultural nuances
		messages := buildCriticPrompt(state.UserProfile, state.DraftReview, state.UserHistory, actualWordCount, minWords, maxWords)

		opts := []llm.CompletionOption{llm.WithJSONSchemaResponse("critic_response", buildCriticSchema())}
		if m := state.ModelFor("critic"); m != "" {
			opts = append(opts, llm.WithModel(m))
		}

		response, err := model.Complete(ctx, messages, opts...)
		if err != nil {
			return state, fmt.Errorf("critic LLM call failed: %w", err)
		}

		verdict, feedback := parseCriticResponse(response)

		state.Iterations++
		state.CriticVerdict = verdict
		state.CriticFeedback = feedback

		if verdict == "PASS" {
			state.FinalReview = state.DraftReview
		}

		return state, nil
	}
}

// buildCriticPrompt constructs the prompt for strict QA evaluation.
func buildCriticPrompt(profile *UserProfile, draftReview string, history []HistoryEntry, actualWordCount, minWords, maxWords int) []llm.Message {
	historyStr := buildHistorySample(history, 3)

	userPrompt := fmt.Sprintf(`You are a ruthless, highly-critical QA Auditor. Your job is to reject AI-generated reviews that fail to perfectly mimic the target user.

<USER_BEHAVIORAL_PROFILE>
%s
</USER_BEHAVIORAL_PROFILE>

<USER_HISTORY_SAMPLE>
%s
</USER_HISTORY_SAMPLE>

<DRAFT_TO_EVALUATE>
"%s"
</DRAFT_TO_EVALUATE>

<HARD_FAIL_CONDITIONS>
You MUST return "FAIL" if ANY of the following are true:
1. LENGTH: The draft is exactly %d words long. The strict limit is between %d and %d words. If %d is outside this range, you must FAIL it.
2. FORMATTING: The user's capitalization style is "%s" and punctuation is "%s". If the draft violates this (e.g., using proper caps when the user writes in all lowercase), you must FAIL it.
3. AI SPEAK: If you detect any generic AI fluff ("delve", "tapestry", "seamless", "elevate", "commendable", "noteworthy").
4. CULTURAL TONE: The user is a "%s" from Nigeria. If the draft sounds like a generic American AI rather than a localized Nigerian internet user, you must FAIL it.
</HARD_FAIL_CONDITIONS>

If you return FAIL, your feedback MUST explicitly tell the Drafter what to fix (e.g., "Make it shorter, remove capital letters, use 'Sapa' instead of 'financially constrained'").

Respond with the exact JSON schema provided.`,
		formatStructuredProfile(profile),
		historyStr,
		draftReview,
		actualWordCount, minWords, maxWords, actualWordCount,
		profile.FormattingQuirks.CapitalizationStyle,
		profile.FormattingQuirks.PunctuationHabits,
		profile.ConsumerPersona)

	return []llm.Message{
		{Role: "system", Content: "You are an aggressive behavioral consistency auditor. Decide if the review authentically mimics the user or sounds like an AI."},
		{Role: "user", Content: userPrompt},
	}
}

// strictLocalValidation performs instant Go-native checks to save API tokens and time.
func strictLocalValidation(text string, wordCount, minWords, maxWords int) (string, string) {
	lowerText := strings.ToLower(text)

	// 1. Enforce Hard Word Count Bounds
	if wordCount < minWords {
		return "FAIL", fmt.Sprintf("The review is too short (%d words). It MUST be between %d and %d words based on the user's history. Expand on their favorite topics.", wordCount, minWords, maxWords)
	}
	if wordCount > maxWords {
		return "FAIL", fmt.Sprintf("The review is too long (%d words). It MUST be between %d and %d words. Cut out the fluff and be more concise.", wordCount, minWords, maxWords)
	}

	// 2. Strict AI "Red Flag" checking
	redFlags := []string{
		"delve", "tapestry", "curate", "elevate", "seamless", "commendable",
		"journey", "paradigm", "game-changer", "must-have", "pleasantly surprised",
		"transform", "revolutionize", "testament", "beacon", "in conclusion",
	}

	for _, flag := range redFlags {
		if strings.Contains(lowerText, flag) {
			return "FAIL", fmt.Sprintf("Review contains banned AI language: '%s'. You MUST rewrite this using natural, human colloquialisms. Avoid generic marketing speak.", flag)
		}
	}

	return "PASS", ""
}

// buildHistorySample selects a sample of past reviews to include in the prompt.
func buildHistorySample(history []HistoryEntry, sampleSize int) string {
	if len(history) == 0 {
		return "No past reviews available."
	}
	if sampleSize > len(history) {
		sampleSize = len(history)
	}
	start := max(len(history)-sampleSize, 0)

	var sb strings.Builder
	for i := start; i < len(history); i++ {
		entry := history[i]
		fmt.Fprintf(&sb, "Review %d:\nRating: %.1f stars\nText: %s\n\n", i-start+1, entry.StarRating, entry.ReviewText)
	}
	return sb.String()
}

// parseCriticResponse parses the critic's response and performs validation checks.
func parseCriticResponse(responseText string) (string, string) {
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

	var criticResp CriticResponse
	err := json.Unmarshal([]byte(jsonStr), &criticResp)

	// Fallback if parsing fails totally
	if err != nil || (criticResp.Verdict != "PASS" && criticResp.Verdict != "FAIL") {
		return "PASS", "" // Default to pass to avoid infinite error loops
	}

	if criticResp.Verdict == "FAIL" && criticResp.Feedback == "" {
		criticResp.Feedback = "The review did not match the user's authentic style. Please rewrite focusing closer on the behavioral profile."
	}

	return criticResp.Verdict, criticResp.Feedback
}

// buildCriticSchema constructs the strict response schema for the Critic LLM call.
func buildCriticSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"verdict": map[string]any{
				"type":        "string",
				"enum":        []string{"PASS", "FAIL"},
				"description": "Whether the draft perfectly mimics the user.",
			},
			"feedback": map[string]any{
				"type":        "string",
				"description": "If FAIL, give harsh, explicit instructions on what the Drafter must fix. If PASS, leave empty.",
			},
		},
		"required":             []string{"verdict", "feedback"},
		"additionalProperties": false,
	}
}
