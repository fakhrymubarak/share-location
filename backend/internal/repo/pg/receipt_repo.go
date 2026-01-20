package pg

import (
	"context"

	"github.com/google/uuid"

	"github.com/fakhry/share-location/backend/internal/domain"
)

// ReceiptRepository implements repo.ReceiptRepository using Postgres.
type ReceiptRepository struct {
	DB PgxPool
}

func NewReceiptRepository(db PgxPool) *ReceiptRepository {
	return &ReceiptRepository{DB: db}
}

func (r *ReceiptRepository) Upsert(ctx context.Context, receipt domain.DeliveryReceipt) error {
	const query = `
INSERT INTO delivery_receipts (message_id, user_id, status, updated_at)
VALUES ($1, $2, $3, $4)
ON CONFLICT (message_id, user_id)
DO UPDATE SET status=EXCLUDED.status, updated_at=EXCLUDED.updated_at`
	_, err := r.DB.Exec(ctx, query, receipt.MessageID, receipt.UserID, receipt.Status, receipt.UpdatedAt)
	return err
}

func (r *ReceiptRepository) ListByMessage(ctx context.Context, messageID uuid.UUID) ([]domain.DeliveryReceipt, error) {
	const query = `SELECT message_id, user_id, status, updated_at FROM delivery_receipts WHERE message_id=$1`
	rows, err := r.DB.Query(ctx, query, messageID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var receipts []domain.DeliveryReceipt
	for rows.Next() {
		var rc domain.DeliveryReceipt
		if err := rows.Scan(&rc.MessageID, &rc.UserID, &rc.Status, &rc.UpdatedAt); err != nil {
			return nil, err
		}
		receipts = append(receipts, rc)
	}
	return receipts, rows.Err()
}

// ensure interface compliance at compile time
var _ interface {
	Upsert(context.Context, domain.DeliveryReceipt) error
	ListByMessage(context.Context, uuid.UUID) ([]domain.DeliveryReceipt, error)
} = (*ReceiptRepository)(nil)
