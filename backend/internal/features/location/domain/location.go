package domain

import (
	"context"
	"time"

	"github.com/google/uuid"

	shareddomain "github.com/fakhry/share-location/backend/internal/shared/domain"
)

var (
	ErrNotFound     = shareddomain.ErrNotFound
	ErrInvalidInput = shareddomain.ErrInvalidInput
	ErrForbidden    = shareddomain.ErrForbidden
)

// LiveLocationSession tracks active publisher/subscriber session metadata.
type LiveLocationSession struct {
	ID             uuid.UUID
	UserID         uuid.UUID
	ConversationID uuid.UUID
	ExpiresAt      time.Time
	CreatedAt      time.Time
}

// LiveLocationPoint represents the latest location for a stream.
type LiveLocationPoint struct {
	StreamID       uuid.UUID
	UserID         uuid.UUID
	ConversationID uuid.UUID
	Lat            float64
	Lng            float64
	Accuracy       float64
	Speed          float64
	Heading        float64
	UpdatedAt      time.Time
	ExpiresAt      time.Time
}

type LiveLocationRepository interface {
	UpsertSession(ctx context.Context, session LiveLocationSession) (LiveLocationSession, error)
	GetSession(ctx context.Context, sessionID uuid.UUID) (LiveLocationSession, error)
	SavePoint(ctx context.Context, point LiveLocationPoint) error
	GetPoint(ctx context.Context, streamID uuid.UUID) (LiveLocationPoint, error)
}

type LiveLocationPublisher interface {
	PublishLiveLocation(ctx context.Context, point LiveLocationPoint) error
}
