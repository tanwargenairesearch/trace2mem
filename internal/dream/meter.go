package dream

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/trace2mem/trace2mem/internal/domain"
	"github.com/trace2mem/trace2mem/internal/model"
)

type metered struct {
	engine        *Engine
	provider      model.Provider
	config        domain.ModelConfig
	tenant, space string
}

func (e *Engine) Meter(p model.Provider, c domain.ModelConfig, t, sp string) model.Provider {
	return &metered{e, p, c, t, sp}
}
func (m *metered) Generate(ctx context.Context, turns []model.Turn, tools []model.Tool) (model.Reply, error) {
	b, e := json.Marshal(struct {
		Turns []model.Turn
		Tools []model.Tool
	}{turns, tools})
	if e != nil {
		return model.Reply{}, e
	}
	if len(b) > 3<<20 {
		return model.Reply{}, errors.New("model input exceeds 3 MiB")
	}
	reserve := int64(len(b)*2 + 4096)
	id, e := m.engine.Store.Reserve(ctx, m.tenant, m.space, "generation", reserve, m.config.DailyTokens)
	if e != nil {
		return model.Reply{}, e
	}
	r, e := m.provider.Generate(ctx, turns, tools)
	if e != nil {
		return r, e
	}
	if r.Usage.Total() == 0 {
		r.Usage = domain.Usage{Input: int64(len(b) * 2), Output: int64(len(r.Text) * 2), Estimated: true}
	}
	if e = m.engine.Store.Reconcile(ctx, id, r.Usage); e != nil {
		return r, e
	}
	return r, nil
}
func (m *metered) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	var bytes int
	for _, t := range texts {
		bytes += len(t)
	}
	if bytes > 2<<20 {
		return nil, errors.New("embedding input exceeds 2 MiB")
	}
	estimate := int64(bytes*2 + len(texts))
	id, e := m.engine.Store.Reserve(ctx, m.tenant, m.space, "embedding", estimate, m.config.DailyTokens)
	if e != nil {
		return nil, e
	}
	v, e := m.provider.Embed(ctx, texts)
	if e != nil {
		return nil, e
	}
	if e = m.engine.Store.Reconcile(ctx, id, domain.Usage{Input: estimate, Estimated: true}); e != nil {
		return nil, e
	}
	return v, nil
}
