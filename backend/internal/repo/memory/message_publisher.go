package memory

import (
	"context"
	"sync"

	"github.com/fakhry/share-location/backend/internal/domain"
)

// MessagePublisher captures published messages for testing.
type MessagePublisher struct {
	mu        sync.Mutex
	published []domain.Message
}

func NewMessagePublisher() *MessagePublisher {
	return &MessagePublisher{}
}

func (p *MessagePublisher) PublishMessage(ctx context.Context, msg domain.Message) error {
	_ = ctx
	p.mu.Lock()
	defer p.mu.Unlock()
	p.published = append(p.published, msg)
	return nil
}

func (p *MessagePublisher) Published() []domain.Message {
	p.mu.Lock()
	defer p.mu.Unlock()
	out := make([]domain.Message, len(p.published))
	copy(out, p.published)
	return out
}
