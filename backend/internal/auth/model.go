package auth

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// User represents an authenticated person identified by username.
type User struct {
	ID           uuid.UUID
	Username     string
	DisplayName  string
	PasswordHash string
	CreatedAt    time.Time
}

// UserRepository manages user records.
type UserRepository interface {
	UpsertByUsername(ctx context.Context, username, passwordHash, displayName string) (User, error)
	GetByID(ctx context.Context, id uuid.UUID) (User, error)
	GetByUsername(ctx context.Context, username string) (User, error)
}
