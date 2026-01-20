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

func TestMembershipAdd(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	now := time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)
	repo := NewMembershipRepository(mock, func() time.Time { return now })

	convID := uuid.New()
	userA := uuid.New()
	userB := uuid.New()
	members := []domain.ConversationMember{
		{ConversationID: convID, UserID: userA, Role: "member"},
		{ConversationID: convID, UserID: userB, Role: "member"},
	}

	mock.ExpectExec(`INSERT INTO conversation_members`).
		WithArgs(convID, userA, "member", now, (*uuid.UUID)(nil), convID, userB, "member", now, (*uuid.UUID)(nil)).
		WillReturnResult(pgxmock.NewResult("INSERT", 2))

	require.NoError(t, repo.Add(context.Background(), members))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestMembershipIsMember(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	repo := NewMembershipRepository(mock, nil)
	convID := uuid.New()
	userID := uuid.New()

	mock.ExpectQuery(`SELECT 1 FROM conversation_members WHERE conversation_id=`).
		WithArgs(convID, userID).
		WillReturnRows(pgxmock.NewRows([]string{"1"}).AddRow(1))

	ok, err := repo.IsMember(context.Background(), convID, userID)
	require.NoError(t, err)
	require.True(t, ok)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestMembershipIsMemberFalse(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	repo := NewMembershipRepository(mock, nil)
	convID := uuid.New()
	userID := uuid.New()

	mock.ExpectQuery(`SELECT 1 FROM conversation_members WHERE conversation_id=`).
		WithArgs(convID, userID).
		WillReturnError(pgx.ErrNoRows)

	ok, err := repo.IsMember(context.Background(), convID, userID)
	require.NoError(t, err)
	require.False(t, ok)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestMembershipListByConversation(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	repo := NewMembershipRepository(mock, nil)
	convID := uuid.New()
	userA := uuid.New()
	userB := uuid.New()
	now := time.Now()

	mock.ExpectQuery(`SELECT conversation_id, user_id, role, joined_at, last_read_message_id FROM conversation_members WHERE conversation_id=`).
		WithArgs(convID).
		WillReturnRows(pgxmock.NewRows([]string{"conversation_id", "user_id", "role", "joined_at", "last_read_message_id"}).
			AddRow(convID, userA, "member", now, nil).
			AddRow(convID, userB, "admin", now, userA.String()))

	members, err := repo.ListByConversation(context.Background(), convID)
	require.NoError(t, err)
	require.Len(t, members, 2)
	require.Equal(t, userA, members[0].UserID)
	require.Equal(t, "member", members[0].Role)
	require.Equal(t, "admin", members[1].Role)
	require.NotNil(t, members[1].LastReadMessageID)
	require.Equal(t, userA, *members[1].LastReadMessageID)
	require.NoError(t, mock.ExpectationsWereMet())
}
