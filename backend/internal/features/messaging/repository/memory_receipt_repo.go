package repository

import (
	"context"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/fakhry/share-location/backend/internal/features/messaging/domain"
)

type MemoryReceiptRepository struct {
	mu       sync.Mutex
	receipts map[uuid.UUID]map[uuid.UUID]domain.Receipt // messageID -> userID -> receipt
	now      func() time.Time
}

func NewReceiptRepository(now func() time.Time) *MemoryReceiptRepository {
	if now == nil {
		now = time.Now
	}
	return &MemoryReceiptRepository{
		receipts: make(map[uuid.UUID]map[uuid.UUID]domain.Receipt),
		now:      now,
	}
}

func (r *MemoryReceiptRepository) Upsert(ctx context.Context, receipt domain.Receipt) error {
	_ = ctx
	r.mu.Lock()
	defer r.mu.Unlock()

	if receipt.UpdatedAt.IsZero() {
		receipt.UpdatedAt = r.now()
	}

	msgMap, ok := r.receipts[receipt.MessageID]
	if !ok {
		msgMap = make(map[uuid.UUID]domain.Receipt)
		r.receipts[receipt.MessageID] = msgMap
	}
	msgMap[receipt.UserID] = receipt
	return nil
}

func (r *MemoryReceiptRepository) ListByMessage(ctx context.Context, messageID uuid.UUID) ([]domain.Receipt, error) {
	_ = ctx
	r.mu.Lock()
	defer r.mu.Unlock()

	msgMap, ok := r.receipts[messageID]
	if !ok {
		return nil, nil
	}
	out := make([]domain.Receipt, 0, len(msgMap))
	for _, rc := range msgMap {
		out = append(out, rc)
	}
	return out, nil
}
