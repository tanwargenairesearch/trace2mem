package dream

import (
	"context"
	"encoding/json"
	"github.com/mohit-lendmind/trace2mem/internal/domain"
	"github.com/mohit-lendmind/trace2mem/internal/model"
	"github.com/mohit-lendmind/trace2mem/internal/store"
	"os"
	"testing"
)

type citationRepairProvider struct {
	model.Scripted
	calls, checks int
	keepInvalid   bool
}

func (p *citationRepairProvider) Generate(ctx context.Context, turns []model.Turn, tools []model.Tool) (model.Reply, error) {
	if tools[0].Name == "verify" {
		p.checks++
		reply, err := p.Scripted.Generate(ctx, turns, tools)
		reply.Usage = domain.Usage{Output: 1}
		return reply, err
	}
	p.calls++
	citation := "e1"
	if p.calls == 2 && !p.keepInvalid {
		citation = "e2"
	}
	draft := wikiDraft{Sessions: []draftPage{{Name: "s1", Text: "Original fact [cite:e1]."}, {Name: "s2", Text: "Changed fact [cite:" + citation + "]."}}, Subjects: []draftPage{{Name: "Project", Text: "Current fact [cite:e2]."}}}
	data, _ := json.Marshal(draft)
	return model.Reply{Calls: []model.Call{{ID: "draft", Name: "compose_wiki", Arguments: data}}, Usage: domain.Usage{Output: 1}}, nil
}
func TestCompositionRepairsCitationOnceBeforeVerification(t *testing.T) {
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
	for _, invalid := range []bool{false, true} {
		p := domain.Principal{Tenant: "repair-" + store.ID(), Subject: "u"}
		if err := s.EnsureMemory(ctx, p); err != nil {
			t.Fatal(err)
		}
		if _, err := s.DB.Exec(ctx, "INSERT INTO jobs(tenant,space,status,fence,lease_until) VALUES($1,$2,'running',1,now()+interval '1 minute')", p.Tenant, p.MemoryID()); err != nil {
			t.Fatal(err)
		}
		provider := &citationRepairProvider{keepInvalid: invalid}
		engine := Engine{Store: s}
		records := []domain.Record{{ID: "e1", Session: "s1", Role: "user", Text: "Original fact"}, {ID: "e2", Session: "s2", Role: "user", Text: "Changed fact"}}
		obs := []domain.Observation{{Subject: "Project", Text: "Changed fact", Origin: "user", Status: "current", Citations: []string{"e2"}}}
		_, usage, err := engine.compose(ctx, domain.Lease{Tenant: p.Tenant, Space: p.MemoryID(), Fence: 1}, provider, obs, records, nil)
		if provider.calls != 2 {
			t.Fatal("repair not bounded", provider.calls)
		}
		if invalid {
			if err == nil || provider.checks != 0 || usage != 2 {
				t.Fatal("invalid repair proceeded", err, provider.checks, usage)
			}
		} else if err != nil || provider.checks != 1 || usage != 3 {
			t.Fatal("repaired draft skipped verification or usage", err, provider.checks, usage)
		}
		var rejected int
		if err := s.DB.QueryRow(ctx, "SELECT count(*) FROM proposals WHERE tenant=$1 AND status='rejected'", p.Tenant).Scan(&rejected); err != nil {
			t.Fatal(err)
		}
		if rejected < 1 {
			t.Fatal("rejected draft diagnostics lost")
		}
	}
}
