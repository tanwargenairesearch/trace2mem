package model

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/brainmemory/brain/internal/domain"
	"golang.org/x/oauth2/google"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Gemini supports API-key Gemini access and IAM-authenticated Vertex prediction.
type Gemini struct {
	Config domain.ModelConfig
	Client *http.Client
}

func (g *Gemini) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	c := g.Config
	if !domain.ValidID(c.EmbeddingModel) {
		return nil, errors.New("valid explicit Gemini model identifier required")
	}
	client := g.Client
	endpoint := strings.TrimRight(c.EmbeddingEndpoint, "/")
	if client == nil {
		if c.EmbeddingProvider == "vertex" {
			if !domain.ValidID(c.EmbeddingProject) || !domain.ValidID(c.EmbeddingLocation) {
				return nil, errors.New("Vertex project and location required")
			}
			var e error
			client, e = google.DefaultClient(ctx, "https://www.googleapis.com/auth/cloud-platform")
			if e != nil {
				return nil, e
			}
			client.Timeout = 90 * time.Second
		} else {
			client = &http.Client{Timeout: 90 * time.Second}
		}
	}
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return errors.New("embedding redirects disabled") }
	if endpoint == "" {
		if c.EmbeddingProvider == "vertex" {
			host := c.EmbeddingLocation + "-aiplatform.googleapis.com"
			if c.EmbeddingLocation == "global" {
				host = "aiplatform.googleapis.com"
			}
			endpoint = "https://" + host + "/v1/projects/" + url.PathEscape(c.EmbeddingProject) + "/locations/" + url.PathEscape(c.EmbeddingLocation) + "/publishers/google/models"
		} else {
			endpoint = "https://generativelanguage.googleapis.com/v1beta/models"
		}
	}
	out := make([][]float32, 0, len(texts))
	for _, text := range texts {
		var body any
		path := "/" + c.EmbeddingModel + ":embedContent"
		if c.EmbeddingProvider == "vertex" {
			path = "/" + c.EmbeddingModel + ":predict"
			params := map[string]any{"autoTruncate": false}
			if c.EmbeddingDimensions > 0 {
				params["outputDimensionality"] = c.EmbeddingDimensions
			}
			body = map[string]any{"instances": []any{map[string]any{"content": text}}, "parameters": params}
		} else {
			b := map[string]any{"model": "models/" + c.EmbeddingModel, "content": map[string]any{"parts": []any{map[string]string{"text": text}}}}
			if c.EmbeddingDimensions > 0 {
				b["outputDimensionality"] = c.EmbeddingDimensions
			}
			body = b
		}
		raw, e := json.Marshal(body)
		if e != nil {
			return nil, e
		}
		var payload []byte
		for attempt := 0; attempt < 3; attempt++ {
			req, e := http.NewRequestWithContext(ctx, "POST", endpoint+path, bytes.NewReader(raw))
			if e != nil {
				return nil, e
			}
			req.Header.Set("Content-Type", "application/json")
			if c.EmbeddingProvider == "gemini" {
				if c.EmbeddingKey == "" {
					return nil, errors.New("Gemini API key required")
				}
				req.Header.Set("x-goog-api-key", c.EmbeddingKey)
			}
			res, e := client.Do(req)
			if e != nil {
				return nil, e
			}
			payload, e = io.ReadAll(io.LimitReader(res.Body, 1<<20))
			res.Body.Close()
			if e != nil {
				return nil, e
			}
			if res.StatusCode == 429 || res.StatusCode >= 500 {
				if attempt < 2 {
					timer := time.NewTimer(time.Duration(attempt+1) * time.Second)
					select {
					case <-ctx.Done():
						timer.Stop()
						return nil, ctx.Err()
					case <-timer.C:
						continue
					}
				}
			}
			if res.StatusCode/100 != 2 {
				return nil, fmt.Errorf("embedding provider returned HTTP %d", res.StatusCode)
			}
			break
		}
		var r struct {
			Embedding struct {
				Values []float32 `json:"values"`
			} `json:"embedding"`
			Predictions []struct {
				Embeddings struct {
					Values []float32 `json:"values"`
				} `json:"embeddings"`
			} `json:"predictions"`
		}
		if e = json.Unmarshal(payload, &r); e != nil {
			return nil, e
		}
		v := r.Embedding.Values
		if c.EmbeddingProvider == "vertex" && len(r.Predictions) == 1 {
			v = r.Predictions[0].Embeddings.Values
		}
		if len(v) == 0 || (c.EmbeddingDimensions > 0 && len(v) != c.EmbeddingDimensions) {
			return nil, errors.New("embedding dimensions mismatch")
		}
		out = append(out, v)
	}
	return out, nil
}
