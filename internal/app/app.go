package app

import (
	kms "cloud.google.com/go/kms/apiv1"
	"cloud.google.com/go/storage"
	"context"
	"errors"
	"github.com/trace2mem/trace2mem/internal/blob"
	"github.com/trace2mem/trace2mem/internal/config"
	"github.com/trace2mem/trace2mem/internal/store"
	"os"
	"strings"
)

type App struct {
	Config  config.Config
	Store   *store.Store
	Vault   config.Vault
	Blob    blob.Store
	closers []func()
}

func Open(ctx context.Context) (*App, error) {
	for _, key := range []string{"TRACE2MEM_BOOTSTRAP_TOKEN", "TRACE2MEM_MASTER_KEY"} {
		if f := os.Getenv(key + "_FILE"); f != "" {
			b, e := os.ReadFile(f)
			if e != nil {
				return nil, e
			}
			os.Setenv(key, strings.TrimSpace(string(b)))
		}
	}
	c := config.Load()
	a := &App{Config: c}
	ok := false
	defer func() {
		if !ok {
			a.Close()
		}
	}()
	s, e := store.Open(ctx, c.DatabaseURL)
	if e != nil {
		return nil, e
	}
	a.Store = s
	a.closers = append(a.closers, s.Close)
	if c.KMSKey != "" {
		client, e := kms.NewKeyManagementClient(ctx)
		if e != nil {
			return nil, e
		}
		a.Vault = config.KMSVault{Client: client, Key: c.KMSKey}
		a.closers = append(a.closers, func() { client.Close() })
	} else {
		a.Vault, e = config.NewVault(c.MasterKey)
		if e != nil {
			return nil, e
		}
	}
	if c.GCSBucket != "" {
		client, e := storage.NewClient(ctx)
		if e != nil {
			return nil, e
		}
		a.Blob = blob.GCS{Client: client, Bucket: c.GCSBucket}
		a.closers = append(a.closers, func() { client.Close() })
	} else {
		if c.BlobDir == "" {
			return nil, errors.New("blob directory required")
		}
		a.Blob = blob.Local{Root: c.BlobDir}
	}
	ok = true
	return a, nil
}
func (a *App) Close() {
	for i := len(a.closers) - 1; i >= 0; i-- {
		a.closers[i]()
	}
}
