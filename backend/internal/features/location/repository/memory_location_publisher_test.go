package repository

import (
	"context"
	"testing"

	"github.com/fakhry/share-location/backend/internal/features/location/domain"
	"github.com/google/uuid"
)

func TestMemoryLocationPublisher_Publish(t *testing.T) {
	p := NewMemoryLocationPublisher()
	point := domain.LiveLocationPoint{StreamID: uuid.New(), Lat: 1, Lng: 2}
	if err := p.PublishLiveLocation(context.Background(), point); err != nil {
		t.Fatalf("PublishLiveLocation error: %v", err)
	}
	published := p.Published()
	if len(published) != 1 || published[0].Lat != 1 {
		t.Fatalf("expected one published point")
	}
}
