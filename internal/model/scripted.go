package model

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/trace2mem/trace2mem/internal/domain"
	"sort"
	"strings"
)

// Scripted is a deterministic fixture provider, never a substitute for evaluated model quality.
type Scripted struct{}

func (Scripted) Generate(ctx context.Context, t []Turn, tools []Tool) (Reply, error) {
	if e := ctx.Err(); e != nil {
		return Reply{}, e
	}
	r := Reply{Usage: domain.Usage{Input: 1, Output: 1}}
	has := func(n string) bool {
		for _, v := range tools {
			if v.Name == n {
				return true
			}
		}
		return false
	}
	if has("probe") {
		r.Calls = []Call{{"probe", "probe", json.RawMessage(`{"value":"ok"}`)}}
		return r, nil
	}
	if has("compose_wiki") {
		var input struct {
			Observations []domain.Observation `json:"observations"`
			Evidence     []domain.Record      `json:"evidence"`
		}
		if err := json.Unmarshal([]byte(t[len(t)-1].Text), &input); err != nil {
			return Reply{}, err
		}
		sessions := map[string]string{}
		subjects := map[string]string{}
		for _, v := range input.Evidence {
			text := []rune(v.Text)
			if len(text) > 80 {
				text = text[:80]
			}
			sessions[v.Session] += fmt.Sprintf("%s reported %s [cite:%s]. ", v.Role, string(text), v.ID)
		}
		for _, o := range input.Observations {
			subjects[o.Subject] += fmt.Sprintf("%s (%s, %s)", o.Text, o.Status, o.Origin)
			for _, id := range o.Citations {
				subjects[o.Subject] += " [cite:" + id + "]"
			}
			subjects[o.Subject] += "\n\n"
		}
		list := func(values map[string]string) []map[string]any {
			keys := []string{}
			for k := range values {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			out := []map[string]any{}
			for _, k := range keys {
				out = append(out, map[string]any{"name": k, "text": values[k], "related": []string{}, "removed_links": []any{}})
			}
			return out
		}
		b, _ := json.Marshal(map[string]any{"sessions": list(sessions), "subjects": list(subjects)})
		r.Calls = []Call{{ID: "compose", Name: "compose_wiki", Arguments: b}}
		return r, nil
	}
	if has("read_history") {
		if len(t) < 3 {
			r.Calls = []Call{{"history", "read_history", json.RawMessage(`{}`)}}
			return r, nil
		}
		var records []domain.Record
		for _, v := range t {
			if v.Result != nil && v.Result.Name == "read_history" {
				if err := json.Unmarshal([]byte(v.Result.Text), &records); err != nil {
					return Reply{}, err
				}
			}
		}
		wikiRead := false
		old := []domain.Observation{}
		for _, v := range t {
			if v.Result != nil && v.Result.Name == "read_wiki" {
				wikiRead = true
				var a struct {
					Notes []domain.Observation `json:"notes"`
				}
				if err := json.Unmarshal([]byte(v.Result.Text), &a); err != nil {
					return Reply{}, err
				}
				old = a.Notes
			}
		}
		if !wikiRead && len(records) > 0 {
			subject, _, _ := strings.Cut(records[0].Text, ": ")
			b, _ := json.Marshal(map[string]string{"query": subject})
			r.Calls = []Call{{ID: "wiki", Name: "read_wiki", Arguments: b}}
			return r, nil
		}
		obs := []domain.Observation{}
		for _, v := range records {
			if strings.TrimSpace(v.Text) == "" {
				continue
			}
			sub := "general"
			text := v.Text
			if a, b, ok := strings.Cut(text, ": "); ok && len(a) < 60 {
				sub = a
				text = b
			}
			for i := range obs {
				if obs[i].Subject == sub {
					obs[i].Status = "superseded"
				}
			}
			supersedes := []string{}
			for _, o := range old {
				if o.Subject == sub && o.Status == "current" {
					supersedes = append(supersedes, o.StableID())
				}
			}
			obs = append(obs, domain.Observation{Subject: sub, Text: text, Origin: v.Role, Status: "current", Citations: []string{v.ID}, Supersedes: supersedes})
		}
		b, _ := json.Marshal(map[string]any{"observations": obs})
		r.Calls = []Call{{"proposal", "propose", b}}
		return r, nil
	}
	if has("verify") {
		r.Calls = []Call{{"verify", "verify", json.RawMessage(`{"supported":true,"reason":"scripted fixture verification"}`)}}
		return r, nil
	}
	if has("search") {
		if len(t) < 3 {
			r.Calls = []Call{{"search", "search", json.RawMessage(`{"query":"","limit":8}`)}}
			return r, nil
		}
		if t[len(t)-1].Result != nil {
			r.Text = t[len(t)-1].Result.Text
		} else {
			r.Text = t[len(t)-1].Text
		}
		return r, nil
	}
	r.Text = "Use explicit temporal evidence and preserve citations."
	return r, nil
}
func (Scripted) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	out := make([][]float32, len(texts))
	for i, t := range texts {
		v := make([]float32, 16)
		for _, r := range strings.ToLower(t) {
			v[int(r)%16]++
		}
		out[i] = v
	}
	return out, ctx.Err()
}
