package integration

import (
	"bytes"
	"connectrpc.com/connect"
	"context"
	v1 "github.com/trace2mem/trace2mem/gen/trace2mem/v1"
	"github.com/trace2mem/trace2mem/internal/domain"
	"github.com/trace2mem/trace2mem/internal/store"
	"github.com/trace2mem/trace2mem/sdk"
	"google.golang.org/protobuf/types/known/timestamppb"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestUserMemoryOwnership(t *testing.T) {
	s := database(t)
	url := os.Getenv("TRACE2MEM_TEST_URL")
	if url == "" {
		t.Skip("server required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	tenant := "ownership-" + store.ID()
	ids := map[string]bool{}
	for _, user := range []string{"alice", "bob"} {
		p := domain.Principal{Tenant: tenant, Subject: user, Scopes: map[string]bool{"read": true, "ingest": true, "manage": true}}
		var wg sync.WaitGroup
		errs := make(chan error, 8)
		for i := 0; i < 8; i++ {
			wg.Add(1)
			go func() { defer wg.Done(); errs <- s.EnsureMemory(ctx, p) }()
		}
		wg.Wait()
		close(errs)
		for err := range errs {
			if err != nil {
				t.Fatal(err)
			}
		}
		token, err := s.CreateToken(ctx, p, []string{"read", "ingest", "manage"})
		if err != nil {
			t.Fatal(err)
		}
		c := sdk.New(url, token)
		ev := &v1.Event{EventId: "same-event", SessionId: "same-session", OccurredAt: timestamppb.Now(), Actor: &v1.Actor{Role: "user"}, Source: &v1.Source{Id: "adapter"}, Payload: &v1.Event_Message{Message: &v1.Message{Text: user + " private evidence"}}}
		if _, err = c.Ingestion.AppendEvents(ctx, connect.NewRequest(&v1.AppendEventsRequest{Events: []*v1.Event{ev}})); err != nil {
			t.Fatal(err)
		}
		grpc := sdk.New(url, token, connect.WithGRPC())
		got, err := grpc.Memory.GetEvidence(ctx, connect.NewRequest(&v1.GetEvidenceRequest{EventId: "same-event"}))
		if err != nil || got.Msg.Event.GetMessage().Text != ev.GetMessage().Text {
			t.Fatalf("gRPC evidence isolation: %v %v", got, err)
		}
		manifest, err := c.Memory.GetManifest(ctx, connect.NewRequest(&v1.GetManifestRequest{}))
		if err != nil {
			t.Fatal(err)
		}
		if ids[manifest.Msg.MemoryId] || manifest.Msg.MemoryId == "" {
			t.Fatal("shared memory identifier")
		}
		ids[manifest.Msg.MemoryId] = true
		for _, alias := range []string{"space", "spaceId", "space_id"} {
			req, _ := http.NewRequestWithContext(ctx, "POST", url+"/api/forget?"+alias+"=legacy", strings.NewReader(`{"event_id":"same-event"}`))
			req.Header.Set("Authorization", "Bearer "+token)
			req.Header.Set("Content-Type", "application/json")
			res, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatal(err)
			}
			res.Body.Close()
			if res.StatusCode != 400 {
				t.Fatal("legacy selector accepted", alias, res.StatusCode)
			}
		}
		req, _ := http.NewRequestWithContext(ctx, "POST", url+"/mcp", bytes.NewBufferString(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"memory_evidence","arguments":{"event_id":"same-event"}}}`))
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json, text/event-stream")
		req.Header.Set("MCP-Protocol-Version", "2025-06-18")
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		body, err := io.ReadAll(res.Body)
		res.Body.Close()
		if err != nil || !bytes.Contains(body, []byte(user+" private evidence")) {
			t.Fatalf("MCP isolation %s: %s %v", user, body, err)
		}
	}
	var n int
	if err := s.DB.QueryRow(ctx, "SELECT count(*) FROM user_memories WHERE tenant=$1", tenant).Scan(&n); err != nil || n != 2 {
		t.Fatalf("concurrent provisioning: %d %v", n, err)
	}
}
