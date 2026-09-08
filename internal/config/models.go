package config

import (
	"errors"
	"github.com/trace2mem/trace2mem/internal/domain"
	"go.yaml.in/yaml/v3"
	"io"
	"os"
	"strings"
)

type ModelSelection struct {
	ReasoningEffort       string `yaml:"reasoning_effort"`
	RequestTimeoutSeconds int    `yaml:"request_timeout_seconds"`
	Provider              string `yaml:"provider"`
	Model                 string `yaml:"model"`
	Endpoint              string `yaml:"endpoint"`
	APIKeyEnv             string `yaml:"api_key_env"`
	Project               string `yaml:"project"`
	Location              string `yaml:"location"`
	Dimensions            int    `yaml:"dimensions"`
}
type ModelsFile struct {
	Version    int            `yaml:"version"`
	Generation ModelSelection `yaml:"generation"`
	Embedding  ModelSelection `yaml:"embedding"`
	Budgets    struct {
		MaxOutputTokens int   `yaml:"max_output_tokens"`
		MaxSteps        int   `yaml:"max_steps"`
		MaxTokens       int   `yaml:"max_tokens"`
		DailyTokens     int64 `yaml:"daily_tokens"`
	} `yaml:"budgets"`
}

func ParseModels(r io.Reader) (domain.ModelConfig, error) {
	var f ModelsFile
	decoder := yaml.NewDecoder(r)
	decoder.KnownFields(true)
	if e := decoder.Decode(&f); e != nil {
		return domain.ModelConfig{}, e
	}
	var extra any
	if e := decoder.Decode(&extra); e != io.EOF {
		return domain.ModelConfig{}, errors.New("one YAML document required")
	}
	if f.Embedding.ReasoningEffort != "" {
		return domain.ModelConfig{}, errors.New("reasoning_effort is supported only for generation")
	}
	if f.Embedding.RequestTimeoutSeconds != 0 {
		return domain.ModelConfig{}, errors.New("request_timeout_seconds is supported only for generation")
	}
	if f.Budgets.MaxOutputTokens < 0 || f.Budgets.MaxOutputTokens > 32768 || f.Generation.RequestTimeoutSeconds < 0 || f.Generation.RequestTimeoutSeconds > 300 {
		return domain.ModelConfig{}, errors.New("model output limit must be 0–32768 and timeout 0–300 seconds")
	}
	if f.Version != 1 {
		return domain.ModelConfig{}, errors.New("model config version must be 1")
	}
	for _, s := range []ModelSelection{f.Generation, f.Embedding} {
		if s.Provider == "" || s.Model == "" || strings.Contains(s.Model, "REPLACE_") || (s.Model == "latest" || strings.HasSuffix(s.Model, ":latest")) {
			return domain.ModelConfig{}, errors.New("each role requires an explicit provider and model")
		}
	}
	key := func(name string) (string, error) {
		if name == "" {
			return "", nil
		}
		v, ok := os.LookupEnv(name)
		if !ok || v == "" {
			return "", errors.New("configured API key environment variable is unset: " + name)
		}
		return v, nil
	}
	generation, e := key(f.Generation.APIKeyEnv)
	if e != nil {
		return domain.ModelConfig{}, e
	}
	embedding, e := key(f.Embedding.APIKeyEnv)
	if e != nil {
		return domain.ModelConfig{}, e
	}
	return domain.ModelConfig{ReasoningEffort: f.Generation.ReasoningEffort, MaxOutputTokens: f.Budgets.MaxOutputTokens, RequestTimeoutSeconds: f.Generation.RequestTimeoutSeconds, Provider: f.Generation.Provider, Model: f.Generation.Model, Endpoint: f.Generation.Endpoint, Key: generation, EmbeddingProvider: f.Embedding.Provider, EmbeddingModel: f.Embedding.Model, EmbeddingEndpoint: f.Embedding.Endpoint, EmbeddingKey: embedding, EmbeddingProject: f.Embedding.Project, EmbeddingLocation: f.Embedding.Location, EmbeddingDimensions: f.Embedding.Dimensions, MaxSteps: f.Budgets.MaxSteps, MaxTokens: f.Budgets.MaxTokens, DailyTokens: f.Budgets.DailyTokens}, nil
}
