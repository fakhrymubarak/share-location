package memory

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/fakhry/share-location/backend/internal/domain"
	"github.com/google/uuid"
)

type UserRepository struct {
	mu          sync.Mutex
	usersByName map[string]domain.User
	usersByID   map[uuid.UUID]domain.User
	now         func() time.Time
}

// NewUserRepository returns an in-memory user repository.
func NewUserRepository(now func() time.Time) *UserRepository {
	if now == nil {
		now = time.Now
	}
	return &UserRepository{
		usersByName: make(map[string]domain.User),
		usersByID:   make(map[uuid.UUID]domain.User),
		now:         now,
	}
}

func (r *UserRepository) UpsertByUsername(ctx context.Context, username, displayName string) (domain.User, error) {
	_ = ctx
	key := strings.ToLower(username)

	r.mu.Lock()
	defer r.mu.Unlock()

	if user, ok := r.usersByName[key]; ok {
		return user, nil
	}

	u := domain.User{
		ID:          uuid.New(),
		Username:    username,
		DisplayName: displayName,
		CreatedAt:   r.now(),
	}
	r.usersByName[key] = u
	r.usersByID[u.ID] = u
	return u, nil
}

func (r *UserRepository) GetByID(ctx context.Context, id uuid.UUID) (domain.User, error) {
	_ = ctx
	r.mu.Lock()
	defer r.mu.Unlock()

	if user, ok := r.usersByID[id]; ok {
		return user, nil
	}
	return domain.User{}, domain.ErrNotFound
}

func (r *UserRepository) GetByUsername(ctx context.Context, username string) (domain.User, error) {
	_ = ctx
	r.mu.Lock()
	defer r.mu.Unlock()

	if user, ok := r.usersByName[strings.ToLower(username)]; ok {
		return user, nil
	}
	return domain.User{}, domain.ErrNotFound
}
