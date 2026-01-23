package repository

import (
	"context"
	"testing"
	"time"

	"github.com/fakhry/share-location/backend/internal/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/pashagolub/pgxmock/v4"
	"github.com/stretchr/testify/require"
)

func TestUserUpsertByUsername(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	repo := NewPgUserRepository(mock)
	id := uuid.New()
	now := time.Now()

	mock.ExpectQuery(`INSERT INTO users`).
		WithArgs("alice", "Alice").
		WillReturnRows(pgxmock.NewRows([]string{"id", "username", "display_name", "created_at"}).
			AddRow(id, "alice", "Alice", now))

	user, err := repo.UpsertByUsername(context.Background(), "alice", "Alice")
	require.NoError(t, err)
	require.Equal(t, id, user.ID)
	require.Equal(t, "alice", user.Username)
	require.Equal(t, "Alice", user.DisplayName)
	require.WithinDuration(t, now, user.CreatedAt, time.Second)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestUserGetByID(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	repo := NewPgUserRepository(mock)
	id := uuid.New()
	now := time.Now()

	mock.ExpectQuery(`SELECT id, username, display_name, created_at FROM users WHERE id=`).
		WithArgs(id).
		WillReturnRows(pgxmock.NewRows([]string{"id", "username", "display_name", "created_at"}).
			AddRow(id, "bob", "Bob", now))

	user, err := repo.GetByID(context.Background(), id)
	require.NoError(t, err)
	require.Equal(t, "bob", user.Username)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestUserGetByID_NotFound(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	repo := NewPgUserRepository(mock)
	id := uuid.New()

	mock.ExpectQuery(`SELECT id, username, display_name, created_at FROM users WHERE id=`).
		WithArgs(id).
		WillReturnError(pgx.ErrNoRows)

	_, err = repo.GetByID(context.Background(), id)
	require.ErrorIs(t, err, domain.ErrNotFound)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestUserGetByUsername(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	repo := NewPgUserRepository(mock)
	id := uuid.New()
	now := time.Now()

	mock.ExpectQuery(`SELECT id, username, display_name, created_at FROM users WHERE username=`).
		WithArgs("carol").
		WillReturnRows(pgxmock.NewRows([]string{"id", "username", "display_name", "created_at"}).
			AddRow(id, "carol", "Carol", now))

	user, err := repo.GetByUsername(context.Background(), "carol")
	require.NoError(t, err)
	require.Equal(t, id, user.ID)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestUserGetByUsername_NotFound(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	repo := NewPgUserRepository(mock)

	mock.ExpectQuery(`SELECT id, username, display_name, created_at FROM users WHERE username=`).
		WithArgs("missing").
		WillReturnError(pgx.ErrNoRows)

	_, err = repo.GetByUsername(context.Background(), "missing")
	require.ErrorIs(t, err, domain.ErrNotFound)
	require.NoError(t, mock.ExpectationsWereMet())
}
