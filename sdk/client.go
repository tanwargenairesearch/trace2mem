package sdk

import (
	"connectrpc.com/connect"
	"context"
	brainv1 "github.com/brainmemory/brain/gen/brain/v1"
	"github.com/brainmemory/brain/gen/brain/v1/brainv1connect"
	"net/http"
)

type Client struct {
	Ingestion  brainv1connect.IngestionServiceClient
	Memory     brainv1connect.MemoryServiceClient
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
	return &Client{brainv1connect.NewIngestionServiceClient(hc, url, opts...), brainv1connect.NewMemoryServiceClient(hc, url, opts...), hc, url, token}
}
func (c *Client) Import(ctx context.Context, space string, a Adapter) error {
	batch := []*brainv1.Event{}
	flush := func() error {
		if len(batch) == 0 {
			return nil
		}
		_, e := c.Ingestion.AppendEvents(ctx, connect.NewRequest(&brainv1.AppendEventsRequest{SpaceId: space, Events: batch}))
		if e == nil {
			batch = nil
		}
		return e
	}
	e := a.Read(ctx, func(v *brainv1.Event) error {
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
