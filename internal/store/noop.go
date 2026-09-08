package store

import (
	"context"
	"encoding/json"
	"github.com/trace2mem/trace2mem/internal/domain"
	"strings"
)

// PublishNoop records verification and advances the processed watermark under
// publication fencing, without changing the published revision.
func (s *Store) PublishNoop(ctx context.Context, l domain.Lease, reason string, verification any) error {
	if strings.TrimSpace(reason) == "" || len(reason) > 4096 {
		return domain.ErrConflict
	}
	tx, err := s.DB.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var parent string
	var epoch int64
	var suppressed bool
	err = tx.QueryRow(ctx, "SELECT revision,generation,suppressed FROM spaces WHERE tenant=$1 AND id=$2 FOR UPDATE", l.Tenant, l.Space).Scan(&parent, &epoch, &suppressed)
	if err != nil {
		return err
	}
	if parent != l.Parent || epoch != l.Epoch || suppressed || l.Reindex {
		return domain.ErrLease
	}
	var valid bool
	err = tx.QueryRow(ctx, "SELECT status='running' AND fence=$3 AND lease_until>now() FROM jobs WHERE tenant=$1 AND space=$2 FOR UPDATE", l.Tenant, l.Space, l.Fence).Scan(&valid)
	if err != nil {
		return err
	}
	if !valid {
		return domain.ErrLease
	}
	content, _ := json.Marshal(map[string]any{"reason": reason, "watermark": l.Watermark})
	judgment, err := json.Marshal(verification)
	if err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, "INSERT INTO proposals(id,tenant,space,parent,status,content,verification) VALUES($1,$2,$3,$4,'no-op',$5,$6)", ID(), l.Tenant, l.Space, l.Parent, content, judgment); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, "UPDATE spaces SET watermark=$3 WHERE tenant=$1 AND id=$2", l.Tenant, l.Space, l.Watermark); err != nil {
		return err
	}
	if err = finishCompilation(ctx, tx, l); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
