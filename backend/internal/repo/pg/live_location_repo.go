package pg

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/fakhry/share-location/backend/internal/domain"
)

// LivePointStore provides persistence for live location points (e.g., Redis).
type LivePointStore interface {
	SavePoint(ctx context.Context, point domain.LiveLocationPoint) error
	GetPoint(ctx context.Context, streamID uuid.UUID) (domain.LiveLocationPoint, error)
}

// LiveLocationRepository stores session metadata in Postgres and delegates point storage.
type LiveLocationRepository struct {
	DB     PgxPool
	Points LivePointStore
	now    func() time.Time
}

func NewLiveLocationRepository(db PgxPool, points LivePointStore, now func() time.Time) *LiveLocationRepository {
	return &LiveLocationRepository{DB: db, Points: points, now: now}
}

func (r *LiveLocationRepository) UpsertSession(ctx context.Context, session domain.LiveLocationSession) (domain.LiveLocationSession, error) {
	if session.ID == uuid.Nil {
		session.ID = uuid.New()
	}
	if session.CreatedAt.IsZero() {
		session.CreatedAt = nowOr(r.now, time.Now())
	}
	const query = `
INSERT INTO live_location_sessions (id, user_id, conversation_id, expires_at)
VALUES ($1, $2, $3, $4)
ON CONFLICT (id) DO UPDATE
SET user_id=EXCLUDED.user_id, conversation_id=EXCLUDED.conversation_id, expires_at=EXCLUDED.expires_at
RETURNING id, user_id, conversation_id, expires_at`

	var stored domain.LiveLocationSession
	if err := r.DB.QueryRow(ctx, query, session.ID, session.UserID, session.ConversationID, session.ExpiresAt).
		Scan(&stored.ID, &stored.UserID, &stored.ConversationID, &stored.ExpiresAt); err != nil {
		return domain.LiveLocationSession{}, err
	}
	return stored, nil
}

func (r *LiveLocationRepository) GetSession(ctx context.Context, streamID uuid.UUID) (domain.LiveLocationSession, error) {
	const query = `SELECT id, user_id, conversation_id, expires_at FROM live_location_sessions WHERE id=$1 AND expires_at > now()`
	var session domain.LiveLocationSession
	if err := r.DB.QueryRow(ctx, query, streamID).
		Scan(&session.ID, &session.UserID, &session.ConversationID, &session.ExpiresAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.LiveLocationSession{}, domain.ErrNotFound
		}
		return domain.LiveLocationSession{}, err
	}
	return session, nil
}

func (r *LiveLocationRepository) SavePoint(ctx context.Context, point domain.LiveLocationPoint) error {
	if r.Points == nil {
		return nil
	}
	return r.Points.SavePoint(ctx, point)
}

func (r *LiveLocationRepository) GetPoint(ctx context.Context, streamID uuid.UUID) (domain.LiveLocationPoint, error) {
	if r.Points == nil {
		return domain.LiveLocationPoint{}, domain.ErrNotFound
	}
	return r.Points.GetPoint(ctx, streamID)
}
