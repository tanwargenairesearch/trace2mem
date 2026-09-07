package live

import (
	"context"
	"github.com/brainmemory/brain/internal/domain"
	"github.com/brainmemory/brain/internal/model"
	"os"
	"testing"
	"time"
)

func TestProviderCapabilities(t *testing.T) {
	provider := os.Getenv("BRAIN_LIVE_PROVIDER")
	if provider == "" {
		t.Skip("opt-in real model test")
	}
	generation, embedding := os.Getenv("BRAIN_LIVE_MODEL"), os.Getenv("BRAIN_LIVE_EMBEDDING_MODEL")
	if generation == "" || embedding == "" {
		t.Fatal("explicit BRAIN_LIVE_MODEL and BRAIN_LIVE_EMBEDDING_MODEL required")
	}
	p, e := model.New(domain.ModelConfig{Provider: provider, EmbeddingProvider:os.Getenv("BRAIN_LIVE_EMBEDDING_PROVIDER"),EmbeddingEndpoint:os.Getenv("BRAIN_LIVE_EMBEDDING_ENDPOINT"),EmbeddingKey:os.Getenv("BRAIN_LIVE_EMBEDDING_KEY"), Model: generation, EmbeddingModel: embedding, Endpoint: os.Getenv("BRAIN_LIVE_ENDPOINT"), Key: os.Getenv("OPENAI_API_KEY")})
	if e != nil {
		t.Fatal(e)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	if e = model.Probe(ctx, p); e != nil {
		t.Fatal(e)
	}
}
