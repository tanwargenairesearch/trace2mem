package store

import (
	"context"
	"encoding/json"
	"github.com/trace2mem/trace2mem/internal/domain"
)

func (s *Store) PublishReindex(ctx context.Context, l domain.Lease, identity string) error {
	tx, err := s.DB.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var parent string
	var epoch int64
	var pending bool
	err = tx.QueryRow(ctx, "SELECT revision,generation,pending_model IS NOT NULL FROM spaces WHERE tenant=$1 AND id=$2 FOR UPDATE", l.Tenant, l.Space).Scan(&parent, &epoch, &pending)
	if err != nil {
		return err
	}
	if parent != l.Parent || epoch != l.Epoch || !pending {
		return domain.ErrLease
	}
	var valid bool
	err = tx.QueryRow(ctx, "SELECT fence=$3 AND status='running' AND lease_until>now() FROM jobs WHERE tenant=$1 AND space=$2 FOR UPDATE", l.Tenant, l.Space, l.Fence).Scan(&valid)
	if err != nil {
		return err
	}
	if !valid {
		return domain.ErrLease
	}
	var missing bool
	err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pages p WHERE tenant=$1 AND space=$2 AND revision=$3 AND NOT EXISTS(SELECT 1 FROM reindex_pages r WHERE r.tenant=p.tenant AND r.space=p.space AND r.epoch=$4 AND r.path=p.path))`, l.Tenant, l.Space, l.Parent, l.Epoch).Scan(&missing)
	if err != nil {
		return err
	}
	if missing {
		return domain.ErrConflict
	}
	id := ID()
	verification, _ := json.Marshal(map[string]string{"operation": "reindex", "embedding": identity})
	_, err = tx.Exec(ctx, "INSERT INTO revisions(tenant,space,id,parent,watermark,verification) VALUES($1,$2,$3,$4,$5,$6)", l.Tenant, l.Space, id, parent, l.Watermark, verification)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO pages(tenant,space,revision,path,content,hash,citations,embedding,embedding_model) SELECT p.tenant,p.space,$4,p.path,p.content,p.hash,p.citations,r.embedding,$6 FROM pages p JOIN reindex_pages r ON r.tenant=p.tenant AND r.space=p.space AND r.path=p.path AND r.epoch=$5 WHERE p.tenant=$1 AND p.space=$2 AND p.revision=$3`, l.Tenant, l.Space, parent, id, l.Epoch, identity)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, "UPDATE spaces SET revision=$3,model=pending_model,credential=pending_credential,pending_model=NULL,pending_credential=NULL WHERE tenant=$1 AND id=$2", l.Tenant, l.Space, id)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, "DELETE FROM reindex_pages WHERE tenant=$1 AND space=$2", l.Tenant, l.Space)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, "UPDATE jobs SET status=CASE WHEN requested OR EXISTS(SELECT 1 FROM events WHERE tenant=$1 AND space=$2 AND ordinal>$3) THEN 'pending' ELSE 'done' END,lease_until=NULL,attempts=0,error='' WHERE tenant=$1 AND space=$2", l.Tenant, l.Space, l.Watermark)
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}
