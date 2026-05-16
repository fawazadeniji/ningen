package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// OpenAIConfig holds the details for any OpenAI-compatible provider (Kimi, Gemini, Azure, OpenAI).
type OpenAIConfig struct {
	Name        string
	BaseURL     string
	APIKey      string
	Model       string
	IsAzure     bool
	ExtraHeader map[string]string
}

type GenericOpenAIClient struct {
	cfg  OpenAIConfig
	http *http.Client
}

func NewGenericOpenAIClient(cfg OpenAIConfig) *GenericOpenAIClient {
	return &GenericOpenAIClient{
		cfg: cfg,
		http: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

func (c *GenericOpenAIClient) Name() string { return c.cfg.Name }

func (c *GenericOpenAIClient) Complete(ctx context.Context, messages []Message) (string, error) {
	return c.call(ctx, messages)
}

func (c *GenericOpenAIClient) Humanize(ctx context.Context, rawText string, userContext string) (string, error) {
	messages := []Message{
		{Role: "system", Content: humanizerSystemPrompt},
		{Role: "user", Content: fmt.Sprintf("User context: %s\n\nText to rewrite:\n%s", userContext, rawText)},
	}
	return c.call(ctx, messages)
}

type oaiMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type oaiRequest struct {
	Model    string       `json:"model,omitempty"`
	Messages []oaiMessage `json:"messages"`
}

type oaiResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

func (c *GenericOpenAIClient) call(ctx context.Context, messages []Message) (string, error) {
	wire := make([]oaiMessage, len(messages))
	for i, m := range messages {
		wire[i] = oaiMessage{Role: m.Role, Content: m.Content}
	}

	// For Azure, model is usually part of the URL, so we omit it from the body
	model := c.cfg.Model
	if c.cfg.IsAzure {
		model = ""
	}

	body, err := json.Marshal(oaiRequest{Model: model, Messages: wire})
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.cfg.BaseURL, bytes.NewReader(body))
	if err != nil {
		return "", err
	}

	req.Header.Set("Content-Type", "application/json")
	
	// Handle Auth
	if c.cfg.IsAzure {
		req.Header.Set("api-key", c.cfg.APIKey)
	} else {
		req.Header.Set("Authorization", "Bearer "+c.cfg.APIKey)
	}

	// Add any extra headers (useful for future-proofing)
	for k, v := range c.cfg.ExtraHeader {
		req.Header.Set(k, v)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var result oaiResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("decode error: %w", err)
	}

	if result.Error != nil {
		return "", fmt.Errorf("api error: %s", result.Error.Message)
	}

	if len(result.Choices) == 0 {
		return "", fmt.Errorf("empty response")
	}

	return result.Choices[0].Message.Content, nil
}
