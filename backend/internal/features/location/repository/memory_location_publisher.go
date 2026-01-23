package repository

import (
	"context"
	"sync"

	"github.com/fakhry/share-location/backend/internal/features/location/domain"
)

// MemoryLocationPublisher implements LiveLocationPublisher in memory (noop/stub).
type MemoryLocationPublisher struct {
	mu        sync.Mutex
	published []domain.LiveLocationPoint
}

func NewMemoryLocationPublisher() *MemoryLocationPublisher {
	return &MemoryLocationPublisher{}
}

func (p *MemoryLocationPublisher) PublishLiveLocation(ctx context.Context, point domain.LiveLocationPoint) error {
	_ = ctx
	p.mu.Lock()
	defer p.mu.Unlock()
	p.published = append(p.published, point)
	return nil
}

func (p *MemoryLocationPublisher) Published() []domain.LiveLocationPoint {
	p.mu.Lock()
	defer p.mu.Unlock()
	out := make([]domain.LiveLocationPoint, len(p.published))
	copy(out, p.published)
	return out
}
