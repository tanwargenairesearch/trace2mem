package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/trace2mem/trace2mem/internal/domain"
	"github.com/trace2mem/trace2mem/internal/model"
)

func TestConditionIsolation(t *testing.T) {
	pages := map[string]string{
		"knowledge/index.md":        "WIKI_SECRET",
		"knowledge/subjects/x.md":   "WIKI_SECRET",
		"notes/x.md":                "permitted note",
		"sessions/s/summary.md":     "permitted summary",
		"sessions/evidence/e1.json": `{"event":{"eventId":"e1","message":{"text":"original"}},"excerpt":{"sequence":1}}`,
	}
	if strings.Contains(orientation("notes_sessions", pages), "knowledge/") {
		t.Fatal("wiki in notes orientation")
	}
	for _, condition := range []string{"existing_memory", "notes_sessions"} {
		for _, call := range []model.Call{{Name: "memory_read", Arguments: json.RawMessage(`{"path":"knowledge/index.md","offset":0}`)}, {Name: "memory_search", Arguments: json.RawMessage(`{"query":"WIKI_SECRET"}`)}} {
			b, _ := json.Marshal(execute(condition, pages, call))
			if strings.Contains(string(b), "WIKI_SECRET") {
				t.Fatalf("wiki leaked to %s", condition)
			}
		}
	}
	body, err := history(pages)
	if err != nil || strings.Contains(body, "WIKI_SECRET") || strings.Contains(body, "permitted summary") || !strings.Contains(body, "original") {
		t.Fatalf("baseline history: %s %v", body, err)
	}
	for _, path := range []string{"../secret", "/etc/passwd", "sessions/evidence/not-published.json"} {
		args, _ := json.Marshal(map[string]any{"path": path, "offset": 0})
		value := execute("trace2mem", pages, model.Call{Name: "memory_read", Arguments: args}).(map[string]any)
		if value["error"] == nil {
			t.Fatalf("accepted %s", path)
		}
	}
}

func TestSnapshotIntegrity(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "memory/notes"), 0700); err != nil {
		t.Fatal(err)
	}
	body := []byte("fixture")
	hash := sha256.Sum256(body)
	manifest := fmt.Sprintf(`{"revision":"r1","files":[{"path":"notes/x.md","size":"7","sha256":%q}]}`, hex.EncodeToString(hash[:]))
	for path, data := range map[string][]byte{"manifest.json": []byte(manifest), "memory/notes/x.md": body} {
		if err := os.WriteFile(filepath.Join(dir, path), data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	if _, _, err := load(dir); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "memory/notes/x.md"), []byte("changed"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := load(dir); err == nil {
		t.Fatal("accepted changed snapshot")
	}
}

func TestAgentProtocolAndUsage(t *testing.T) {
	host := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
		}
		if list, ok := body["tools"].([]any); ok && len(list) != 0 {
			t.Error("baseline received memory tools")
		}
		fmt.Fprint(w, `{"output":[{"type":"message","content":[{"type":"output_text","text":"{\"launch_date\":\"2026-12-01\"}"}]}],"usage":{"input_tokens":100,"output_tokens":20}}`)
	}))
	defer host.Close()
	cfg := domain.ModelConfig{Provider: "openai", Model: "fixture", Endpoint: host.URL, EmbeddingProvider: "scripted", EmbeddingModel: "fixture"}
	result := run(context.Background(), request{Condition: "existing_memory", MaxTokens: 10000, Model: "fixture", Input: "date?"}, cfg, snapshot{Revision: "r1"}, map[string]string{})
	if result["error"] != nil {
		t.Fatal(result)
	}
	if result["artifact"].(map[string]any)["launch_date"] != "2026-12-01" {
		t.Fatal(result)
	}
	if result["usage"].(domain.Usage).Total() != 120 {
		t.Fatal("usage missing")
	}
	result = run(context.Background(), request{Condition: "existing_memory", MaxTokens: 1}, cfg, snapshot{Revision: "r1"}, nil)
	if result["error"] != "token_budget" {
		t.Fatal("budget not enforced")
	}
}

func TestReadOffsets(t *testing.T) {
	pages := map[string]string{"notes/long.md": strings.Repeat("é", 8001)}
	value := execute("notes_sessions", pages, model.Call{Name: "memory_read", Arguments: json.RawMessage(`{"path":"notes/long.md","offset":0}`)}).(map[string]any)
	if len([]rune(value["content"].(string))) != 8000 || value["next_offset"] != 8000 {
		t.Fatal(value["next_offset"])
	}
	value = execute("notes_sessions", pages, model.Call{Name: "memory_read", Arguments: json.RawMessage(`{"path":"notes/long.md","offset":8000}`)}).(map[string]any)
	if value["content"] != "é" || value["next_offset"] != nil {
		t.Fatal(value)
	}
}

func TestMissingUsageAndSingleAttempt(t *testing.T) {
	for _, status := range []int{200, 429, 503} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			calls := 0
			host := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				w.WriteHeader(status)
				fmt.Fprint(w, `{"output":[{"type":"message","content":[{"type":"output_text","text":"{\"ok\":true}"}]}]}`)
			}))
			defer host.Close()
			cfg := domain.ModelConfig{Provider: "openai", Model: "fixture", Endpoint: host.URL}
			result := run(context.Background(), request{Condition: "existing_memory", MaxTokens: 10000}, cfg, snapshot{Revision: "r1"}, nil)
			if calls != 1 {
				t.Fatalf("provider called %d times", calls)
			}
			usage := result["usage"].(domain.Usage)
			if usage.Total() == 0 || !usage.Estimated || !usage.Unresolved {
				t.Fatal(usage)
			}
			if status != 200 && result["error"] != "provider_failure" {
				t.Fatal(result)
			}
		})
	}
}

func TestFingerprintAndStartupErrors(t *testing.T) {
	cfg := domain.ModelConfig{Provider: "openai", Model: "fixture", Endpoint: "http://localhost", Key: "secret"}
	public, a := settings(cfg)
	cfg.Key = "rotated"
	_, b := settings(cfg)
	if a != b {
		t.Fatal("credential entered fingerprint")
	}
	encoded, _ := json.Marshal(public)
	if strings.Contains(string(encoded), "secret") {
		t.Fatal("credential exposed")
	}
	cfg.ReasoningEffort = "none"
	_, b = settings(cfg)
	if a == b {
		t.Fatal("reasoning configuration not frozen")
	}
	result := start(request{History: "wrong"})
	if result["error"] != "invalid_request" {
		t.Fatal(result)
	}
}
