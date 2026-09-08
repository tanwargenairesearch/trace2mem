package integration

import (
	"bytes"
	"connectrpc.com/connect"
	"context"
	"encoding/json"
	"fmt"
	"github.com/tanwargenairesearch/trace2mem/filesystem"
	trace2memv1 "github.com/tanwargenairesearch/trace2mem/gen/trace2mem/v1"
	"github.com/tanwargenairesearch/trace2mem/internal/domain"
	"github.com/tanwargenairesearch/trace2mem/internal/store"
	"github.com/tanwargenairesearch/trace2mem/sdk"
	"google.golang.org/protobuf/types/known/timestamppb"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestMemoryLifecycle(t *testing.T) {
	url := os.Getenv("TRACE2MEM_TEST_URL")
	if url == "" {
		t.Skip("requires Docker integration stack")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	db := database(t)
	p := domain.Principal{Tenant: "integration", Subject: store.ID(), Scopes: map[string]bool{"read": true, "ingest": true, "manage": true}}
	if err := db.EnsureMemory(ctx, p); err != nil {
		t.Fatal(err)
	}
	token, err := db.CreateToken(ctx, p, []string{"read", "ingest", "manage"})
	if err != nil {
		t.Fatal(err)
	}
	c := sdk.New(url, token)
	post := func(route string, body any) map[string]any {
		t.Helper()
		b, _ := json.Marshal(body)
		req, e := http.NewRequestWithContext(ctx, "POST", url+"/api/"+route, bytes.NewReader(b))
		if e != nil {
			t.Fatal(e)
		}
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", "application/json")
		res, e := http.DefaultClient.Do(req)
		if e != nil {
			t.Fatal(e)
		}
		defer res.Body.Close()
		data, _ := io.ReadAll(res.Body)
		if res.StatusCode != 200 {
			t.Fatalf("%s: %d %s", route, res.StatusCode, data)
		}
		var v map[string]any
		if e = json.Unmarshal(data, &v); e != nil {
			t.Fatal(e)
		}
		return v
	}
	sp := "personal"
	post("model", map[string]string{"provider": "scripted", "model": "fixture-v1", "embedding_model": "fixture-v1", "embedding_provider": "scripted"})
	event := func(id, text string, offset time.Duration) *trace2memv1.Event {
		return &trace2memv1.Event{EventId: id, SessionId: "s1", Actor: &trace2memv1.Actor{Role: "user"}, Source: &trace2memv1.Source{Id: "fixture"}, OccurredAt: timestamppb.New(time.Now().Add(offset)), Payload: &trace2memv1.Event_Message{Message: &trace2memv1.Message{Text: text}}}
	}
	e1 := event("e1", "Launch: September", -time.Hour)
	e2 := event("e2", "Format: PDF", -30*time.Minute)
	batch := &trace2memv1.AppendEventsRequest{Events: []*trace2memv1.Event{e1, e2}}
	a, e := c.Ingestion.AppendEvents(ctx, connect.NewRequest(batch))
	if e != nil || a.Msg.Accepted != 2 {
		t.Fatalf("append: %v %v", a, e)
	}
	a, e = c.Ingestion.AppendEvents(ctx, connect.NewRequest(batch))
	if e != nil || a.Msg.Duplicates != 2 {
		t.Fatalf("duplicate: %v %v", a, e)
	}
	e1.GetMessage().Text = "changed"
	if _, e = c.Ingestion.AppendEvents(ctx, connect.NewRequest(batch)); connect.CodeOf(e) != connect.CodeFailedPrecondition {
		t.Fatalf("conflicting duplicate accepted: %v", e)
	}
	e1.GetMessage().Text = "Launch: September"
	wait := func(old string) string {
		t.Helper()
		if _, err := c.Ingestion.RequestCompilation(ctx, connect.NewRequest(&trace2memv1.RequestCompilationRequest{})); err != nil {
			t.Fatal(err)
		}
		ticker := time.NewTicker(250 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				t.Fatal("compilation timed out")
				return ""
			case <-ticker.C:
				r, e := c.Ingestion.GetIngestionStatus(ctx, connect.NewRequest(&trace2memv1.GetIngestionStatusRequest{}))
				if e != nil {
					t.Fatal(e)
				}
				if r.Msg.JobStatus == "failed" {
					t.Fatal(r.Msg.LastError)
				}
				if r.Msg.Revision != "" && r.Msg.Revision != old && r.Msg.Pending == 0 {
					return r.Msg.Revision
				}
			}
		}
	}
	first := wait("")
	cases := []any{}
	for _, split := range []string{"development", "heldout"} {
		cases = append(cases, map[string]any{"query": "Launch", "split": split, "facts": []any{map[string]any{"id": "launch", "answer_pattern": "September", "evidence_pattern": "Launch: September", "citations": []string{"e1"}}}})
	}
	evaluation := post("evaluate", map[string]any{"cases": cases})
	report := evaluation["report"].(map[string]any)
	if report["revision"] != first || len(report["results"].([]any)) != 4 || report["tokens"].(float64) <= 0 {
		t.Fatal("invalid paired evaluation")
	}
	candidate := post("candidates", map[string]any{"evaluation_id": evaluation["id"]})
	if candidate["heldout_report"].(map[string]any)["optimization"] == nil {
		t.Fatal("missing optimizer accounting")
	}
	if _, err := db.DB.Exec(ctx, "UPDATE spaces SET pending_model=model WHERE tenant=$1 AND id=$2", p.Tenant, p.MemoryID()); err != nil {
		t.Fatal(err)
	}
	promotion, _ := json.Marshal(map[string]any{"promote_id": candidate["id"]})
	pendingRequest, _ := http.NewRequestWithContext(ctx, "POST", url+"/api/candidates", bytes.NewReader(promotion))
	pendingRequest.Header.Set("Authorization", "Bearer "+token)
	pendingRequest.Header.Set("Content-Type", "application/json")
	pendingResponse, err := http.DefaultClient.Do(pendingRequest)
	if err != nil {
		t.Fatal(err)
	}
	pendingResponse.Body.Close()
	if pendingResponse.StatusCode != 409 {
		t.Fatal("promotion accepted during reindex", pendingResponse.StatusCode)
	}
	if _, err := db.DB.Exec(ctx, "UPDATE spaces SET pending_model=NULL WHERE tenant=$1 AND id=$2", p.Tenant, p.MemoryID()); err != nil {
		t.Fatal(err)
	}
	post("candidates", map[string]any{"promote_id": candidate["id"]})
	if _, err := db.DB.Exec(ctx, `UPDATE spaces SET generation=generation+1,model=jsonb_set(model,'{provider}','"unavailable-fixture"') WHERE tenant=$1 AND id=$2`, p.Tenant, p.MemoryID()); err != nil {
		t.Fatal(err)
	}
	failed := post("evaluate", map[string]any{"cases": cases})["report"].(map[string]any)["results"].([]any)
	for _, result := range failed {
		row := result.(map[string]any)
		if row["error"] == nil || row["correct"] != false {
			t.Fatal("failed answer not recorded")
		}
	}
	if _, err := db.DB.Exec(ctx, `UPDATE spaces SET generation=generation+1,model=jsonb_set(model,'{provider}','"scripted"') WHERE tenant=$1 AND id=$2`, p.Tenant, p.MemoryID()); err != nil {
		t.Fatal(err)
	}
	manifest, e := c.Memory.GetManifest(ctx, connect.NewRequest(&trace2memv1.GetManifestRequest{}))
	if e != nil || len(manifest.Msg.Files) < 5 {
		t.Fatalf("manifest: %v %v", manifest, e)
	}
	grpc := sdk.New(url, token, connect.WithGRPC())
	g, e := grpc.Memory.GetManifest(ctx, connect.NewRequest(&trace2memv1.GetManifestRequest{}))
	if e != nil {
		t.Fatal("gRPC", e)
	}
	if g.Msg.Revision != first {
		t.Fatal("gRPC revision differs")
	}
	reqBody := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"memory_index","arguments":{}}}`
	req, _ := http.NewRequestWithContext(ctx, "POST", url+"/mcp", strings.NewReader(reqBody))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	req.Header.Set("MCP-Protocol-Version", "2025-06-18")
	res, e := http.DefaultClient.Do(req)
	if e != nil {
		t.Fatal(e)
	}
	mb, _ := io.ReadAll(res.Body)
	res.Body.Close()
	if res.StatusCode != 200 || !bytes.Contains(mb, []byte("Knowledge index")) {
		t.Fatalf("MCP: %d %s", res.StatusCode, mb)
	}
	cache, e := filesystem.New(ctx, c, first, t.TempDir(), 8<<20)
	if e != nil {
		t.Fatal(e)
	}
	target := filepath.Join(t.TempDir(), "snapshot")
	if e = cache.Sync(ctx, target); e != nil {
		t.Fatal(e)
	}
	if _, e = os.ReadFile(filepath.Join(target, "knowledge/index.md")); e != nil {
		t.Fatal(e)
	}
	cache.Client = sdk.New("http://127.0.0.1:1", token)
	if _, e = cache.Read(ctx, "knowledge/index.md"); e != nil {
		t.Fatal("warm cache required network", e)
	}
	if _, e = cache.Read(ctx, "../../etc/passwd"); e == nil {
		t.Fatal("traversal accepted")
	}
	_, e = c.Ingestion.AppendEvents(ctx, connect.NewRequest(&trace2memv1.AppendEventsRequest{Events: []*trace2memv1.Event{event("e3", "Launch: October", 0)}}))
	if e != nil {
		t.Fatal(e)
	}
	second := wait(first)
	search, e := c.Memory.Search(ctx, connect.NewRequest(&trace2memv1.SearchRequest{Query: "October"}))
	if e != nil || len(search.Msg.Hits) == 0 {
		t.Fatalf("correction not retrieved: %v %v", search, e)
	}
	ev, e := c.Memory.GetEvidence(ctx, connect.NewRequest(&trace2memv1.GetEvidenceRequest{EventId: "e3"}))
	if e != nil || ev.Msg.Event.GetMessage().Text != "Launch: October" {
		t.Fatal("evidence mismatch", e)
	}
	contextRes, e := c.Memory.GetContext(ctx, connect.NewRequest(&trace2memv1.GetContextRequest{Query: "Launch"}))
	if e != nil || !strings.Contains(contextRes.Msg.Synthesis, "October") {
		t.Fatalf("context: %v %v", contextRes, e)
	}
	post("model", map[string]string{"provider": "scripted", "model": "fixture-v1", "embedding_provider": "scripted", "embedding_model": "fixture-v2"})
	reindexed := wait(second)
	if reindexed == second {
		t.Fatal("embedding change did not publish an index")
	}
	afterIndex, e := c.Memory.Search(ctx, connect.NewRequest(&trace2memv1.SearchRequest{Query: "PDF"}))
	if e != nil || len(afterIndex.Msg.Hits) == 0 {
		t.Fatal("unchanged knowledge lost during reindex", e)
	}
	post("forget", map[string]string{"event_id": "e3"})
	if _, e = c.Memory.GetEvidence(ctx, connect.NewRequest(&trace2memv1.GetEvidenceRequest{EventId: "e3"})); connect.CodeOf(e) != connect.CodeNotFound {
		t.Fatal("forgotten evidence readable", e)
	}
	if _, e = c.Memory.GetManifest(ctx, connect.NewRequest(&trace2memv1.GetManifestRequest{Revision: second})); e == nil {
		t.Fatal("old revision after forgetting readable")
	}
	wait("")
	if _, e = c.Ingestion.AppendEvents(ctx, connect.NewRequest(&trace2memv1.AppendEventsRequest{Events: []*trace2memv1.Event{event("e3", "Launch: October", 0)}})); e == nil {
		t.Fatal("tombstoned event resurrected")
	}
	bad := sdk.New(url, "invalid")
	if _, e = bad.Memory.GetManifest(ctx, connect.NewRequest(&trace2memv1.GetManifestRequest{})); e == nil {
		t.Fatal("unauthenticated read accepted")
	}
	if path := os.Getenv("TRACE2MEM_FUSE_TOKEN_FILE"); path != "" {
		if err := os.WriteFile(path, []byte(token), 0600); err != nil {
			t.Fatal(err)
		}
	}
	t.Logf("personal memory verified revisions %s -> %s", first, second)
	fmt.Fprint(io.Discard, sp)
}
