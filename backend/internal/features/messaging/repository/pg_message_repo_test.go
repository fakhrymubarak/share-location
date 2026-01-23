package repository

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/fakhry/share-location/backend/internal/features/messaging/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/pashagolub/pgxmock/v4"
	"github.com/stretchr/testify/require"
)

func TestMessageInsertOrGet_Insert(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	now := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	repo := NewPgMessageRepository(mock, func() time.Time { return now })

	msg := domain.Message{
		ConversationID: uuid.New(),
		SenderID:       uuid.New(),
		Body:           map[string]any{"text": "hi"},
		MessageType:    "text",
		ClientMsgID:    "c1",
	}
	bodyBytes, _ := json.Marshal(msg.Body)

	mock.ExpectQuery(`INSERT INTO messages`).
		WithArgs(pgxmock.AnyArg(), msg.ConversationID, msg.SenderID, bodyBytes, msg.MessageType, msg.ClientMsgID, now).
		WillReturnRows(pgxmock.NewRows([]string{"id", "conversation_id", "sender_id", "body", "message_type", "client_msg_id", "created_at"}).
			AddRow(uuid.New(), msg.ConversationID, msg.SenderID, bodyBytes, msg.MessageType, msg.ClientMsgID, now))

	stored, created, err := repo.InsertOrGet(context.Background(), msg)
	require.NoError(t, err)
	require.True(t, created)
	require.Equal(t, msg.ConversationID, stored.ConversationID)
	require.Equal(t, msg.ClientMsgID, stored.ClientMsgID)
	require.Equal(t, msg.Body["text"], stored.Body["text"])
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestMessageInsertOrGet_OnConflictFetchExisting(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	now := time.Now()
	repo := NewPgMessageRepository(mock, func() time.Time { return now })

	convID := uuid.New()
	senderID := uuid.New()
	bodyBytes, _ := json.Marshal(map[string]any{"text": "hello"})
	existingID := uuid.New()

	mock.ExpectQuery(`INSERT INTO messages`).
		WithArgs(pgxmock.AnyArg(), convID, senderID, bodyBytes, "text", "c1", now).
		WillReturnError(pgx.ErrNoRows)

	mock.ExpectQuery(`SELECT id, conversation_id, sender_id, body, message_type, client_msg_id, created_at FROM messages`).
		WithArgs(convID, senderID, "c1").
		WillReturnRows(pgxmock.NewRows([]string{"id", "conversation_id", "sender_id", "body", "message_type", "client_msg_id", "created_at"}).
			AddRow(existingID, convID, senderID, bodyBytes, "text", "c1", now))

	stored, created, err := repo.InsertOrGet(context.Background(), domain.Message{
		ConversationID: convID,
		SenderID:       senderID,
		Body:           map[string]any{"text": "hello"},
		MessageType:    "text",
		ClientMsgID:    "c1",
		CreatedAt:      now,
	})
	require.NoError(t, err)
	require.False(t, created)
	require.Equal(t, existingID, stored.ID)
	require.Equal(t, "hello", stored.Body["text"])
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestMessageListHistory(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	repo := NewPgMessageRepository(mock, nil)
	convID := uuid.New()
	bodyBytes, _ := json.Marshal(map[string]any{"text": "a"})
	now := time.Now()

	mock.ExpectQuery(`SELECT id, conversation_id, sender_id, body, message_type, client_msg_id, created_at FROM messages WHERE conversation_id=`).
		WithArgs(convID, 2).
		WillReturnRows(pgxmock.NewRows([]string{"id", "conversation_id", "sender_id", "body", "message_type", "client_msg_id", "created_at"}).
			AddRow(uuid.New(), convID, uuid.New(), bodyBytes, "text", "c1", now).
			AddRow(uuid.New(), convID, uuid.New(), bodyBytes, "text", "c2", now))

	msgs, err := repo.ListHistory(context.Background(), convID, 2)
	require.NoError(t, err)
	require.Len(t, msgs, 2)
	require.Equal(t, "c2", msgs[1].ClientMsgID)
	require.NoError(t, mock.ExpectationsWereMet())
}
