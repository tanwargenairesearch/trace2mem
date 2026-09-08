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

func TestControlledReadBeforeAnswerAndCitationResolution(t *testing.T) {
	pages := map[string]string{"notes/x.md": "value from e1", "sessions/evidence/e1.json": `{"event":{"eventId":"e1","message":{"text":"value"}}}`}
	seen := false
	sources := map[string]bool{}
	coverage := map[string]int{}
	answer := `{"action":"answer","paths":[],"offset":0,"query":"","answer_json":"{\"value\":\"value\",\"citations\":{\"value\":[\"e1\"]}}"}`
	call := model.Call{Name: "memory_step", Arguments: json.RawMessage(answer)}
	value, artifact := controlledStep("notes_sessions", pages, call, &seen, sources, coverage)
	if artifact != nil || value.(map[string]any)["error"] == nil {
		t.Fatal("accepted without reading")
	}
	controlledStep("notes_sessions", pages, model.Call{Name: "memory_step", Arguments: json.RawMessage(`{"action":"read","paths":["notes/x.md"],"offset":0,"query":"","answer_json":""}`)}, &seen, sources, coverage)
	if _, artifact = controlledStep("notes_sessions", pages, call, &seen, sources, coverage); artifact != nil {
		t.Fatal("accepted unresolved citation")
	}
	controlledStep("notes_sessions", pages, model.Call{Name: "memory_step", Arguments: json.RawMessage(`{"action":"read","paths":["sessions/evidence/e1.json"],"offset":0,"query":"","answer_json":""}`)}, &seen, sources, coverage)
	if _, artifact = controlledStep("notes_sessions", pages, call, &seen, sources, coverage); artifact == nil {
		t.Fatal("rejected resolved evidence")
	}
	if _, artifact = controlledStep("existing_memory", pages, call, new(bool), map[string]bool{}, map[string]int{}); artifact == nil {
		t.Fatal("baseline should cite supplied source without tools")
	}
}

func TestControlledProviderProtocol(t *testing.T) {
	calls := 0
	host := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req map[string]any
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatal(err)
		}
		if req["tool_choice"].(map[string]any)["name"] != "memory_step" {
			t.Error("action not required")
		}
		args := map[string]any{"action": "read", "paths": []string{"sessions/evidence/e1.json"}, "offset": 0, "query": "", "answer_json": ""}
		if calls > 0 {
			args = map[string]any{"action": "answer", "paths": []string{}, "offset": 0, "query": "", "answer_json": `{"value":"remembered","citations":{"value":["e1"]}}`}
		}
		calls++
		encoded, _ := json.Marshal(args)
		if err := json.NewEncoder(w).Encode(map[string]any{"output": []any{map[string]any{"type": "function_call", "call_id": fmt.Sprint(calls), "name": "memory_step", "arguments": string(encoded)}}, "usage": map[string]any{"input_tokens": 100, "output_tokens": 20}}); err != nil {
			t.Error(err)
		}
	}))
	defer host.Close()
	cfg := domain.ModelConfig{Provider: "openai", Model: "fixture", Endpoint: host.URL}
	pages := map[string]string{"knowledge/index.md": "source e1", "sessions/evidence/e1.json": `{"event":{"eventId":"e1","message":{"text":"remembered"}}}`}
	result := run(context.Background(), request{Condition: "trace2mem", Protocol: "controlled_v1", MaxTokens: 32000}, cfg, snapshot{Revision: "r1"}, pages)
	if calls != 2 || result["error"] != nil {
		t.Fatal(calls, result["error"])
	}
	if result["artifact"].(map[string]any)["value"] != "remembered" {
		t.Fatal(result["artifact"])
	}
	if len(result["evidence_read"].([]string)) != 1 {
		t.Fatal("resolved source missing")
	}
}

func TestPaginatedEvidenceCoverage(t *testing.T) {
	pages := map[string]string{"sessions/evidence/e1.json": strings.Repeat("é", 8001)}
	coverage := map[string]int{}
	sources := map[string]bool{}
	read := func(offset int) {
		b, _ := json.Marshal(map[string]any{"path": "sessions/evidence/e1.json", "offset": offset})
		recordRead(execute("notes_sessions", pages, model.Call{Name: "memory_read", Arguments: b}).(map[string]any), coverage, sources)
	}
	read(8000)
	if sources["e1"] {
		t.Fatal("skipped first range accepted")
	}
	read(0)
	if sources["e1"] {
		t.Fatal("incomplete source accepted")
	}
	read(8000)
	if !sources["e1"] {
		t.Fatal("complete paginated evidence rejected")
	}
}

func TestControlledRejectsToolFreeProvider(t *testing.T) {
	host := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"output":[{"type":"message","content":[{"type":"output_text","text":"{\"value\":\"guessed\",\"citations\":{}}"}]}],"usage":{"input_tokens":100,"output_tokens":20}}`)
	}))
	defer host.Close()
	result := run(context.Background(), request{Condition: "trace2mem", Protocol: "controlled_v1", MaxTokens: 32000}, domain.ModelConfig{Provider: "openai", Model: "fixture", Endpoint: host.URL}, snapshot{Revision: "r1"}, nil)
	if result["error"] == nil || len(result["artifact"].(map[string]any)) > 0 {
		t.Fatal("accepted tool-free final answer")
	}
}
