package repo

import (
	"context"

	"github.com/google/uuid"
	"github.com/fakhry/share-location/backend/internal/domain"
)

// UserRepository manages user records.
type UserRepository interface {
	UpsertByUsername(ctx context.Context, username, displayName string) (domain.User, error)
	GetByID(ctx context.Context, id uuid.UUID) (domain.User, error)
	GetByUsername(ctx context.Context, username string) (domain.User, error)
}
