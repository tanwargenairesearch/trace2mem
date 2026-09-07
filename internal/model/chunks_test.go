package model

import (
	"context"
	"strings"
	"testing"
	"unicode/utf8"
)

type checkingEmbedder struct{ calls int }

func (p *checkingEmbedder) Embed(_ context.Context, texts []string) ([][]float32, error) {
	p.calls++
	if len(texts) != 1 || len(texts[0]) > 1024 || !utf8.ValidString(texts[0]) {
		panic("unbounded or split UTF8")
	}
	return [][]float32{{3, 4}}, nil
}
func TestChunkedUTF8(t *testing.T) {
	p := &checkingEmbedder{}
	v, e := (chunked{p}).Embed(context.Background(), []string{strings.Repeat("界", 1500)})
	if e != nil || p.calls < 4 || len(v) != 1 || v[0][0] != 0.6 || v[0][1] != 0.8 {
		t.Fatalf("%v %v calls=%d", v, e, p.calls)
	}
}
