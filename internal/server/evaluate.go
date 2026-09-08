package server

import (
	"connectrpc.com/connect"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	trace2memv1 "github.com/tanwargenairesearch/trace2mem/gen/trace2mem/v1"
	"github.com/tanwargenairesearch/trace2mem/internal/domain"
	"github.com/tanwargenairesearch/trace2mem/internal/dream"
	"github.com/tanwargenairesearch/trace2mem/internal/model"
	"github.com/tanwargenairesearch/trace2mem/internal/store"
	"google.golang.org/protobuf/encoding/protojson"
	"net/http"
	"time"
)

type evalCase struct {
	Query string         `json:"query"`
	Facts []expectedFact `json:"facts"`
	Split string         `json:"split"`
}
type evaluationTraceKey struct{}
type evaluationTrace struct {
	Turns            []model.Turn `json:"turns"`
	Tools            []model.Tool `json:"tools"`
	SemanticStatuses []string     `json:"semantic_statuses"`
}
type evalResult struct {
	Repeat         int             `json:"repeat"`
	EvidenceRecall float64         `json:"retrieved_evidence_recall"`
	Error          string          `json:"error,omitempty"`
	Query          string          `json:"query"`
	Split          string          `json:"split"`
	Wiki           bool            `json:"wiki"`
	Correct        bool            `json:"correct"`
	Facts          []factScore     `json:"facts"`
	Recall         float64         `json:"answer_citation_recall"`
	LatencyMS      int64           `json:"latency_ms"`
	Answer         string          `json:"answer"`
	Generation     domain.Usage    `json:"generation_usage"`
	Embedding      domain.Usage    `json:"embedding_usage"`
	Trace          evaluationTrace `json:"trace"`
}
type evaluationPrices struct {
	GenerationInput  float64 `json:"generation_input_per_million_usd"`
	GenerationOutput float64 `json:"generation_output_per_million_usd"`
	EmbeddingInput   float64 `json:"embedding_input_per_million_usd"`
}

func (p evaluationPrices) cost(generation, embedding domain.Usage) float64 {
	return (float64(generation.Input)*p.GenerationInput + float64(generation.Output)*p.GenerationOutput + float64(embedding.Input)*p.EmbeddingInput) / 1e6
}

type evalReport struct {
	Repeats               int                 `json:"repeats"`
	Optimization          *optimizationReport `json:"optimization,omitempty"`
	Prices                *evaluationPrices   `json:"operator_prices,omitempty"`
	RetrievalCostUSD      *float64            `json:"retrieval_cost_usd"`
	CompilationGeneration domain.Usage        `json:"recorded_compilation_generation"`
	CompilationEmbedding  domain.Usage        `json:"recorded_compilation_embedding"`
	CompilationCostUSD    *float64            `json:"recorded_compilation_cost_usd"`

	Revision   string             `json:"revision"`
	Watermark  int64              `json:"watermark"`
	Generation int64              `json:"configuration_generation"`
	Cases      []evalCase         `json:"cases"`
	Results    []evalResult       `json:"results"`
	Tokens     int64              `json:"tokens"`
	Model      domain.ModelConfig `json:"model"`
	Prompt     string             `json:"retrieval_prompt"`
	Evidence   map[string]string  `json:"evidence"`
	CreatedAt  time.Time          `json:"created_at"`
	Scoring    string             `json:"scoring"`
}
type optimizationReport struct {
	BaselineID string       `json:"baseline_id"`
	Turns      []model.Turn `json:"turns"`
	Reply      model.Reply  `json:"reply"`
	Usage      domain.Usage `json:"usage"`
	LatencyMS  int64        `json:"latency_ms"`
	CostUSD    *float64     `json:"cost_usd"`
}
type retrievalPromptKey struct{}

