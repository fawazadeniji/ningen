package llm

import "testing"

func TestNewGenericOpenAIClient_RequiresModel(t *testing.T) {
	_, err := NewGenericOpenAIClient(OpenAIConfig{
		Name:    "openai",
		BaseURL: "https://api.openai.com/v1/chat/completions",
		APIKey:  "test-key",
	})
	if err == nil {
		t.Fatal("expected error when model is empty")
	}
}

func TestNewGenericOpenAIClient_AllowsConfiguredModel(t *testing.T) {
	client, err := NewGenericOpenAIClient(OpenAIConfig{
		Name:    "openai",
		BaseURL: "https://api.openai.com/v1/chat/completions",
		APIKey:  "test-key",
		Model:   "gpt-4o-mini",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if client == nil {
		t.Fatal("expected client")
	}
}
