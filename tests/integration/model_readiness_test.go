package integration

import (
	"context"
	"github.com/tanwargenairesearch/trace2mem/internal/domain"
	"github.com/tanwargenairesearch/trace2mem/internal/store"
	"testing"
	"time"
)

func TestUnconfiguredMemoryResumesAfterConfiguration(t *testing.T) {
	s := database(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	p := domain.Principal{Tenant: "readiness-" + store.ID(), Subject: "user"}
	if err := s.EnsureMemory(ctx, p); err != nil {
		t.Fatal(err)
	}
	_, _, _, _, err := s.Append(ctx, p.Tenant, p.MemoryID(), []store.InputEvent{{ID: "e1", Session: "s1", Hash: "h", Occurred: time.Now(), JSON: []byte(`{"eventId":"e1","sessionId":"s1","occurredAt":"2026-09-07T12:00:00Z","actor":{"role":"user"},"source":{"id":"test"},"message":{"text":"Project: Go"}}`)}})
	if err != nil {
		t.Fatal(err)
	}
	wait := func(want string) {
		t.Helper()
		for {
			var status string
			var attempts int
			err := s.DB.QueryRow(ctx, "SELECT status,attempts FROM jobs WHERE tenant=$1 AND space=$2", p.Tenant, p.MemoryID()).Scan(&status, &attempts)
			if err != nil {
				t.Fatal(err)
			}
			if status == want {
				if want == "blocked" && attempts != 0 {
					t.Fatal("unconfigured provider was attempted")
				}
				return
			}
			select {
			case <-ctx.Done():
				t.Fatalf("expected %s, got %s", want, status)
			case <-time.After(100 * time.Millisecond):
			}
		}
	}
	if err = s.Schedule(ctx, p.Tenant, p.MemoryID()); err != nil {
		t.Fatal(err)
	}
	wait("blocked")
	config := domain.ModelConfig{Provider: "scripted", Model: "fixture", EmbeddingProvider: "scripted", EmbeddingModel: "fixture", MaxSteps: 12, MaxTokens: 32000, DailyTokens: 100000000}
	if err = s.SetConfig(ctx, p.Tenant, p.MemoryID(), config, nil); err != nil {
		t.Fatal(err)
	}
	wait("done")
}