func (s *Server) runEvaluation(ctx context.Context, p domain.Principal, sp string, cases []evalCase, prices *evaluationPrices, repeats int) (evalReport, error) {
	var out evalReport
	if repeats == 0 {
		repeats = 1
	}
	if repeats < 1 || repeats > 5 {
		return out, errors.New("repeats must be between 1 and 5")
	}
	out.Repeats = repeats
	out.Prices = prices
	if prices != nil && (prices.GenerationInput < 0 || prices.GenerationOutput < 0 || prices.EmbeddingInput < 0) {
		return out, errors.New("prices cannot be negative")
	}
	if err := validateCases(cases); err != nil {
		return out, err
	}
	if err := s.Store.DB.QueryRow(ctx, "SELECT generation FROM spaces WHERE tenant=$1 AND id=$2", p.Tenant, sp).Scan(&out.Generation); err != nil {
		return out, err
	}
	v, err := s.Store.Snapshot(ctx, p.Tenant, sp, "")
	if err != nil {
		return out, err
	}
	cfg, _, err := s.Store.Config(ctx, p.Tenant, sp)
	if err != nil {
		return out, err
	}
	cfg.Key, cfg.EmbeddingKey = "", ""
	out.Revision, out.Watermark, out.Cases, out.Model = v.Revision, v.Watermark, cases, cfg
	out.CreatedAt = time.Now().UTC()
	for operation, total := range map[string]*domain.Usage{"compilation/generation": &out.CompilationGeneration, "compilation/embedding": &out.CompilationEmbedding} {
		if err := s.Store.DB.QueryRow(ctx, "SELECT COALESCE(sum(input_tokens),0),COALESCE(sum(output_tokens),0),COALESCE(bool_or(estimated),false) FROM usage WHERE tenant=$1 AND space=$2 AND operation=$3 AND created_at<=$4", p.Tenant, sp, operation, out.CreatedAt).Scan(&total.Input, &total.Output, &total.Estimated); err != nil {
			return out, err
		}
	}
	if prices != nil {
		cost := prices.cost(out.CompilationGeneration, out.CompilationEmbedding)
		out.CompilationCostUSD = &cost
		out.RetrievalCostUSD = new(float64)
	}

	out.Scoring = "case-specific RE2 facts, same-line supporting citations and forbidden assertions; lexical support only, no independent semantic judge"
	out.Prompt = cfg.RetrievalPrompt
	if override, ok := ctx.Value(retrievalPromptKey{}).(string); ok {
		out.Prompt = override
	}
	out.Evidence = map[string]string{}
	published := map[string]bool{}
	for _, page := range v.Pages {
		for _, id := range page.Citations {
			published[id] = true
		}
	}
	for _, c := range cases {
		for _, f := range c.Facts {
			for _, id := range f.Citations {
				if !published[id] {
					return out, fmt.Errorf("expected citation %s is absent from pinned revision", id)
				}
				if _, ok := out.Evidence[id]; ok {
					continue
				}
				raw, err := s.Store.EventJSON(ctx, p.Tenant, sp, id)
				if err != nil {
					return out, err
				}
				var event trace2memv1.Event
				if err := protojson.Unmarshal(raw, &event); err != nil {
					return out, err
				}
				out.Evidence[id] = event.GetMessage().GetText() + "\n" + event.GetToolResult().GetText()
			}
		}
	}
	for repeat := 0; repeat < repeats; repeat++ {
		for caseIndex, c := range cases {
			for _, wiki := range conditionOrder(caseIndex, repeat) {
				measured, usage := dream.WithUsage(ctx)
				trace := evaluationTrace{}
				measured = context.WithValue(measured, evaluationTraceKey{}, &trace)
				start := time.Now()
				res, err := s.GetContext(measured, connect.NewRequest(&trace2memv1.GetContextRequest{Query: c.Query, WithoutWiki: !wiki, Revision: v.Revision}))
				var answer, failure string
				if err != nil {
					if ctx.Err() != nil {
						return out, ctx.Err()
					}
					failure = err.Error()
				} else {
					answer = res.Msg.Synthesis
				}
				var generation int64
				if err := s.Store.DB.QueryRow(ctx, "SELECT generation FROM spaces WHERE tenant=$1 AND id=$2", p.Tenant, sp).Scan(&generation); err != nil {
					return out, err
				}
				if generation != out.Generation || (res != nil && res.Msg.Revision != v.Revision) {
					return out, domain.ErrConflict
				}
				facts, recall, correct := scoreFacts(c, answer, out.Evidence)
				gen, embed := usage.Snapshot()
				out.Tokens += gen.Total() + embed.Total()
				if prices != nil {
					*out.RetrievalCostUSD += prices.cost(gen, embed)
				}
				out.Results = append(out.Results, evalResult{Repeat: repeat + 1, EvidenceRecall: retrievedEvidenceRecall(c, trace), Error: failure, Query: c.Query, Split: c.Split, Wiki: wiki, Correct: correct && failure == "", Facts: facts, Recall: recall, LatencyMS: time.Since(start).Milliseconds(), Answer: answer, Generation: gen, Embedding: embed, Trace: trace})
				b, err := json.Marshal(out)
				if err != nil {
					return out, err
				}
				if len(b) > 16<<20 {
					return out, errors.New("evaluation report exceeds 16 MiB; split the cases")
				}
			}
		}
	}
	return out, nil
}
func (s *Server) evaluate(w http.ResponseWriter, r *http.Request, p domain.Principal, sp string) {
	if r.Method != "POST" {
		http.Error(w, "POST required", 405)
		return
	}
	var a struct {
		Repeats int               `json:"repeats,omitempty"`
		Cases   []evalCase        `json:"cases"`
		Prices  *evaluationPrices `json:"prices,omitempty"`
	}
	if !decode(w, r, &a) {
		return
	}
	if a.Repeats < 0 || a.Repeats > 5 {
		http.Error(w, "repeats must be between 1 and 5 (or omitted)", 400)
		return
	}
	if err := validateCases(a.Cases); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Minute)
	defer cancel()
	report, e := s.runEvaluation(ctx, p, sp, a.Cases, a.Prices, a.Repeats)
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
	var generation int64
	e = tx.QueryRow(ctx, "SELECT revision,generation FROM spaces WHERE tenant=$1 AND id=$2 FOR UPDATE", p.Tenant, sp).Scan(&current, &generation)
	if e != nil {
		failure(w, e)
		return
	}
	if current != report.Revision || generation != report.Generation {
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
	respond(w, map[string]any{"id": id, "report": report, "scoring": report.Scoring})
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
		var current string
		var generation int64
		var pending bool
		if e = tx.QueryRow(ctx, "SELECT revision,generation,pending_model IS NOT NULL FROM spaces WHERE tenant=$1 AND id=$2 FOR UPDATE", p.Tenant, sp).Scan(&current, &generation, &pending); e != nil {
			failure(w, e)
			return
		}
		var prompt string
		var testedRevision string
		var testedGeneration int64
		e = tx.QueryRow(ctx, "SELECT c.prompt,e.revision,(e.report->>'configuration_generation')::bigint FROM candidates c JOIN evaluations e ON e.id=c.evaluation_id AND e.tenant=c.tenant AND e.space=c.space WHERE c.tenant=$1 AND c.space=$2 AND c.id=$3", p.Tenant, sp, a.PromoteID).Scan(&prompt, &testedRevision, &testedGeneration)
		if e != nil {
			failure(w, domain.ErrNotFound)
			return
		}
		if pending || current != testedRevision || generation != testedGeneration {
			failure(w, domain.ErrConflict)
			return
		}
		_, e = tx.Exec(ctx, "UPDATE spaces SET generation=generation+1,model=jsonb_set(model,'{retrieval_prompt}',to_jsonb($3::text)) WHERE tenant=$1 AND id=$2", p.Tenant, sp, prompt)
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
	var baselineCurrent string
	var baselineGeneration int64
	if e := s.Store.DB.QueryRow(ctx, "SELECT revision,generation FROM spaces WHERE tenant=$1 AND id=$2", p.Tenant, sp).Scan(&baselineCurrent, &baselineGeneration); e != nil {
		failure(w, e)
		return
	}
	if baselineCurrent != baseline.Revision || baseline.Revision == "" || baselineGeneration != baseline.Generation {
		failure(w, domain.ErrConflict)
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
	optimization := optimizationReport{BaselineID: a.EvaluationID, Turns: []model.Turn{{Role: "system", Text: "Propose a concise retrieval instruction to improve these development results. Preserve evidence citation and untrusted-data handling. Output only the instruction."}, {Role: "user", Text: string(dev)}}}
	measured, usage := dream.WithUsage(ctx)
	started := time.Now()
	reply, e := provider.Generate(measured, optimization.Turns, nil)
	if e != nil {
		failure(w, e)
		return
	}
	optimization.Reply = reply
	optimization.Usage, _ = usage.Snapshot()
	optimization.LatencyMS = time.Since(started).Milliseconds()
	if baseline.Prices != nil {
		cost := baseline.Prices.cost(optimization.Usage, domain.Usage{})
		optimization.CostUSD = &cost
	}

	if reply.Text == "" {
		http.Error(w, "empty candidate", 422)
		return
	}
	report, e := s.runEvaluation(context.WithValue(ctx, retrievalPromptKey{}, reply.Text), p, sp, heldout, baseline.Prices, baseline.Repeats)
	if e != nil {
		failure(w, e)
		return
	}
	report.Optimization = &optimization
	id, eid := store.ID(), store.ID()
	rb, _ := json.Marshal(report)
	tx, e := s.Store.DB.Begin(ctx)
	if e != nil {
		failure(w, e)
		return
	}
	defer tx.Rollback(ctx)
	var current string
	var generation int64
	e = tx.QueryRow(ctx, "SELECT revision,generation FROM spaces WHERE tenant=$1 AND id=$2 FOR UPDATE", p.Tenant, sp).Scan(&current, &generation)
	if e != nil {
		failure(w, e)
		return
	}
	if current != report.Revision || current != baseline.Revision || generation != report.Generation || generation != baseline.Generation {
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

func conditionOrder(caseIndex, repeat int) []bool {
	if (caseIndex+repeat)%2 == 0 {
		return []bool{true, false}
	}
	return []bool{false, true}
}
func retrievedEvidenceRecall(c evalCase, trace evaluationTrace) float64 {
	expected := map[string]bool{}
	for _, f := range c.Facts {
		for _, id := range f.Citations {
			expected[id] = true
		}
	}
	read := map[string]bool{}
	for _, turn := range trace.Turns {
		if turn.Result == nil || turn.Result.Name != "memory_evidence" {
			continue
		}
		var result trace2memv1.GetEvidenceResponse
		if protojson.Unmarshal([]byte(turn.Result.Text), &result) == nil && result.Event != nil {
			read[result.Event.EventId] = true
		}
	}
	matched := 0
	for id := range expected {
		if read[id] {
			matched++
		}
	}
	if len(expected) == 0 {
		return 0
	}
	return float64(matched) / float64(len(expected))
}
