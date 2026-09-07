package store

import (
	"cloud.google.com/go/cloudsqlconn"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/trace2mem/trace2mem/internal/domain"
	"github.com/trace2mem/trace2mem/migrations"
	"net"
	"os"
	"strings"
	"time"
)

type Store struct {
	DB     *pgxpool.Pool
	dialer *cloudsqlconn.Dialer
}

func Open(ctx context.Context, url string) (*Store, error) {
	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		return nil, err
	}
	var dialer *cloudsqlconn.Dialer
	if instance := os.Getenv("INSTANCE_CONNECTION_NAME"); instance != "" {
		dialer, err = cloudsqlconn.NewDialer(ctx, cloudsqlconn.WithIAMAuthN())
		if err != nil {
			return nil, err
		}
		cfg.ConnConfig.DialFunc = func(ctx context.Context, network, address string) (net.Conn, error) {
			return dialer.Dial(ctx, instance, cloudsqlconn.WithPrivateIP())
		}
		cfg.ConnConfig.TLSConfig = nil
	}
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		if dialer != nil {
			dialer.Close()
		}
		return nil, err
	}
	if err = pool.Ping(ctx); err != nil {
		pool.Close()
		if dialer != nil {
			dialer.Close()
		}
		return nil, err
	}
	return &Store{DB: pool, dialer: dialer}, nil
}
func (s *Store) Close() {
	s.DB.Close()
	if s.dialer != nil {
		s.dialer.Close()
	}
}

func ID() string {
	b := make([]byte, 16)
	if _, e := rand.Read(b); e != nil {
		panic(e)
	}
	return hex.EncodeToString(b)
}
func (s *Store) Migrate(ctx context.Context) error {
	c, e := s.DB.Acquire(ctx)
	if e != nil {
		return e
	}
	defer c.Release()
	if _, e = c.Exec(ctx, "SELECT pg_advisory_lock(18762341)"); e != nil {
		return e
	}
	defer c.Exec(context.Background(), "SELECT pg_advisory_unlock(18762341)")
	if _, e = c.Exec(ctx, "CREATE TABLE IF NOT EXISTS schema_migrations(name text PRIMARY KEY)"); e != nil {
		return e
	}
	files, e := migrations.Files.ReadDir(".")
	if e != nil {
		return e
	}
	for _, f := range files {
		var done bool
		e = c.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE name=$1)", f.Name()).Scan(&done)
		if e != nil {
			return e
		}
		if done {
			continue
		}
		b, e := migrations.Files.ReadFile(f.Name())
		if e != nil {
			return e
		}
		tx, e := c.Begin(ctx)
		if e != nil {
			return e
		}
		if _, e = tx.Exec(ctx, string(b)); e == nil {
			_, e = tx.Exec(ctx, "INSERT INTO schema_migrations VALUES($1)", f.Name())
		}
		if e != nil {
			tx.Rollback(ctx)
			return e
		}
		if e = tx.Commit(ctx); e != nil {
			return e
		}
	}
	return nil
}
func (s *Store) Authorize(ctx context.Context, p domain.Principal, space string, write bool) error {
	if !domain.ValidID(space) {
		return domain.ErrNotFound
	}
	scope := "read"
	if write {
		scope = "write"
	}
	if !p.Admin && !p.Scopes[scope] {
		return domain.ErrForbidden
	}
	var role string
	err := s.DB.QueryRow(ctx, "SELECT role FROM members WHERE tenant=$1 AND space=$2 AND subject=$3", p.Tenant, space, p.Subject).Scan(&role)
	if p.Admin {
		var ok bool
		e := s.DB.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM spaces WHERE tenant=$1 AND id=$2)", p.Tenant, space).Scan(&ok)
		if e != nil {
			return e
		}
		if ok {
			return nil
		}
		return domain.ErrNotFound
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ErrForbidden
	}
	if err != nil {
		return err
	}
	if write && role == "reader" {
		return domain.ErrForbidden
	}
	return nil
}
func (s *Store) Owner(ctx context.Context, p domain.Principal, space string) error {
	if e := s.Authorize(ctx, p, space, true); e != nil {
		return e
	}
	if p.Admin {
		return nil
	}
	var role string
	e := s.DB.QueryRow(ctx, "SELECT role FROM members WHERE tenant=$1 AND space=$2 AND subject=$3", p.Tenant, space, p.Subject).Scan(&role)
	if e != nil {
		return e
	}
	if role != "owner" {
		return domain.ErrForbidden
	}
	return nil
}
func (s *Store) CreateSpace(ctx context.Context, p domain.Principal, name string) (string, error) {
	if !p.Admin && !p.Scopes["write"] {
		return "", domain.ErrForbidden
	}
	id := ID()
	tx, e := s.DB.Begin(ctx)
	if e != nil {
		return "", e
	}
	defer tx.Rollback(ctx)
	_, e = tx.Exec(ctx, "INSERT INTO spaces(tenant,id,name) VALUES($1,$2,$3)", p.Tenant, id, name)
	if e != nil {
		return "", e
	}
	_, e = tx.Exec(ctx, "INSERT INTO members VALUES($1,$2,$3,'owner')", p.Tenant, id, p.Subject)
	if e != nil {
		return "", e
	}
	return id, tx.Commit(ctx)
}
func (s *Store) Spaces(ctx context.Context, p domain.Principal) ([]domain.Space, error) {
	rows, e := s.DB.Query(ctx, `SELECT s.id,s.name,COALESCE(m.role,'owner'),s.revision FROM spaces s LEFT JOIN members m ON m.tenant=s.tenant AND m.space=s.id AND m.subject=$2 WHERE s.tenant=$1 AND ($3 OR m.subject IS NOT NULL) ORDER BY s.name`, p.Tenant, p.Subject, p.Admin)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []domain.Space{}
	for rows.Next() {
		var v domain.Space
		if e = rows.Scan(&v.ID, &v.Name, &v.Role, &v.Revision); e != nil {
			return nil, e
		}
		out = append(out, v)
	}
	return out, rows.Err()
}
func Schedule(ctx context.Context, tx pgx.Tx, tenant, space string) error {
	_, e := tx.Exec(ctx, `INSERT INTO jobs(tenant,space) VALUES($1,$2) ON CONFLICT(tenant,space) DO UPDATE SET requested=true,status=CASE WHEN jobs.status='running' THEN 'running' ELSE 'pending' END,available_at=now(),error=''`, tenant, space)
	return e
}
func (s *Store) Schedule(ctx context.Context, t, sp string) error {
	tx, e := s.DB.Begin(ctx)
	if e != nil {
		return e
	}
	defer tx.Rollback(ctx)
	if e = Schedule(ctx, tx, t, sp); e != nil {
		return e
	}
	return tx.Commit(ctx)
}

