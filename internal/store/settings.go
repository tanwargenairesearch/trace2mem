package store

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/jackc/pgx/v5"
	"github.com/trace2mem/trace2mem/internal/domain"
	"time"
)

func (s *Store) Config(ctx context.Context, t, sp string) (domain.ModelConfig, []byte, error) {
	var c domain.ModelConfig
	var b, key []byte
	e := s.DB.QueryRow(ctx, "SELECT model,credential FROM spaces WHERE tenant=$1 AND id=$2", t, sp).Scan(&b, &key)
	if e != nil {
		return c, nil, e
	}
	e = json.Unmarshal(b, &c)
	if c.MaxSteps == 0 {
		c.MaxSteps = 12
	}
	if c.MaxTokens == 0 {
		c.MaxTokens = 32000
	}
	if c.DailyTokens == 0 {
		c.DailyTokens = 1000000
	}
	return c, key, e
}
func (s *Store) SetConfig(ctx context.Context, t, sp string, c domain.ModelConfig, key []byte) error {
	c.Key = ""
	c.EmbeddingKey = ""
	b, e := json.Marshal(c)
	if e != nil {
		return e
	}
	tx, e := s.DB.Begin(ctx)
	if e != nil {
		return e
	}
	defer tx.Rollback(ctx)
	var oldBytes []byte
	var revision string
	if e = tx.QueryRow(ctx, "SELECT model,revision FROM spaces WHERE tenant=$1 AND id=$2 FOR UPDATE", t, sp).Scan(&oldBytes, &revision); e != nil {
		return e
	}
	var old domain.ModelConfig
	if e = json.Unmarshal(oldBytes, &old); e != nil {
		return e
	}
	if revision != "" && old.EmbeddingIdentity() != c.EmbeddingIdentity() {
		_, e = tx.Exec(ctx, "UPDATE spaces SET pending_model=$3,pending_credential=$4,generation=generation+1 WHERE tenant=$1 AND id=$2", t, sp, b, key)
		if e == nil {
			e = Schedule(ctx, tx, t, sp)
		}
	} else {
		_, e = tx.Exec(ctx, "UPDATE spaces SET model=$3,credential=$4,pending_model=NULL,pending_credential=NULL,generation=generation+1 WHERE tenant=$1 AND id=$2", t, sp, b, key)
	}
	if e != nil {
		return e
	}
	if _, e = tx.Exec(ctx, "DELETE FROM reindex_pages WHERE tenant=$1 AND space=$2", t, sp); e != nil {
		return e
	}
	return tx.Commit(ctx)
}
func (s *Store) RecordUsage(ctx context.Context, t, sp, operation string, u domain.Usage) error {
	_, e := s.DB.Exec(ctx, "INSERT INTO usage(tenant,space,operation,input_tokens,output_tokens,estimated) VALUES($1,$2,$3,$4,$5,$6)", t, sp, operation, u.Input, u.Output, u.Estimated)
	return e
}
func (s *Store) Used(ctx context.Context, t, sp string) (int64, error) {
	var n int64
	e := s.DB.QueryRow(ctx, "SELECT COALESCE(sum(input_tokens+output_tokens),0) FROM usage WHERE tenant=$1 AND space=$2 AND created_at>=date_trunc('day',now())", t, sp).Scan(&n)
	return n, e
}
func (s *Store) Token(ctx context.Context, token string) (domain.Principal, error) {
	var p domain.Principal
	var scopes []string
	e := s.DB.QueryRow(ctx, "SELECT tenant,subject,scopes FROM tokens WHERE hash=$1 AND NOT revoked AND expires_at>now()", domain.Hash([]byte(token))).Scan(&p.Tenant, &p.Subject, &scopes)
	if errors.Is(e, pgx.ErrNoRows) {
		return p, domain.ErrForbidden
	}
	p.Scopes = map[string]bool{}
	for _, v := range scopes {
		p.Scopes[v] = true
	}
	return p, e
}
func (s *Store) CreateToken(ctx context.Context, p domain.Principal, scopes []string) (string, error) {
	if !p.Scopes["manage"] || len(scopes) == 0 {
		return "", domain.ErrForbidden
	}
	for _, scope := range scopes {
		if scope != "read" && scope != "ingest" && scope != "manage" {
			return "", domain.ErrForbidden
		}
		if !p.Scopes[scope] {
			return "", domain.ErrForbidden
		}
	}
	token := ID() + ID()
	_, e := s.DB.Exec(ctx, "INSERT INTO tokens VALUES($1,$2,$3,$4,$5,false)", domain.Hash([]byte(token)), p.Tenant, p.Subject, scopes, time.Now().Add(90*24*time.Hour))
	return token, e
}
func (s *Store) Proposal(ctx context.Context, l domain.Lease, content any, status string, verification any) (string, error) {
	tx, err := s.DB.Begin(ctx)
	if err != nil {
		return "", err
	}
	defer tx.Rollback(ctx)
	var epoch int64
	err = tx.QueryRow(ctx, "SELECT generation FROM spaces WHERE tenant=$1 AND id=$2 FOR UPDATE", l.Tenant, l.Space).Scan(&epoch)
	if err != nil {
		return "", err
	}
	if epoch != l.Epoch {
		return "", domain.ErrLease
	}
	var valid bool
	err = tx.QueryRow(ctx, "SELECT fence=$3 AND status='running' AND lease_until>now() FROM jobs WHERE tenant=$1 AND space=$2", l.Tenant, l.Space, l.Fence).Scan(&valid)
	if err != nil {
		return "", err
	}
	if !valid {
		return "", domain.ErrLease
	}
	id := ID()
	b, err := json.Marshal(content)
	if err != nil {
		return "", err
	}
	v, err := json.Marshal(verification)
	if err != nil {
		return "", err
	}
	_, err = tx.Exec(ctx, "INSERT INTO proposals(id,tenant,space,parent,status,content,verification) VALUES($1,$2,$3,$4,$5,$6,$7)", id, l.Tenant, l.Space, l.Parent, status, b, v)
	if err != nil {
		return "", err
	}
	return id, tx.Commit(ctx)
}

