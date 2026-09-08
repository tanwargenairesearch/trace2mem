package server

import (
	"bytes"
	"connectrpc.com/connect"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	v1 "github.com/mohit-lendmind/trace2mem/gen/trace2mem/v1"
	"github.com/mohit-lendmind/trace2mem/internal/config"
	"github.com/mohit-lendmind/trace2mem/internal/domain"
	"github.com/mohit-lendmind/trace2mem/internal/dream"
	"github.com/mohit-lendmind/trace2mem/internal/store"
	"os"
	"strings"
	"testing"
)

func TestProgressiveRetrievalLargePagesAndSuppression(t *testing.T) {
	dsn := os.Getenv("TRACE2MEM_TEST_DATABASE")
	if dsn == "" {
		t.Skip("requires test database")
	}
	ctx := context.Background()
	db, err := store.Open(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err = db.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	p := domain.Principal{Tenant: "retrieval-" + store.ID(), Subject: "u", Scopes: map[string]bool{"read": true, "manage": true, "ingest": true}}
	ctx = context.WithValue(ctx, principalKey{}, p)
	if err = db.EnsureMemory(ctx, p); err != nil {
		t.Fatal(err)
	}
	vault, err := config.NewVault(base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{42}, 32)))
	if err != nil {
		t.Fatal(err)
	}
	sealed, err := vault.Seal(ctx, []byte(`{}`), []byte(p.Tenant+"/"+p.MemoryID()))
	if err != nil {
		t.Fatal(err)
	}
	cfg := domain.ModelConfig{Provider: "scripted", Model: "fixture", EmbeddingProvider: "scripted", EmbeddingModel: "fixture", MaxTokens: 32000, DailyTokens: 1000000}
	if err = db.SetConfig(ctx, p.Tenant, p.MemoryID(), cfg, sealed); err != nil {
		t.Fatal(err)
	}
	if _, err = db.DB.Exec(ctx, "INSERT INTO revisions(tenant,space,id,parent,watermark,verification) VALUES($1,$2,'rev','',1,'{}')", p.Tenant, p.MemoryID()); err != nil {
		t.Fatal(err)
	}
	if _, err = db.DB.Exec(ctx, "UPDATE spaces SET revision='rev',watermark=1 WHERE tenant=$1 AND id=$2", p.Tenant, p.MemoryID()); err != nil {
		t.Fatal(err)
	}
	raw := `{"eventId":"e1","sessionId":"s1","actor":{"role":"user"},"message":{"text":"needle: approved Go"}}`
	if _, err = db.DB.Exec(ctx, "INSERT INTO events(tenant,space,id,session,hash,payload,occurred_at) VALUES($1,$2,'e1','s1','fixture',$3,now())", p.Tenant, p.MemoryID(), json.RawMessage(raw)); err != nil {
		t.Fatal(err)
	}
	pages := map[string]string{"knowledge/index.md": "# Index\n[[knowledge/page-0.md]]", "sessions/evidence/e1.json": raw}
	for i := 0; i < 8; i++ {
		pages[fmt.Sprintf("knowledge/page-%d.md", i)] = "needle [cite:e1] " + strings.Repeat("界", 11000)
	}
	for path, content := range pages {
		if _, err = db.DB.Exec(ctx, "INSERT INTO pages(tenant,space,revision,path,content,hash,citations) VALUES($1,$2,'rev',$3,$4,$5,$6)", p.Tenant, p.MemoryID(), path, content, domain.Hash([]byte(content)), []string{"e1"}); err != nil {
			t.Fatal(err)
		}
	}
	engine := &dream.Engine{Store: db, Vault: vault, Config: config.Config{Scripted: true}}
	api := &Server{Store: db, Engine: engine}
	manifest, err := db.ReadView(ctx, p.Tenant, p.MemoryID(), "rev", "", true)
	if err != nil {
		t.Fatal(err)
	}
	for _, page := range manifest.Pages {
		if page.Content != "" || page.Size != int64(len(pages[page.Path])) {
			t.Fatalf("manifest loaded content or lost byte size: %+v", page)
		}
	}
	one, err := db.ReadView(ctx, p.Tenant, p.MemoryID(), "rev", "knowledge/page-0.md", false)
	if err != nil || len(one.Pages) != 1 {
		t.Fatal("path read", err)
	}
	trace := evaluationTrace{}
	ctx = context.WithValue(ctx, evaluationTraceKey{}, &trace)
	result, err := api.GetContext(ctx, connect.NewRequest(&v1.GetContextRequest{Query: "needle", Revision: "rev"}))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result.Msg.Synthesis, "approved Go [cite:e1]") {
		t.Fatal(result.Msg.Synthesis)
	}
	seen := map[string]bool{}
	for _, turn := range trace.Turns {
		if turn.Result != nil {
			seen[turn.Result.Name] = true
			if len(turn.Result.Text) > 128<<10 {
				t.Fatal("oversize tool result")
			}
		}
	}
	for _, tool := range []string{"memory_index", "search", "memory_read", "memory_evidence"} {
		if !seen[tool] {
			t.Fatal("tool never executed", tool)
		}
	}
	if err = db.Forget(ctx, p.Tenant, p.MemoryID(), "e1"); err != nil {
		t.Fatal(err)
	}
	if _, err = api.ReadFile(ctx, connect.NewRequest(&v1.ReadFileRequest{Revision: "rev", Path: "knowledge/page-0.md"})); err == nil {
		t.Fatal("targeted read bypassed suppression")
	}
}

func TestPageExcerptReportsContinuation(t *testing.T) {
	p := domain.Page{Content: "αβγδε"}
	a := pageExcerpt(p, 0, 2)
	b := pageExcerpt(p, a["next_offset"].(int), 3)
	if a["content"] != "αβ" || a["truncated"] != true || b["content"] != "γδε" || b["truncated"] != false {
		t.Fatal(a, b)
	}
}
