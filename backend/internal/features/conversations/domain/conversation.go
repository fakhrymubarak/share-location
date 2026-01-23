package domain

import (
	"context"
	"time"

	"github.com/google/uuid"
)

type ConversationType string

const (
	ConversationTypeDirect ConversationType = "direct"
	ConversationTypeGroup  ConversationType = "group"
)

// Participant summary for lists.
type Participant struct {
	ID       uuid.UUID `json:"id"`
	Username string    `json:"username"`
}

// Conversation aggregates members and messages.
// Note: LastMessage is a pointer to avoid circular dependency if Message is moved to another feature.
// For now, we will define a minimal Message wrapper or interface if needed,
// but since this is just a struct, we might need to import Messaging domain if we want strict typing,
// OR keep Message in shared/legacy for now.
//
// However, to keep features independent, 'LastMessage' here is tricky if Message is in 'messaging'.
// Option 1: 'LastMessage' is just an `any` or a local struct.
// Option 2: 'Message' struct is shared.
//
// Let's see original domain.go: `LastMessage *Message`.
// If I move Message to `features/messaging`, then `features/conversations` depends on `features/messaging`.
// This violates "features should not depend on each other" ideally.
// But Conversations *aggregates* messages.
//
// Plan: Copy basic Message struct or Reference it?
// Best practice: Use a shared kernel for "Summary" or DTOs if they cross boundaries.
// OR: `LastMessage` is actually part of the Conversation *View*.
// For the Domain entity, maybe we don't strictly need it to be the full Message entity from Messaging feature.
//
// Let's look at `domain.go` again. `Message` struct is there.
// I will temporarily define `MessageSummary` or similar if needed, or just import it if I must.
// BUT, I haven't moved Message yet. So I can explicitly import `github.com/fakhry/share-location/backend/internal/domain` aliased? No that creates cycle.
//
// Solution: For now, I will NOT move `Message` struct yet.
// I will define `Conversation` struct in `features/conversations/domain`.
// Does it *need* `LastMessage` field of type `*Message`?
// Yes.
//
// I will create a `MessageSummary` in this file or generic map?
// Actually, `LastMessage` in `Conversation` is likely just for display.
// I'll define a local `LastMessage` struct or `any` for now, OR:
//
// Wait, `Message` is its own feature.
// If I import "github.com/fakhry/share-location/backend/internal/features/messaging/domain" it's a forward reference if that doesn't exist yet.
//
// Pragmatic approach: Define `Message` interface or struct stub in `conversations/domain`? No, duplicate.
//
// BETTER: Leave `LastMessage` as `any` or `interface{}`? No, type safety.
//
// Let's punt on `LastMessage` type for a second. `Conversation` entities need `Message`.
// Maybe I should move `Message` to `Shared`? No, it's a core feature.
//
// What if `Conversation` only holds `LastMessageID`? And the service stitches it?
// The current `Conversation` struct has `LastMessage *Message`.
//
// Let's use `stub` for Message in this file for now, matching the shape?
// Or import from `internal/domain` (legacy) which works for now until I refactor Messaging?
// NO: `internal/domain` will import `features/conversations/domain` (for aliases). Cycle!
//
// Solution: Move `Message` struct to `internal/shared/domain`? No, that's heavy.
// Solution: Decouple.
//
// Let's look at the usage.
// I will define `Message` inside `features/conversations/domain` as a "View" model?
// Or: Just define `LastMessage` as `*Message` where `Message` is defined in THIS file too?
// But `Message` is the core of Messaging feature.
//
// OK, strict feature separation says: Conversation domain shouldn't know about full Message domain.
// But `LastMessage` is a property of a Conversation in the list view.
//
// I will define a minimal `MessagePreview` struct in `conversations` domain.
// And mapped to it.
//
// Let's look at `Conversation` struct:
/*
type Conversation struct {
	ID            uuid.UUID
	Type          ConversationType
	Title         string
	DirectPairKey string
	CreatedAt     time.Time
	Participants  []Participant
	LastMessage   *Message
}
*/
//
// I'll define `Message` in `conversations/domain` as well? NO.
// I'll define `ConversationMessage` struct in `conversations/domain` that matches strictly what's needed.
//
// Actually, let's see `domain.go`. `Message` is:
/*
type Message struct {
	ID             uuid.UUID
	ConversationID uuid.UUID
	SenderID       uuid.UUID
	Body           map[string]any
	MessageType    string
	ClientMsgID    string
	CreatedAt      time.Time
}
*/
//
// I will duplicate this struct in `conversations/domain` but call it `Message` or something?
// Better: Use `any` for `LastMessage` to break dependency?
// Better: Define `LastMessage` struct inline or `MessagePreview`.
//
// I will go with `MessagePreview` struct in this file.
// AND I need `ConversationMember`.

type ConversationMessage struct {
	ID             uuid.UUID
	ConversationID uuid.UUID
	SenderID       uuid.UUID
	Body           map[string]any
	MessageType    string
	ClientMsgID    string
	CreatedAt      time.Time
}

type Conversation struct {
	ID            uuid.UUID
	Type          ConversationType
	Title         string
	DirectPairKey string
	CreatedAt     time.Time
	Participants  []Participant
	LastMessage   *ConversationMessage
}

type ConversationMember struct {
	ConversationID    uuid.UUID
	UserID            uuid.UUID
	Role              string
	JoinedAt          time.Time
	LastReadMessageID *uuid.UUID
}

// ConversationRepository manages conversations.
type ConversationRepository interface {
	Create(ctx context.Context, conv Conversation) (Conversation, error)
	GetByID(ctx context.Context, id uuid.UUID) (Conversation, error)
	FindByDirectPairKey(ctx context.Context, pairKey string) (Conversation, error)
	ListByUser(ctx context.Context, userID uuid.UUID) ([]Conversation, error)
}

// MembershipRepository manages conversation members.
type MembershipRepository interface {
	Add(ctx context.Context, members []ConversationMember) error
	IsMember(ctx context.Context, conversationID, userID uuid.UUID) (bool, error)
	ListByConversation(ctx context.Context, conversationID uuid.UUID) ([]ConversationMember, error)
}
