package integration

import (
	"context"
	"crypto/sha256"
	"encoding/hex"

	"connectrpc.com/connect"
	"errors"
	"fmt"
	trace2memv1 "github.com/trace2mem/trace2mem/gen/trace2mem/v1"
	"github.com/trace2mem/trace2mem/internal/domain"
	"github.com/trace2mem/trace2mem/internal/store"
	"github.com/trace2mem/trace2mem/sdk"
	"google.golang.org/protobuf/types/known/timestamppb"
	"os"
	"strings"
	"sync"
	"testing"
	"time"
)

func database(t *testing.T) *store.Store {
	t.Helper()
	url := os.Getenv("TRACE2MEM_TEST_DATABASE")
	if url == "" {
		t.Skip("requires Docker database")
	}
	s, e := store.Open(context.Background(), url)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(s.Close)
	return s
}
func TestDeletionFencesProposalsAndPublication(t *testing.T) {
	s := database(t)
	ctx := context.Background()
	tenant := "regression-" + store.ID()
	p := domain.Principal{Tenant: tenant, Subject: "owner", Admin: true}
	sp, e := s.CreateSpace(ctx, p, "fencing")
	if e != nil {
		t.Fatal(e)
	}
	_, _, _, e = s.Append(ctx, tenant, sp, []store.InputEvent{{ID: "e1", Session: "s1", Hash: "hash", JSON: []byte(`{"message":{"text":"secret"}}`), Occurred: time.Now()}})
	if e != nil {
		t.Fatal(e)
	}
	l := &domain.Lease{Tenant: tenant, Space: sp}
	e = s.DB.QueryRow(ctx, "UPDATE jobs SET status='running',fence=fence+1,lease_until=now()+interval '90 seconds' WHERE tenant=$1 AND space=$2 RETURNING fence", tenant, sp).Scan(&l.Fence)
	if e != nil {
		t.Fatal(e)
	}
	e = s.DB.QueryRow(ctx, "SELECT revision,generation,watermark FROM spaces WHERE tenant=$1 AND id=$2", tenant, sp).Scan(&l.Parent, &l.Epoch, &l.Watermark)
	if e != nil {
		t.Fatal(e)
	}
	if e = s.Forget(ctx, tenant, sp, "e1"); e != nil {
		t.Fatal(e)
	}
	if _, e = s.Proposal(ctx, *l, []string{"secret"}, "validated", true); !errors.Is(e, domain.ErrLease) {
		t.Fatal("stale proposal retained", e)
	}
	if _, e = s.Publish(ctx, *l, []domain.Page{{Path: "knowledge/index.md", Content: "secret"}}, true, ""); !errors.Is(e, domain.ErrLease) {
		t.Fatal("stale publication accepted", e)
	}
	var n int
	if e = s.DB.QueryRow(ctx, "SELECT count(*) FROM proposals WHERE tenant=$1", tenant).Scan(&n); e != nil || n != 0 {
		t.Fatal("forgotten content retained", n, e)
	}
	if _, e = s.DB.Exec(ctx, "DELETE FROM jobs WHERE tenant=$1", tenant); e != nil {
		t.Fatal(e)
	}
}
func TestConcurrentBudgetReservation(t *testing.T) {
	s := database(t)
	ctx := context.Background()
	tenant := "budget-" + store.ID()
	sp, e := s.CreateSpace(ctx, domain.Principal{Tenant: tenant, Subject: "owner", Admin: true}, "budget")
	if e != nil {
		t.Fatal(e)
	}
	var wg sync.WaitGroup
	results := make(chan error, 20)
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); _, e := s.Reserve(ctx, tenant, sp, "test", 10, 50); results <- e }()
	}
	wg.Wait()
	close(results)
	accepted := 0
	for e := range results {
		if e == nil {
			accepted++
		} else if !errors.Is(e, store.ErrBudget) {
			t.Fatal(e)
		}
	}
	if accepted != 5 {
		t.Fatalf("budget overspent: %d reservations", accepted)
	}
}
func TestValidTokenIsolation(t *testing.T) {
	s := database(t)
	ctx := context.Background()
	url := os.Getenv("TRACE2MEM_TEST_URL")
	if url == "" {
		t.Skip("server required")
	}
	tenant := "isolation-" + store.ID()
	owner := domain.Principal{Tenant: tenant, Subject: "owner", Admin: true}
	sp, e := s.CreateSpace(ctx, owner, "private")
	if e != nil {
		t.Fatal(e)
	}
	other, e := s.CreateSpace(ctx, owner, "other")
	if e != nil {
		t.Fatal(e)
	}
	for _, role := range []string{"reader", "editor"} {
		_, e = s.DB.Exec(ctx, "INSERT INTO members VALUES($1,$2,$3,$4)", tenant, sp, role, role)
		if e != nil {
			t.Fatal(e)
		}
		token := "token-" + store.ID()
		h := sha256.Sum256([]byte(token))
		_, e = s.DB.Exec(ctx, "INSERT INTO tokens VALUES($1,$2,$3,$4,$5,false)", hex.EncodeToString(h[:]), tenant, role, []string{"read", "write"}, time.Now().Add(time.Hour))
		if e != nil {
			t.Fatal(e)
		}
		c := sdk.New(url, token)
		if _, e = c.Memory.GetManifest(ctx, connect.NewRequest(&trace2memv1.GetManifestRequest{SpaceId: sp})); e != nil {
			t.Fatal("authorized read rejected", e)
		}
		if _, e = c.Memory.GetManifest(ctx, connect.NewRequest(&trace2memv1.GetManifestRequest{SpaceId: other})); connect.CodeOf(e) != connect.CodePermissionDenied {
			t.Fatal("cross-space access allowed", role, e)
		}
		_, e = c.Ingestion.RequestCompilation(ctx, connect.NewRequest(&trace2memv1.RequestCompilationRequest{SpaceId: sp}))
		if role == "reader" && connect.CodeOf(e) != connect.CodePermissionDenied {
			t.Fatal("reader write allowed", e)
		}
		if role == "editor" && e != nil {
			t.Fatal("editor write rejected", e)
		}
	}
	s.DB.Exec(ctx, "DELETE FROM jobs WHERE tenant=$1", tenant)
}
func TestIncrementalHistoryBeyondPromptLimit(t *testing.T) {
	s := database(t)
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	tenant := "large-" + store.ID()
	sp, e := s.CreateSpace(ctx, domain.Principal{Tenant: tenant, Subject: "owner", Admin: true}, "large history")
	if e != nil {
		t.Fatal(e)
	}
	cfg := domain.ModelConfig{Provider: "scripted", Model: "fixture", EmbeddingProvider: "scripted", EmbeddingModel: "fixture", MaxSteps: 12, MaxTokens: 32000, DailyTokens: 100000000}
	if e = s.SetConfig(ctx, tenant, sp, cfg, nil); e != nil {
		t.Fatal(e)
	}
	events := []store.InputEvent{}
	for i := 0; i < 180; i++ {
		ev := &trace2memv1.Event{EventId: fmt.Sprintf("e%03d", i), SessionId: "s1", OccurredAt: timestamppb.Now(), Actor: &trace2memv1.Actor{Role: "user"}, Source: &trace2memv1.Source{Id: "fixture"}, Payload: &trace2memv1.Event_Message{Message: &trace2memv1.Message{Text: fmt.Sprintf("Topic%03d: ", i) + strings.Repeat("bounded evidence ", 240)}}}
		// Use generated Protobuf JSON, not the Go oneof representation.
		raw := fmt.Sprintf(`{"eventId":"e%03d","sessionId":"s1","occurredAt":"2026-09-07T12:00:00Z","actor":{"role":"user"},"source":{"id":"fixture"},"message":{"text":%q}}`, i, ev.GetMessage().Text)
		events = append(events, store.InputEvent{ID: ev.EventId, Session: "s1", Hash: domain.Hash([]byte(raw)), JSON: []byte(raw), Occurred: time.Now()})
	}
	if _, _, _, e = s.Append(ctx, tenant, sp, events); e != nil {
		t.Fatal(e)
	}
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			t.Fatal("large history did not finish")
		case <-ticker.C:
			var status, reason string
			var accepted, compiled int
			e = s.DB.QueryRow(ctx, `SELECT j.status,j.error,(SELECT count(*) FROM events WHERE tenant=$1 AND space=$2),(SELECT count(*) FROM events WHERE tenant=$1 AND space=$2 AND ordinal<=s.watermark) FROM spaces s JOIN jobs j ON j.tenant=s.tenant AND j.space=s.id WHERE s.tenant=$1 AND s.id=$2`, tenant, sp).Scan(&status, &reason, &accepted, &compiled)
			if e != nil {
				t.Fatal(e)
			}
			if status == "failed" {
				t.Fatal(reason)
			}
			if accepted == compiled && status == "done" {
				var n int
				s.DB.QueryRow(ctx, "SELECT count(*) FROM revisions WHERE tenant=$1 AND space=$2", tenant, sp).Scan(&n)
				if n < 2 {
					t.Fatal("expected incremental revisions", n)
				}
				snap, e := s.Snapshot(ctx, tenant, sp, "")
				if e != nil {
					t.Fatal(e)
				}
				subjects := 0
				for _, p := range snap.Pages {
					if strings.HasPrefix(p.Path, "knowledge/subjects/") {
						subjects++
					}
				}
				if subjects != 180 {
					t.Fatalf("lost unchanged subjects: %d", subjects)
				}
				return
			}
		}
	}
}
