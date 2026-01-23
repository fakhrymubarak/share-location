package repository

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/fakhry/share-location/backend/internal/features/conversations/domain"
	"github.com/fakhry/share-location/backend/internal/repo/pg"
	shareddomain "github.com/fakhry/share-location/backend/internal/shared/domain"
)

// PgConversationRepository implements repo.ConversationRepository using Postgres.
type PgConversationRepository struct {
	DB  pg.PgxPool
	now func() time.Time
}

func NewPgConversationRepository(db pg.PgxPool, now func() time.Time) *PgConversationRepository {
	return &PgConversationRepository{DB: db, now: now}
}

func (r *PgConversationRepository) Create(ctx context.Context, conv domain.Conversation) (domain.Conversation, error) {
	if conv.ID == uuid.Nil {
		conv.ID = uuid.New()
	}
	if conv.CreatedAt.IsZero() {
		conv.CreatedAt = nowOr(r.now, time.Now())
	}

	const query = `
INSERT INTO conversations (id, type, title, direct_pair_key, created_at)
VALUES ($1, $2, $3, $4, $5)
RETURNING id, type, title, direct_pair_key, created_at`

	var stored domain.Conversation
	if err := r.DB.QueryRow(ctx, query, conv.ID, conv.Type, conv.Title, conv.DirectPairKey, conv.CreatedAt).
		Scan(&stored.ID, &stored.Type, &stored.Title, &stored.DirectPairKey, &stored.CreatedAt); err != nil {
		return domain.Conversation{}, err
	}
	return stored, nil
}

func (r *PgConversationRepository) GetByID(ctx context.Context, id uuid.UUID) (domain.Conversation, error) {
	const query = `SELECT id, type, title, direct_pair_key, created_at FROM conversations WHERE id=$1`
	var conv domain.Conversation
	if err := r.DB.QueryRow(ctx, query, id).Scan(&conv.ID, &conv.Type, &conv.Title, &conv.DirectPairKey, &conv.CreatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Conversation{}, shareddomain.ErrNotFound
		}
		return domain.Conversation{}, err
	}
	return conv, nil
}

func (r *PgConversationRepository) FindByDirectPairKey(ctx context.Context, pairKey string) (domain.Conversation, error) {
	const query = `SELECT id, type, title, direct_pair_key, created_at FROM conversations WHERE direct_pair_key=$1`
	var conv domain.Conversation
	if err := r.DB.QueryRow(ctx, query, pairKey).Scan(&conv.ID, &conv.Type, &conv.Title, &conv.DirectPairKey, &conv.CreatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Conversation{}, shareddomain.ErrNotFound
		}
	}
	return conv, nil
}

func (r *PgConversationRepository) ListByUser(ctx context.Context, userID uuid.UUID) ([]domain.Conversation, error) {
	const query = `
		SELECT 
			c.id, 
			c.type, 
			c.title, 
			c.direct_pair_key, 
			c.created_at,
			COALESCE(
				json_agg(
					json_build_object('id', u.id, 'username', u.username)
				) FILTER (WHERE u.id IS NOT NULL),
				'[]'
			) as participants,
			(
				SELECT json_build_object(
					'id', m.id,
					'conversation_id', m.conversation_id,
					'sender_id', m.sender_id,
					'message_type', m.message_type,
					'body', m.body,
					'created_at', m.created_at
				)
				FROM messages m
				WHERE m.conversation_id = c.id
				ORDER BY m.created_at DESC, m.id DESC
				LIMIT 1
			) as last_message
		FROM conversations c
		JOIN conversation_members cm ON c.id = cm.conversation_id
		JOIN conversation_members cm_all ON c.id = cm_all.conversation_id
		JOIN users u ON cm_all.user_id = u.id
		WHERE cm.user_id = $1
		GROUP BY c.id
		ORDER BY c.created_at DESC`

	rows, err := r.DB.Query(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var convs []domain.Conversation
	for rows.Next() {
		var c domain.Conversation
		var lastMsgBytes []byte
		if err := rows.Scan(&c.ID, &c.Type, &c.Title, &c.DirectPairKey, &c.CreatedAt, &c.Participants, &lastMsgBytes); err != nil {
			return nil, err
		}
		if len(lastMsgBytes) > 0 {
			var msg domain.ConversationMessage
			if err := json.Unmarshal(lastMsgBytes, &msg); err == nil {
				c.LastMessage = &msg
			}
		}
		convs = append(convs, c)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return convs, nil
}

func nowOr(now func() time.Time, defaultTime time.Time) time.Time {
	if now == nil {
		return defaultTime
	}
	return now()
}
