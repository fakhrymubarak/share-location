package httpserver

import (
	"bytes"
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"errors"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"

	"github.com/fakhry/share-location/backend/internal/auth"
	convdomain "github.com/fakhry/share-location/backend/internal/features/conversations/domain"
	convrepo "github.com/fakhry/share-location/backend/internal/features/conversations/repository"
	convservice "github.com/fakhry/share-location/backend/internal/features/conversations/service"
	locrepo "github.com/fakhry/share-location/backend/internal/features/location/repository"
	locservice "github.com/fakhry/share-location/backend/internal/features/location/service"
	msgrepo "github.com/fakhry/share-location/backend/internal/features/messaging/repository"
	msgservice "github.com/fakhry/share-location/backend/internal/features/messaging/service"
	shareddomain "github.com/fakhry/share-location/backend/internal/shared/domain"
)

func TestLoginAndConversationEndpoints(t *testing.T) {
	now := time.Date(2026, 1, 7, 9, 0, 0, 0, time.UTC)
	usersRepo := auth.NewMemoryUserRepository(func() time.Time { return now })
	convRepo := convrepo.NewMemoryConversationRepository(func() time.Time { return now })
	memberRepo := convrepo.NewMemoryMembershipRepository(func() time.Time { return now })
	msgRepo := msgrepo.NewMessageRepository(func() time.Time { return now })
	rcRepo := msgrepo.NewReceiptRepository(func() time.Time { return now })
	msgSvc := msgservice.NewMessagingService(msgRepo, memberRepo, nil, rcRepo, func() time.Time { return now })
	convSvc := convservice.NewConversationService(usersRepo, convRepo, memberRepo, func() time.Time { return now })
	liveSvc := locservice.NewLiveLocationService(locrepo.NewMemoryLocationRepository(func() time.Time { return now }), memberRepo, locrepo.NewMemoryLocationPublisher(), 15*time.Minute, func() time.Time { return now })

	app := New(Deps{
		Users:         auth.NewUserService(usersRepo),
		Conversations: convSvc,
		Messaging:     msgSvc,
		LiveLocation:  liveSvc,
	})

	// Seed user alice (password hash must match simple hash logic: "hashed:"+password)
	// We'll use "password" as password in login request.
	_, _ = usersRepo.UpsertByUsername(context.Background(), "alice", "hashed:password", "alice")

	loginBody, _ := json.Marshal(map[string]string{
		"username": "alice",
		"password": "password",
	})
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
		_, _, err := msgSvc.SendChat(context.Background(), msgservice.SendChatRequest{
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

	req = httptest.NewRequest("GET", "/v1/conversations?user_id="+alice.ID.String(), nil)
	resp, err = app.Test(req)
	if err != nil {
		t.Fatalf("list conversations request failed: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("list conversations status = %d", resp.StatusCode)
	}
	var listResp []map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&listResp)
	if len(listResp) != 1 {
		t.Fatalf("expected 1 conversation, got %d", len(listResp))
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

	// Health
	req = httptest.NewRequest("GET", "/health", nil)
	resp, err = app.Test(req)
	if err != nil {
		t.Fatalf("health request failed: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("health status = %d", resp.StatusCode)
	}
}

func TestHandlers_ReturnValidationErrors(t *testing.T) {
	now := time.Date(2026, 1, 7, 9, 30, 0, 0, time.UTC)
	usersRepo := auth.NewMemoryUserRepository(func() time.Time { return now })
	convRepo := convrepo.NewMemoryConversationRepository(func() time.Time { return now })
	memberRepo := convrepo.NewMemoryMembershipRepository(func() time.Time { return now })
	msgRepo := msgrepo.NewMessageRepository(func() time.Time { return now })
	rcRepo := msgrepo.NewReceiptRepository(func() time.Time { return now })
	liveSvc := locservice.NewLiveLocationService(locrepo.NewMemoryLocationRepository(func() time.Time { return now }), memberRepo, locrepo.NewMemoryLocationPublisher(), 15*time.Minute, func() time.Time { return now })

	app := New(Deps{
		Users:         auth.NewUserService(usersRepo),
		Conversations: convservice.NewConversationService(usersRepo, convRepo, memberRepo, func() time.Time { return now }),
		Messaging:     msgservice.NewMessagingService(msgRepo, memberRepo, nil, rcRepo, func() time.Time { return now }),
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

	// Bad live-location payload.
	llBody, _ := json.Marshal(map[string]any{
		"user_id":         "not-a-uuid",
		"conversation_id": "also-bad",
		"ttl_seconds":     10,
	})
	req = httptest.NewRequest("POST", "/v1/live-locations", bytes.NewReader(llBody))
	req.Header.Set("Content-Type", "application/json")
	resp, _ = app.Test(req)
	if resp.StatusCode != 400 {
		t.Fatalf("live location status = %d, want 400", resp.StatusCode)
	}
}

func TestWebSocketHandshake(t *testing.T) {
	now := time.Date(2026, 1, 7, 10, 0, 0, 0, time.UTC)
	usersRepo := auth.NewMemoryUserRepository(func() time.Time { return now })
	convRepo := convrepo.NewMemoryConversationRepository(func() time.Time { return now })
	memberRepo := convrepo.NewMemoryMembershipRepository(func() time.Time { return now })
	msgRepo := msgrepo.NewMessageRepository(func() time.Time { return now })
	rcRepo := msgrepo.NewReceiptRepository(func() time.Time { return now })
	liveSvc := locservice.NewLiveLocationService(locrepo.NewMemoryLocationRepository(func() time.Time { return now }), memberRepo, locrepo.NewMemoryLocationPublisher(), 15*time.Minute, func() time.Time { return now })

	app := New(Deps{
		Users:         auth.NewUserService(usersRepo),
		Conversations: convservice.NewConversationService(usersRepo, convRepo, memberRepo, func() time.Time { return now }),
		Messaging:     msgservice.NewMessagingService(msgRepo, memberRepo, nil, rcRepo, func() time.Time { return now }),
		LiveLocation:  liveSvc,
	})
	usersRepo.UpsertByUsername(context.Background(), "carol", "", "carol")

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

	// Send Pong to trigger heartbeat
	if err := conn.WriteMessage(websocket.PongMessage, nil); err != nil {
		t.Fatalf("write pong error: %v", err)
	}
	// Give it a moment to process
	time.Sleep(50 * time.Millisecond)
}

func TestWebSocketHandshakeInvalidUsername(t *testing.T) {
	now := time.Date(2026, 1, 7, 10, 30, 0, 0, time.UTC)
	usersRepo := auth.NewMemoryUserRepository(func() time.Time { return now })
	convRepo := convrepo.NewMemoryConversationRepository(func() time.Time { return now })
	memberRepo := convrepo.NewMemoryMembershipRepository(func() time.Time { return now })
	msgRepo := msgrepo.NewMessageRepository(func() time.Time { return now })
	rcRepo := msgrepo.NewReceiptRepository(func() time.Time { return now })
	liveSvc := locservice.NewLiveLocationService(locrepo.NewMemoryLocationRepository(func() time.Time { return now }), memberRepo, locrepo.NewMemoryLocationPublisher(), 15*time.Minute, func() time.Time { return now })

	app := New(Deps{
		Users:         auth.NewUserService(usersRepo),
		Conversations: convservice.NewConversationService(usersRepo, convRepo, memberRepo, func() time.Time { return now }),
		Messaging:     msgservice.NewMessagingService(msgRepo, memberRepo, nil, rcRepo, func() time.Time { return now }),
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
	var resp map[string]any
	if err := json.Unmarshal(data, &resp); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if resp["event"] != "ERROR" {
		t.Fatalf("expected ERROR event, got %+v", resp)
	}
}

func TestWebSocketSendChat(t *testing.T) {
	now := time.Date(2026, 1, 7, 11, 0, 0, 0, time.UTC)
	usersRepo := auth.NewMemoryUserRepository(func() time.Time { return now })
	convRepo := convrepo.NewMemoryConversationRepository(func() time.Time { return now })
	memberRepo := convrepo.NewMemoryMembershipRepository(func() time.Time { return now })
	msgRepo := msgrepo.NewMessageRepository(func() time.Time { return now })
	rcRepo := msgrepo.NewReceiptRepository(func() time.Time { return now })
	liveSvc := locservice.NewLiveLocationService(locrepo.NewMemoryLocationRepository(func() time.Time { return now }), memberRepo, locrepo.NewMemoryLocationPublisher(), 15*time.Minute, func() time.Time { return now })

	convSvc := convservice.NewConversationService(usersRepo, convRepo, memberRepo, func() time.Time { return now })
	msgSvc := msgservice.NewMessagingService(msgRepo, memberRepo, nil, rcRepo, func() time.Time { return now })
	app := New(Deps{
		Users:         auth.NewUserService(usersRepo),
		Conversations: convSvc,
		Messaging:     msgSvc,
		LiveLocation:  liveSvc,
	})
	usersRepo.UpsertByUsername(context.Background(), "alice", "", "alice")
	usersRepo.UpsertByUsername(context.Background(), "bob", "", "bob")

	// Seed conversation and membership.
	conv, _, err := convSvc.CreateConversation(context.Background(), convservice.CreateConversationRequest{
		Type:                 convdomain.ConversationTypeDirect,
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

func TestWebSocketSendChatBroadcastsToOtherUser(t *testing.T) {
	now := time.Date(2026, 1, 7, 11, 5, 0, 0, time.UTC)
	usersRepo := auth.NewMemoryUserRepository(func() time.Time { return now })
	convRepo := convrepo.NewMemoryConversationRepository(func() time.Time { return now })
	memberRepo := convrepo.NewMemoryMembershipRepository(func() time.Time { return now })
	msgRepo := msgrepo.NewMessageRepository(func() time.Time { return now })
	rcRepo := msgrepo.NewReceiptRepository(func() time.Time { return now })
	liveSvc := locservice.NewLiveLocationService(locrepo.NewMemoryLocationRepository(func() time.Time { return now }), memberRepo, locrepo.NewMemoryLocationPublisher(), 15*time.Minute, func() time.Time { return now })

	convSvc := convservice.NewConversationService(usersRepo, convRepo, memberRepo, func() time.Time { return now })
	msgSvc := msgservice.NewMessagingService(msgRepo, memberRepo, nil, rcRepo, func() time.Time { return now })
	app := New(Deps{
		Users:         auth.NewUserService(usersRepo),
		Conversations: convSvc,
		Messaging:     msgSvc,
		LiveLocation:  liveSvc,
	})
	usersRepo.UpsertByUsername(context.Background(), "alice", "", "alice")
	usersRepo.UpsertByUsername(context.Background(), "bob", "", "bob")

	conv, _, err := convSvc.CreateConversation(context.Background(), convservice.CreateConversationRequest{
		Type:                 convdomain.ConversationTypeDirect,
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
	aliceConn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("websocket dial error: %v", err)
	}
	defer aliceConn.Close()
	_, _, _ = aliceConn.ReadMessage() // CONNECTED

	wsURL = "ws://" + ln.Addr().String() + "/ws?username=bob"
	bobConn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("websocket dial error: %v", err)
	}
	defer bobConn.Close()
	_, _, _ = bobConn.ReadMessage() // CONNECTED

	req := map[string]any{
		"event":  "SEND_CHAT",
		"req_id": "r1",
		"data": map[string]any{
			"conversation_id": conv.ID.String(),
			"body":            map[string]any{"text": "hey"},
			"message_type":    "text",
			"client_msg_id":   "c1",
		},
	}
	reqBytes, _ := json.Marshal(req)
	_ = aliceConn.WriteMessage(websocket.TextMessage, reqBytes)

	// Alice receives ack
	_, _, err = aliceConn.ReadMessage()
	if err != nil {
		t.Fatalf("alice read error: %v", err)
	}

	// Bob receives broadcast
	_, data, err := bobConn.ReadMessage()
	if err != nil {
		t.Fatalf("bob read error: %v", err)
	}
	var resp map[string]any
	_ = json.Unmarshal(data, &resp)
	if resp["event"] != "RECEIVE_CHAT" {
		t.Fatalf("expected RECEIVE_CHAT for bob, got %+v", resp)
	}
}

func TestWebSocketSendChatInvalidConversation(t *testing.T) {
	now := time.Date(2026, 1, 7, 11, 15, 0, 0, time.UTC)
	usersRepo := auth.NewMemoryUserRepository(func() time.Time { return now })
	convRepo := convrepo.NewMemoryConversationRepository(func() time.Time { return now })
	memberRepo := convrepo.NewMemoryMembershipRepository(func() time.Time { return now })
	msgRepo := msgrepo.NewMessageRepository(func() time.Time { return now })
	rcRepo := msgrepo.NewReceiptRepository(func() time.Time { return now })
	app := New(Deps{
		Users:         auth.NewUserService(usersRepo),
		Conversations: convservice.NewConversationService(usersRepo, convRepo, memberRepo, func() time.Time { return now }),
		Messaging:     msgservice.NewMessagingService(msgRepo, memberRepo, nil, rcRepo, func() time.Time { return now }),
		LiveLocation:  locservice.NewLiveLocationService(locrepo.NewMemoryLocationRepository(func() time.Time { return now }), memberRepo, locrepo.NewMemoryLocationPublisher(), 15*time.Minute, func() time.Time { return now }),
	})
	usersRepo.UpsertByUsername(context.Background(), "alice", "", "alice")

	ln, _ := net.Listen("tcp", "127.0.0.1:0")
	defer ln.Close()
	go app.Listener(ln)

	wsURL := "ws://" + ln.Addr().String() + "/ws?username=alice"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial error: %v", err)
	}
	defer conn.Close()
	_, _, _ = conn.ReadMessage() // connected

	req := map[string]any{
		"event":  "SEND_CHAT",
		"req_id": "bad1",
		"data": map[string]any{
			"conversation_id": "bad",
			"body":            map[string]any{"text": "hi"},
			"message_type":    "text",
			"client_msg_id":   "c1",
		},
	}
	reqBytes, _ := json.Marshal(req)
	_ = conn.WriteMessage(websocket.TextMessage, reqBytes)
	_, data, _ := conn.ReadMessage()
	var resp map[string]any
	_ = json.Unmarshal(data, &resp)
	if resp["event"] != "ERROR" || resp["req_id"] != "bad1" {
		t.Fatalf("expected error response for bad stream, got %+v", resp)
	}
}

func TestErrorHandlerForbidden(t *testing.T) {
	app := fiber.New(fiber.Config{ErrorHandler: errorHandler})
	app.Get("/forbidden", func(c *fiber.Ctx) error {
		return shareddomain.ErrForbidden
	})
	req := httptest.NewRequest("GET", "/forbidden", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", resp.StatusCode)
	}
}

func TestWebSocketPublishLiveLocation(t *testing.T) {
	now := time.Date(2026, 1, 7, 11, 30, 0, 0, time.UTC)
	usersRepo := auth.NewMemoryUserRepository(func() time.Time { return now })
	convRepo := convrepo.NewMemoryConversationRepository(func() time.Time { return now })
	memberRepo := convrepo.NewMemoryMembershipRepository(func() time.Time { return now })
	msgRepo := msgrepo.NewMessageRepository(func() time.Time { return now })
	rcRepo := msgrepo.NewReceiptRepository(func() time.Time { return now })
	liveRepo := locrepo.NewMemoryLocationRepository(func() time.Time { return now })
	livePub := locrepo.NewMemoryLocationPublisher()

	convSvc := convservice.NewConversationService(usersRepo, convRepo, memberRepo, func() time.Time { return now })
	msgSvc := msgservice.NewMessagingService(msgRepo, memberRepo, nil, rcRepo, func() time.Time { return now })
	liveSvc := locservice.NewLiveLocationService(liveRepo, memberRepo, livePub, 15*time.Minute, func() time.Time { return now })
	app := New(Deps{
		Users:         auth.NewUserService(usersRepo),
		Conversations: convSvc,
		Messaging:     msgSvc,
		LiveLocation:  liveSvc,
	})
	usersRepo.UpsertByUsername(context.Background(), "alice", "", "alice")
	usersRepo.UpsertByUsername(context.Background(), "bob", "", "bob")

	conv, _, err := convSvc.CreateConversation(context.Background(), convservice.CreateConversationRequest{
		Type:                 convdomain.ConversationTypeDirect,
		ParticipantUsernames: []string{"alice", "bob"},
	})
	if err != nil {
		t.Fatalf("CreateConversation seed error: %v", err)
	}
	alice, _ := usersRepo.GetByUsername(context.Background(), "alice")
	session, err := liveSvc.StartSession(context.Background(), locservice.StartSessionRequest{
		UserID:         alice.ID,
		ConversationID: conv.ID,
		RequestedTTL:   5 * time.Minute,
	})
	if err != nil {
		t.Fatalf("StartSession error: %v", err)
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
	_, _, _ = conn.ReadMessage() // CONNECTED

	req := map[string]any{
		"event":  "PUBLISH_LIVE_LOCATION",
		"req_id": "r2",
		"data": map[string]any{
			"stream_id":       session.ID.String(),
			"conversation_id": conv.ID.String(),
			"lat":             1.1,
			"lng":             2.2,
			"accuracy":        3.3,
			"speed":           4.4,
			"heading":         5.5,
			"expires_at":      session.ExpiresAt.Format(time.RFC3339),
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
	if resp["event"] != "SUBSCRIBE_LIVE_LOCATION" || resp["req_id"] != "r2" {
		t.Fatalf("unexpected response %+v", resp)
	}

	// Subscribe to get cached point.
	req = map[string]any{
		"event":  "SUBSCRIBE_LIVE_LOCATION",
		"req_id": "r3",
		"data": map[string]any{
			"stream_id": session.ID.String(),
		},
	}
	reqBytes, _ = json.Marshal(req)
	if err := conn.WriteMessage(websocket.TextMessage, reqBytes); err != nil {
		t.Fatalf("write subscribe error: %v", err)
	}
	_, data, err = conn.ReadMessage()
	if err != nil {
		t.Fatalf("read message error: %v", err)
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if resp["event"] != "SUBSCRIBE_LIVE_LOCATION" || resp["req_id"] != "r3" {
		t.Fatalf("unexpected subscribe response %+v", resp)
	}
}

func TestWebSocketSubscribeInvalidStream(t *testing.T) {
	now := time.Date(2026, 1, 7, 12, 0, 0, 0, time.UTC)
	usersRepo := auth.NewMemoryUserRepository(func() time.Time { return now })
	convRepo := convrepo.NewMemoryConversationRepository(func() time.Time { return now })
	memberRepo := convrepo.NewMemoryMembershipRepository(func() time.Time { return now })
	msgRepo := msgrepo.NewMessageRepository(func() time.Time { return now })
	rcRepo := msgrepo.NewReceiptRepository(func() time.Time { return now })
	app := New(Deps{
		Users:         auth.NewUserService(usersRepo),
		Conversations: convservice.NewConversationService(usersRepo, convRepo, memberRepo, func() time.Time { return now }),
		Messaging:     msgservice.NewMessagingService(msgRepo, memberRepo, nil, rcRepo, func() time.Time { return now }),
		LiveLocation:  locservice.NewLiveLocationService(locrepo.NewMemoryLocationRepository(func() time.Time { return now }), memberRepo, locrepo.NewMemoryLocationPublisher(), 15*time.Minute, func() time.Time { return now }),
	})
	usersRepo.UpsertByUsername(context.Background(), "alice", "", "alice")

	ln, _ := net.Listen("tcp", "127.0.0.1:0")
	defer ln.Close()
	go app.Listener(ln)

	wsURL := "ws://" + ln.Addr().String() + "/ws?username=alice"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial error: %v", err)
	}
	defer conn.Close()
	_, _, _ = conn.ReadMessage()

	req := map[string]any{
		"event":  "SUBSCRIBE_LIVE_LOCATION",
		"req_id": "badstream",
		"data": map[string]any{
			"stream_id": "not-a-uuid",
		},
	}
	reqBytes, _ := json.Marshal(req)
	_ = conn.WriteMessage(websocket.TextMessage, reqBytes)
	_, data, _ := conn.ReadMessage()
	var resp map[string]any
	_ = json.Unmarshal(data, &resp)
	if resp["event"] != "ERROR" || resp["req_id"] != "badstream" {
		t.Fatalf("expected error response for bad stream, got %+v", resp)
	}
}

func TestWebSocketPublishLiveLocationMissingSession(t *testing.T) {
	now := time.Date(2026, 1, 7, 12, 15, 0, 0, time.UTC)
	usersRepo := auth.NewMemoryUserRepository(func() time.Time { return now })
	convRepo := convrepo.NewMemoryConversationRepository(func() time.Time { return now })
	memberRepo := convrepo.NewMemoryMembershipRepository(func() time.Time { return now })
	msgRepo := msgrepo.NewMessageRepository(func() time.Time { return now })
	rcRepo := msgrepo.NewReceiptRepository(func() time.Time { return now })
	liveRepo := locrepo.NewMemoryLocationRepository(func() time.Time { return now })
	app := New(Deps{
		Users:         auth.NewUserService(usersRepo),
		Conversations: convservice.NewConversationService(usersRepo, convRepo, memberRepo, func() time.Time { return now }),
		Messaging:     msgservice.NewMessagingService(msgRepo, memberRepo, nil, rcRepo, func() time.Time { return now }),
		LiveLocation:  locservice.NewLiveLocationService(liveRepo, memberRepo, locrepo.NewMemoryLocationPublisher(), 15*time.Minute, func() time.Time { return now }),
	})
	usersRepo.UpsertByUsername(context.Background(), "alice", "", "alice")

	ln, _ := net.Listen("tcp", "127.0.0.1:0")
	defer ln.Close()
	go app.Listener(ln)

	wsURL := "ws://" + ln.Addr().String() + "/ws?username=alice"
	conn, _, _ := websocket.DefaultDialer.Dial(wsURL, nil)
	defer conn.Close()
	_, _, _ = conn.ReadMessage()

	req := map[string]any{
		"event":  "PUBLISH_LIVE_LOCATION",
		"req_id": "missing",
		"data": map[string]any{
			"stream_id":       uuid.New().String(),
			"conversation_id": uuid.New().String(),
			"lat":             0,
			"lng":             0,
		},
	}
	reqBytes, _ := json.Marshal(req)
	_ = conn.WriteMessage(websocket.TextMessage, reqBytes)
	_, data, _ := conn.ReadMessage()
	var resp map[string]any
	_ = json.Unmarshal(data, &resp)
	if resp["event"] != "ERROR" || resp["req_id"] != "missing" {
		t.Fatalf("expected error response for missing session, got %+v", resp)
	}
}

func TestErrorCodeHelper(t *testing.T) {
	if code := errorCode(shareddomain.ErrInvalidInput); code != "invalid_input" {
		t.Fatalf("unexpected code %s", code)
	}
	if code := errorCode(shareddomain.ErrForbidden); code != "forbidden" {
		t.Fatalf("unexpected code %s", code)
	}
	if code := errorCode(shareddomain.ErrNotFound); code != "not_found" {
		t.Fatalf("unexpected code %s", code)
	}
	if code := errorCode(errors.New("x")); code != "internal" {
		t.Fatalf("unexpected code %s", code)
	}
}
