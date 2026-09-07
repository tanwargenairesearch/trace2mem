package blob

import (
	"context"
	"errors"
	"github.com/trace2mem/trace2mem/internal/domain"
	"os"
	"path/filepath"
)

type Store interface {
	Put(context.Context, string, []byte) error
	Get(context.Context, string) ([]byte, error)
	Delete(context.Context, string) error
}
type Local struct{ Root string }

func (l Local) filename(k string) (string, error) {
	if !domain.ValidPath(k) {
		return "", errors.New("invalid blob key")
	}
	return filepath.Join(l.Root, filepath.FromSlash(k)), nil
}
func (l Local) Put(ctx context.Context, k string, b []byte) error {
	p, e := l.filename(k)
	if e != nil {
		return e
	}
	if e = os.MkdirAll(filepath.Dir(p), 0700); e != nil {
		return e
	}
	f, e := os.CreateTemp(filepath.Dir(p), ".upload-")
	if e != nil {
		return e
	}
	defer os.Remove(f.Name())
	if _, e = f.Write(b); e != nil {
		f.Close()
		return e
	}
	if e = f.Sync(); e != nil {
		f.Close()
		return e
	}
	if e = f.Close(); e != nil {
		return e
	}
	return os.Rename(f.Name(), p)
}
func (l Local) Get(ctx context.Context, k string) ([]byte, error) {
	p, e := l.filename(k)
	if e != nil {
		return nil, e
	}
	return os.ReadFile(p)
}
func (l Local) Delete(ctx context.Context, k string) error {
	p, e := l.filename(k)
	if e != nil {
		return e
	}
	e = os.Remove(p)
	if os.IsNotExist(e) {
		return nil
	}
	return e
}
