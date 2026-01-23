package repository

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/fakhry/share-location/backend/internal/features/auth/domain"
	"github.com/fakhry/share-location/backend/internal/repo/pg"
	shareddomain "github.com/fakhry/share-location/backend/internal/shared/domain"
)

// PgUserRepository implements repo.UserRepository backed by Postgres.
type PgUserRepository struct {
	DB pg.PgxPool
}

func NewPgUserRepository(db pg.PgxPool) *PgUserRepository {
	return &PgUserRepository{DB: db}
}

func (r *PgUserRepository) UpsertByUsername(ctx context.Context, username, displayName string) (domain.User, error) {
	const query = `
INSERT INTO users (username, display_name)
VALUES ($1, $2)
ON CONFLICT (username) DO UPDATE
SET display_name = EXCLUDED.display_name
RETURNING id, username, display_name, created_at`

	var user domain.User
	if err := r.DB.QueryRow(ctx, query, username, displayName).Scan(&user.ID, &user.Username, &user.DisplayName, &user.CreatedAt); err != nil {
		return domain.User{}, err
	}
	return user, nil
}

func (r *PgUserRepository) GetByID(ctx context.Context, id uuid.UUID) (domain.User, error) {
	const query = `SELECT id, username, display_name, created_at FROM users WHERE id=$1`
	var user domain.User
	if err := r.DB.QueryRow(ctx, query, id).Scan(&user.ID, &user.Username, &user.DisplayName, &user.CreatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.User{}, shareddomain.ErrNotFound
		}
		return domain.User{}, err
	}
	return user, nil
}

func (r *PgUserRepository) GetByUsername(ctx context.Context, username string) (domain.User, error) {
	const query = `SELECT id, username, display_name, created_at FROM users WHERE username=$1`
	var user domain.User
	if err := r.DB.QueryRow(ctx, query, username).Scan(&user.ID, &user.Username, &user.DisplayName, &user.CreatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.User{}, shareddomain.ErrNotFound
		}
		return domain.User{}, err
	}
	return user, nil
}
