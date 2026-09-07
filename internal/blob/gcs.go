package blob

import (
	"cloud.google.com/go/storage"
	"context"
	"io"
)

type GCS struct {
	Client *storage.Client
	Bucket string
}

func (g GCS) Put(ctx context.Context, k string, b []byte) error {
	w := g.Client.Bucket(g.Bucket).Object(k).NewWriter(ctx)
	if _, e := w.Write(b); e != nil {
		w.Close()
		return e
	}
	return w.Close()
}
func (g GCS) Get(ctx context.Context, k string) ([]byte, error) {
	r, e := g.Client.Bucket(g.Bucket).Object(k).NewReader(ctx)
	if e != nil {
		return nil, e
	}
	defer r.Close()
	return io.ReadAll(io.LimitReader(r, 9<<20))
}
func (g GCS) Delete(ctx context.Context, k string) error {
	e := g.Client.Bucket(g.Bucket).Object(k).Delete(ctx)
	if e == storage.ErrObjectNotExist {
		return nil
	}
	return e
}
