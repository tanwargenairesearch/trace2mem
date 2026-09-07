package dream

import (
	"context"
	"encoding/json"
	"errors"
	brainv1 "github.com/brainmemory/brain/gen/brain/v1"
	"github.com/brainmemory/brain/internal/domain"
	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/encoding/protojson"
	"strings"
)

func appendUnique(dst, src []domain.Record) []domain.Record {
	seen := map[string]bool{}
	for _, r := range dst {
		seen[r.ID] = true
	}
	for _, r := range src {
		if !seen[r.ID] {
			dst = append(dst, r)
			seen[r.ID] = true
		}
	}
	return dst
}
func (e *Engine) prior(ctx context.Context, l domain.Lease, query string) (string, []domain.Record, error) {
	if len(query) < 1 {
		return "[]", nil, nil
	}
	rows, err := e.Store.DB.Query(ctx, "SELECT content,citations FROM pages WHERE tenant=$1 AND space=$2 AND revision=$3 AND path LIKE 'notes/%' AND strpos(lower(content),lower($4))>0 ORDER BY path LIMIT 64", l.Tenant, l.Space, l.Parent, query)
	if err != nil {
		return "", nil, err
	}
	var notes []json.RawMessage
	ids := map[string]bool{}
	size := 0
	for rows.Next() {
		var content string
		var cites []string
		if err = rows.Scan(&content, &cites); err != nil {
			rows.Close()
			return "", nil, err
		}
		size += len(content)
		if size > 128<<10 {
			rows.Close()
			return "", nil, errors.New("prior subject exceeds tool budget; narrow the query")
		}
		notes = append(notes, json.RawMessage(content))
		for _, id := range cites {
			ids[id] = true
		}
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return "", nil, err
	}
	records, err := e.sources(ctx, l, ids)
	if err != nil {
		return "", nil, err
	}
	b, _ := json.Marshal(map[string]any{"notes": notes, "sources": records})
	return string(b), records, nil
}
func (e *Engine) sources(ctx context.Context, l domain.Lease, ids map[string]bool) ([]domain.Record, error) {
	out := []domain.Record{}
	for id := range ids {
		b, err := e.Store.EventJSON(ctx, l.Tenant, l.Space, id)
		if err != nil {
			return nil, err
		}
		var v brainv1.Event
		if err = protojson.Unmarshal(b, &v); err != nil {
			return nil, err
		}
		out = append(out, Record(&v))
	}
	return out, nil
}
func (e *Engine) merge(ctx context.Context, l domain.Lease, updates []domain.Observation, records []domain.Record) ([]domain.Observation, []domain.Record, error) {
	subjects := map[string]bool{}
	replaced := map[string]bool{}
	keys := map[string]bool{}
	for _, o := range updates {
		subjects[o.Subject] = true
		keys[o.Subject+"\x00"+strings.Join(o.Citations, ",")] = true
		for _, id := range o.Supersedes {
			replaced[id] = true
		}
	}
	merged := append([]domain.Observation{}, updates...)
	ids := map[string]bool{}
	for subject := range subjects {
		rows, err := e.Store.DB.Query(ctx, "SELECT content FROM pages WHERE tenant=$1 AND space=$2 AND revision=$3 AND path LIKE $4 ORDER BY path LIMIT 1001", l.Tenant, l.Space, l.Parent, "notes/"+slug(subject)+"/%")
		if err != nil {
			return nil, nil, err
		}
		count := 0
		for rows.Next() {
			count++
			if count > 1000 {
				rows.Close()
				return nil, nil, errors.New("subject observation budget exceeded")
			}
			var content string
			if err = rows.Scan(&content); err != nil {
				rows.Close()
				return nil, nil, err
			}
			var old domain.Observation
			if err = json.Unmarshal([]byte(content), &old); err != nil {
				rows.Close()
				return nil, nil, err
			}
			if keys[old.Subject+"\x00"+strings.Join(old.Citations, ",")] {
				continue
			}
			for _, id := range old.Citations {
				ids[id] = true
				if replaced[id] {
					old.Status = "superseded"
				}
			}
			merged = append(merged, old)
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return nil, nil, err
		}
	}
	extra, err := e.sources(ctx, l, ids)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil, domain.ErrLease
	}
	return merged, appendUnique(records, extra), err
}
