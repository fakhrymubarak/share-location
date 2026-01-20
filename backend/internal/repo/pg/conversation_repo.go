package pg

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/fakhry/share-location/backend/internal/domain"
)

// ConversationRepository implements repo.ConversationRepository using Postgres.
type ConversationRepository struct {
	DB  PgxPool
	now func() time.Time
}

func NewConversationRepository(db PgxPool, now func() time.Time) *ConversationRepository {
	return &ConversationRepository{DB: db, now: now}
}

func (r *ConversationRepository) Create(ctx context.Context, conv domain.Conversation) (domain.Conversation, error) {
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

func (r *ConversationRepository) GetByID(ctx context.Context, id uuid.UUID) (domain.Conversation, error) {
	const query = `SELECT id, type, title, direct_pair_key, created_at FROM conversations WHERE id=$1`
	var conv domain.Conversation
	if err := r.DB.QueryRow(ctx, query, id).Scan(&conv.ID, &conv.Type, &conv.Title, &conv.DirectPairKey, &conv.CreatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Conversation{}, domain.ErrNotFound
		}
		return domain.Conversation{}, err
	}
	return conv, nil
}

func (r *ConversationRepository) FindByDirectPairKey(ctx context.Context, pairKey string) (domain.Conversation, error) {
	const query = `SELECT id, type, title, direct_pair_key, created_at FROM conversations WHERE direct_pair_key=$1`
	var conv domain.Conversation
	if err := r.DB.QueryRow(ctx, query, pairKey).Scan(&conv.ID, &conv.Type, &conv.Title, &conv.DirectPairKey, &conv.CreatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Conversation{}, domain.ErrNotFound
		}
		return domain.Conversation{}, err
	}
	return conv, nil
}
