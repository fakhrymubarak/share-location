package memory

import (
	"context"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/fakhry/share-location/backend/internal/domain"
)

type ReceiptRepository struct {
	mu        sync.Mutex
	receipts  map[uuid.UUID]map[uuid.UUID]domain.DeliveryReceipt // messageID -> userID -> receipt
	now       func() time.Time
}

// NewReceiptRepository returns an in-memory receipt repository.
func NewReceiptRepository(now func() time.Time) *ReceiptRepository {
	if now == nil {
		now = time.Now
	}
	return &ReceiptRepository{
		receipts: make(map[uuid.UUID]map[uuid.UUID]domain.DeliveryReceipt),
		now:      now,
	}
}

func (r *ReceiptRepository) Upsert(ctx context.Context, receipt domain.DeliveryReceipt) error {
	_ = ctx
	r.mu.Lock()
	defer r.mu.Unlock()

	if receipt.UpdatedAt.IsZero() {
		receipt.UpdatedAt = r.now()
	}
	userMap, ok := r.receipts[receipt.MessageID]
	if !ok {
		userMap = make(map[uuid.UUID]domain.DeliveryReceipt)
		r.receipts[receipt.MessageID] = userMap
	}
	userMap[receipt.UserID] = receipt
	return nil
}

func (r *ReceiptRepository) ListByMessage(ctx context.Context, messageID uuid.UUID) ([]domain.DeliveryReceipt, error) {
	_ = ctx
	r.mu.Lock()
	defer r.mu.Unlock()
	userMap, ok := r.receipts[messageID]
	if !ok {
		return nil, nil
	}
	out := make([]domain.DeliveryReceipt, 0, len(userMap))
	for _, rc := range userMap {
		out = append(out, rc)
	}
	return out, nil
}
