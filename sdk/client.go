package sdk

import (
	"connectrpc.com/connect"
	"context"
	trace2memv1 "github.com/trace2mem/trace2mem/gen/trace2mem/v1"
	"github.com/trace2mem/trace2mem/gen/trace2mem/v1/trace2memv1connect"
	"net/http"
)

type Client struct {
	Ingestion  trace2memv1connect.IngestionServiceClient
	Memory     trace2memv1connect.MemoryServiceClient
	HTTP       *http.Client
	URL, Token string
}

func New(url, token string, opts ...connect.ClientOption) *Client {
	hc := &http.Client{}
	auth := connect.WithInterceptors(connect.UnaryInterceptorFunc(func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, r connect.AnyRequest) (connect.AnyResponse, error) {
			r.Header().Set("Authorization", "Bearer "+token)
			return next(ctx, r)
		}
	}))
	opts = append(opts, auth)
	return &Client{trace2memv1connect.NewIngestionServiceClient(hc, url, opts...), trace2memv1connect.NewMemoryServiceClient(hc, url, opts...), hc, url, token}
}
func (c *Client) Import(ctx context.Context, a Adapter) error {
	batch := []*trace2memv1.Event{}
	flush := func() error {
		if len(batch) == 0 {
			return nil
		}
		_, e := c.Ingestion.AppendEvents(ctx, connect.NewRequest(&trace2memv1.AppendEventsRequest{Events: batch}))
		if e == nil {
			batch = nil
		}
		return e
	}
	e := a.Read(ctx, func(v *trace2memv1.Event) error {
		batch = append(batch, v)
		if len(batch) == 64 {
			return flush()
		}
		return nil
	})
	if e != nil {
		return e
	}
	return flush()
}
