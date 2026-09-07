package server

import (
	"connectrpc.com/connect"
	"context"
	"encoding/json"
	brainv1 "github.com/brainmemory/brain/gen/brain/v1"
	"github.com/brainmemory/brain/internal/domain"
	"github.com/brainmemory/brain/internal/model"
	"github.com/brainmemory/brain/internal/store"
	"net/http"
	"strings"
	"time"
)

type evalCase struct {
	Query     string   `json:"query"`
	Expected  string   `json:"expected"`
	Citations []string `json:"citations"`
	Split     string   `json:"split"`
}
type evalResult struct {
	Query     string  `json:"query"`
	Split     string  `json:"split"`
	Wiki      bool    `json:"wiki"`
	Correct   bool    `json:"correct"`
	Recall    float64 `json:"recall"`
	LatencyMS int64   `json:"latency_ms"`
	Answer    string  `json:"answer"`
}
type evalReport struct {
	Revision string             `json:"revision"`
	Cases    []evalCase         `json:"cases"`
	Results  []evalResult       `json:"results"`
	Tokens   int64              `json:"tokens"`
	Model    domain.ModelConfig `json:"model"`
}
type retrievalPromptKey struct{}

func (s *Server) runEvaluation(ctx context.Context, p domain.Principal, sp string, cases []evalCase) (evalReport, error) {
	v, e := s.Store.Snapshot(ctx, p.Tenant, sp, "")
	if e != nil {
		return evalReport{}, e
	}
	cfg, _, e := s.Store.Config(ctx, p.Tenant, sp)
	if e != nil {
		return evalReport{}, e
	}
	cfg.Key = ""
	out := evalReport{Revision: v.Revision, Cases: cases, Model: cfg}
	before, e := s.Store.Used(ctx, p.Tenant, sp)
	if e != nil {
		return out, e
	}
	for _, c := range cases {
		for _, wiki := range []bool{true, false} {
			start := time.Now()
			res, e := s.GetContext(ctx, connect.NewRequest(&brainv1.GetContextRequest{SpaceId: sp, Query: c.Query, WithoutWiki: !wiki}))
			if e != nil {
				return out, e
			}
			if res.Msg.Revision != v.Revision {
				return out, domain.ErrConflict
			}
			hit := 0
			for _, id := range c.Citations {
				if strings.Contains(res.Msg.Synthesis, "[cite:"+id+"]") {
					hit++
				}
			}
			recall := 1.
			if len(c.Citations) > 0 {
				recall = float64(hit) / float64(len(c.Citations))
			}
			out.Results = append(out.Results, evalResult{Query: c.Query, Split: c.Split, Wiki: wiki, Correct: strings.Contains(strings.ToLower(res.Msg.Synthesis), strings.ToLower(c.Expected)), Recall: recall, LatencyMS: time.Since(start).Milliseconds(), Answer: res.Msg.Synthesis})
		}
	}
	after, e := s.Store.Used(ctx, p.Tenant, sp)
	out.Tokens = after - before
	return out, e
}
func (s *Server) evaluate(w http.ResponseWriter, r *http.Request, p domain.Principal, sp string) {
	if r.Method != "POST" {
		http.Error(w, "POST required", 405)
		return
	}
	var a struct {
		Cases []evalCase `json:"cases"`
	}
	if !decode(w, r, &a) {
		return
	}
	if len(a.Cases) < 1 || len(a.Cases) > 20 {
		http.Error(w, "provide 1–20 cases", 400)
		return
	}
	for _, c := range a.Cases {
		if c.Query == "" || c.Expected == "" || (c.Split != "development" && c.Split != "heldout") {
			http.Error(w, "query, expected and development/heldout split required", 400)
			return
		}
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Minute)
	defer cancel()
	report, e := s.runEvaluation(ctx, p, sp, a.Cases)
	if e != nil {
		failure(w, e)
		return
	}
	id := store.ID()
	b, _ := json.Marshal(report)
	tx, e := s.Store.DB.Begin(ctx)
	if e != nil {
		failure(w, e)
		return
	}
	defer tx.Rollback(ctx)
	var current string
	e = tx.QueryRow(ctx, "SELECT revision FROM spaces WHERE tenant=$1 AND id=$2 FOR UPDATE", p.Tenant, sp).Scan(&current)
	if e != nil {
		failure(w, e)
		return
	}
	if current != report.Revision {
		failure(w, domain.ErrConflict)
		return
	}
	_, e = tx.Exec(ctx, "INSERT INTO evaluations(id,tenant,space,revision,report) VALUES($1,$2,$3,$4,$5)", id, p.Tenant, sp, report.Revision, b)
	if e != nil {
		failure(w, e)
		return
	}
	if e = tx.Commit(ctx); e != nil {
		failure(w, e)
		return
	}
	respond(w, map[string]any{"id": id, "report": report, "scoring": "case-insensitive expected substring and exact citation recall; not an independent correctness judge"})
}
func (s *Server) candidates(w http.ResponseWriter, r *http.Request, p domain.Principal, sp string) {
	if e := s.Store.Owner(r.Context(), p, sp); e != nil {
		failure(w, e)
		return
	}
	if r.Method != "POST" {
		http.Error(w, "POST required", 405)
		return
	}
	var a struct {
		EvaluationID string `json:"evaluation_id"`
		PromoteID    string `json:"promote_id"`
	}
	if !decode(w, r, &a) {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Minute)
	defer cancel()
	if a.PromoteID != "" {
		tx, e := s.Store.DB.Begin(ctx)
		if e != nil {
			failure(w, e)
			return
		}
		defer tx.Rollback(ctx)
		var prompt string
		e = tx.QueryRow(ctx, "SELECT prompt FROM candidates WHERE tenant=$1 AND space=$2 AND id=$3", p.Tenant, sp, a.PromoteID).Scan(&prompt)
		if e != nil {
			failure(w, domain.ErrNotFound)
			return
		}
		_, e = tx.Exec(ctx, "UPDATE spaces SET model=jsonb_set(model,'{retrieval_prompt}',to_jsonb($3::text)) WHERE tenant=$1 AND id=$2", p.Tenant, sp, prompt)
		if e == nil {
			_, e = tx.Exec(ctx, "UPDATE candidates SET promoted=true WHERE id=$1", a.PromoteID)
		}
		if e == nil {
			e = tx.Commit(ctx)
		}
		if e != nil {
			failure(w, e)
			return
		}
		respond(w, map[string]bool{"promoted": true})
		return
	}
	var b []byte
	if e := s.Store.DB.QueryRow(ctx, "SELECT report FROM evaluations WHERE tenant=$1 AND space=$2 AND id=$3", p.Tenant, sp, a.EvaluationID).Scan(&b); e != nil {
		failure(w, domain.ErrNotFound)
		return
	}
	var baseline evalReport
	if e := json.Unmarshal(b, &baseline); e != nil {
		failure(w, e)
		return
	}
	development := []evalResult{}
	heldout := []evalCase{}
	for _, v := range baseline.Results {
		if v.Split == "development" {
			development = append(development, v)
		}
	}
	for _, v := range baseline.Cases {
		if v.Split == "heldout" {
			heldout = append(heldout, v)
		}
	}
	if len(development) == 0 || len(heldout) == 0 {
		http.Error(w, "baseline must include development and heldout cases", 400)
		return
	}
	provider, _, e := s.Engine.Provider(ctx, p.Tenant, sp)
	if e != nil {
		failure(w, e)
		return
	}
	dev, _ := json.Marshal(development)
	reply, e := provider.Generate(ctx, []model.Turn{{Role: "system", Text: "Propose a concise retrieval instruction to improve these development results. Preserve evidence citation and untrusted-data handling. Output only the instruction."}, {Role: "user", Text: string(dev)}}, nil)
	if e != nil {
		failure(w, e)
		return
	}

	if reply.Text == "" {
		http.Error(w, "empty candidate", 422)
		return
	}
	report, e := s.runEvaluation(context.WithValue(ctx, retrievalPromptKey{}, reply.Text), p, sp, heldout)
	if e != nil {
		failure(w, e)
		return
	}
	id, eid := store.ID(), store.ID()
	rb, _ := json.Marshal(report)
	tx, e := s.Store.DB.Begin(ctx)
	if e != nil {
		failure(w, e)
		return
	}
	defer tx.Rollback(ctx)
	var current string
	e = tx.QueryRow(ctx, "SELECT revision FROM spaces WHERE tenant=$1 AND id=$2 FOR UPDATE", p.Tenant, sp).Scan(&current)
	if e != nil {
		failure(w, e)
		return
	}
	if current != report.Revision {
		failure(w, domain.ErrConflict)
		return
	}
	_, e = tx.Exec(ctx, "INSERT INTO evaluations(id,tenant,space,revision,report) VALUES($1,$2,$3,$4,$5)", eid, p.Tenant, sp, report.Revision, rb)
	if e == nil {
		_, e = tx.Exec(ctx, "INSERT INTO candidates(id,tenant,space,prompt,evaluation_id) VALUES($1,$2,$3,$4,$5)", id, p.Tenant, sp, reply.Text, eid)
	}
	if e == nil {
		e = tx.Commit(ctx)
	}
	if e != nil {
		failure(w, e)
		return
	}
	respond(w, map[string]any{"id": id, "prompt": reply.Text, "heldout_report": report, "baseline_id": a.EvaluationID, "promoted": false})
}
