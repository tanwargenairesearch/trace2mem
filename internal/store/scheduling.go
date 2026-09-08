package store

import (
	"context"
	"errors"
	"github.com/jackc/pgx/v5"
	"github.com/tanwargenairesearch/trace2mem/internal/domain"
	"time"
	_ "time/tzdata"
)

type CompilationSchedule struct {
	Mode     string     `json:"mode"`
	Time     string     `json:"time"`
	Timezone string     `json:"timezone"`
	NextRun  *time.Time `json:"next_run,omitempty"`
}

// NormalizeAndValidate fills the default daily time and timezone before validation.
func (c *CompilationSchedule) NormalizeAndValidate() error {
	if c.Time == "" {
		c.Time = "02:00"
	}
	if c.Timezone == "" {
		c.Timezone = "UTC"
	}
	if c.Mode != "automatic" && c.Mode != "daily" && c.Mode != "manual" {
		return errors.New("mode must be automatic, daily or manual")
	}
	if _, err := time.Parse("15:04", c.Time); err != nil {
		return errors.New("time must be HH:MM")
	}
	if _, err := time.LoadLocation(c.Timezone); err != nil {
		return errors.New("timezone must be an IANA timezone")
	}
	return nil
}
func nextDaily(now time.Time, clock, zone string) (time.Time, error) {
	loc, err := time.LoadLocation(zone)
	if err != nil {
		return time.Time{}, err
	}
	parsed, err := time.Parse("15:04", clock)
	if err != nil {
		return time.Time{}, err
	}
	local := now.In(loc)
	wanted := parsed.Hour()*60 + parsed.Minute()
	for day := 0; day < 3; day++ {
		start := time.Date(local.Year(), local.Month(), local.Day()+day, 0, 0, 0, 0, loc)
		end := start.AddDate(0, 0, 1)
		for at := start; at.Before(end); at = at.Add(time.Minute) {
			wall := at.In(loc)
			if wall.Hour()*60+wall.Minute() >= wanted {
				// One occurrence per local day; a spring gap advances to its first valid minute.
				if at.After(now) {
					return at, nil
				}
				break
			}
		}
	}
	return time.Time{}, errors.New("cannot determine next daily compilation")
}
func queueCompilation(ctx context.Context, tx pgx.Tx, t, m string, watermark int64) error {
	var c CompilationSchedule
	var now time.Time
	if err := tx.QueryRow(ctx, "SELECT compilation_mode,compilation_time,compilation_timezone,clock_timestamp() FROM spaces WHERE tenant=$1 AND id=$2", t, m).Scan(&c.Mode, &c.Time, &c.Timezone, &now); err != nil {
		return err
	}
	if c.Mode == "manual" {
		return nil
	}
	due := now.Add(time.Minute)
	if c.Mode == "daily" {
		var err error
		due, err = nextDaily(now, c.Time, c.Timezone)
		if err != nil {
			return err
		}
	}
	_, err := tx.Exec(ctx, `INSERT INTO compilation_requests VALUES($1,$2,$3,$4,$5) ON CONFLICT(tenant,memory) DO UPDATE SET watermark=GREATEST(compilation_requests.watermark,EXCLUDED.watermark),due_at=CASE WHEN $6='automatic' THEN LEAST(EXCLUDED.due_at,compilation_requests.first_received+interval '5 minutes') ELSE compilation_requests.due_at END`, t, m, now, due, watermark, c.Mode)
	return err
}
func (s *Store) GetSchedule(ctx context.Context, t, m string) (CompilationSchedule, error) {
	var c CompilationSchedule
	err := s.DB.QueryRow(ctx, "SELECT compilation_mode,compilation_time,compilation_timezone,(SELECT due_at FROM compilation_requests WHERE tenant=$1 AND memory=$2) FROM spaces WHERE tenant=$1 AND id=$2", t, m).Scan(&c.Mode, &c.Time, &c.Timezone, &c.NextRun)
	return c, err
}
func (s *Store) SetSchedule(ctx context.Context, t, m string, c CompilationSchedule) error {
	if err := c.NormalizeAndValidate(); err != nil {
		return err
	}
	tx, err := s.DB.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var watermark int64
	if err = tx.QueryRow(ctx, "SELECT watermark FROM spaces WHERE tenant=$1 AND id=$2 FOR UPDATE", t, m).Scan(&watermark); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, "UPDATE spaces SET compilation_mode=$3,compilation_time=$4,compilation_timezone=$5 WHERE tenant=$1 AND id=$2", t, m, c.Mode, c.Time, c.Timezone); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, "DELETE FROM compilation_requests WHERE tenant=$1 AND memory=$2", t, m); err != nil {
		return err
	}
	var latest int64
	if err = tx.QueryRow(ctx, "SELECT COALESCE(max(ordinal),0) FROM events WHERE tenant=$1 AND space=$2", t, m).Scan(&latest); err != nil {
		return err
	}
	if latest > watermark {
		if err = queueCompilation(ctx, tx, t, m, latest); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}
func (s *Store) promoteDue(ctx context.Context) error {
	tx, err := s.DB.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var t, m string
	// All queue mutations use memory-before-job/request locking.
	err = tx.QueryRow(ctx, `SELECT s.tenant,s.id FROM spaces s WHERE EXISTS(SELECT 1 FROM compilation_requests r WHERE r.tenant=s.tenant AND r.memory=s.id AND r.due_at<=now()) ORDER BY s.tenant,s.id FOR UPDATE SKIP LOCKED LIMIT 1`).Scan(&t, &m)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	var watermark int64
	if err = tx.QueryRow(ctx, "SELECT watermark FROM compilation_requests WHERE tenant=$1 AND memory=$2", t, m).Scan(&watermark); err != nil {
		return err
	}
	if err = scheduleTarget(ctx, tx, t, m, watermark); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, "DELETE FROM compilation_requests WHERE tenant=$1 AND memory=$2", t, m); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
func finishCompilation(ctx context.Context, tx pgx.Tx, l domain.Lease) error {
	_, err := tx.Exec(ctx, `UPDATE jobs SET status=CASE WHEN requested OR target_watermark>$3 THEN 'pending' ELSE 'done' END,lease_until=NULL,attempts=0,error='',available_at=now() WHERE tenant=$1 AND space=$2`, l.Tenant, l.Space, l.Watermark)
	return err
}
