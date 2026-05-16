package llm

import (
	"fmt"
	"os"
)

// Registry holds all successfully initialised LLM providers keyed by their name.
// Providers whose API key is absent at startup are silently omitted.
type Registry map[string]LLMProvider

// Build initialises every provider whose required env var is present.
// At least one provider must be available or an error is returned.
func Build() (Registry, error) {
	reg := make(Registry)

	// 1. Kimi (Moonshot)
	if key := os.Getenv("MOONSHOT_API_KEY"); key != "" {
		reg["kimi"] = NewGenericOpenAIClient(OpenAIConfig{
			Name:    "kimi",
			BaseURL: "https://api.moonshot.ai/v1/chat/completions",
			APIKey:  key,
			Model:   "kimi-k2.6", // 2026 Flagship model
		})
	}

	// 2. Gemini (OpenAI Compatibility Layer)
	if key := os.Getenv("GEMINI_API_KEY"); key != "" {
		model := os.Getenv("GEMINI_MODEL")
		if model == "" {
			model = "gemini-1.5-flash"
		}
		reg["gemini"] = NewGenericOpenAIClient(OpenAIConfig{
			Name:    "gemini",
			BaseURL: "https://generativelanguage.googleapis.com/v1beta/openai/chat/completions",
			APIKey:  key,
			Model:   model,
		})
	}

	// 3. OpenAI / Azure OpenAI
	azureURL := os.Getenv("AZURE_OPENAI_URL")
	azureKey := os.Getenv("AZURE_OPENAI_KEY")
	if azureURL != "" && azureKey != "" {
		reg["openai"] = NewGenericOpenAIClient(OpenAIConfig{
			Name:    "openai",
			BaseURL: azureURL,
			APIKey:  azureKey,
			IsAzure: true,
			Model:   os.Getenv("OPENAI_MODEL"),
		})
	} else if key := os.Getenv("OPENAI_API_KEY"); key != "" {
		model := os.Getenv("OPENAI_MODEL")
		if model == "" {
			model = "gpt-4o-mini"
		}
		reg["openai"] = NewGenericOpenAIClient(OpenAIConfig{
			Name:    "openai",
			BaseURL: "https://api.openai.com/v1/chat/completions",
			APIKey:  key,
			Model:   model,
		})
	}

	if len(reg) == 0 {
		return nil, fmt.Errorf("no LLM providers available: set MOONSHOT_API_KEY, GEMINI_API_KEY, or OPENAI_API_KEY")
	}

	return reg, nil
}

// Get returns the provider by name, or the first available provider as a fallback.
func (r Registry) Get(name string) (LLMProvider, error) {
	if p, ok := r[name]; ok {
		return p, nil
	}

	// Fallback to any available provider
	for _, p := range r {
		return p, nil
	}

	return nil, fmt.Errorf("no LLM providers registered")
}

