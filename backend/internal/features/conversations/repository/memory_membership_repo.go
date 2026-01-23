package repository

import (
	"context"
	"sync"
	"time"

	"github.com/fakhry/share-location/backend/internal/features/conversations/domain"
	"github.com/google/uuid"
)

type MemoryMembershipRepository struct {
	mu      sync.Mutex
	members map[uuid.UUID]map[uuid.UUID]domain.ConversationMember
	now     func() time.Time
}

// NewMemoryMembershipRepository returns an in-memory membership repository.
func NewMemoryMembershipRepository(now func() time.Time) *MemoryMembershipRepository {
	if now == nil {
		now = time.Now
	}
	return &MemoryMembershipRepository{
		members: make(map[uuid.UUID]map[uuid.UUID]domain.ConversationMember),
		now:     now,
	}
}

func (r *MemoryMembershipRepository) Add(ctx context.Context, members []domain.ConversationMember) error {
	_ = ctx
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, m := range members {
		convMap, ok := r.members[m.ConversationID]
		if !ok {
			convMap = make(map[uuid.UUID]domain.ConversationMember)
			r.members[m.ConversationID] = convMap
		}
		if m.JoinedAt.IsZero() {
			m.JoinedAt = r.now()
		}
		convMap[m.UserID] = m
	}
	return nil
}

func (r *MemoryMembershipRepository) IsMember(ctx context.Context, conversationID, userID uuid.UUID) (bool, error) {
	_ = ctx
	r.mu.Lock()
	defer r.mu.Unlock()

	if convMap, ok := r.members[conversationID]; ok {
		_, exists := convMap[userID]
		return exists, nil
	}
	return false, nil
}

func (r *MemoryMembershipRepository) ListByConversation(ctx context.Context, conversationID uuid.UUID) ([]domain.ConversationMember, error) {
	_ = ctx
	r.mu.Lock()
	defer r.mu.Unlock()

	convMap, ok := r.members[conversationID]
	if !ok {
		return nil, nil
	}
	out := make([]domain.ConversationMember, 0, len(convMap))
	for _, m := range convMap {
		out = append(out, m)
	}
	return out, nil
}