func (s *Store) PendingConfig(ctx context.Context, t, sp string) (domain.ModelConfig, []byte, error) {
	var c domain.ModelConfig
	var b, key []byte
	e := s.DB.QueryRow(ctx, "SELECT pending_model,pending_credential FROM spaces WHERE tenant=$1 AND id=$2 AND pending_model IS NOT NULL", t, sp).Scan(&b, &key)
	if e == nil {
		e = json.Unmarshal(b, &c)
	}
	return c, key, e
}

func (s *Store) StageEmbedding(ctx context.Context, l domain.Lease, path string, vector []float32) error {
	tx, err := s.DB.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var epoch int64
	if err = tx.QueryRow(ctx, "SELECT generation FROM spaces WHERE tenant=$1 AND id=$2 FOR UPDATE", l.Tenant, l.Space).Scan(&epoch); err != nil {
		return err
	}
	if epoch != l.Epoch {
		return domain.ErrLease
	}
	b, err := json.Marshal(vector)
	if err != nil {
		return err
	}
	tag, err := tx.Exec(ctx, `INSERT INTO reindex_pages SELECT $1,$2,$3,$4,$5::vector WHERE EXISTS(SELECT 1 FROM jobs WHERE tenant=$1 AND space=$2 AND fence=$6 AND status='running' AND lease_until>now()) ON CONFLICT(tenant,space,epoch,path) DO UPDATE SET embedding=EXCLUDED.embedding`, l.Tenant, l.Space, l.Epoch, path, string(b), l.Fence)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return domain.ErrLease
	}
	return tx.Commit(ctx)
}
func (s *Store) Yield(ctx context.Context, l domain.Lease) error {
	tag, err := s.DB.Exec(ctx, "UPDATE jobs SET status='pending',lease_until=NULL,available_at=now(),attempts=0 WHERE tenant=$1 AND space=$2 AND fence=$3 AND status='running'", l.Tenant, l.Space, l.Fence)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return domain.ErrLease
	}
	return nil
}
