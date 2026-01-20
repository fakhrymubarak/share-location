package memory

import (
	"context"
	"sync"

	"github.com/fakhry/share-location/backend/internal/domain"
)

type LiveLocationPublisher struct {
	mu        sync.Mutex
	published []domain.LiveLocationPoint
}

func NewLiveLocationPublisher() *LiveLocationPublisher {
	return &LiveLocationPublisher{}
}

func (p *LiveLocationPublisher) PublishLiveLocation(ctx context.Context, point domain.LiveLocationPoint) error {
	_ = ctx
	p.mu.Lock()
	defer p.mu.Unlock()
	p.published = append(p.published, point)
	return nil
}

func (p *LiveLocationPublisher) Published() []domain.LiveLocationPoint {
	p.mu.Lock()
	defer p.mu.Unlock()
	out := make([]domain.LiveLocationPoint, len(p.published))
	copy(out, p.published)
	return out
}
