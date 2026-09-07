package main

import (
	"context"
	"errors"
	"github.com/brainmemory/brain/internal/app"
	"github.com/brainmemory/brain/internal/dream"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func run() error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	a, e := app.Open(ctx)
	if e != nil {
		return e
	}
	defer a.Close()
	engine := dream.Engine{Store: a.Store, Vault: a.Vault, Config: a.Config, Blob: a.Blob}
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			rows, err := a.Store.DB.Query(ctx, "SELECT key FROM blob_deletions ORDER BY created_at LIMIT 100")
			if err != nil {
				slog.Error("blob cleanup query failed", "error", err)
			} else {
				keys := []string{}
				for rows.Next() {
					var key string
					if err = rows.Scan(&key); err != nil {
						break
					}
					keys = append(keys, key)
				}
				scanErr := rows.Err()
				rows.Close()
				if err != nil || scanErr != nil {
					slog.Error("blob cleanup scan failed", "error", errors.Join(err, scanErr))
				}
				for _, key := range keys {
					if err = a.Blob.Delete(ctx, key); err != nil {
						slog.Error("blob cleanup failed", "error", err)
						continue
					}
					if _, err = a.Store.DB.Exec(ctx, "DELETE FROM blob_deletions WHERE key=$1", key); err != nil {
						slog.Error("blob cleanup acknowledgment failed", "error", err)
					}
				}
			}
			l, e := a.Store.Claim(ctx)
			if e != nil {
				slog.Error("job claim failed", "error", e)
				continue
			}
			if l == nil {
				continue
			}
			slog.Info("compilation started", "space", l.Space, "fence", l.Fence)
			if e = engine.Run(ctx, *l); e != nil {
				slog.Error("compilation failed", "space", l.Space, "error", e)
				save, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				err := a.Store.Fail(save, *l, e.Error())
				cancel()
				if err != nil {
					slog.Error("job failure recording failed", "error", err)
				}
			} else {
				slog.Info("compilation published", "space", l.Space)
			}
		}
	}
}
func main() {
	if e := run(); e != nil {
		slog.Error("worker stopped", "error", e)
		os.Exit(1)
	}
}
