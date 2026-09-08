package dream

import (
	"context"
	"sync"

	"github.com/mohit-lendmind/trace2mem/internal/domain"
)

type usageKey struct{}

// UsageCounter measures calls belonging to one context, excluding concurrent work.
type UsageCounter struct {
	mu                    sync.Mutex
	generation, embedding domain.Usage
}

// WithUsage installs a fresh counter for metered generation/embedding calls,
// including unresolved reservations after dispatch failures. Nested counters
// replace their parent; they do not contribute to the parent's totals.
func WithUsage(ctx context.Context) (context.Context, *UsageCounter) {
	c := &UsageCounter{}
	return context.WithValue(ctx, usageKey{}, c), c
}

func (c *UsageCounter) Snapshot() (generation, embedding domain.Usage) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.generation, c.embedding
}

func countUsage(ctx context.Context, embedding bool, u domain.Usage) {
	c, ok := ctx.Value(usageKey{}).(*UsageCounter)
	if !ok {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	v := &c.generation
	if embedding {
		v = &c.embedding
	}
	v.Input += u.Input
	v.Output += u.Output
	v.Estimated = v.Estimated || u.Estimated
	v.Unresolved = v.Unresolved || u.Unresolved
}
