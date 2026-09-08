package model

import (
	"context"
	"encoding/json"
	"github.com/tanwargenairesearch/trace2mem/internal/domain"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGenerationLimitsAndIncompleteResponse(t *testing.T) {
	host := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
		}
		if body["reasoning"].(map[string]any)["effort"] != "low" {
			t.Error("reasoning effort missing")
		}
		if body["max_output_tokens"] != float64(8192) {
			t.Errorf("output limit %v", body["max_output_tokens"])
		}
		json.NewEncoder(w).Encode(map[string]any{"status": "incomplete", "usage": map[string]int{"input_tokens": 10, "output_tokens": 8192}, "output": []any{map[string]any{"type": "function_call", "call_id": "partial", "name": "write", "arguments": "{}"}}})
	}))
	defer host.Close()
	p := &HTTP{Config: domain.ModelConfig{Endpoint: host.URL, Model: "test", MaxTokens: 96000, MaxOutputTokens: 8192, ReasoningEffort: "low"}, Client: host.Client()}
	reply, err := p.Generate(context.Background(), nil, nil)
	if err == nil || !strings.Contains(err.Error(), "incomplete") || len(reply.Calls) != 0 {
		t.Fatal("incomplete calls must not execute", err)
	}
	if reply.Usage.Output != 8192 {
		t.Fatal("missing incomplete-response usage")
	}
}

func TestModelRequestBounds(t *testing.T) {
	cfg := domain.ModelConfig{Provider: "openai", Model: "test", EmbeddingProvider: "scripted", EmbeddingModel: "test"}
	for _, limit := range []int{-1, 32769} {
		cfg.MaxOutputTokens = limit
		if _, err := New(cfg); err == nil {
			t.Fatal("invalid output limit accepted")
		}
	}
	cfg.MaxOutputTokens = 8192
	cfg.RequestTimeoutSeconds = 301
	if _, err := New(cfg); err == nil {
		t.Fatal("invalid timeout accepted")
	}
	for _, tc := range []struct {
		cfg  domain.ModelConfig
		want int
	}{
		{domain.ModelConfig{}, 4096},
		{domain.ModelConfig{Provider: "ollama"}, 4096},
		{domain.ModelConfig{MaxOutputTokens: 8192, MaxTokens: 1000}, 1000},
		{domain.ModelConfig{MaxOutputTokens: 8192, MaxTokens: 96000}, 8192},
	} {
		if got := tc.cfg.GenerationOutputLimit(); got != tc.want {
			t.Errorf("limit=%d want=%d", got, tc.want)
		}
	}
}

func TestOllamaOmittedOutputLimitCompatibility(t *testing.T) {
	host := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
		}
		if body["options"].(map[string]any)["num_predict"] != float64(2048) {
			t.Error("changed omitted Ollama limit")
		}
		json.NewEncoder(w).Encode(map[string]any{"message": map[string]string{"content": "ok"}})
	}))
	defer host.Close()
	p := &HTTP{Config: domain.ModelConfig{Provider: "ollama", Endpoint: host.URL, MaxTokens: 1000}, Client: host.Client()}
	if _, err := p.Generate(context.Background(), nil, nil); err != nil {
		t.Fatal(err)
	}
}

func TestRequiredToolSelection(t *testing.T) {
	for _, provider := range []string{"openai", "ollama"} {
		t.Run(provider, func(t *testing.T) {
			host := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var body map[string]any
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Error(err)
				}
				if provider == "openai" {
					choice, ok := body["tool_choice"].(map[string]any)
					if !ok || choice["name"] != "verify" || choice["type"] != "function" {
						t.Error("missing forced tool choice")
					}
					json.NewEncoder(w).Encode(map[string]any{"status": "completed", "output": []any{}, "usage": map[string]int{"input_tokens": 10, "output_tokens": 3}})
				} else {
					json.NewEncoder(w).Encode(map[string]any{"message": map[string]string{"content": "prose only"}, "prompt_eval_count": 10, "eval_count": 3})
				}
			}))
			defer host.Close()
			p := &HTTP{Config: domain.ModelConfig{Provider: provider, Endpoint: host.URL}, Client: host.Client()}
			reply, err := p.Generate(context.Background(), nil, []Tool{{Name: "verify", Required: true, Parameters: Object(map[string]any{})}})
			if err == nil || len(reply.Calls) != 0 || reply.Usage.Total() != 13 {
				t.Fatal("required-tool failure or usage lost", reply, err)
			}
		})
	}
}
