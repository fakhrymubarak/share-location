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

type stubPointStore struct {
	saved domain.LiveLocationPoint
}

func (s *stubPointStore) SavePoint(ctx context.Context, point domain.LiveLocationPoint) error {
	s.saved = point
	return nil
}

func (s *stubPointStore) GetPoint(ctx context.Context, streamID uuid.UUID) (domain.LiveLocationPoint, error) {
	if s.saved.StreamID == streamID {
		return s.saved, nil
	}
	return domain.LiveLocationPoint{}, domain.ErrNotFound
}

func TestLiveLocationUpsertSession(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	now := time.Now()
	repo := NewLiveLocationRepository(mock, nil, func() time.Time { return now })
	session := domain.LiveLocationSession{
		UserID:    uuid.New(),
		ExpiresAt: now.Add(10 * time.Minute),
	}

	mock.ExpectQuery(`INSERT INTO live_location_sessions`).
		WithArgs(pgxmock.AnyArg(), session.UserID, session.ConversationID, session.ExpiresAt).
		WillReturnRows(pgxmock.NewRows([]string{"id", "user_id", "conversation_id", "expires_at"}).
			AddRow(uuid.New(), session.UserID, session.ConversationID, session.ExpiresAt))

	saved, err := repo.UpsertSession(context.Background(), session)
	require.NoError(t, err)
	require.Equal(t, session.UserID, saved.UserID)
	require.NotEqual(t, uuid.Nil, saved.ID)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestLiveLocationGetSession(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	repo := NewLiveLocationRepository(mock, nil, nil)
	streamID := uuid.New()
	now := time.Now().Add(5 * time.Minute)

	mock.ExpectQuery(`SELECT id, user_id, conversation_id, expires_at FROM live_location_sessions WHERE id=`).
		WithArgs(streamID).
		WillReturnRows(pgxmock.NewRows([]string{"id", "user_id", "conversation_id", "expires_at"}).
			AddRow(streamID, uuid.New(), uuid.Nil, now))

	session, err := repo.GetSession(context.Background(), streamID)
	require.NoError(t, err)
	require.Equal(t, streamID, session.ID)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestLiveLocationGetSessionNotFound(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	repo := NewLiveLocationRepository(mock, nil, nil)
	streamID := uuid.New()

	mock.ExpectQuery(`SELECT id, user_id, conversation_id, expires_at FROM live_location_sessions WHERE id=`).
		WithArgs(streamID).
		WillReturnError(pgx.ErrNoRows)

	_, err = repo.GetSession(context.Background(), streamID)
	require.ErrorIs(t, err, domain.ErrNotFound)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestLiveLocationPointDelegation(t *testing.T) {
	store := &stubPointStore{}
	repo := NewLiveLocationRepository(nil, store, nil)

	point := domain.LiveLocationPoint{
		StreamID: uuid.New(),
		UserID:   uuid.New(),
	}
	require.NoError(t, repo.SavePoint(context.Background(), point))

	got, err := repo.GetPoint(context.Background(), point.StreamID)
	require.NoError(t, err)
	require.Equal(t, point.StreamID, got.StreamID)
}
