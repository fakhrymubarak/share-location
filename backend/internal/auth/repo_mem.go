package auth

import (
	"context"
	"strings"
	"sync"
	"time"

	shareddomain "github.com/fakhry/share-location/backend/internal/shared/domain"
	"github.com/google/uuid"
)

type MemoryUserRepository struct {
	mu          sync.Mutex
	usersByName map[string]User
	usersByID   map[uuid.UUID]User
	now         func() time.Time
}

// NewMemoryUserRepository returns an in-memory user repository.
func NewMemoryUserRepository(now func() time.Time) *MemoryUserRepository {
	if now == nil {
		now = time.Now
	}
	return &MemoryUserRepository{
		usersByName: make(map[string]User),
		usersByID:   make(map[uuid.UUID]User),
		now:         now,
	}
}

func (r *MemoryUserRepository) UpsertByUsername(ctx context.Context, username, passwordHash, displayName string) (User, error) {
	_ = ctx
	key := strings.ToLower(username)

	r.mu.Lock()
	defer r.mu.Unlock()

	if user, ok := r.usersByName[key]; ok {
		return user, nil
	}

	u := User{
		ID:           uuid.New(),
		Username:     username,
		DisplayName:  displayName,
		PasswordHash: passwordHash,
		CreatedAt:    r.now(),
	}
	r.usersByName[key] = u
	r.usersByID[u.ID] = u
	return u, nil
}

func (r *MemoryUserRepository) GetByID(ctx context.Context, id uuid.UUID) (User, error) {
	_ = ctx
	r.mu.Lock()
	defer r.mu.Unlock()

	if user, ok := r.usersByID[id]; ok {
		return user, nil
	}
	return User{}, shareddomain.ErrNotFound
}

func (r *MemoryUserRepository) GetByUsername(ctx context.Context, username string) (User, error) {
	_ = ctx
	r.mu.Lock()
	defer r.mu.Unlock()

	if user, ok := r.usersByName[strings.ToLower(username)]; ok {
		return user, nil
	}
	return User{}, shareddomain.ErrNotFound
}
