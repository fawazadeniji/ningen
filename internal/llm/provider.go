package llm

import (
	"context"

	"github.com/openai/openai-go"
)

// Message is a single chat turn exchanged with an LLM.
type Message struct {
	Role    string // "system" | "user" | "assistant"
	Content string
}

// CompletionOption customizes a single completion request.
type CompletionOption func(*completionConfig)

type completionConfig struct {
	responseFormat *openai.ChatCompletionNewParamsResponseFormatUnion
	modelOverride  string
}

// WithJSONSchemaResponse instructs the model to return structured JSON that
// matches the supplied schema.
func WithJSONSchemaResponse(name string, schema map[string]any) CompletionOption {
	return func(cfg *completionConfig) {
		cfg.responseFormat = &openai.ChatCompletionNewParamsResponseFormatUnion{
			OfJSONSchema: &openai.ResponseFormatJSONSchemaParam{
				JSONSchema: openai.ResponseFormatJSONSchemaJSONSchemaParam{
					Name:   name,
					Strict: openai.Bool(true),
					Schema: schema,
				},
			},
		}
	}
}

// WithModel allows overriding the provider model for a single completion call.
func WithModel(model string) CompletionOption {
	return func(cfg *completionConfig) {
		cfg.modelOverride = model
	}
}

// LLMProvider is the contract every LLM backend must satisfy.
// Implementations must be safe for concurrent use.
type LLMProvider interface {
	// Name returns the canonical identifier for this provider (e.g. "kimi").
	Name() string

	// Complete sends a conversation to the model and returns its reply.
	Complete(ctx context.Context, messages []Message, opts ...CompletionOption) (string, error)

	// Humanize post-processes rawText through a culturally-aware rewrite
	// that grounds the output in everyday Nigerian contexts.
	// userContext provides persona information to tailor the tone.
	Humanize(ctx context.Context, rawText string, userContext string) (string, error)
}

// humanizerSystemPrompt is injected as the system message for every Humanize call.
// It instructs the model to ground responses authentically in Nigerian everyday life
// without resorting to stereotype or caricature.
const humanizerSystemPrompt = `You are a world-class Nigerian content strategist fluent in the nuances
of Nigerian everyday life — from the hustle of Lagos markets and the serenity of Calabar evenings,
to NEPA outages, the jollof rice debate, and the warmth of community. Your sole task is to rewrite
the provided text so it feels like it was written by a knowledgeable, witty Nigerian who lives and
breathes this culture — never forced, never a cliché. Keep all facts and recommendations intact.
Do not add commentary about what you are doing; output only the rewritten text.`
