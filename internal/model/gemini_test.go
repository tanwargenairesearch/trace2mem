package model

import (
	"context"
	"encoding/json"
	"github.com/tanwargenairesearch/trace2mem/internal/domain"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGeminiEmbeddingWire(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("x-goog-api-key") != "test-key" {
			t.Error("missing Gemini key header")
		}
		if r.URL.Path != "/gemini-embedding-001:embedContent" {
			t.Error(r.URL.Path)
		}
		var b map[string]any
		if e := json.NewDecoder(r.Body).Decode(&b); e != nil {
			t.Fatal(e)
		}
		if b["outputDimensionality"] != float64(3) {
			t.Error("dimension config missing")
		}
		w.Write([]byte(`{"embedding":{"values":[1,2,3]}}`))
	}))
	defer srv.Close()
	g := &Gemini{Config: domain.ModelConfig{EmbeddingProvider: "gemini", EmbeddingModel: "gemini-embedding-001", EmbeddingEndpoint: srv.URL, EmbeddingKey: "test-key", EmbeddingDimensions: 3}, Client: srv.Client()}
	v, e := g.Embed(context.Background(), []string{"text"})
	if e != nil || len(v) != 1 || len(v[0]) != 3 {
		t.Fatalf("%v %v", v, e)
	}
}
