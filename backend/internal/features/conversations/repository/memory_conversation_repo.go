package repository

import (
	"context"
	"sync"
	"time"

	"github.com/fakhry/share-location/backend/internal/features/conversations/domain"
	shareddomain "github.com/fakhry/share-location/backend/internal/shared/domain"
	"github.com/google/uuid"
)

type MemoryConversationRepository struct {
	mu        sync.Mutex
	convs     map[uuid.UUID]domain.Conversation
	pairIndex map[string]uuid.UUID
	now       func() time.Time
}

// NewMemoryConversationRepository creates an in-memory conversation repository.
func NewMemoryConversationRepository(now func() time.Time) *MemoryConversationRepository {
	if now == nil {
		now = time.Now
	}
	return &MemoryConversationRepository{
		convs:     make(map[uuid.UUID]domain.Conversation),
		pairIndex: make(map[string]uuid.UUID),
		now:       now,
	}
}

func (r *MemoryConversationRepository) Create(ctx context.Context, conv domain.Conversation) (domain.Conversation, error) {
	_ = ctx
	r.mu.Lock()
	defer r.mu.Unlock()

	if conv.ID == uuid.Nil {
		conv.ID = uuid.New()
	}
	if conv.CreatedAt.IsZero() {
		conv.CreatedAt = r.now()
	}
	r.convs[conv.ID] = conv
	if conv.DirectPairKey != "" {
		r.pairIndex[conv.DirectPairKey] = conv.ID
	}
	return conv, nil
}

func (r *MemoryConversationRepository) GetByID(ctx context.Context, id uuid.UUID) (domain.Conversation, error) {
	_ = ctx
	r.mu.Lock()
	defer r.mu.Unlock()

	if conv, ok := r.convs[id]; ok {
		return conv, nil
	}
	return domain.Conversation{}, shareddomain.ErrNotFound
}

func (r *MemoryConversationRepository) FindByDirectPairKey(ctx context.Context, pairKey string) (domain.Conversation, error) {
	_ = ctx
	r.mu.Lock()
	defer r.mu.Unlock()

	if id, ok := r.pairIndex[pairKey]; ok {
		return r.convs[id], nil
	}
	return domain.Conversation{}, shareddomain.ErrNotFound
}

func (r *MemoryConversationRepository) ListByUser(ctx context.Context, userID uuid.UUID) ([]domain.Conversation, error) {
	_ = ctx
	r.mu.Lock()
	defer r.mu.Unlock()

	var result []domain.Conversation
	for _, c := range r.convs {
		isMember := false
		for _, p := range c.Participants {
			if p.ID == userID {
				isMember = true
				break
			}
		}
		if isMember {
			result = append(result, c)
		}
	}
	return result, nil
}
