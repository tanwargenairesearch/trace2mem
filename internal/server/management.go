package server

import (
	"encoding/json"
	"github.com/trace2mem/trace2mem/internal/domain"
	"github.com/trace2mem/trace2mem/internal/model"
	"net/http"
	"strings"
	"time"
)

func (s *Server) management(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	p := principal(ctx)
	route := strings.TrimPrefix(r.URL.Path, "/api/")
	sp := r.URL.Query().Get("space")
	if route == "spaces" {
		if r.Method == "GET" {
			v, e := s.Store.Spaces(ctx, p)
			if e != nil {
				failure(w, e)
				return
			}
			respond(w, v)
			return
		}
		if r.Method == "POST" {
			var a struct {
				Name string `json:"name"`
			}
			if !decode(w, r, &a) {
				return
			}
			if strings.TrimSpace(a.Name) == "" || len(a.Name) > 200 {
				http.Error(w, "space name required", 400)
				return
			}
			id, e := s.Store.CreateSpace(ctx, p, a.Name)
			if e != nil {
				failure(w, e)
				return
			}
			respond(w, map[string]string{"id": id})
			return
		}
	}
	if route == "tokens" {
		if r.Method != "POST" {
			http.Error(w, "POST required", 405)
			return
		}
		var a struct {
			Scopes []string `json:"scopes"`
		}
		if !decode(w, r, &a) {
			return
		}
		v, e := s.Store.CreateToken(ctx, p, a.Scopes)
		if e != nil {
			failure(w, e)
			return
		}
		respond(w, map[string]string{"token": v})
		return
	}
	write := r.Method != "GET"
	if e := s.Store.Authorize(ctx, p, sp, write); e != nil {
		failure(w, e)
		return
	}
	switch route {
	case "model":
		if e := s.Store.Owner(ctx, p, sp); e != nil {
			failure(w, e)
			return
		}
		if r.Method == "GET" {
			c, _, e := s.Store.Config(ctx, p.Tenant, sp)
			if e != nil {
				failure(w, e)
				return
			}
			c.Key = ""
			c.EmbeddingKey = ""
			respond(w, c)
			return
		}
		if r.Method != "POST" {
			http.Error(w, "POST required", 405)
			return
		}
		var c domain.ModelConfig
		if !decode(w, r, &c) {
			return
		}
		if c.MaxSteps == 0 {
			c.MaxSteps = 12
		}
		if c.MaxTokens == 0 {
			c.MaxTokens = 32000
		}
		if c.DailyTokens == 0 {
			c.DailyTokens = 1000000
		}
		if c.MaxSteps < 1 || c.MaxSteps > 32 || c.MaxTokens < 100 || c.MaxTokens > 200000 || c.DailyTokens < 1 {
			http.Error(w, "invalid model budgets", 400)
			return
		}
		if ((c.Provider == "scripted" || c.EmbeddingProvider == "scripted") && !s.Config.Scripted) || !s.Config.EndpointAllowed(c.Provider, c.Endpoint) || !s.Config.EndpointAllowed(c.EmbeddingProvider, c.EmbeddingEndpoint) {
			http.Error(w, "provider or endpoint not enabled by operator", 400)
			return
		}
		provider, e := model.New(c)
		if e != nil {
			http.Error(w, e.Error(), 400)
			return
		}
		provider = s.Engine.Meter(provider, c, p.Tenant, sp)
		if e = model.Probe(ctx, provider); e != nil {
			http.Error(w, "model probe failed: "+e.Error(), 400)
			return
		}
		bundle, _ := json.Marshal(map[string]string{"generation": c.Key, "embedding": c.EmbeddingKey})
		key, e := s.Vault.Seal(ctx, bundle, []byte(p.Tenant+"/"+sp))
		if e == nil {
			e = s.Store.SetConfig(ctx, p.Tenant, sp, c, key)
		}
		if e != nil {
			failure(w, e)
			return
		}
		respond(w, map[string]bool{"configured": true})
	case "members":
		if e := s.Store.Owner(ctx, p, sp); e != nil {
			failure(w, e)
			return
		}
		if r.Method == "GET" {
			rows, e := s.Store.DB.Query(ctx, "SELECT subject,role FROM members WHERE tenant=$1 AND space=$2 ORDER BY subject", p.Tenant, sp)
			if e != nil {
				failure(w, e)
				return
			}
			defer rows.Close()
			out := []map[string]string{}
			for rows.Next() {
				var subject, role string
				if e = rows.Scan(&subject, &role); e != nil {
					failure(w, e)
					return
				}
				out = append(out, map[string]string{"subject": subject, "role": role})
			}
			if e = rows.Err(); e != nil {
				failure(w, e)
				return
			}
			respond(w, out)
			return
		}
		var a struct {
			Subject string `json:"subject"`
			Role    string `json:"role"`
		}
		if !decode(w, r, &a) {
			return
		}
		if a.Subject == "" || len(a.Subject) > 256 || (a.Role != "owner" && a.Role != "editor" && a.Role != "reader") {
			http.Error(w, "valid subject and role required", 400)
			return
		}
		if a.Subject == p.Subject && !p.Admin {
			http.Error(w, "cannot change own role", 400)
			return
		}
		_, e := s.Store.DB.Exec(ctx, "INSERT INTO members VALUES($1,$2,$3,$4) ON CONFLICT(tenant,space,subject) DO UPDATE SET role=$4", p.Tenant, sp, a.Subject, a.Role)
		if e != nil {
			failure(w, e)
			return
		}
		respond(w, map[string]bool{"saved": true})
	case "forget":
		if r.Method != "POST" {
			http.Error(w, "POST required", 405)
			return
		}
		if e := s.Store.Owner(ctx, p, sp); e != nil {
			failure(w, e)
			return
		}
		var a struct {
			EventID string `json:"event_id"`
		}
		if !decode(w, r, &a) {
			return
		}
		if !domain.ValidID(a.EventID) {
			http.Error(w, "invalid event ID", 400)
			return
		}
		if e := s.Store.Forget(ctx, p.Tenant, sp, a.EventID); e != nil {
			failure(w, e)
			return
		}
		respond(w, map[string]string{"status": "suppressed; recompilation queued"})
	case "usage":
		v, e := s.Store.Used(ctx, p.Tenant, sp)
		if e != nil {
			failure(w, e)
			return
		}
		respond(w, map[string]any{"daily_tokens": v, "as_of": time.Now().UTC()})
	case "compile":
		if r.Method != "POST" {
			http.Error(w, "POST required", 405)
			return
		}
		if e := s.Store.Schedule(ctx, p.Tenant, sp); e != nil {
			failure(w, e)
			return
		}
		respond(w, map[string]bool{"scheduled": true})
	case "revisions":
		rows, e := s.Store.DB.Query(ctx, "SELECT id,parent,watermark,created_at FROM revisions WHERE tenant=$1 AND space=$2 ORDER BY created_at DESC LIMIT 100", p.Tenant, sp)
		if e != nil {
			failure(w, e)
			return
		}
		defer rows.Close()
		out := []map[string]any{}
		for rows.Next() {
			var id, parent string
			var wm int64
			var at time.Time
			if e = rows.Scan(&id, &parent, &wm, &at); e != nil {
				failure(w, e)
				return
			}
			out = append(out, map[string]any{"id": id, "parent": parent, "watermark": wm, "created_at": at})
		}
		if e = rows.Err(); e != nil {
			failure(w, e)
			return
		}
		respond(w, out)
	case "export":
		v, e := s.Store.Snapshot(ctx, p.Tenant, sp, r.URL.Query().Get("revision"))
		if e != nil {
			failure(w, e)
			return
		}
		respond(w, v)
	case "evaluate":
		s.evaluate(w, r, p, sp)
	case "candidates":
		s.candidates(w, r, p, sp)
	default:
		http.NotFound(w, r)
	}
}
