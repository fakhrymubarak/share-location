package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/fakhry/share-location/backend/internal/features/messaging/domain"
)

type PgxPoolReceipt interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// PgReceiptRepository implements domain.ReceiptRepository using Postgres.
type PgReceiptRepository struct {
	DB PgxPoolReceipt
}

func NewPgReceiptRepository(db PgxPoolReceipt) *PgReceiptRepository {
	return &PgReceiptRepository{DB: db}
}

func (r *PgReceiptRepository) Upsert(ctx context.Context, receipt domain.Receipt) error {
	const query = `
INSERT INTO delivery_receipts (message_id, user_id, status, updated_at)
VALUES ($1, $2, $3, $4)
ON CONFLICT (message_id, user_id)
DO UPDATE SET status=EXCLUDED.status, updated_at=EXCLUDED.updated_at`
	_, err := r.DB.Exec(ctx, query, receipt.MessageID, receipt.UserID, receipt.Status, receipt.UpdatedAt)
	return err
}

func (r *PgReceiptRepository) ListByMessage(ctx context.Context, messageID uuid.UUID) ([]domain.Receipt, error) {
	const query = `SELECT message_id, user_id, status, updated_at FROM delivery_receipts WHERE message_id=$1`
	rows, err := r.DB.Query(ctx, query, messageID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var receipts []domain.Receipt
	for rows.Next() {
		var rc domain.Receipt
		if err := rows.Scan(&rc.MessageID, &rc.UserID, &rc.Status, &rc.UpdatedAt); err != nil {
			return nil, err
		}
		receipts = append(receipts, rc)
	}
	return receipts, rows.Err()
}
