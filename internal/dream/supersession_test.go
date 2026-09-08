package dream

import (
	"context"
	"encoding/json"
	"github.com/trace2mem/trace2mem/internal/domain"
	"github.com/trace2mem/trace2mem/internal/store"
	"os"
	"testing"
)

func TestSupersessionAcrossSubjects(t *testing.T) {
	dsn := os.Getenv("TRACE2MEM_TEST_DATABASE")
	if dsn == "" {
		t.Skip("requires Docker database")
	}
	ctx := context.Background()
	s, err := store.Open(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if err = s.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	p := domain.Principal{Tenant: "supersession-" + store.ID(), Subject: "u"}
	if err = s.EnsureMemory(ctx, p); err != nil {
		t.Fatal(err)
	}
	rev := "parent"
	if _, err = s.DB.Exec(ctx, "INSERT INTO revisions(tenant,space,id,parent,watermark,verification) VALUES($1,$2,$3,'',0,'{}')", p.Tenant, p.MemoryID(), rev); err != nil {
		t.Fatal(err)
	}
	old := domain.Observation{Subject: "Old project", Text: "Launch September", Origin: "user", Status: "current", Citations: []string{"old-event"}}
	content := `<!-- trace2mem-observation {"subject":"Old project","text":"Launch September","origin":"user","status":"current","citations":["old-event"]} -->
# Old project

Launch September
`
	if _, err = s.DB.Exec(ctx, "INSERT INTO pages(tenant,space,revision,path,content,hash,citations) VALUES($1,$2,$3,$4,$5,$6,$7)", p.Tenant, p.MemoryID(), rev, "notes/old-project-a68937a0/"+old.StableID()+".md", content, domain.Hash([]byte(content)), old.Citations); err != nil {
		t.Fatal(err)
	}
	payload := `{"eventId":"old-event","sessionId":"session","actor":{"role":"user"},"message":{"text":"Launch September"}}`
	if _, err = s.DB.Exec(ctx, "INSERT INTO events(tenant,space,id,session,hash,payload,occurred_at) VALUES($1,$2,'old-event','session','fixture',$3,now())", p.Tenant, p.MemoryID(), json.RawMessage(payload)); err != nil {
		t.Fatal(err)
	}
	engine := Engine{Store: s}
	lease := domain.Lease{Tenant: p.Tenant, Space: p.MemoryID(), Parent: rev}
	update := domain.Observation{Subject: "Renamed project", Text: "Launch October", Origin: "user", Status: "current", Citations: []string{"new-event"}, Supersedes: []string{old.StableID()}}
	merged, _, err := engine.merge(ctx, lease, []domain.Observation{update}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(merged) != 2 || merged[1].Subject != old.Subject || merged[1].Status != "superseded" {
		t.Fatalf("old subject not updated: %+v", merged)
	}
	merged, _, err = engine.merge(ctx, lease, []domain.Observation{old, update}, nil)
	if err != nil || len(merged) != 2 || merged[0].Status != "superseded" {
		t.Fatalf("incoming target remained current: %+v %v", merged, err)
	}
	update.Supersedes = []string{"missing"}
	if _, _, err = engine.merge(ctx, lease, []domain.Observation{update}, nil); err == nil {
		t.Fatal("dangling target accepted")
	}
	update.Supersedes = []string{update.StableID()}
	if _, _, err = engine.merge(ctx, lease, []domain.Observation{update}, nil); err == nil {
		t.Fatal("self cycle accepted")
	}
	a, b := update, old
	a.Supersedes = []string{b.StableID()}
	b.Supersedes = []string{a.StableID()}
	if _, _, err = engine.merge(ctx, lease, []domain.Observation{a, b}, nil); err == nil {
		t.Fatal("cross-subject cycle accepted")
	}
}

func TestMergeRejectsUnboundedUpdatesBeforeStorage(t *testing.T) {
	e := Engine{}
	if _, _, err := e.merge(context.Background(), domain.Lease{}, make([]domain.Observation, 1001), nil); err == nil {
		t.Fatal("unbounded update count")
	}
	o := domain.Observation{Text: string(make([]byte, 2<<20))}
	if _, _, err := e.merge(context.Background(), domain.Lease{}, []domain.Observation{o}, nil); err == nil {
		t.Fatal("unbounded update bytes")
	}
}
