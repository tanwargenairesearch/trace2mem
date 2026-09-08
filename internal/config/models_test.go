package config

import (
	"strings"
	"testing"
)

func TestExplicitIndependentModels(t *testing.T) {
	for _, s := range []string{"version: 1\n", "version: 1\ngeneration:\n  provider: ollama\n  model: local\nembedding:\n  provider: gemini\n"} {
		if _, e := ParseModels(strings.NewReader(s)); e == nil {
			t.Fatal("missing model accepted")
		}
	}
	c, e := ParseModels(strings.NewReader("version: 1\ngeneration:\n  provider: ollama\n  model: local-tool-model\n  endpoint: http://localhost:11434\nembedding:\n  provider: vertex\n  model: gemini-embedding-001\n  project: example-project\n  location: europe-west2\n"))
	if e != nil {
		t.Fatal(e)
	}
	if c.Provider != "ollama" || c.EmbeddingProvider != "vertex" {
		t.Fatal("model roles coupled")
	}
}

func TestGenerationRequestBudgets(t *testing.T) {
	c, err := ParseModels(strings.NewReader("version: 1\ngeneration:\n  provider: openai\n  model: explicit-model\n  request_timeout_seconds: 180\n  reasoning_effort: low\nembedding:\n  provider: vertex\n  model: explicit-embedding\nbudgets:\n  max_tokens: 96000\n  max_output_tokens: 8192\n"))
	if err != nil {
		t.Fatal(err)
	}
	if c.ReasoningEffort != "low" || c.MaxOutputTokens != 8192 || c.RequestTimeoutSeconds != 180 || c.GenerationOutputLimit() != 8192 {
		t.Fatal("request budget lost")
	}
}
