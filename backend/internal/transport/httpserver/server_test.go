package httpserver

import (
	"bytes"
	"encoding/json"
	"net"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	"github.com/fakhry/share-location/backend/internal/repo/memory"
	"github.com/fakhry/share-location/backend/internal/service"
)

func TestLoginAndConversationEndpoints(t *testing.T) {
	now := time.Date(2026, 1, 7, 9, 0, 0, 0, time.UTC)
	usersRepo := memory.NewUserRepository(func() time.Time { return now })
	convRepo := memory.NewConversationRepository(func() time.Time { return now })
	memberRepo := memory.NewMembershipRepository(func() time.Time { return now })

	app := New(Deps{
		Users:         service.NewUserService(usersRepo),
		Conversations: service.NewConversationService(usersRepo, convRepo, memberRepo, func() time.Time { return now }),
	})

	loginBody, _ := json.Marshal(map[string]string{"username": "alice"})
	req := httptest.NewRequest("POST", "/v1/login", bytes.NewReader(loginBody))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("login request failed: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("login status = %d", resp.StatusCode)
	}
	var loginResp map[string]string
	_ = json.NewDecoder(resp.Body).Decode(&loginResp)
	if loginResp["username"] != "alice" || loginResp["user_id"] == "" {
		t.Fatalf("unexpected login response %+v", loginResp)
	}

	convBody, _ := json.Marshal(map[string]any{
		"type":      "direct",
		"usernames": []string{"alice", "bob"},
	})
	req = httptest.NewRequest("POST", "/v1/conversations", bytes.NewReader(convBody))
	req.Header.Set("Content-Type", "application/json")
	resp, err = app.Test(req)
	if err != nil {
		t.Fatalf("conversation request failed: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("conversation status = %d", resp.StatusCode)
	}
	var convResp map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&convResp)
	if convResp["id"] == "" || convResp["type"] != "direct" {
		t.Fatalf("unexpected conversation response %+v", convResp)
	}
}

func TestHandlers_ReturnValidationErrors(t *testing.T) {
	now := time.Date(2026, 1, 7, 9, 30, 0, 0, time.UTC)
	usersRepo := memory.NewUserRepository(func() time.Time { return now })
	convRepo := memory.NewConversationRepository(func() time.Time { return now })
	memberRepo := memory.NewMembershipRepository(func() time.Time { return now })

	app := New(Deps{
		Users:         service.NewUserService(usersRepo),
		Conversations: service.NewConversationService(usersRepo, convRepo, memberRepo, func() time.Time { return now }),
	})

	// Blank username should trigger validation.
	loginBody, _ := json.Marshal(map[string]string{"username": " "})
	req := httptest.NewRequest("POST", "/v1/login", bytes.NewReader(loginBody))
	req.Header.Set("Content-Type", "application/json")
	resp, _ := app.Test(req)
	if resp.StatusCode != 400 {
		t.Fatalf("login status = %d, want 400", resp.StatusCode)
	}

	// Invalid conversation type triggers error handler.
	convBody, _ := json.Marshal(map[string]any{
		"type":      "unknown",
		"usernames": []string{"alice", "bob"},
	})
	req = httptest.NewRequest("POST", "/v1/conversations", bytes.NewReader(convBody))
	req.Header.Set("Content-Type", "application/json")
	resp, _ = app.Test(req)
	if resp.StatusCode != 400 {
		t.Fatalf("conversation status = %d, want 400", resp.StatusCode)
	}
}

func TestWebSocketHandshake(t *testing.T) {
	now := time.Date(2026, 1, 7, 10, 0, 0, 0, time.UTC)
	usersRepo := memory.NewUserRepository(func() time.Time { return now })
	convRepo := memory.NewConversationRepository(func() time.Time { return now })
	memberRepo := memory.NewMembershipRepository(func() time.Time { return now })

	app := New(Deps{
		Users:         service.NewUserService(usersRepo),
		Conversations: service.NewConversationService(usersRepo, convRepo, memberRepo, func() time.Time { return now }),
	})

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen error: %v", err)
	}
	defer ln.Close()
	go app.Listener(ln)

	wsURL := "ws://" + ln.Addr().String() + "/ws?username=carol"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("websocket dial error: %v", err)
	}
	defer conn.Close()

	_, data, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("read message error: %v", err)
	}
	var payload struct {
		Event string `json:"event"`
		Data  struct {
			UserID   string `json:"user_id"`
			Username string `json:"username"`
		} `json:"data"`
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if payload.Event != "CONNECTED" || payload.Data.Username != "carol" || payload.Data.UserID == "" {
		t.Fatalf("unexpected payload %+v", payload)
	}
}

func TestWebSocketHandshakeInvalidUsername(t *testing.T) {
	now := time.Date(2026, 1, 7, 10, 30, 0, 0, time.UTC)
	usersRepo := memory.NewUserRepository(func() time.Time { return now })
	convRepo := memory.NewConversationRepository(func() time.Time { return now })
	memberRepo := memory.NewMembershipRepository(func() time.Time { return now })

	app := New(Deps{
		Users:         service.NewUserService(usersRepo),
		Conversations: service.NewConversationService(usersRepo, convRepo, memberRepo, func() time.Time { return now }),
	})

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen error: %v", err)
	}
	defer ln.Close()
	go app.Listener(ln)

	wsURL := "ws://" + ln.Addr().String() + "/ws"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("websocket dial error: %v", err)
	}
	defer conn.Close()

	_, data, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("read message error: %v", err)
	}
	if string(data) == "" {
		t.Fatalf("expected error payload")
	}
}
