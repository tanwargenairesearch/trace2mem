package integration

import (
	"context"
	"github.com/tanwargenairesearch/trace2mem/internal/domain"
	"github.com/tanwargenairesearch/trace2mem/internal/store"
	"testing"
	"time"
)

func TestNoopAdvancesOnlyProcessedWatermark(t *testing.T) {
	s := database(t)
	ctx := context.Background()
	p := domain.Principal{Tenant: "noop-" + store.ID(), Subject: "user"}
	if err := s.EnsureMemory(ctx, p); err != nil {
		t.Fatal(err)
	}
	_, _, watermark, _, err := s.Append(ctx, p.Tenant, p.MemoryID(), []store.InputEvent{{ID: "lifecycle", Session: "s1", Hash: "h", JSON: []byte(`{"sessionLifecycle":{"state":"started"}}`), Occurred: time.Now()}})
	if err != nil {
		t.Fatal(err)
	}
	if err = s.Schedule(ctx, p.Tenant, p.MemoryID()); err != nil {
		t.Fatal(err)
	}
	l := domain.Lease{Tenant: p.Tenant, Space: p.MemoryID(), Watermark: watermark}
	if err = s.DB.QueryRow(ctx, "UPDATE jobs SET status='running',fence=fence+1,requested=false,lease_until=now()+interval '90 seconds' WHERE tenant=$1 AND space=$2 RETURNING fence", l.Tenant, l.Space).Scan(&l.Fence); err != nil {
		t.Fatal(err)
	}
	if err = s.PublishNoop(ctx, l, "Lifecycle event adds no factual memory", map[string]any{"supported": true}); err != nil {
		t.Fatal(err)
	}
	var got int64
	var revision, status string
	if err = s.DB.QueryRow(ctx, "SELECT s.watermark,s.revision,j.status FROM spaces s JOIN jobs j ON j.tenant=s.tenant AND j.space=s.id WHERE s.tenant=$1 AND s.id=$2", l.Tenant, l.Space).Scan(&got, &revision, &status); err != nil {
		t.Fatal(err)
	}
	if got != watermark || revision != "" || status != "done" {
		t.Fatalf("no-op created or lost state: %d %s %s", got, revision, status)
	}
	if err = s.PublishNoop(ctx, l, "stale worker", true); err == nil {
		t.Fatal("stale no-op published")
	}
}