type InputEvent struct {
	ID, Session, Hash string
	JSON              []byte
	Occurred          time.Time
}

func (s *Store) Append(ctx context.Context, t, sp string, events []InputEvent) (accepted, duplicates, watermark int64, err error) {
	tx, e := s.DB.Begin(ctx)
	if e != nil {
		err = e
		return
	}
	defer tx.Rollback(ctx)
	// Serialize event order with publication and forgetting for this space.
	var exists string
	e = tx.QueryRow(ctx, "SELECT id FROM spaces WHERE tenant=$1 AND id=$2 FOR UPDATE", t, sp).Scan(&exists)
	if e != nil {
		err = e
		return
	}
	for _, v := range events {
		var blocked bool
		e = tx.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM deletions WHERE tenant=$1 AND space=$2 AND event_id=$3)", t, sp, v.ID).Scan(&blocked)
		if e != nil {
			err = e
			return
		}
		if blocked {
			err = domain.ErrConflict
			return
		}
		tag, e := tx.Exec(ctx, `INSERT INTO events(tenant,space,id,session,hash,payload,occurred_at) VALUES($1,$2,$3,$4,$5,$6,$7) ON CONFLICT DO NOTHING`, t, sp, v.ID, v.Session, v.Hash, v.JSON, v.Occurred)
		if e != nil {
			err = e
			return
		}
		if tag.RowsAffected() == 0 {
			var hash string
			e = tx.QueryRow(ctx, "SELECT hash FROM events WHERE tenant=$1 AND space=$2 AND id=$3", t, sp, v.ID).Scan(&hash)
			if e != nil {
				err = e
				return
			}
			if hash != v.Hash {
				err = domain.ErrConflict
				return
			}
			duplicates++
		} else {
			accepted++
		}
	}
	e = tx.QueryRow(ctx, "SELECT COALESCE(max(ordinal),0) FROM events WHERE tenant=$1 AND space=$2", t, sp).Scan(&watermark)
	if e != nil {
		err = e
		return
	}
	if accepted > 0 {
		if e = Schedule(ctx, tx, t, sp); e != nil {
			err = e
			return
		}
	}
	err = tx.Commit(ctx)
	return
}
func (s *Store) Claim(ctx context.Context) (*domain.Lease, error) {
	tx, e := s.DB.Begin(ctx)
	if e != nil {
		return nil, e
	}
	defer tx.Rollback(ctx)
	l := &domain.Lease{}
	e = tx.QueryRow(ctx, `UPDATE jobs SET status='running',fence=fence+1,lease_until=now()+interval '90 seconds',requested=false,attempts=attempts+1 WHERE (tenant,space)=(SELECT tenant,space FROM jobs WHERE (status='pending' AND available_at<=now()) OR (status='running' AND lease_until<now()) ORDER BY available_at FOR UPDATE SKIP LOCKED LIMIT 1) RETURNING tenant,space,fence`).Scan(&l.Tenant, &l.Space, &l.Fence)
	if errors.Is(e, pgx.ErrNoRows) {
		return nil, nil
	}
	if e != nil {
		return nil, e
	}
	e = tx.QueryRow(ctx, `SELECT revision,generation,pending_model IS NOT NULL,CASE WHEN pending_model IS NOT NULL THEN watermark ELSE (SELECT COALESCE(max(ordinal),s.watermark) FROM (SELECT ordinal,sum(COALESCE(octet_length(payload::text),0)) OVER(ORDER BY ordinal) AS bytes FROM events WHERE tenant=$1 AND space=$2 AND ordinal>s.watermark ORDER BY ordinal LIMIT 128) batch WHERE bytes<=262144) END FROM spaces s WHERE tenant=$1 AND id=$2`, l.Tenant, l.Space).Scan(&l.Parent, &l.Epoch, &l.Reindex, &l.Watermark)
	if e != nil {
		return nil, e
	}
	return l, tx.Commit(ctx)
}
func (s *Store) Renew(ctx context.Context, l domain.Lease) error {
	tag, e := s.DB.Exec(ctx, "UPDATE jobs SET lease_until=now()+interval '90 seconds' WHERE tenant=$1 AND space=$2 AND fence=$3 AND status='running' AND lease_until>now()", l.Tenant, l.Space, l.Fence)
	if e != nil {
		return e
	}
	if tag.RowsAffected() != 1 {
		return domain.ErrLease
	}
	return nil
}
func (s *Store) Fail(ctx context.Context, l domain.Lease, reason string) error {
	_, e := s.DB.Exec(ctx, `UPDATE jobs SET status=CASE WHEN attempts<3 THEN 'pending' ELSE 'failed' END,available_at=now()+interval '30 seconds',error=$4,lease_until=NULL WHERE tenant=$1 AND space=$2 AND fence=$3`, l.Tenant, l.Space, l.Fence, reason)
	return e
}
func (s *Store) EventJSON(ctx context.Context, t, sp, id string) ([]byte, error) {
	var b []byte
	e := s.DB.QueryRow(ctx, "SELECT payload FROM events WHERE tenant=$1 AND space=$2 AND id=$3 AND NOT deleted", t, sp, id).Scan(&b)
	if errors.Is(e, pgx.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	return b, e
}
func (s *Store) Events(ctx context.Context, l domain.Lease) ([][]byte, error) {
	rows, e := s.DB.Query(ctx, "SELECT payload FROM events WHERE tenant=$1 AND space=$2 AND ordinal<=$3 AND ordinal>(SELECT watermark FROM spaces WHERE tenant=$1 AND id=$2) AND NOT deleted ORDER BY occurred_at,ordinal", l.Tenant, l.Space, l.Watermark)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	var out [][]byte
	for rows.Next() {
		var b []byte
		if e = rows.Scan(&b); e != nil {
			return nil, e
		}
		out = append(out, b)
	}
	return out, rows.Err()
}
func (s *Store) Publish(ctx context.Context, l domain.Lease, pages []domain.Page, verification any, embeddingModel string) (string, error) {
	tx, e := s.DB.Begin(ctx)
	if e != nil {
		return "", e
	}
	defer tx.Rollback(ctx)
	var parent string
	var epoch int64
	e = tx.QueryRow(ctx, "SELECT revision,generation FROM spaces WHERE tenant=$1 AND id=$2 FOR UPDATE", l.Tenant, l.Space).Scan(&parent, &epoch)
	if e != nil {
		return "", e
	}
	if parent != l.Parent || epoch != l.Epoch {
		return "", domain.ErrLease
	}
	var valid bool
	e = tx.QueryRow(ctx, "SELECT fence=$3 AND status='running' AND lease_until>now() FROM jobs WHERE tenant=$1 AND space=$2 FOR UPDATE", l.Tenant, l.Space, l.Fence).Scan(&valid)
	if e != nil {
		return "", e
	}
	if !valid {
		return "", domain.ErrLease
	}
	id := ID()
	ver, e := json.Marshal(verification)
	if e != nil {
		return "", e
	}
	_, e = tx.Exec(ctx, "INSERT INTO revisions(tenant,space,id,parent,watermark,verification) VALUES($1,$2,$3,$4,$5,$6)", l.Tenant, l.Space, id, parent, l.Watermark, ver)
	if e != nil {
		return "", e
	}
	if parent != "" {
		_, e = tx.Exec(ctx, `INSERT INTO pages SELECT tenant,space,$4,path,content,hash,citations,embedding,embedding_model FROM pages WHERE tenant=$1 AND space=$2 AND revision=$3`, l.Tenant, l.Space, parent, id)
		if e != nil {
			return "", e
		}
	}
	for _, p := range pages {
		if strings.HasPrefix(p.Path, "knowledge/subjects/") {
			subject := strings.TrimSuffix(strings.TrimPrefix(p.Path, "knowledge/subjects/"), ".md")
			_, e = tx.Exec(ctx, "DELETE FROM pages WHERE tenant=$1 AND space=$2 AND revision=$3 AND path LIKE $4", l.Tenant, l.Space, id, "notes/"+subject+"/%")
			if e != nil {
				return "", e
			}
		}
	}
	for _, p := range pages {
		if !domain.ValidPath(p.Path) {
			return "", fmt.Errorf("invalid page path")
		}
		if p.Citations == nil {
			p.Citations = []string{}
		}
		var vec any
		if len(p.Vector) > 0 {
			b, _ := json.Marshal(p.Vector)
			vec = string(b)
		}
		_, e = tx.Exec(ctx, "INSERT INTO pages(tenant,space,revision,path,content,hash,citations,embedding,embedding_model) VALUES($1,$2,$3,$4,$5,$6,$7,$8::vector,$9) ON CONFLICT(tenant,space,revision,path) DO UPDATE SET content=EXCLUDED.content,hash=EXCLUDED.hash,citations=EXCLUDED.citations,embedding=EXCLUDED.embedding,embedding_model=EXCLUDED.embedding_model", l.Tenant, l.Space, id, p.Path, p.Content, domain.Hash([]byte(p.Content)), p.Citations, vec, embeddingModel)
		if e != nil {
			return "", e
		}
	}
	rows, err := tx.Query(ctx, "SELECT path,split_part(content,E'\\n',1) FROM pages WHERE tenant=$1 AND space=$2 AND revision=$3 AND path LIKE 'knowledge/subjects/%' ORDER BY path LIMIT 200", l.Tenant, l.Space, id)
	if err != nil {
		return "", err
	}
	var index strings.Builder
	index.WriteString("# Knowledge index\n\nCompact map (up to 200 subjects). Search the service for additional subjects.\n\n")
	for rows.Next() {
		var path, title string
		if err = rows.Scan(&path, &title); err != nil {
			rows.Close()
			return "", err
		}
		fmt.Fprintf(&index, "- [[%s]] — %s\n", path, strings.TrimPrefix(title, "# "))
	}
	if err = rows.Err(); err != nil {
		rows.Close()
		return "", err
	}
	rows.Close()
	_, err = tx.Exec(ctx, "UPDATE pages SET content=$4,hash=$5,embedding=NULL WHERE tenant=$1 AND space=$2 AND revision=$3 AND path='knowledge/index.md'", l.Tenant, l.Space, id, index.String(), domain.Hash([]byte(index.String())))
	if err != nil {
		return "", err
	}

	_, e = tx.Exec(ctx, "UPDATE spaces SET revision=$3,watermark=$4,suppressed=false WHERE tenant=$1 AND id=$2", l.Tenant, l.Space, id, l.Watermark)
	if e != nil {
		return "", e
	}
	_, e = tx.Exec(ctx, "UPDATE jobs SET status=CASE WHEN requested OR EXISTS(SELECT 1 FROM events WHERE tenant=$1 AND space=$2 AND ordinal>$3) THEN 'pending' ELSE 'done' END,lease_until=NULL,attempts=0,error='' WHERE tenant=$1 AND space=$2", l.Tenant, l.Space, l.Watermark)
	if e != nil {
		return "", e
	}
	return id, tx.Commit(ctx)
}
func (s *Store) Snapshot(ctx context.Context, t, sp, revision string) (domain.Snapshot, error) {
	out := domain.Snapshot{Pages: []domain.Page{}}
	tx, e := s.DB.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if e != nil {
		return out, e
	}
	defer tx.Rollback(ctx)
	var suppressed bool
	e = tx.QueryRow(ctx, "SELECT revision,watermark,suppressed FROM spaces WHERE tenant=$1 AND id=$2", t, sp).Scan(&out.Revision, &out.Watermark, &suppressed)
	if e != nil {
		return out, e
	}
	if suppressed {
		return out, domain.ErrConflict
	}
	if revision != "" {
		out.Revision = revision
		e = tx.QueryRow(ctx, "SELECT watermark FROM revisions WHERE tenant=$1 AND space=$2 AND id=$3", t, sp, revision).Scan(&out.Watermark)
		if errors.Is(e, pgx.ErrNoRows) {
			return out, domain.ErrNotFound
		}
		if e != nil {
			return out, e
		}
	}
	rows, e := tx.Query(ctx, "SELECT path,content,hash,citations FROM pages WHERE tenant=$1 AND space=$2 AND revision=$3 ORDER BY path", t, sp, out.Revision)
	if e != nil {
		return out, e
	}
	defer rows.Close()
	for rows.Next() {
		var p domain.Page
		if e = rows.Scan(&p.Path, &p.Content, &p.Hash, &p.Citations); e != nil {
			return out, e
		}
		out.Pages = append(out.Pages, p)
	}
	if e = rows.Err(); e != nil {
		return out, e
	}
	rows.Close()
	return out, tx.Commit(ctx)
}
func (s *Store) Forget(ctx context.Context, t, sp, id string) error {
	tx, e := s.DB.Begin(ctx)
	if e != nil {
		return e
	}
	defer tx.Rollback(ctx)
	if _, e = tx.Exec(ctx, "DELETE FROM reindex_pages WHERE tenant=$1 AND space=$2", t, sp); e != nil {
		return e
	}
	_, e = tx.Exec(ctx, "UPDATE spaces SET generation=generation+1,suppressed=true,revision='',watermark=0,model=COALESCE(pending_model,model),credential=COALESCE(pending_credential,credential),pending_model=NULL,pending_credential=NULL WHERE tenant=$1 AND id=$2", t, sp)
	if e != nil {
		return e
	}
	_, e = tx.Exec(ctx, "INSERT INTO deletions(tenant,space,event_id) VALUES($1,$2,$3) ON CONFLICT DO NOTHING", t, sp, id)
	if e != nil {
		return e
	}
	var artifact string
	err := tx.QueryRow(ctx, "SELECT COALESCE(payload->'artifactReference'->>'artifactId','') FROM events WHERE tenant=$1 AND space=$2 AND id=$3", t, sp, id).Scan(&artifact)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	if artifact != "" {
		_, err = tx.Exec(ctx, `INSERT INTO blob_deletions(key,tenant,space) SELECT key,tenant,space FROM artifacts WHERE tenant=$1 AND space=$2 AND id=$3 AND NOT EXISTS(SELECT 1 FROM events WHERE tenant=$1 AND space=$2 AND id<>$4 AND NOT deleted AND payload->'artifactReference'->>'artifactId'=$3) ON CONFLICT DO NOTHING`, t, sp, artifact, id)
		if err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `DELETE FROM artifacts WHERE tenant=$1 AND space=$2 AND id=$3 AND key IN(SELECT key FROM blob_deletions)`, t, sp, artifact)
		if err != nil {
			return err
		}
	}
	_, e = tx.Exec(ctx, "UPDATE events SET deleted=true,payload=NULL WHERE tenant=$1 AND space=$2 AND id=$3", t, sp, id)
	if e != nil {
		return e
	}
	for _, table := range []string{"revisions", "proposals", "evaluations", "candidates"} {
		_, e = tx.Exec(ctx, "DELETE FROM "+table+" WHERE tenant=$1 AND space=$2", t, sp)
		if e != nil {
			return e
		}
	}
	if e = Schedule(ctx, tx, t, sp); e != nil {
		return e
	}
	return tx.Commit(ctx)
}
