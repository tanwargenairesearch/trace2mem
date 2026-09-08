package model

import (
	"context"
	"encoding/json"
	"github.com/trace2mem/trace2mem/internal/domain"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestToolContinuation(t *testing.T) {
	for _, provider := range []string{"openai", "ollama"} {
		t.Run(provider, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var in map[string]json.RawMessage
				if e := json.NewDecoder(r.Body).Decode(&in); e != nil {
					t.Fatal(e)
				}
				var msgs []map[string]any
				if provider == "openai" {
					json.Unmarshal(in["input"], &msgs)
					if len(msgs) != 3 || msgs[1]["type"] != "function_call" || msgs[2]["type"] != "function_call_output" || msgs[1]["call_id"] != msgs[2]["call_id"] {
						t.Errorf("unpaired OpenAI call: %v", msgs)
					}
					w.Write([]byte(`{"output":[{"type":"reasoning","content":[{"type":"reasoning_text","text":"private reasoning [cite:placeholder]"}]},{"type":"message","content":[{"type":"output_text","text":"ok"},{"type":"reasoning_text","text":"excluded"}]}],"usage":{"input_tokens":10,"output_tokens":2}}`))
				} else {
					json.Unmarshal(in["messages"], &msgs)
					if len(msgs) != 3 || msgs[1]["role"] != "assistant" || msgs[1]["tool_calls"] == nil || msgs[2]["role"] != "tool" {
						t.Errorf("unpaired Ollama call: %v", msgs)
					}
					w.Write([]byte(`{"message":{"content":"ok"},"prompt_eval_count":10,"eval_count":2}`))
				}
			}))
			defer srv.Close()
			p := &HTTP{Config: domain.ModelConfig{Provider: provider, Endpoint: srv.URL, Model: "test"}, Client: srv.Client()}
			r, e := p.Generate(context.Background(), []Turn{{Role: "user", Text: "question"}, {Role: "assistant", Calls: []Call{{ID: "call1", Name: "read", Arguments: json.RawMessage(`{}`)}}}, {Role: "tool", Result: &ToolResult{ID: "call1", Name: "read", Text: "evidence"}}}, nil)
			if e != nil || r.Text != "ok" {
				t.Fatalf("%v %v", r, e)
			}
		})
	}
}

func TestZeroArgumentToolSchema(t *testing.T) {
	b, e := json.Marshal(Object(map[string]any{}))
	if e != nil {
		t.Fatal(e)
	}
	var schema map[string]any
	if e = json.Unmarshal(b, &schema); e != nil {
		t.Fatal(e)
	}
	required, ok := schema["required"].([]any)
	if !ok || len(required) != 0 {
		t.Fatalf("invalid no-argument schema %s", b)
	}
}
