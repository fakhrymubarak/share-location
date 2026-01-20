package service

import (
	"context"
	"strings"

	"github.com/fakhry/share-location/backend/internal/domain"
	"github.com/fakhry/share-location/backend/internal/repo"
)

// UserService handles user lifecycle actions such as handshake upsert.
type UserService struct {
	Users repo.UserRepository
}

func NewUserService(users repo.UserRepository) *UserService {
	return &UserService{Users: users}
}

// Handshake upserts a user by username, returning the resolved user.
func (s *UserService) Handshake(ctx context.Context, username string) (domain.User, error) {
	username = strings.TrimSpace(username)
	if username == "" {
		return domain.User{}, domain.ErrInvalidInput
	}
	displayName := username
	return s.Users.UpsertByUsername(ctx, username, displayName)
}
