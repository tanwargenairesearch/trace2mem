package store

import (
	"context"
	"errors"
	"github.com/jackc/pgx/v5"
)

func (s *Store) blockUnconfigured(ctx context.Context) error {
	tx, err := s.DB.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var tenant, memory string
	// Match configuration writes' memory-then-job lock order to avoid stale blocking.
	err = tx.QueryRow(ctx, `SELECT tenant,id FROM spaces s WHERE (COALESCE(model->>'provider','')='' OR COALESCE(model->>'model','')='' OR COALESCE(model->>'embedding_provider','')='' OR COALESCE(model->>'embedding_model','')='') AND EXISTS(SELECT 1 FROM jobs j WHERE j.tenant=s.tenant AND j.space=s.id AND j.status IN ('pending','failed')) FOR UPDATE SKIP LOCKED LIMIT 1`).Scan(&tenant, &memory)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, "UPDATE jobs SET status='blocked',error='Configure generation and embedding models to compile memory' WHERE tenant=$1 AND space=$2 AND status IN ('pending','failed')", tenant, memory); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
