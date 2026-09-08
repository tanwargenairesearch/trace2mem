package dream

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5"
	trace2memv1 "github.com/mohit-lendmind/trace2mem/gen/trace2mem/v1"
	"github.com/mohit-lendmind/trace2mem/internal/domain"
	"google.golang.org/protobuf/encoding/protojson"
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
func (e *Engine) prior(ctx context.Context, l domain.Lease, query, cursor string) (string, []domain.Record, error) {
	if cursor != "" && !domain.ValidPath(cursor) {
		return "", nil, errors.New("invalid prior-memory cursor")
	}
	if len(query) < 1 {
		return "[]", nil, nil
	}
	rows, err := e.Store.DB.Query(ctx, "SELECT path,content,citations FROM pages WHERE tenant=$1 AND space=$2 AND revision=$3 AND path LIKE 'notes/%' AND strpos(lower(content),lower($4))>0 AND path>$5 ORDER BY path LIMIT 65", l.Tenant, l.Space, l.Parent, query, cursor)
	if err != nil {
		return "", nil, err
	}
	var notes []domain.Observation
	ids := map[string]bool{}
	size := 0
	next := ""
	more := false
	for rows.Next() {
		if len(notes) == 64 {
			more = true
			break
		}
		var path, content string
		var cites []string
		if err = rows.Scan(&path, &content, &cites); err != nil {
			rows.Close()
			return "", nil, err
		}
		if size+len(content) > 128<<10 {
			if len(notes) == 0 {
				rows.Close()
				return "", nil, errors.New("individual note exceeds tool budget")
			}
			more = true
			break
		}
		size += len(content)
		note, parseErr := parseNote(content)
		if parseErr != nil {
			rows.Close()
			return "", nil, parseErr
		}
		next = path
		notes = append(notes, note)
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
	b, _ := json.Marshal(map[string]any{"notes": notes, "sources": records, "truncated": more, "next_cursor": next})
	return string(b), records, nil
}
func (e *Engine) sources(ctx context.Context, l domain.Lease, ids map[string]bool) ([]domain.Record, error) {
	out := []domain.Record{}
	size := 0
	if len(ids) > 1024 {
		return nil, errors.New("source count exceeds investigation budget")
	}
	for id := range ids {
		b, err := e.Store.EventJSON(ctx, l.Tenant, l.Space, id)
		if err != nil {
			return nil, err
		}
		size += len(b)
		if size > 2<<20 {
			return nil, errors.New("source bytes exceed investigation budget")
		}
		var v trace2memv1.Event
		if err = protojson.Unmarshal(b, &v); err != nil {
			return nil, err
		}
		out = append(out, Record(&v))
	}
	return out, nil
}
func (e *Engine) merge(ctx context.Context, l domain.Lease, updates []domain.Observation, records []domain.Record) ([]domain.Observation, []domain.Record, error) {
	encoded, err := json.Marshal(updates)
	if err != nil {
		return nil, nil, err
	}
	if len(updates) > 1000 || len(encoded) > 2<<20 {
		return nil, nil, errors.New("updates exceed merge budget")
	}
	accumulated := len(encoded)
	subjects := map[string]bool{}
	replaced := map[string]bool{}
	keys := map[string]bool{}
	for i := range updates {
		updates[i].ID = updates[i].StableID()
		o := updates[i]
		subjects[o.Subject] = true
		keys[o.ID] = true
		for _, id := range o.Supersedes {
			replaced[id] = true
		}
	}
	// Follow supersession IDs across subjects before loading the affected notes.
	nodes := map[string]domain.Observation{}
	pending := []string{}
	for _, o := range updates {
		if _, exists := nodes[o.ID]; exists {
			return nil, nil, errors.New("duplicate observation ID")
		}
		nodes[o.ID] = o
		pending = append(pending, o.Supersedes...)
	}
	for len(pending) > 0 {
		id := pending[0]
		pending = pending[1:]
		if _, ok := nodes[id]; ok {
			continue
		}
		if len(nodes) >= 1000 {
			return nil, nil, errors.New("supersession graph exceeds budget")
		}
		rows, err := e.Store.DB.Query(ctx, "SELECT content FROM pages WHERE tenant=$1 AND space=$2 AND revision=$3 AND path LIKE 'notes/%' AND split_part(path,'/',3)=$4 LIMIT 2", l.Tenant, l.Space, l.Parent, id+".md")
		if err != nil {
			return nil, nil, err
		}
		found := 0
		var old domain.Observation
		for rows.Next() {
			found++
			var content string
			if err = rows.Scan(&content); err != nil {
				rows.Close()
				return nil, nil, err
			}
			accumulated += len(content)
			if accumulated > 2<<20 {
				rows.Close()
				return nil, nil, errors.New("supersession bytes exceed merge budget")
			}
			old, err = parseNote(content)
			if err != nil {
				rows.Close()
				return nil, nil, err
			}
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return nil, nil, err
		}
		if found != 1 || old.StableID() != id {
			return nil, nil, fmt.Errorf("unresolved supersession target %q", id)
		}
		old.ID = id
		nodes[id] = old
		subjects[old.Subject] = true
		pending = append(pending, old.Supersedes...)
	}
	state := map[string]int{}
	var visit func(string) error
	visit = func(id string) error {
		if state[id] == 1 {
			return errors.New("cyclic supersession")
		}
		if state[id] == 2 {
			return nil
		}
		state[id] = 1
		for _, target := range nodes[id].Supersedes {
			if err := visit(target); err != nil {
				return err
			}
		}
		state[id] = 2
		return nil
	}
	for id := range nodes {
		if err := visit(id); err != nil {
			return nil, nil, err
		}
	}
	merged := append([]domain.Observation{}, updates...)
	for i := range merged {
		if replaced[merged[i].ID] {
			merged[i].Status = "superseded"
		}
	}
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
			old, parseErr := parseNote(content)
			if parseErr != nil {
				rows.Close()
				return nil, nil, parseErr
			}
			old.ID = old.StableID()
			if keys[old.ID] {
				continue
			}
			for _, id := range old.Citations {
				ids[id] = true

			}
			if replaced[old.ID] {
				old.Status = "superseded"
			}
			accumulated += len(content)
			if len(merged) >= 1000 || accumulated > 2<<20 {
				rows.Close()
				return nil, nil, errors.New("affected notes exceed merge budget")
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
