package store

import (
	"context"
	"errors"
	"github.com/trace2mem/trace2mem/internal/domain"
)

var ErrBudget = errors.New("daily provider token budget exhausted")

// Reserve records a conservative charge before dispatch. Unknown outcomes remain charged.
func (s *Store) Reserve(ctx context.Context, t, sp, op string, tokens, limit int64) (int64, error) {
	tx, e := s.DB.Begin(ctx)
	if e != nil {
		return 0, e
	}
	defer tx.Rollback(ctx)
	var id string
	e = tx.QueryRow(ctx, "SELECT id FROM spaces WHERE tenant=$1 AND id=$2 FOR UPDATE", t, sp).Scan(&id)
	if e != nil {
		return 0, e
	}
	var used int64
	e = tx.QueryRow(ctx, "SELECT COALESCE(sum(input_tokens+output_tokens),0) FROM usage WHERE tenant=$1 AND space=$2 AND created_at>=date_trunc('day',now())", t, sp).Scan(&used)
	if e != nil {
		return 0, e
	}
	if tokens < 0 || used+tokens > limit {
		return 0, ErrBudget
	}
	var reservation int64
	e = tx.QueryRow(ctx, "INSERT INTO usage(tenant,space,operation,input_tokens,output_tokens,estimated) VALUES($1,$2,$3,$4,0,true) RETURNING id", t, sp, op, tokens).Scan(&reservation)
	if e != nil {
		return 0, e
	}
	return reservation, tx.Commit(ctx)
}
func (s *Store) Reconcile(ctx context.Context, id int64, u domain.Usage) error {
	_, e := s.DB.Exec(ctx, "UPDATE usage SET input_tokens=$2,output_tokens=$3,estimated=$4 WHERE id=$1", id, u.Input, u.Output, u.Estimated)
	return e
}
