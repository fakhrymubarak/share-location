package repository

import (
	"context"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/fakhry/share-location/backend/internal/features/messaging/domain"
)

type MemoryMessageRepository struct {
	mu           sync.Mutex
	messages     map[uuid.UUID][]domain.Message // conversationID -> messages
	messagesByID map[uuid.UUID]domain.Message   // ID -> message
	keyIndex     map[string]uuid.UUID           // dedupe key -> ID
	now          func() time.Time
}

func NewMessageRepository(now func() time.Time) *MemoryMessageRepository {
	if now == nil {
		now = time.Now
	}
	return &MemoryMessageRepository{
		messages:     make(map[uuid.UUID][]domain.Message),
		messagesByID: make(map[uuid.UUID]domain.Message),
		keyIndex:     make(map[string]uuid.UUID),
		now:          now,
	}
}

func (r *MemoryMessageRepository) InsertOrGet(ctx context.Context, msg domain.Message) (domain.Message, bool, error) {
	_ = ctx
	r.mu.Lock()
	defer r.mu.Unlock()

	key := dedupeKey(msg.ConversationID, msg.SenderID, msg.ClientMsgID)
	if existingID, ok := r.keyIndex[key]; ok {
		return r.messagesByID[existingID], false, nil
	}

	if msg.ID == uuid.Nil {
		msg.ID = uuid.New()
	}
	if msg.CreatedAt.IsZero() {
		msg.CreatedAt = r.now()
	}

	r.keyIndex[key] = msg.ID
	r.messagesByID[msg.ID] = msg
	r.messages[msg.ConversationID] = append(r.messages[msg.ConversationID], msg)
	return msg, true, nil
}

func (r *MemoryMessageRepository) ListHistory(ctx context.Context, conversationID uuid.UUID, limit int) ([]domain.Message, error) {
	_ = ctx
	r.mu.Lock()
	defer r.mu.Unlock()

	msgs, ok := r.messages[conversationID]
	if !ok {
		return []domain.Message{}, nil
	}

	// Sort by time asc (history usually returned newest first or oldest first? Service says ordered. Let's stick to simple sort)
	// Usually chat history is time desc (newest) or asc (oldest)?
	// Let's assume time desc for now based on previous implementation or sort.
	// Actually, let's match postgres implementation: ORDER BY created_at, id (which is ASC usually unless DESC specified)
	// Postgres repo had: ORDER BY created_at, id. So it's ASC (oldest first).

	out := make([]domain.Message, len(msgs))
	copy(out, msgs)
	sort.Slice(out, func(i, j int) bool {
		if out[i].CreatedAt.Equal(out[j].CreatedAt) {
			return out[i].ID.String() < out[j].ID.String()
		}
		return out[i].CreatedAt.Before(out[j].CreatedAt)
	})

	if limit > 0 && len(out) > limit {
		// If we want history, maybe we want latest N?
		// If prompt says "history", usually it implies latest messages.
		// But query in pg says: ORDER BY created_at. That gives oldest messages.
		// If I want latest, I should have used DESC.
		// I will stick to what PG repo does: ASC.
		out = out[:limit]
	}
	return out, nil
}

func dedupeKey(convID, senderID uuid.UUID, clientMsgID string) string {
	return convID.String() + "|" + senderID.String() + "|" + clientMsgID
}
