package dream

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/mohit-lendmind/trace2mem/internal/domain"
	"github.com/mohit-lendmind/trace2mem/internal/model"
)

type metered struct {
	engine        *Engine
	provider      model.Provider
	config        domain.ModelConfig
	tenant, space string
}
type compilationKey struct{}

func meteredOperation(ctx context.Context, operation string) string {
	if ctx.Value(compilationKey{}) != nil {
		return "compilation/" + operation
	}
	return operation
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
	reserve := int64(len(b)*2 + m.config.GenerationOutputLimit())
	id, e := m.engine.Store.Reserve(ctx, m.tenant, m.space, meteredOperation(ctx, "generation"), reserve, m.config.DailyTokens)
	if e != nil {
		return model.Reply{}, e
	}
	charged := domain.Usage{Input: reserve, Estimated: true, Unresolved: true}
	defer func() { countUsage(ctx, false, charged) }()
	r, e := m.provider.Generate(ctx, turns, tools)
	if e != nil {
		if r.Usage.Total() > 0 {
			if err := m.engine.Store.Reconcile(ctx, id, r.Usage); err != nil {
				return r, errors.Join(e, err)
			}
			charged = r.Usage
		}
		return r, e
	}
	if r.Usage.Total() == 0 {
		r.Usage = domain.Usage{Input: int64(len(b) * 2), Output: int64(len(r.Text) * 2), Estimated: true}
	}
	if e = m.engine.Store.Reconcile(ctx, id, r.Usage); e != nil {
		return r, e
	}
	charged = r.Usage
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
	id, e := m.engine.Store.Reserve(ctx, m.tenant, m.space, meteredOperation(ctx, "embedding"), estimate, m.config.DailyTokens)
	if e != nil {
		return nil, e
	}
	charged := domain.Usage{Input: estimate, Estimated: true, Unresolved: true}
	defer func() { countUsage(ctx, true, charged) }()
	v, e := m.provider.Embed(ctx, texts)
	if e != nil {
		return nil, e
	}
	if e = m.engine.Store.Reconcile(ctx, id, domain.Usage{Input: estimate, Estimated: true}); e != nil {
		return nil, e
	}
	charged.Unresolved = false
	return v, nil
}
