package integration

import (
	"connectrpc.com/connect"
	"context"
	"fmt"
	v1 "github.com/tanwargenairesearch/trace2mem/gen/trace2mem/v1"
	"github.com/tanwargenairesearch/trace2mem/internal/domain"
	"github.com/tanwargenairesearch/trace2mem/internal/store"
	"github.com/tanwargenairesearch/trace2mem/sdk"
	"os"
	"testing"
	"time"
)

func scheduleEvent(id string) store.InputEvent {
	raw := []byte(fmt.Sprintf(`{"eventId":%q,"sessionId":"s1","occurredAt":"2026-09-08T12:00:00Z","actor":{"role":"user"},"source":{"id":"schedule-test"},"message":{"text":%q}}`, id, "Project: "+id))
	return store.InputEvent{ID: id, Session: "s1", Hash: domain.Hash(raw), JSON: raw, Occurred: time.Now()}
}
func TestAutomaticQuietPeriodAndMaximumDelay(t *testing.T) {
	s := database(t)
	ctx := context.Background()
	p := domain.Principal{Tenant: "auto-" + store.ID(), Subject: "u"}
	if err := s.EnsureMemory(ctx, p); err != nil {
		t.Fatal(err)
	}
	if _, _, _, _, err := s.Append(ctx, p.Tenant, p.MemoryID(), []store.InputEvent{scheduleEvent("first")}); err != nil {
		t.Fatal(err)
	}
	var delay float64
	if err := s.DB.QueryRow(ctx, "SELECT extract(epoch FROM due_at-first_received) FROM compilation_requests WHERE tenant=$1 AND memory=$2", p.Tenant, p.MemoryID()).Scan(&delay); err != nil || delay != 60 {
		t.Fatal("quiet period", delay, err)
	}
	if _, err := s.DB.Exec(ctx, "UPDATE compilation_requests SET first_received=now()-interval '270 seconds' WHERE tenant=$1 AND memory=$2", p.Tenant, p.MemoryID()); err != nil {
		t.Fatal(err)
	}
	if _, _, _, _, err := s.Append(ctx, p.Tenant, p.MemoryID(), []store.InputEvent{scheduleEvent("second")}); err != nil {
		t.Fatal(err)
	}
	if err := s.DB.QueryRow(ctx, "SELECT extract(epoch FROM due_at-now()) FROM compilation_requests WHERE tenant=$1 AND memory=$2", p.Tenant, p.MemoryID()).Scan(&delay); err != nil || delay > 31 || delay < 20 {
		t.Fatal("continuous traffic postponed deadline", delay, err)
	}
}
func TestManualCompilationPinsAcceptedRange(t *testing.T) {
	s := database(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	p := domain.Principal{Tenant: "manual-" + store.ID(), Subject: "u"}
	if err := s.EnsureMemory(ctx, p); err != nil {
		t.Fatal(err)
	}
	if err := s.SetSchedule(ctx, p.Tenant, p.MemoryID(), store.CompilationSchedule{Mode: "manual"}); err != nil {
		t.Fatal(err)
	}
	_, _, first, _, err := s.Append(ctx, p.Tenant, p.MemoryID(), []store.InputEvent{scheduleEvent("first")})
	if err != nil {
		t.Fatal(err)
	}
	var count int
	if err = s.DB.QueryRow(ctx, "SELECT count(*) FROM jobs WHERE tenant=$1 AND space=$2", p.Tenant, p.MemoryID()).Scan(&count); err != nil || count != 0 {
		t.Fatal("manual ingestion queued compilation", count, err)
	}
	if err = s.Schedule(ctx, p.Tenant, p.MemoryID()); err != nil {
		t.Fatal(err)
	}
	if _, _, _, _, err = s.Append(ctx, p.Tenant, p.MemoryID(), []store.InputEvent{scheduleEvent("late")}); err != nil {
		t.Fatal(err)
	}
	// Exhausted work must resume on credential replacement without approving later evidence.
	if _, err = s.DB.Exec(ctx, "UPDATE jobs SET status='failed',attempts=5 WHERE tenant=$1 AND space=$2", p.Tenant, p.MemoryID()); err != nil {
		t.Fatal(err)
	}
	cfg := domain.ModelConfig{Provider: "scripted", Model: "fixture", EmbeddingProvider: "scripted", EmbeddingModel: "fixture"}
	if err = s.SetConfig(ctx, p.Tenant, p.MemoryID(), cfg, nil); err != nil {
		t.Fatal(err)
	}
	for {
		var processed int64
		var status string
		err = s.DB.QueryRow(ctx, "SELECT watermark,status FROM spaces s JOIN jobs j ON s.tenant=j.tenant AND s.id=j.space WHERE s.tenant=$1 AND s.id=$2", p.Tenant, p.MemoryID()).Scan(&processed, &status)
		if err != nil {
			t.Fatal(err)
		}
		if status == "done" {
			if processed != first {
				t.Fatal("manual run consumed late events", processed, first)
			}
			break
		}
		select {
		case <-ctx.Done():
			t.Fatal("manual compilation did not finish", status)
		case <-time.After(100 * time.Millisecond):
		}
	}
}

func TestCloseSessionReportsSchedulingMode(t *testing.T) {
	url := os.Getenv("TRACE2MEM_TEST_URL")
	if url == "" {
		t.Skip("requires Docker integration stack")
	}
	s := database(t)
	ctx := context.Background()
	for _, mode := range []string{"automatic", "daily", "manual"} {
		t.Run(mode, func(t *testing.T) {
			p := domain.Principal{Tenant: "close-" + store.ID(), Subject: "u", Scopes: map[string]bool{"read": true, "ingest": true, "manage": true}}
			if err := s.EnsureMemory(ctx, p); err != nil {
				t.Fatal(err)
			}
			if err := s.SetSchedule(ctx, p.Tenant, p.MemoryID(), store.CompilationSchedule{Mode: mode}); err != nil {
				t.Fatal(err)
			}
			token, err := s.CreateToken(ctx, p, []string{"read", "ingest"})
			if err != nil {
				t.Fatal(err)
			}
			res, err := sdk.New(url, token).Ingestion.CloseSession(ctx, connect.NewRequest(&v1.CloseSessionRequest{SessionId: "s1"}))
			if err != nil {
				t.Fatal(err)
			}
			if res.Msg.Scheduled != (mode != "manual") {
				t.Fatalf("scheduled=%v for %s", res.Msg.Scheduled, mode)
			}
		})
	}
}
func TestDailyCatchupCoalescesPendingEvidence(t *testing.T) {
	s := database(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	p := domain.Principal{Tenant: "daily-" + store.ID(), Subject: "u"}
	if err := s.EnsureMemory(ctx, p); err != nil {
		t.Fatal(err)
	}
	if err := s.SetSchedule(ctx, p.Tenant, p.MemoryID(), store.CompilationSchedule{Mode: "daily", Time: "02:00", Timezone: "UTC"}); err != nil {
		t.Fatal(err)
	}
	if _, _, _, _, err := s.Append(ctx, p.Tenant, p.MemoryID(), []store.InputEvent{scheduleEvent("first")}); err != nil {
		t.Fatal(err)
	}
	first, err := s.GetSchedule(ctx, p.Tenant, p.MemoryID())
	if err != nil {
		t.Fatal(err)
	}
	if _, _, _, _, err = s.Append(ctx, p.Tenant, p.MemoryID(), []store.InputEvent{scheduleEvent("second")}); err != nil {
		t.Fatal(err)
	}
	second, err := s.GetSchedule(ctx, p.Tenant, p.MemoryID())
	if err != nil || first.NextRun == nil || second.NextRun == nil || !first.NextRun.Equal(*second.NextRun) {
		t.Fatal("daily deadline moved", err)
	}
	if _, err = s.DB.Exec(ctx, "UPDATE compilation_requests SET due_at=now()-interval '2 days' WHERE tenant=$1 AND memory=$2", p.Tenant, p.MemoryID()); err != nil {
		t.Fatal(err)
	}
	for {
		var n int
		var status string
		err = s.DB.QueryRow(ctx, "SELECT (SELECT count(*) FROM compilation_requests WHERE tenant=$1 AND memory=$2),COALESCE((SELECT status FROM jobs WHERE tenant=$1 AND space=$2),'')", p.Tenant, p.MemoryID()).Scan(&n, &status)
		if err != nil {
			t.Fatal(err)
		}
		if n == 0 && status == "blocked" {
			break
		}
		select {
		case <-ctx.Done():
			t.Fatal("daily catchup not promoted", n, status)
		case <-time.After(100 * time.Millisecond):
		}
	}
}
