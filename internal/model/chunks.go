package model

import (
	"context"
	"errors"
	"math"
	"unicode/utf8"
)

// chunked bounds each provider input and pools long pages into one searchable vector.
// The algorithm version is part of the index identity; it must change with this policy.
type chunked struct{ Embedder }

func (p chunked) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	out := make([][]float32, 0, len(texts))
	for _, text := range texts {
		var sum []float64
		for first := true; first || len(text) > 0; first = false {
			n := min(len(text), 1024)
			for n < len(text) && n > 0 && !utf8.RuneStart(text[n]) {
				n--
			}
			part := text[:n]
			text = text[n:]
			vectors, err := p.Embedder.Embed(ctx, []string{part})
			if err != nil {
				return nil, err
			}
			if len(vectors) != 1 || len(vectors[0]) == 0 || len(vectors[0]) > 16000 {
				return nil, errors.New("invalid embedding shape")
			}
			v := vectors[0]
			if sum == nil {
				sum = make([]float64, len(v))
			}
			if len(v) != len(sum) {
				return nil, errors.New("embedding dimensions changed within input")
			}
			for i, x := range v {
				if math.IsNaN(float64(x)) || math.IsInf(float64(x), 0) {
					return nil, errors.New("non-finite embedding")
				}
				sum[i] += float64(x) * float64(max(n, 1))
			}
		}
		var norm float64
		for _, x := range sum {
			norm += x * x
		}
		norm = math.Sqrt(norm)
		if norm == 0 {
			return nil, errors.New("zero embedding vector")
		}
		v := make([]float32, len(sum))
		for i, x := range sum {
			v[i] = float32(x / norm)
		}
		out = append(out, v)
	}
	return out, nil
}
