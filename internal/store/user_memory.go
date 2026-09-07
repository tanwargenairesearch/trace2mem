package store

import (
	"context"
	"github.com/trace2mem/trace2mem/internal/domain"
)

// EnsureMemory provisions the single memory owned by an authenticated identity.
func (s *Store) EnsureMemory(ctx context.Context, p domain.Principal) error {
	if p.Subject == "" || p.Tenant == "" {
		return domain.ErrForbidden
	}
	tx, err := s.DB.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	id := p.MemoryID()
	if _, err = tx.Exec(ctx, "INSERT INTO spaces(tenant,id,name) VALUES($1,$2,'Personal memory') ON CONFLICT DO NOTHING", p.Tenant, id); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, "INSERT INTO user_memories(tenant,subject,memory_id) VALUES($1,$2,$3) ON CONFLICT DO NOTHING", p.Tenant, p.Subject, id); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
