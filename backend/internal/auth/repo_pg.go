package auth

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	shareddomain "github.com/fakhry/share-location/backend/internal/shared/domain"
)

// PgUserRepository implements repo.UserRepository backed by Postgres.
type PgUserRepository struct {
	DB *pgxpool.Pool
}

func NewPgUserRepository(db *pgxpool.Pool) *PgUserRepository {
	return &PgUserRepository{DB: db}
}

func (r *PgUserRepository) UpsertByUsername(ctx context.Context, username, passwordHash, displayName string) (User, error) {
	const query = `
INSERT INTO users (username, password_hash, display_name)
VALUES ($1, $2, $3)
ON CONFLICT (username) DO UPDATE
SET display_name = EXCLUDED.display_name, password_hash = EXCLUDED.password_hash
RETURNING id, username, display_name, password_hash, created_at`

	var user User
	if err := r.DB.QueryRow(ctx, query, username, passwordHash, displayName).Scan(&user.ID, &user.Username, &user.DisplayName, &user.PasswordHash, &user.CreatedAt); err != nil {
		return User{}, err
	}
	return user, nil
}

func (r *PgUserRepository) GetByID(ctx context.Context, id uuid.UUID) (User, error) {
	const query = `SELECT id, username, display_name, password_hash, created_at FROM users WHERE id=$1`
	var user User
	if err := r.DB.QueryRow(ctx, query, id).Scan(&user.ID, &user.Username, &user.DisplayName, &user.PasswordHash, &user.CreatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return User{}, shareddomain.ErrNotFound
		}
		return User{}, err
	}
	return user, nil
}

func (r *PgUserRepository) GetByUsername(ctx context.Context, username string) (User, error) {
	const query = `SELECT id, username, display_name, password_hash, created_at FROM users WHERE username=$1`
	var user User
	if err := r.DB.QueryRow(ctx, query, username).Scan(&user.ID, &user.Username, &user.DisplayName, &user.PasswordHash, &user.CreatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return User{}, shareddomain.ErrNotFound
		}
		return User{}, err
	}
	return user, nil
}
