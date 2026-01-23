package repository

import (
	"context"
	"sync"

	"github.com/fakhry/share-location/backend/internal/features/messaging/domain"
)

type MemoryMessagePublisher struct {
	mu        sync.Mutex
	published []domain.Message
}

func NewMessagePublisher() *MemoryMessagePublisher {
	return &MemoryMessagePublisher{
		published: make([]domain.Message, 0),
	}
}

func (p *MemoryMessagePublisher) Publish(ctx context.Context, msg domain.Message) error {
	_ = ctx
	p.mu.Lock()
	defer p.mu.Unlock()
	p.published = append(p.published, msg)
	return nil
}

func (p *MemoryMessagePublisher) Published() []domain.Message {
	p.mu.Lock()
	defer p.mu.Unlock()
	out := make([]domain.Message, len(p.published))
	copy(out, p.published)
	return out
}
