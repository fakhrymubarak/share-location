package memory

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/fakhry/share-location/backend/internal/domain"
)

func TestReceiptRepository_UpsertAndList(t *testing.T) {
	now := time.Date(2026, 1, 8, 8, 0, 0, 0, time.UTC)
	r := NewReceiptRepository(func() time.Time { return now })

	msgID := uuid.New()
	userA := uuid.New()
	userB := uuid.New()

	err := r.Upsert(context.Background(), domain.DeliveryReceipt{
		MessageID: msgID,
		UserID:    userA,
		Status:    "delivered",
	})
	if err != nil {
		t.Fatalf("Upsert error: %v", err)
	}
	err = r.Upsert(context.Background(), domain.DeliveryReceipt{
		MessageID: msgID,
		UserID:    userB,
		Status:    "read",
	})
	if err != nil {
		t.Fatalf("Upsert error: %v", err)
	}

	list, err := r.ListByMessage(context.Background(), msgID)
	if err != nil {
		t.Fatalf("ListByMessage error: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("expected 2 receipts, got %d", len(list))
	}
}
