package model

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/trace2mem/trace2mem/internal/domain"
	"io"
	"net/http"
	"strings"
	"time"
)

type Tool struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  map[string]any `json:"parameters"`
}
type Call struct {
	ID        string          `json:"id"`
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}
type ToolResult struct {
	ID   string
	Name string
	Text string
}
type Turn struct {
	Calls  []Call
	Result *ToolResult
	Role   string `json:"role"`
	Text   string `json:"text"`
}
type Reply struct {
	Text  string
	Calls []Call
	Usage domain.Usage
}
type Generator interface {
	Generate(context.Context, []Turn, []Tool) (Reply, error)
}
type Embedder interface {
	Embed(context.Context, []string) ([][]float32, error)
}
type Provider interface {
	Generator
	Embedder
}
type HTTP struct {
	Config domain.ModelConfig
	Client *http.Client
}

type combined struct {
	Generator
	Embedder
}

func New(c domain.ModelConfig) (Provider, error) {
	for _, m := range []string{c.Model, c.EmbeddingModel} {
		if strings.TrimSpace(m) == "" || strings.Contains(m, "REPLACE_") || m == "latest" || strings.HasSuffix(m, ":latest") {
			return nil, errors.New("explicit generation and embedding model versions required")
		}
	}

	if c.EmbeddingProvider == "" || c.EmbeddingModel == "" {
		return nil, errors.New("explicit embedding provider and model required")
	}
	gen, err := newGeneration(c)
	if err != nil {
		return nil, err
	}
	var embed Embedder
	switch c.EmbeddingProvider {
	case "scripted":
		embed = Scripted{}
	case "gemini", "vertex":
		embed = &Gemini{Config: c}
	case "openai", "ollama":
		ec := c
		ec.Provider = c.EmbeddingProvider
		ec.Endpoint = c.EmbeddingEndpoint
		ec.Key = c.EmbeddingKey
		if ec.Provider == "openai" && ec.Endpoint == "" {
			ec.Endpoint = "https://api.openai.com/v1"
		}
		if ec.Provider == "ollama" && ec.Endpoint == "" {
			return nil, errors.New("Ollama embedding endpoint required")
		}
		embed = &HTTP{Config: ec, Client: &http.Client{Timeout: 90 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return errors.New("redirect disabled") }}}
	default:
		return nil, errors.New("unsupported embedding provider")
	}
	return combined{Generator: gen, Embedder: chunked{embed}}, nil
}
func newGeneration(c domain.ModelConfig) (Provider, error) {
	switch c.Provider {
	case "scripted":
		return Scripted{}, nil
	case "openai", "ollama":
		if c.Model == "" || c.EmbeddingModel == "" {
			return nil, errors.New("explicit generation and embedding models required")
		}
		if c.Endpoint == "" {
			if c.Provider == "openai" {
				c.Endpoint = "https://api.openai.com/v1"
			} else {
				c.Endpoint = "http://localhost:11434"
			}
		}
		return &HTTP{c, &http.Client{Timeout: 90 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return errors.New("model endpoint redirects are disabled") }}}, nil
	default:
		return nil, errors.New("configure a model provider before compilation")
	}
}
func (p *HTTP) post(ctx context.Context, path string, in, out any) error {
	body, e := json.Marshal(in)
	if e != nil {
		return e
	}
	for attempt := 0; attempt < 3; attempt++ {
		req, e := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(p.Config.Endpoint, "/")+path, bytes.NewReader(body))
		if e != nil {
			return e
		}
		req.Header.Set("Content-Type", "application/json")
		if p.Config.Key != "" {
			req.Header.Set("Authorization", "Bearer "+p.Config.Key)
		}
		res, e := p.Client.Do(req)
		if e != nil {
			return fmt.Errorf("model request failed: %w", e)
		}
		b, e := io.ReadAll(io.LimitReader(res.Body, 8<<20))
		res.Body.Close()
		if e != nil {
			return e
		}
		if res.StatusCode == 429 || res.StatusCode >= 500 {
			if attempt < 2 {
				timer := time.NewTimer(time.Duration(attempt+1) * time.Second)
				select {
				case <-ctx.Done():
					timer.Stop()
					return ctx.Err()
				case <-timer.C:
					continue
				}
			}
		}
		if res.StatusCode/100 != 2 {
			return fmt.Errorf("model returned HTTP %d", res.StatusCode)
		}
		return json.Unmarshal(b, out)
	}
	return errors.New("model retry limit reached")
}
func (p *HTTP) Generate(ctx context.Context, turns []Turn, tools []Tool) (Reply, error) {
	if p.Config.Provider == "ollama" {
		return p.ollama(ctx, turns, tools)
	}
	msgs := []map[string]any{}
	for _, t := range turns {
		if t.Result != nil {
			msgs = append(msgs, map[string]any{"type": "function_call_output", "call_id": t.Result.ID, "output": t.Result.Text})
			continue
		}
		if t.Text != "" {
			msgs = append(msgs, map[string]any{"role": t.Role, "content": t.Text})
		}
		for _, call := range t.Calls {
			msgs = append(msgs, map[string]any{"type": "function_call", "call_id": call.ID, "name": call.Name, "arguments": string(call.Arguments)})
		}
	}

	ts := []map[string]any{}
	for _, t := range tools {
		ts = append(ts, map[string]any{"type": "function", "name": t.Name, "description": t.Description, "parameters": t.Parameters, "strict": true})
	}
	var res struct {
		Output []struct {
			Type      string `json:"type"`
			ID        string `json:"call_id"`
			Name      string `json:"name"`
			Arguments string `json:"arguments"`
			Content   []struct {
				Text string `json:"text"`
			} `json:"content"`
		} `json:"output"`
		Usage struct {
			Input  int64 `json:"input_tokens"`
			Output int64 `json:"output_tokens"`
		} `json:"usage"`
	}
	limit := p.Config.MaxTokens
	if limit <= 0 || limit > 4096 {
		limit = 4096
	}
	e := p.post(ctx, "/responses", map[string]any{"model": p.Config.Model, "input": msgs, "tools": ts, "store": false, "max_output_tokens": limit}, &res)
	out := Reply{Usage: domain.Usage{Input: res.Usage.Input, Output: res.Usage.Output}}
	for _, o := range res.Output {
		if o.Type == "function_call" {
			out.Calls = append(out.Calls, Call{o.ID, o.Name, json.RawMessage(o.Arguments)})
		}
		for _, c := range o.Content {
			out.Text += c.Text
		}
	}
	return out, e
}
func (p *HTTP) ollama(ctx context.Context, turns []Turn, tools []Tool) (Reply, error) {
	msgs := []map[string]any{}
	for _, t := range turns {
		if t.Result != nil {
			msgs = append(msgs, map[string]any{"role": "tool", "content": t.Result.Text, "tool_name": t.Result.Name})
			continue
		}
		msg := map[string]any{"role": t.Role, "content": t.Text}
		if len(t.Calls) > 0 {
			calls := []map[string]any{}
			for _, c := range t.Calls {
				var args any
				if err := json.Unmarshal(c.Arguments, &args); err != nil {
					return Reply{}, err
				}
				calls = append(calls, map[string]any{"function": map[string]any{"name": c.Name, "arguments": args}})
			}
			msg["tool_calls"] = calls
		}
		msgs = append(msgs, msg)
	}

	ts := []map[string]any{}
	for _, t := range tools {
		ts = append(ts, map[string]any{"type": "function", "function": t})
	}
	var res struct {
		Message struct {
			Content string `json:"content"`
			Calls   []struct {
				Function struct {
					Name      string          `json:"name"`
					Arguments json.RawMessage `json:"arguments"`
				} `json:"function"`
			} `json:"tool_calls"`
		} `json:"message"`
		Input  int64 `json:"prompt_eval_count"`
		Output int64 `json:"eval_count"`
	}
	e := p.post(ctx, "/api/chat", map[string]any{"model": p.Config.Model, "messages": msgs, "tools": ts, "stream": false, "options": map[string]any{"num_predict": 2048}}, &res)
	out := Reply{Text: res.Message.Content, Usage: domain.Usage{Input: res.Input, Output: res.Output}}
	for i, c := range res.Message.Calls {
		out.Calls = append(out.Calls, Call{fmt.Sprint(i), c.Function.Name, c.Function.Arguments})
	}
	return out, e
}
func (p *HTTP) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	if p.Config.Provider == "ollama" {
		var r struct {
			Embeddings [][]float32 `json:"embeddings"`
		}
		e := p.post(ctx, "/api/embed", map[string]any{"model": p.Config.EmbeddingModel, "input": texts}, &r)
		return r.Embeddings, e
	}
	var r struct {
		Data []struct {
			Index     int       `json:"index"`
			Embedding []float32 `json:"embedding"`
		} `json:"data"`
	}
	e := p.post(ctx, "/embeddings", map[string]any{"model": p.Config.EmbeddingModel, "input": texts}, &r)
	if e != nil {
		return nil, e
	}
	out := make([][]float32, len(texts))
	for _, v := range r.Data {
		if v.Index < 0 || v.Index >= len(out) {
			return nil, errors.New("invalid embedding index")
		}
		out[v.Index] = v.Embedding
	}
	return out, nil
}
func Object(properties map[string]any, required ...string) map[string]any {
	if required == nil {
		required = []string{}
	}
	return map[string]any{"type": "object", "properties": properties, "required": required, "additionalProperties": false}
}
func Probe(ctx context.Context, p Provider) error {
	tool := Tool{"probe", "Call this with value ok", Object(map[string]any{"value": map[string]any{"type": "string"}}, "value")}
	r, e := p.Generate(ctx, []Turn{{Role: "system", Text: "Call the probe tool with value ok."}}, []Tool{tool})
	if e != nil {
		return e
	}
	if len(r.Calls) != 1 || r.Calls[0].Name != "probe" {
		return errors.New("model did not support tool calling")
	}
	var v struct {
		Value string `json:"value"`
	}
	if json.Unmarshal(r.Calls[0].Arguments, &v) != nil || v.Value != "ok" {
		return errors.New("model did not produce valid structured arguments")
	}
	vec, e := p.Embed(ctx, []string{"probe"})
	if e != nil {
		return e
	}
	if len(vec) != 1 || len(vec[0]) == 0 {
		return errors.New("model did not produce embeddings")
	}
	return nil
}
