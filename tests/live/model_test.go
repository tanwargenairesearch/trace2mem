package live

import (
	"context"
	"github.com/brainmemory/brain/internal/config"
	"github.com/brainmemory/brain/internal/domain"
	"github.com/brainmemory/brain/internal/model"
	"os"
	"testing"
	"time"
)

func TestProviderCapabilities(t *testing.T) {
	provider := os.Getenv("BRAIN_LIVE_PROVIDER")
	if provider == "" && os.Getenv("BRAIN_LIVE_CONFIG") == "" {
		t.Skip("opt-in real model test")
	}
	var c domain.ModelConfig
	if path := os.Getenv("BRAIN_LIVE_CONFIG"); path != "" {
		f, e := os.Open(path)
		if e != nil {
			t.Fatal(e)
		}
		defer f.Close()
		c, e = config.ParseModels(f)
		if e != nil {
			t.Fatal(e)
		}
	} else {
		c = domain.ModelConfig{Provider: provider, Model: os.Getenv("BRAIN_LIVE_MODEL"), EmbeddingProvider: os.Getenv("BRAIN_LIVE_EMBEDDING_PROVIDER"), EmbeddingModel: os.Getenv("BRAIN_LIVE_EMBEDDING_MODEL"), EmbeddingEndpoint: os.Getenv("BRAIN_LIVE_EMBEDDING_ENDPOINT"), EmbeddingKey: os.Getenv("BRAIN_LIVE_EMBEDDING_KEY"), EmbeddingProject: os.Getenv("BRAIN_LIVE_EMBEDDING_PROJECT"), EmbeddingLocation: os.Getenv("BRAIN_LIVE_EMBEDDING_LOCATION"), Endpoint: os.Getenv("BRAIN_LIVE_ENDPOINT"), Key: os.Getenv("OPENAI_API_KEY")}
	}
	p, e := model.New(c)
	if e != nil {
		t.Fatal(e)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	if e = model.Probe(ctx, p); e != nil {
		t.Fatal(e)
	}
}
