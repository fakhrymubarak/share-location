package pg

import (
	"context"
	"testing"
	"time"

	"github.com/fakhry/share-location/backend/internal/domain"
	"github.com/google/uuid"
	"github.com/pashagolub/pgxmock/v4"
	"github.com/stretchr/testify/require"
)

func TestReceiptUpsert(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	repo := NewReceiptRepository(mock)
	rc := domain.DeliveryReceipt{
		MessageID: uuid.New(),
		UserID:    uuid.New(),
		Status:    "read",
		UpdatedAt: time.Now(),
	}

	mock.ExpectExec(`INSERT INTO delivery_receipts`).
		WithArgs(rc.MessageID, rc.UserID, rc.Status, rc.UpdatedAt).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))

	require.NoError(t, repo.Upsert(context.Background(), rc))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestReceiptListByMessage(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	repo := NewReceiptRepository(mock)
	msgID := uuid.New()
	now := time.Now()

	mock.ExpectQuery(`SELECT message_id, user_id, status, updated_at FROM delivery_receipts WHERE message_id=`).
		WithArgs(msgID).
		WillReturnRows(pgxmock.NewRows([]string{"message_id", "user_id", "status", "updated_at"}).
			AddRow(msgID, uuid.New(), "delivered", now).
			AddRow(msgID, uuid.New(), "read", now))

	receipts, err := repo.ListByMessage(context.Background(), msgID)
	require.NoError(t, err)
	require.Len(t, receipts, 2)
	require.Equal(t, "read", receipts[1].Status)
	require.NoError(t, mock.ExpectationsWereMet())
}
