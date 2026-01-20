package registry

import (
	"slices"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestAddAndConnections(t *testing.T) {
	now := time.Now()
	r := New(time.Minute, func() time.Time { return now })

	userID := uuid.New()
	connID := "conn-1"

	isNew := r.Add(userID, connID)
	if !isNew {
		t.Fatal("expected first add to be new")
	}

	again := r.Add(userID, connID)
	if again {
		t.Fatal("expected duplicate add to return false")
	}

	conns := r.Connections(userID)
	if !slices.Equal(conns, []string{connID}) {
		t.Fatalf("unexpected connections: %v", conns)
	}
}

func TestHeartbeatExtendsTTL(t *testing.T) {
	base := time.Now()
	current := base
	r := New(2*time.Minute, func() time.Time { return current })

	userID := uuid.New()
	connID := "conn-1"
	r.Add(userID, connID)

	// advance close to TTL and heartbeat
	current = base.Add(90 * time.Second)
	r.Heartbeat(userID, connID)

	// advance past original TTL but within extended TTL
	current = base.Add(3 * time.Minute)
	conns := r.Connections(userID)
	if !slices.Equal(conns, []string{connID}) {
		t.Fatalf("expected connection to remain after heartbeat, got %v", conns)
	}
}

func TestPruneExpired(t *testing.T) {
	now := time.Now()
	nowPtr := &now
	r := New(time.Minute, func() time.Time { return *nowPtr })

	userID := uuid.New()
	connID := "conn-1"
	r.Add(userID, connID)

	*nowPtr = now.Add(2 * time.Minute)
	r.PruneExpired()

	if len(r.Connections(userID)) != 0 {
		t.Fatalf("expected connections pruned, got %v", r.Connections(userID))
	}
}

func TestRemove(t *testing.T) {
	now := time.Now()
	r := New(time.Minute, func() time.Time { return now })

	userID := uuid.New()
	connA := "conn-a"
	connB := "conn-b"

	r.Add(userID, connA)
	r.Add(userID, connB)

	r.Remove(userID, connA)
	conns := r.Connections(userID)
	if !slices.Equal(conns, []string{connB}) {
		t.Fatalf("expected remaining conn %v, got %v", connB, conns)
	}

	r.Remove(userID, connB)
	if len(r.Connections(userID)) != 0 {
		t.Fatalf("expected empty connections after remove, got %v", r.Connections(userID))
	}
}
