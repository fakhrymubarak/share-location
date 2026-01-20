package httpserver

import (
	"bytes"
	"context"
	"encoding/json"
	"net"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"

	"github.com/fakhry/share-location/backend/internal/domain"
	"github.com/fakhry/share-location/backend/internal/repo/memory"
	"github.com/fakhry/share-location/backend/internal/service"
)

func TestLoginAndConversationEndpoints(t *testing.T) {
	now := time.Date(2026, 1, 7, 9, 0, 0, 0, time.UTC)
	usersRepo := memory.NewUserRepository(func() time.Time { return now })
	convRepo := memory.NewConversationRepository(func() time.Time { return now })
	memberRepo := memory.NewMembershipRepository(func() time.Time { return now })
	msgRepo := memory.NewMessageRepository(func() time.Time { return now })
	rcRepo := memory.NewReceiptRepository(func() time.Time { return now })
	msgSvc := service.NewMessagingService(msgRepo, memberRepo, nil, rcRepo, func() time.Time { return now })
	convSvc := service.NewConversationService(usersRepo, convRepo, memberRepo, func() time.Time { return now })
	liveSvc := service.NewLiveLocationService(memory.NewLiveLocationRepository(func() time.Time { return now }), memberRepo, memory.NewLiveLocationPublisher(), 15*time.Minute, func() time.Time { return now })

	app := New(Deps{
		Users:         service.NewUserService(usersRepo),
		Conversations: convSvc,
		Messaging:     msgSvc,
		LiveLocation:  liveSvc,
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

	convID := convResp["id"].(string)
	alice, _ := usersRepo.GetByUsername(context.Background(), "alice")
	bob, _ := usersRepo.GetByUsername(context.Background(), "bob")

	// Seed messages via service.
	for _, cID := range []string{"c1", "c2"} {
		_, _, err := msgSvc.SendChat(context.Background(), service.SendChatRequest{
			ConversationID: uuid.MustParse(convID),
			SenderID:       alice.ID,
			Body:           map[string]any{"text": cID},
			MessageType:    "text",
			ClientMsgID:    cID,
		})
		if err != nil {
			t.Fatalf("SendChat seed error: %v", err)
		}
	}

	req = httptest.NewRequest("GET", "/v1/conversations/"+convID+"/messages?limit=10", nil)
	resp, err = app.Test(req)
	if err != nil {
		t.Fatalf("list messages request failed: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("list messages status = %d", resp.StatusCode)
	}
	var msgs []map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&msgs)
	if len(msgs) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(msgs))
	}
	firstMsgID, _ := msgs[0]["id"].(string)

	rcBody, _ := json.Marshal(map[string]string{
		"user_id": bob.ID.String(),
		"status":  "read",
	})
	req = httptest.NewRequest("POST", "/v1/messages/"+firstMsgID+"/receipt", bytes.NewReader(rcBody))
	req.Header.Set("Content-Type", "application/json")
	resp, err = app.Test(req)
	if err != nil {
		t.Fatalf("receipt request failed: %v", err)
	}
	if resp.StatusCode != 204 {
		t.Fatalf("receipt status = %d", resp.StatusCode)
	}

	llBody, _ := json.Marshal(map[string]any{
		"user_id":         alice.ID.String(),
		"conversation_id": convID,
		"ttl_seconds":     600,
	})
	req = httptest.NewRequest("POST", "/v1/live-locations", bytes.NewReader(llBody))
	req.Header.Set("Content-Type", "application/json")
	resp, err = app.Test(req)
	if err != nil {
		t.Fatalf("live location request failed: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("live location status = %d", resp.StatusCode)
	}
}

func TestHandlers_ReturnValidationErrors(t *testing.T) {
	now := time.Date(2026, 1, 7, 9, 30, 0, 0, time.UTC)
	usersRepo := memory.NewUserRepository(func() time.Time { return now })
	convRepo := memory.NewConversationRepository(func() time.Time { return now })
	memberRepo := memory.NewMembershipRepository(func() time.Time { return now })
	msgRepo := memory.NewMessageRepository(func() time.Time { return now })
	rcRepo := memory.NewReceiptRepository(func() time.Time { return now })
	liveSvc := service.NewLiveLocationService(memory.NewLiveLocationRepository(func() time.Time { return now }), memberRepo, memory.NewLiveLocationPublisher(), 15*time.Minute, func() time.Time { return now })

	app := New(Deps{
		Users:         service.NewUserService(usersRepo),
		Conversations: service.NewConversationService(usersRepo, convRepo, memberRepo, func() time.Time { return now }),
		Messaging:     service.NewMessagingService(msgRepo, memberRepo, nil, rcRepo, func() time.Time { return now }),
		LiveLocation:  liveSvc,
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
	msgRepo := memory.NewMessageRepository(func() time.Time { return now })
	rcRepo := memory.NewReceiptRepository(func() time.Time { return now })
	liveSvc := service.NewLiveLocationService(memory.NewLiveLocationRepository(func() time.Time { return now }), memberRepo, memory.NewLiveLocationPublisher(), 15*time.Minute, func() time.Time { return now })

	app := New(Deps{
		Users:         service.NewUserService(usersRepo),
		Conversations: service.NewConversationService(usersRepo, convRepo, memberRepo, func() time.Time { return now }),
		Messaging:     service.NewMessagingService(msgRepo, memberRepo, nil, rcRepo, func() time.Time { return now }),
		LiveLocation:  liveSvc,
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
	msgRepo := memory.NewMessageRepository(func() time.Time { return now })
	rcRepo := memory.NewReceiptRepository(func() time.Time { return now })
	liveSvc := service.NewLiveLocationService(memory.NewLiveLocationRepository(func() time.Time { return now }), memberRepo, memory.NewLiveLocationPublisher(), 15*time.Minute, func() time.Time { return now })

	app := New(Deps{
		Users:         service.NewUserService(usersRepo),
		Conversations: service.NewConversationService(usersRepo, convRepo, memberRepo, func() time.Time { return now }),
		Messaging:     service.NewMessagingService(msgRepo, memberRepo, nil, rcRepo, func() time.Time { return now }),
		LiveLocation:  liveSvc,
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

func TestWebSocketSendChat(t *testing.T) {
	now := time.Date(2026, 1, 7, 11, 0, 0, 0, time.UTC)
	usersRepo := memory.NewUserRepository(func() time.Time { return now })
	convRepo := memory.NewConversationRepository(func() time.Time { return now })
	memberRepo := memory.NewMembershipRepository(func() time.Time { return now })
	msgRepo := memory.NewMessageRepository(func() time.Time { return now })
	rcRepo := memory.NewReceiptRepository(func() time.Time { return now })
	liveSvc := service.NewLiveLocationService(memory.NewLiveLocationRepository(func() time.Time { return now }), memberRepo, memory.NewLiveLocationPublisher(), 15*time.Minute, func() time.Time { return now })

	convSvc := service.NewConversationService(usersRepo, convRepo, memberRepo, func() time.Time { return now })
	msgSvc := service.NewMessagingService(msgRepo, memberRepo, nil, rcRepo, func() time.Time { return now })
	app := New(Deps{
		Users:         service.NewUserService(usersRepo),
		Conversations: convSvc,
		Messaging:     msgSvc,
		LiveLocation:  liveSvc,
	})

	// Seed conversation and membership.
	conv, _, err := convSvc.CreateConversation(context.Background(), service.CreateConversationRequest{
		Type:                 domain.ConversationTypeDirect,
		ParticipantUsernames: []string{"alice", "bob"},
	})
	if err != nil {
		t.Fatalf("CreateConversation seed error: %v", err)
	}

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen error: %v", err)
	}
	defer ln.Close()
	go app.Listener(ln)

	wsURL := "ws://" + ln.Addr().String() + "/ws?username=alice"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("websocket dial error: %v", err)
	}
	defer conn.Close()

	_, _, _ = conn.ReadMessage() // consume CONNECTED

	req := map[string]any{
		"event":  "SEND_CHAT",
		"req_id": "r1",
		"data": map[string]any{
			"conversation_id": conv.ID.String(),
			"body":            map[string]any{"text": "hello"},
			"message_type":    "text",
			"client_msg_id":   "c1",
		},
	}
	reqBytes, _ := json.Marshal(req)
	if err := conn.WriteMessage(websocket.TextMessage, reqBytes); err != nil {
		t.Fatalf("write message error: %v", err)
	}

	_, data, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("read message error: %v", err)
	}
	var resp map[string]any
	if err := json.Unmarshal(data, &resp); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if resp["event"] != "RECEIVE_CHAT" || resp["req_id"] != "r1" {
		t.Fatalf("unexpected response %+v", resp)
	}
}
