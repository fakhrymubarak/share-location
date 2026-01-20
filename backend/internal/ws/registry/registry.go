package registry

import (
	"sync"
	"time"

	"github.com/google/uuid"
)

// Registry tracks active WebSocket connections per user with heartbeat-based expiry.
type Registry struct {
	mu    sync.Mutex
	conns map[uuid.UUID]map[string]time.Time
	now   func() time.Time
	ttl   time.Duration
}

// New returns a Registry with the given heartbeat TTL.
func New(ttl time.Duration, now func() time.Time) *Registry {
	if now == nil {
		now = time.Now
	}
	if ttl <= 0 {
		ttl = time.Minute
	}
	return &Registry{
		conns: make(map[uuid.UUID]map[string]time.Time),
		now:   now,
		ttl:   ttl,
	}
}

// Add registers a connection id for a user. Returns true if new, false if existed.
func (r *Registry) Add(userID uuid.UUID, connID string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	now := r.now()
	userConns, ok := r.conns[userID]
	if !ok {
		userConns = make(map[string]time.Time)
		r.conns[userID] = userConns
	}
	_, existed := userConns[connID]
	userConns[connID] = now
	return !existed
}

// Heartbeat updates last-seen for a connection if present.
func (r *Registry) Heartbeat(userID uuid.UUID, connID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if userConns, ok := r.conns[userID]; ok {
		if _, ok := userConns[connID]; ok {
			userConns[connID] = r.now()
		}
	}
}

// Remove deletes a connection for a user.
func (r *Registry) Remove(userID uuid.UUID, connID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if userConns, ok := r.conns[userID]; ok {
		delete(userConns, connID)
		if len(userConns) == 0 {
			delete(r.conns, userID)
		}
	}
}

// Connections returns active connection IDs for a user (pruned view).
func (r *Registry) Connections(userID uuid.UUID) []string {
	r.PruneExpired()
	r.mu.Lock()
	defer r.mu.Unlock()
	userConns, ok := r.conns[userID]
	if !ok {
		return nil
	}
	out := make([]string, 0, len(userConns))
	for id := range userConns {
		out = append(out, id)
	}
	return out
}

// PruneExpired removes stale connections beyond TTL.
func (r *Registry) PruneExpired() {
	r.mu.Lock()
	defer r.mu.Unlock()
	now := r.now()
	for userID, userConns := range r.conns {
		for connID, ts := range userConns {
			if now.Sub(ts) > r.ttl {
				delete(userConns, connID)
			}
		}
		if len(userConns) == 0 {
			delete(r.conns, userID)
		}
	}
}
