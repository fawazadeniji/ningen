package llm

import (
	"context"
	"fmt"

	"github.com/openai/openai-go"
	"github.com/openai/openai-go/azure"
	"github.com/openai/openai-go/option"
)

// OpenAIConfig holds the details for any OpenAI-compatible provider (Kimi, Gemini, Azure, OpenAI).
type OpenAIConfig struct {
	Name            string
	BaseURL         string
	APIKey          string
	Model           string
	IsAzure         bool
	AzureEndpoint   string
	AzureAPIVersion string
	ExtraHeader     map[string]string
}

type GenericOpenAIClient struct {
	cfg    OpenAIConfig
	client openai.Client
}

func NewGenericOpenAIClient(cfg OpenAIConfig) (*GenericOpenAIClient, error) {
	opts := make([]option.RequestOption, 0, 4)

	if cfg.Model == "" {
		return nil, fmt.Errorf("model is required")
	}

	if cfg.IsAzure {
		if cfg.AzureEndpoint == "" {
			return nil, fmt.Errorf("azure endpoint is required")
		}
		if cfg.AzureAPIVersion == "" {
			cfg.AzureAPIVersion = "2024-06-01"
		}
		opts = append(opts, azure.WithEndpoint(cfg.AzureEndpoint, cfg.AzureAPIVersion))
		opts = append(opts, azure.WithAPIKey(cfg.APIKey))
	} else {
		if cfg.BaseURL != "" {
			opts = append(opts, option.WithBaseURL(cfg.BaseURL))
		}
		opts = append(opts, option.WithAPIKey(cfg.APIKey))
	}

	for key, value := range cfg.ExtraHeader {
		opts = append(opts, option.WithHeader(key, value))
	}

	client := openai.NewClient(opts...)
	return &GenericOpenAIClient{cfg: cfg, client: client}, nil
}

func (c *GenericOpenAIClient) Name() string { return c.cfg.Name }

func (c *GenericOpenAIClient) Complete(ctx context.Context, messages []Message, opts ...CompletionOption) (string, error) {
	return c.call(ctx, messages, opts...)
}

func (c *GenericOpenAIClient) Humanize(ctx context.Context, rawText string, userContext string) (string, error) {
	messages := []Message{
		{Role: "system", Content: humanizerSystemPrompt},
		{Role: "user", Content: fmt.Sprintf("User context: %s\n\nText to rewrite:\n%s", userContext, rawText)},
	}
	return c.call(ctx, messages)
}

func (c *GenericOpenAIClient) call(ctx context.Context, messages []Message, opts ...CompletionOption) (string, error) {
	config := completionConfig{}
	for _, opt := range opts {
		opt(&config)
	}

	chatMessages := make([]openai.ChatCompletionMessageParamUnion, 0, len(messages))
	for _, message := range messages {
		chatMessages = append(chatMessages, toChatMessage(message))
	}

	params := openai.ChatCompletionNewParams{
		Messages: chatMessages,
		Model:    openai.ChatModel(c.cfg.Model),
	}
	if config.responseFormat != nil {
		params.ResponseFormat = *config.responseFormat
	}

	response, err := c.client.Chat.Completions.New(ctx, params)
	if err != nil {
		return "", err
	}

	if len(response.Choices) == 0 {
		return "", fmt.Errorf("empty response")
	}

	content := response.Choices[0].Message.Content
	if content == "" {
		return "", fmt.Errorf("empty response content")
	}

	return content, nil
}

func toChatMessage(message Message) openai.ChatCompletionMessageParamUnion {
	switch message.Role {
	case "system":
		return openai.SystemMessage(message.Content)
	case "assistant":
		return openai.AssistantMessage(message.Content)
	default:
		return openai.UserMessage(message.Content)
	}
}
