package dream

import (
	"context"
	"github.com/tanwargenairesearch/trace2mem/internal/domain"
	"github.com/tanwargenairesearch/trace2mem/internal/model"
	"github.com/tanwargenairesearch/trace2mem/internal/store"
	"os"
	"sync"
	"testing"
)

func TestUsageIsScopedAndConcurrent(t *testing.T) {
	a, ca := WithUsage(context.Background())
	b, cb := WithUsage(context.Background())
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Go(func() {
			countUsage(a, false, domain.Usage{Input: 1})
			countUsage(b, true, domain.Usage{Input: 2, Estimated: true})
		})
	}
	wg.Wait()
	ag, ae := ca.Snapshot()
	bg, be := cb.Snapshot()
	if ag.Input != 20 || ae.Total() != 0 || bg.Total() != 0 || be.Input != 40 || !be.Estimated {
		t.Fatal(ag, ae, bg, be)
	}
}

type failedEmbedding struct{ model.Scripted }

func (failedEmbedding) Embed(context.Context, []string) ([][]float32, error) {
	return nil, context.DeadlineExceeded
}

func TestFailedEmbeddingRetainsScopedReservation(t *testing.T) {
	dsn := os.Getenv("TRACE2MEM_TEST_DATABASE")
	if dsn == "" {
		t.Skip("requires Docker database")
	}
	ctx, usage := WithUsage(context.Background())
	s, err := store.Open(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	p := domain.Principal{Tenant: "usage-" + store.ID(), Subject: "u"}
	if err := s.EnsureMemory(ctx, p); err != nil {
		t.Fatal(err)
	}
	e := Engine{Store: s}
	provider := e.Meter(failedEmbedding{}, domain.ModelConfig{DailyTokens: 100000}, p.Tenant, p.MemoryID())
	if _, err := provider.Embed(ctx, []string{"keyword fallback"}); err == nil {
		t.Fatal("expected timeout")
	}
	_, embed := usage.Snapshot()
	used, err := s.Used(ctx, p.Tenant, p.MemoryID())
	if err != nil || embed.Total() != used || used == 0 || !embed.Estimated || !embed.Unresolved {
		t.Fatal(embed, used, err)
	}
}

type incompleteGeneration struct{ model.Scripted }

func (incompleteGeneration) Generate(context.Context, []model.Turn, []model.Tool) (model.Reply, error) {
	return model.Reply{Usage: domain.Usage{Input: 42, Output: 8}}, context.DeadlineExceeded
}
func TestReportedFailureUsageReconcilesReservation(t *testing.T) {
	dsn := os.Getenv("TRACE2MEM_TEST_DATABASE")
	if dsn == "" {
		t.Skip("requires Docker database")
	}
	ctx, usage := WithUsage(context.Background())
	s, err := store.Open(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	p := domain.Principal{Tenant: "usage-" + store.ID(), Subject: "u"}
	if err := s.EnsureMemory(ctx, p); err != nil {
		t.Fatal(err)
	}
	engine := Engine{Store: s}
	provider := engine.Meter(incompleteGeneration{}, domain.ModelConfig{DailyTokens: 100000, MaxOutputTokens: 8192}, p.Tenant, p.MemoryID())
	if _, err := provider.Generate(ctx, nil, nil); err == nil {
		t.Fatal("expected incomplete generation error")
	}
	gen, _ := usage.Snapshot()
	used, err := s.Used(ctx, p.Tenant, p.MemoryID())
	if err != nil || used != 50 || gen.Total() != 50 || gen.Estimated || gen.Unresolved {
		t.Fatal(gen, used, err)
	}
}
