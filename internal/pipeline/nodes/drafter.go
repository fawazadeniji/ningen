package nodes

import (
	"context"
	"fmt"
	"strings"

	"ningen/internal/llm"
)

// Drafter creates a node function that generates a persona-driven review.
// It localizes context to Nigerian settings and incorporates feedback from previous iterations.
func Drafter(model llm.LLMProvider) func(context.Context, AgentState) (AgentState, error) {
	return func(ctx context.Context, state AgentState) (AgentState, error) {
		if state.UserProfile == nil {
			return state, fmt.Errorf("user profile is missing")
		}

		localizedProduct := LocalizeContext(&state.TargetProduct)

		messages := buildDrafterPrompt(state.UserProfile, &localizedProduct, state.PredictedRating, state.CriticFeedback)

		opts := []llm.CompletionOption{}
		if m := state.ModelFor("drafter"); m != "" {
			opts = append(opts, llm.WithModel(m))
		}

		response, err := model.Complete(ctx, messages, opts...)
		if err != nil {
			return state, fmt.Errorf("drafter LLM call failed: %w", err)
		}

		draftReview := strings.TrimSpace(response)
		state.DraftReview = draftReview

		return state, nil
	}
}

// buildDrafterPrompt constructs the prompt for generating a persona-driven review.
func buildDrafterPrompt(profile *UserProfile, product *TargetProduct, rating float64, criticFeedback string) []llm.Message {
	// 1. Calculate hard word-count boundaries to prevent LLM rambling.
	// We assume ReviewLength metrics are in characters. Avg word length is ~5 chars.
	avgWords := profile.ReviewLength.AverageLength / 5
	minWords := max(avgWords-10,
		// Absolute minimum
		5)
	maxWords := avgWords + 15

	// 2. Handle Critic Feedback dynamically
	feedbackSection := ""
	if criticFeedback != "" {
		feedbackSection = fmt.Sprintf(`
<CRITIC_FEEDBACK>
YOUR PREVIOUS DRAFT WAS REJECTED. You must fix these specific issues:
%s
</CRITIC_FEEDBACK>`, criticFeedback)
	}

	// 3. System Instruction: Frame the LLM as a forensic mimic, not an assistant.
	systemInstruction := `You are a forensic linguistic-mimicry engine. You do NOT write like an AI. You do NOT write helpful essays. 
You act as a digital twin of a specific human internet user. You will adopt their exact tone, grammar flaws, capitalization habits, and regional slang.`

	// 4. The main user prompt with strict XML-style structuring
	userPrompt := fmt.Sprintf(`Generate a highly authentic product review mimicking the target user.

<TARGET_RATING>
You MUST generate a review that reflects exactly %.1f stars.
</TARGET_RATING>

<USER_PROFILE>
%s
</USER_PROFILE>

<PRODUCT_DETAILS>
%s
</PRODUCT_DETAILS>
%s

<STRICT_RULES_FOR_MIMICRY>
1. LENGTH ENFORCEMENT: The user typically writes ~%d words. Your review MUST be strictly between %d and %d words. DO NOT EXCEED THIS.
2. FORMATTING ENFORCEMENT: The user's capitalization style is "%s" and punctuation habit is "%s". You MUST copy this exact mechanical style. If their style is lowercase, use ZERO capital letters. If they use excessive exclamation marks, you must do the same.
3. NIGERIAN LOCALIZATION: You are simulating a Nigerian consumer. Based on their persona ("%s"), inject subtle Nigerian internet vernacular, references, or slang (e.g., 'Sapa', 'Omo', 'To be honest', 'They tried', 'Delivery was somehow'). Do not overdo it to the point of caricature, but make it undeniably Nigerian.
4. BANNED AI WORDS: You are strictly forbidden from using generic AI words like: "delve", "tapestry", "curate", "elevate", "seamless", "commendable", "pleasantly surprised".
5. OUTPUT: Output ONLY the raw review text. No intro, no markdown, no quotation marks.
</STRICT_RULES_FOR_MIMICRY>`,
		rating,
		formatStructuredProfile(profile), // Assuming you have this helper function
		formatStructuredProduct(product), // Assuming you have this helper function
		feedbackSection,
		avgWords, minWords, maxWords,
		profile.FormattingQuirks.CapitalizationStyle,
		profile.FormattingQuirks.PunctuationHabits,
		profile.ConsumerPersona,
	)

	return []llm.Message{
		{Role: "system", Content: systemInstruction},
		{Role: "user", Content: userPrompt},
	}
}
