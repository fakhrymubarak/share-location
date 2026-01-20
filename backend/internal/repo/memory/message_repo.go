package memory

import (
	"context"
	"sort"
	"sync"
	"time"

	"github.com/fakhry/share-location/backend/internal/domain"
	"github.com/google/uuid"
)

type MessageRepository struct {
	mu             sync.Mutex
	messagesByID   map[uuid.UUID]domain.Message
	messagesByConv map[uuid.UUID][]domain.Message
	keyIndex       map[string]uuid.UUID
	now            func() time.Time
}

// NewMessageRepository returns an in-memory message repo with idempotency support.
func NewMessageRepository(now func() time.Time) *MessageRepository {
	if now == nil {
		now = time.Now
	}
	return &MessageRepository{
		messagesByID:   make(map[uuid.UUID]domain.Message),
		messagesByConv: make(map[uuid.UUID][]domain.Message),
		keyIndex:       make(map[string]uuid.UUID),
		now:            now,
	}
}

func (r *MessageRepository) InsertOrGet(ctx context.Context, msg domain.Message) (domain.Message, bool, error) {
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
	r.messagesByConv[msg.ConversationID] = append(r.messagesByConv[msg.ConversationID], msg)
	return msg, true, nil
}

func (r *MessageRepository) ListByConversation(ctx context.Context, conversationID uuid.UUID, limit int) ([]domain.Message, error) {
	_ = ctx
	r.mu.Lock()
	defer r.mu.Unlock()

	msgs := r.messagesByConv[conversationID]
	sort.Slice(msgs, func(i, j int) bool {
		if msgs[i].CreatedAt.Equal(msgs[j].CreatedAt) {
			return msgs[i].ID.String() < msgs[j].ID.String()
		}
		return msgs[i].CreatedAt.Before(msgs[j].CreatedAt)
	})

	if limit > 0 && len(msgs) > limit {
		msgs = msgs[:limit]
	}
	out := make([]domain.Message, len(msgs))
	copy(out, msgs)
	return out, nil
}

func dedupeKey(convID, senderID uuid.UUID, clientMsgID string) string {
	return convID.String() + "|" + senderID.String() + "|" + clientMsgID
}
