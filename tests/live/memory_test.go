package live

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"connectrpc.com/connect"
	v1 "github.com/tanwargenairesearch/trace2mem/gen/trace2mem/v1"
	"github.com/tanwargenairesearch/trace2mem/internal/blob"
	"github.com/tanwargenairesearch/trace2mem/internal/config"
	"github.com/tanwargenairesearch/trace2mem/internal/domain"
	"github.com/tanwargenairesearch/trace2mem/internal/dream"
	"github.com/tanwargenairesearch/trace2mem/internal/server"
	"github.com/tanwargenairesearch/trace2mem/internal/store"
	"github.com/tanwargenairesearch/trace2mem/sdk"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// TestMemoryAblation uses a disposable database and synthetic histories only.
func TestMemoryAblation(t *testing.T) {
	dsn, modelFile, reportDir := os.Getenv("TRACE2MEM_LIVE_DATABASE"), os.Getenv("TRACE2MEM_LIVE_CONFIG"), os.Getenv("TRACE2MEM_LIVE_REPORT_DIR")
	if dsn == "" || modelFile == "" || reportDir == "" {
		t.Skip("opt-in: disposable database, model YAML and report directory required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	f, err := os.Open(modelFile)
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := config.ParseModels(f)
	f.Close()
	if err != nil {
		t.Fatal(err)
	}
	s, err := store.Open(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if err := s.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	p := domain.Principal{Tenant: "live-" + store.ID(), Subject: "fixture-user", Scopes: map[string]bool{"read": true, "ingest": true, "manage": true}}
	if err := s.EnsureMemory(ctx, p); err != nil {
		t.Fatal(err)
	}
	if err := s.SetSchedule(ctx, p.Tenant, p.MemoryID(), store.CompilationSchedule{Mode: "manual"}); err != nil {
		t.Fatal(err)
	}
	vault, err := config.NewVault(base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{42}, 32)))
	if err != nil {
		t.Fatal(err)
	}
	keys, _ := json.Marshal(map[string]string{"generation": cfg.Key, "embedding": cfg.EmbeddingKey})
	sealed, err := vault.Seal(ctx, keys, []byte(p.Tenant+"/"+p.MemoryID()))
	if err != nil {
		t.Fatal(err)
	}
	if err := s.SetConfig(ctx, p.Tenant, p.MemoryID(), cfg, sealed); err != nil {
		t.Fatal(err)
	}
	blobs := blob.Local{Root: t.TempDir()}
	operator := config.Config{AllowedEndpoints: cfg.Endpoint + "," + cfg.EmbeddingEndpoint, Scripted: cfg.Provider == "scripted"}
	engine := &dream.Engine{Store: s, Vault: vault, Blob: blobs, Config: operator}
	api := &server.Server{Store: s, Vault: vault, Blob: blobs, Config: operator, Engine: engine}
	host := httptest.NewServer(api.Handler())
	defer host.Close()
	token, err := s.CreateToken(ctx, p, []string{"read", "ingest", "manage"})
	if err != nil {
		t.Fatal(err)
	}
	client := sdk.New(host.URL, token)
	if err := os.MkdirAll(reportDir, 0700); err != nil {
		t.Fatal(err)
	}
	var history []*v1.Event
	for step, text := range []string{"Launch: Project Alder launches in September. My preferred implementation language is Go.", "Launch: Correction: Project Alder now launches in October, replacing September. The implementation language remains Go."} {
		id := []string{"launch-original", "launch-correction"}[step]
		ev := &v1.Event{EventId: id, SessionId: []string{"conversation-one", "conversation-two"}[step], OccurredAt: timestamppb.New(time.Date(2026, 9, 1+step, 12, 0, 0, 0, time.UTC)), Actor: &v1.Actor{Role: "user", AgentId: "fixture-agent"}, Source: &v1.Source{Id: "ablation-v1"}, Payload: &v1.Event_Message{Message: &v1.Message{Text: text}}}
		history = append(history, ev)
		if _, err := client.Ingestion.AppendEvents(ctx, connect.NewRequest(&v1.AppendEventsRequest{Events: []*v1.Event{ev}})); err != nil {
			t.Fatal(err)
		}
		if err := s.Schedule(ctx, p.Tenant, p.MemoryID()); err != nil {
			t.Fatal(err)
		}
		lease, err := s.Claim(ctx)
		if err != nil || lease == nil {
			t.Fatal("claim", err)
		}
		if lease.Tenant != p.Tenant || lease.Space != p.MemoryID() {
			t.Fatal("use a dedicated database without other queued work")
		}
		if err := engine.Run(ctx, *lease); err != nil {
			t.Fatal("compilation", err)
		}
		month := []string{"September", "October"}[step]
		cases := []any{}
		for _, fact := range []struct{ id, query, pattern, evidence, split string }{{"deadline", "When does Project Alder launch? State the current month, with a citation.", `(?i)(launch(?:es)?|scheduled|deadline)[^\n.]{0,70}(?:in|is|for|:)\s+\**` + month, month, "development"}, {"language", "Which programming language do I prefer? Cite the source.", `\bGo\b`, `\bGo\b`, "heldout"}} {
			forbidden := []string{}
			if step == 1 && fact.id == "deadline" {
				forbidden = []string{`(?i)September (?:remains|is) (?:the )?(?:current )?(?:launch|deadline)`, `(?i)(?:launches|deadline is) (?:in )?September`}
			}
			cases = append(cases, map[string]any{"query": fact.query, "split": fact.split, "facts": []any{map[string]any{"id": fact.id, "answer_pattern": fact.pattern, "evidence_pattern": fact.evidence, "citations": []string{id}, "forbidden_patterns": forbidden}}})
		}
		body, _ := json.Marshal(map[string]any{"cases": cases})
		req, _ := http.NewRequestWithContext(ctx, "POST", host.URL+"/api/evaluate", bytes.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", "application/json")
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		data, err := io.ReadAll(io.LimitReader(res.Body, 17<<20))
		res.Body.Close()
		if err != nil {
			t.Fatal(err)
		}
		if res.StatusCode != 200 {
			t.Fatalf("evaluation HTTP %d: %s", res.StatusCode, data)
		}
		name := []string{"initial.json", "corrected.json"}[step]
		if err := os.WriteFile(filepath.Join(reportDir, name), data, 0600); err != nil {
			t.Fatal(err)
		}
		t.Log("saved", name)
	}
	if err := s.Forget(ctx, p.Tenant, p.MemoryID(), "launch-original"); err != nil {
		t.Fatal(err)
	}
	if _, err := client.Memory.GetEvidence(ctx, connect.NewRequest(&v1.GetEvidenceRequest{EventId: "launch-original"})); err == nil {
		t.Fatal("forgotten evidence readable")
	}
	if _, err := client.Memory.GetManifest(ctx, connect.NewRequest(&v1.GetManifestRequest{})); err == nil {
		t.Fatal("forgetting did not suppress retrieval")
	}
	// Histories are synthetic; persist the exact input envelopes alongside the reports.
	var saved bytes.Buffer
	for _, event := range history {
		data, err := protojson.Marshal(event)
		if err != nil {
			t.Fatal(err)
		}
		saved.Write(data)
		saved.WriteByte('\n')
	}
	if err := os.WriteFile(filepath.Join(reportDir, "history.jsonl"), saved.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}
}
