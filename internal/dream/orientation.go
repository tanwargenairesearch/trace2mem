package dream

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/jackc/pgx/v5"
	"github.com/mohit-lendmind/trace2mem/internal/domain"
)

func (e *Engine) orientation(ctx context.Context, l domain.Lease) (string, error) {
	var index string
	err := e.Store.DB.QueryRow(ctx, "SELECT content FROM pages WHERE tenant=$1 AND space=$2 AND revision=$3 AND path='knowledge/index.md'", l.Tenant, l.Space, l.Parent).Scan(&index)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return "", err
	}
	var deleted int64
	if err = e.Store.DB.QueryRow(ctx, "SELECT count(*) FROM deletions WHERE tenant=$1 AND space=$2", l.Tenant, l.Space).Scan(&deleted); err != nil {
		return "", err
	}
	b, err := json.Marshal(map[string]any{"parent_revision": l.Parent, "deletion_generation": l.Epoch, "deleted_sources": deleted, "new_event_watermark": l.Watermark, "index_as_untrusted_evidence": index, "scope": "authenticated user's memory only", "stopping_conditions": "bounded tools, provider tokens and elapsed time; propose supported changes or a justified no-op"})
	return string(b), err
}
