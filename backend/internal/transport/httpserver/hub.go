package httpserver

import (
	"context"
	"sync"
	"time"

	"github.com/gofiber/contrib/websocket"
	"github.com/google/uuid"

	"github.com/fakhry/share-location/backend/internal/service"
	"github.com/fakhry/share-location/backend/internal/ws/registry"
)

// ConnectionHub tracks active WebSocket connections and dispatches events to members.
type ConnectionHub struct {
	reg     *registry.Registry
	mu      sync.Mutex
	conns   map[string]*websocket.Conn
	owners  map[string]uuid.UUID
	expires time.Duration
}

// NewConnectionHub builds a ConnectionHub with the provided registry.
func NewConnectionHub(reg *registry.Registry) *ConnectionHub {
	if reg == nil {
		reg = registry.New(2*time.Minute, nil)
	}
	return &ConnectionHub{
		reg:    reg,
		conns:  make(map[string]*websocket.Conn),
		owners: make(map[string]uuid.UUID),
	}
}

func (h *ConnectionHub) add(userID uuid.UUID, conn *websocket.Conn) string {
	if h == nil {
		return ""
	}
	connID := uuid.NewString()
	h.mu.Lock()
	h.conns[connID] = conn
	h.owners[connID] = userID
	h.mu.Unlock()
	h.reg.Add(userID, connID)
	return connID
}

func (h *ConnectionHub) heartbeat(userID uuid.UUID, connID string) {
	if h == nil {
		return
	}
	h.reg.Heartbeat(userID, connID)
}

func (h *ConnectionHub) remove(userID uuid.UUID, connID string) {
	if h == nil {
		return
	}
	h.mu.Lock()
	delete(h.conns, connID)
	delete(h.owners, connID)
	h.mu.Unlock()
	h.reg.Remove(userID, connID)
}

// BroadcastToConversation delivers payload to all members' connections except the excluded one.
func (h *ConnectionHub) BroadcastToConversation(ctx context.Context, convSvc *service.ConversationService, convID uuid.UUID, excludeConnID string, payload []byte) {
	if h == nil || convSvc == nil || convID == uuid.Nil {
		return
	}
	members, err := convSvc.ListMembers(ctx, convID)
	if err != nil {
		return
	}
	for _, m := range members {
		h.sendToUser(m.UserID, excludeConnID, payload)
	}
}

func (h *ConnectionHub) sendToUser(userID uuid.UUID, excludeConnID string, payload []byte) {
	ids := h.reg.Connections(userID)
	for _, id := range ids {
		if id == excludeConnID {
			continue
		}
		h.mu.Lock()
		conn := h.conns[id]
		h.mu.Unlock()
		if conn != nil {
			_ = conn.WriteMessage(websocket.TextMessage, payload)
		}
	}
}
