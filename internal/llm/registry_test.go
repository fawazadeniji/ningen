package llm

import (
	"strings"
	"testing"
)

func TestBuild_FailsWhenAzureModelIsMissing(t *testing.T) {
	t.Setenv("MOONSHOT_API_KEY", "")
	t.Setenv("GEMINI_API_KEY", "")
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("AZURE_OPENAI_URL", "https://example.openai.azure.com")
	t.Setenv("AZURE_OPENAI_KEY", "test-key")
	t.Setenv("AZURE_OPENAI_MODEL", "")

	_, err := Build()
	if err == nil {
		t.Fatal("expected Build to fail when AZURE_OPENAI_MODEL is missing")
	}
	if !strings.Contains(err.Error(), "AZURE_OPENAI_MODEL is required") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestBuild_NoProvidersErrorMentionsAzureVars(t *testing.T) {
	t.Setenv("MOONSHOT_API_KEY", "")
	t.Setenv("GEMINI_API_KEY", "")
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("AZURE_OPENAI_URL", "")
	t.Setenv("AZURE_OPENAI_KEY", "")
	t.Setenv("AZURE_OPENAI_MODEL", "")

	_, err := Build()
	if err == nil {
		t.Fatal("expected Build to fail when no providers are configured")
	}
	if !strings.Contains(err.Error(), "AZURE_OPENAI_URL") || !strings.Contains(err.Error(), "AZURE_OPENAI_MODEL") {
		t.Fatalf("error should mention Azure env vars, got: %v", err)
	}
}
