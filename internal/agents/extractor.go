package agents

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"ningen/internal/llm"
	"ningen/internal/models"
)

const extractorSystem = `You are a precision signal extraction engine for a recommendation system.
Your sole output is a single JSON object — no markdown, no explanation, no text outside the JSON.

Extract the user's recommendation intent from their persona and conversation history.
Produce exactly this JSON structure:
{
  "intent": "<one concise phrase describing what they want>",
  "domain": "<one of: books | food | products | mixed>",
  "search_queries": ["<focused search phrase 1>", "<focused search phrase 2>"],
  "mood": "<one word emotional register: adventurous, cozy, curious, urgent, nostalgic, etc.>",
  "constraints": ["<hard filter 1>", "<hard filter 2>"],
  "clarify_needed": false,
  "clarify_reason": ""
}

Rules:
- search_queries must be exactly 2 specific natural-language phrases suited for semantic search over
  review text. Make them diverse: one literal/specific, one thematic/conceptual. Do NOT copy the
  user's exact words — distill their underlying intent.
- If corpus examples are provided, calibrate search_queries to match the vocabulary and style of
  what is actually available — do not generate queries for items unlikely to exist in that corpus.
- If the history is too ambiguous to produce useful search queries, set clarify_needed=true and write
  a single open clarifying question in clarify_reason. Leave search_queries as [].
- constraints captures hard requirements: dietary restrictions, genre exclusions, budget signals, etc.
- Output valid JSON only. No trailing commas. No comments.`

// CorpusExample is a representative item sampled from the live corpus before extraction.
// Feeding these to the Extractor grounds its search_queries in what actually exists.
type CorpusExample struct {
	Domain     string
	SearchText string
}

// Extractor is Stage 1 of the SIGNAL pipeline.
// It converts raw persona + history into a structured UserSignal.
// When examples are provided (from a pre-search), it calibrates queries to the live corpus.
type Extractor struct {
	provider llm.LLMProvider
}

func NewExtractor(p llm.LLMProvider) *Extractor {
	return &Extractor{provider: p}
}

// Extract produces a UserSignal from the user's persona and conversation history.
// examples should be a small sample of items retrieved from a raw embedding pre-search;
// pass nil to skip corpus grounding (e.g. in tests or when embedder is unavailable).
func (e *Extractor) Extract(ctx context.Context, persona string, history []models.ConversationTurn, examples []CorpusExample) (*models.UserSignal, error) {
	messages := []llm.Message{
		{Role: "system", Content: extractorSystem},
		{Role: "user", Content: buildExtractionPrompt(persona, history, examples)},
	}

	raw, err := e.provider.Complete(ctx, messages)
	if err != nil {
		return nil, fmt.Errorf("extractor LLM: %w", err)
	}

	var signal models.UserSignal
	if err := json.Unmarshal([]byte(cleanJSON(raw)), &signal); err != nil {
		return nil, fmt.Errorf("extractor parse: %w (raw: %.300s)", err, raw)
	}

	if len(signal.SearchQueries) == 0 && !signal.ClarifyNeeded {
		signal.SearchQueries = []string{signal.Intent}
	}

	return &signal, nil
}

func buildExtractionPrompt(persona string, history []models.ConversationTurn, examples []CorpusExample) string {
	var sb strings.Builder
	sb.WriteString("Persona: ")
	sb.WriteString(persona)
	sb.WriteString("\n\nConversation history:\n")
	for _, t := range history {
		sb.WriteString(t.Role)
		sb.WriteString(": ")
		sb.WriteString(t.Content)
		sb.WriteByte('\n')
	}

	if len(examples) > 0 {
		sb.WriteString("\nExample items currently available in the corpus (calibrate your search_queries to match this vocabulary and content style):\n")
		for i, ex := range examples {
			text := ex.SearchText
			if len(text) > 150 {
				text = text[:150]
			}
			fmt.Fprintf(&sb, "%d. [%s] %s\n", i+1, ex.Domain, text)
		}
	}

	return sb.String()
}

// cleanJSON strips markdown code fences that some LLMs wrap around JSON responses.
func cleanJSON(s string) string {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "```") {
		leadEnd := strings.Index(s, "\n")
		if leadEnd != -1 {
			s = s[leadEnd+1:]
		}
		// Only strip trailing fence if it is genuinely distinct from the opening line.
		if i := strings.LastIndex(s, "```"); i > 0 {
			s = s[:i]
		}
	}
	return strings.TrimSpace(s)
}
