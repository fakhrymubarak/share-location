package memory

import (
	"context"
	"sync"
	"time"

	"github.com/fakhry/share-location/backend/internal/domain"
	"github.com/google/uuid"
)

type LiveLocationRepository struct {
	mu       sync.Mutex
	sessions map[uuid.UUID]domain.LiveLocationSession
	points   map[uuid.UUID]domain.LiveLocationPoint
	now      func() time.Time
}

// NewLiveLocationRepository returns an in-memory live location repository.
func NewLiveLocationRepository(now func() time.Time) *LiveLocationRepository {
	if now == nil {
		now = time.Now
	}
	return &LiveLocationRepository{
		sessions: make(map[uuid.UUID]domain.LiveLocationSession),
		points:   make(map[uuid.UUID]domain.LiveLocationPoint),
		now:      now,
	}
}

func (r *LiveLocationRepository) UpsertSession(ctx context.Context, session domain.LiveLocationSession) (domain.LiveLocationSession, error) {
	_ = ctx
	r.mu.Lock()
	defer r.mu.Unlock()

	if session.ID == uuid.Nil {
		session.ID = uuid.New()
	}
	if session.CreatedAt.IsZero() {
		session.CreatedAt = r.now()
	}
	r.sessions[session.ID] = session
	return session, nil
}

func (r *LiveLocationRepository) GetSession(ctx context.Context, streamID uuid.UUID) (domain.LiveLocationSession, error) {
	_ = ctx
	r.mu.Lock()
	defer r.mu.Unlock()

	if s, ok := r.sessions[streamID]; ok {
		return s, nil
	}
	return domain.LiveLocationSession{}, domain.ErrNotFound
}

func (r *LiveLocationRepository) SavePoint(ctx context.Context, point domain.LiveLocationPoint) error {
	_ = ctx
	r.mu.Lock()
	defer r.mu.Unlock()
	r.points[point.StreamID] = point
	return nil
}

func (r *LiveLocationRepository) GetPoint(ctx context.Context, streamID uuid.UUID) (domain.LiveLocationPoint, error) {
	_ = ctx
	r.mu.Lock()
	defer r.mu.Unlock()
	if p, ok := r.points[streamID]; ok {
		return p, nil
	}
	return domain.LiveLocationPoint{}, domain.ErrNotFound
}
