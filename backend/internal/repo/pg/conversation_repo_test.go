package pg

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

func TestConversationCreate(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	repo := NewConversationRepository(mock, func() time.Time { return now })

	conv := domain.Conversation{
		Type:          domain.ConversationTypeDirect,
		DirectPairKey: "a:b",
	}

	mock.ExpectQuery(`INSERT INTO conversations`).
		WithArgs(pgxmock.AnyArg(), conv.Type, conv.Title, conv.DirectPairKey, now).
		WillReturnRows(pgxmock.NewRows([]string{"id", "type", "title", "direct_pair_key", "created_at"}).
			AddRow(uuid.New(), conv.Type, conv.Title, conv.DirectPairKey, now))

	created, err := repo.Create(context.Background(), conv)
	require.NoError(t, err)
	require.Equal(t, conv.Type, created.Type)
	require.WithinDuration(t, now, created.CreatedAt, time.Second)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestConversationGetByID(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	repo := NewConversationRepository(mock, nil)
	id := uuid.New()
	now := time.Now()

	mock.ExpectQuery(`SELECT id, type, title, direct_pair_key, created_at FROM conversations WHERE id=`).
		WithArgs(id).
		WillReturnRows(pgxmock.NewRows([]string{"id", "type", "title", "direct_pair_key", "created_at"}).
			AddRow(id, domain.ConversationTypeGroup, "Team", "", now))

	conv, err := repo.GetByID(context.Background(), id)
	require.NoError(t, err)
	require.Equal(t, id, conv.ID)
	require.Equal(t, domain.ConversationTypeGroup, conv.Type)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestConversationFindByDirectPairKey(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	repo := NewConversationRepository(mock, nil)
	id := uuid.New()
	now := time.Now()

	mock.ExpectQuery(`SELECT id, type, title, direct_pair_key, created_at FROM conversations WHERE direct_pair_key=`).
		WithArgs("a:b").
		WillReturnRows(pgxmock.NewRows([]string{"id", "type", "title", "direct_pair_key", "created_at"}).
			AddRow(id, domain.ConversationTypeDirect, "", "a:b", now))

	conv, err := repo.FindByDirectPairKey(context.Background(), "a:b")
	require.NoError(t, err)
	require.Equal(t, id, conv.ID)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestConversationFindByDirectPairKey_NotFound(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	repo := NewConversationRepository(mock, nil)

	mock.ExpectQuery(`SELECT id, type, title, direct_pair_key, created_at FROM conversations WHERE direct_pair_key=`).
		WithArgs("missing").
		WillReturnError(pgx.ErrNoRows)

	_, err = repo.FindByDirectPairKey(context.Background(), "missing")
	require.ErrorIs(t, err, domain.ErrNotFound)
	require.NoError(t, mock.ExpectationsWereMet())
}
