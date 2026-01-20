package pg

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/fakhry/share-location/backend/internal/domain"
)

// MessageRepository implements repo.MessageRepository using Postgres.
type MessageRepository struct {
	DB  PgxPool
	now func() time.Time
}

func NewMessageRepository(db PgxPool, now func() time.Time) *MessageRepository {
	return &MessageRepository{DB: db, now: now}
}

func (r *MessageRepository) InsertOrGet(ctx context.Context, msg domain.Message) (domain.Message, bool, error) {
	if msg.ID == uuid.Nil {
		msg.ID = uuid.New()
	}
	if msg.CreatedAt.IsZero() {
		msg.CreatedAt = nowOr(r.now, time.Now())
	}

	bodyBytes, err := json.Marshal(msg.Body)
	if err != nil {
		return domain.Message{}, false, err
	}

	const insertQuery = `
INSERT INTO messages (id, conversation_id, sender_id, body, message_type, client_msg_id, created_at)
VALUES ($1, $2, $3, $4, $5, $6, $7)
ON CONFLICT (conversation_id, sender_id, client_msg_id) DO NOTHING
RETURNING id, conversation_id, sender_id, body, message_type, client_msg_id, created_at`

	var stored domain.Message
	var storedBody []byte
	err = r.DB.QueryRow(ctx, insertQuery, msg.ID, msg.ConversationID, msg.SenderID, bodyBytes, msg.MessageType, msg.ClientMsgID, msg.CreatedAt).
		Scan(&stored.ID, &stored.ConversationID, &stored.SenderID, &storedBody, &stored.MessageType, &stored.ClientMsgID, &stored.CreatedAt)
	switch {
	case err == nil:
		if err := json.Unmarshal(storedBody, &stored.Body); err != nil {
			return domain.Message{}, false, err
		}
		return stored, true, nil
	case errors.Is(err, pgx.ErrNoRows):
		existing, err := r.fetchExisting(ctx, msg.ConversationID, msg.SenderID, msg.ClientMsgID)
		return existing, false, err
	default:
		return domain.Message{}, false, err
	}
}

func (r *MessageRepository) fetchExisting(ctx context.Context, conversationID, senderID uuid.UUID, clientMsgID string) (domain.Message, error) {
	const selectQuery = `
SELECT id, conversation_id, sender_id, body, message_type, client_msg_id, created_at
FROM messages
WHERE conversation_id=$1 AND sender_id=$2 AND client_msg_id=$3`
	var msg domain.Message
	var body []byte
	if err := r.DB.QueryRow(ctx, selectQuery, conversationID, senderID, clientMsgID).
		Scan(&msg.ID, &msg.ConversationID, &msg.SenderID, &body, &msg.MessageType, &msg.ClientMsgID, &msg.CreatedAt); err != nil {
		return domain.Message{}, err
	}
	if err := json.Unmarshal(body, &msg.Body); err != nil {
		return domain.Message{}, err
	}
	return msg, nil
}

func (r *MessageRepository) ListByConversation(ctx context.Context, conversationID uuid.UUID, limit int) ([]domain.Message, error) {
	if limit <= 0 {
		limit = 50
	}
	const query = `
SELECT id, conversation_id, sender_id, body, message_type, client_msg_id, created_at
FROM messages
WHERE conversation_id=$1
ORDER BY created_at, id
LIMIT $2`
	rows, err := r.DB.Query(ctx, query, conversationID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var msgs []domain.Message
	for rows.Next() {
		var msg domain.Message
		var body []byte
		if err := rows.Scan(&msg.ID, &msg.ConversationID, &msg.SenderID, &body, &msg.MessageType, &msg.ClientMsgID, &msg.CreatedAt); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(body, &msg.Body); err != nil {
			return nil, err
		}
		msgs = append(msgs, msg)
	}
	return msgs, rows.Err()
}
