package service

import (
	"context"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/fakhry/share-location/backend/internal/auth"
	convdomain "github.com/fakhry/share-location/backend/internal/features/conversations/domain"
	shareddomain "github.com/fakhry/share-location/backend/internal/shared/domain"
)

type ConversationService struct {
	Users         auth.UserRepository
	Conversations convdomain.ConversationRepository
	Memberships   convdomain.MembershipRepository
	now           func() time.Time
}

func NewConversationService(users auth.UserRepository, convs convdomain.ConversationRepository, members convdomain.MembershipRepository, now func() time.Time) *ConversationService {
	if now == nil {
		now = time.Now
	}
	return &ConversationService{
		Users:         users,
		Conversations: convs,
		Memberships:   members,
		now:           now,
	}
}

type CreateConversationRequest struct {
	Type                 convdomain.ConversationType
	Title                string
	ParticipantUsernames []string
}

func (s *ConversationService) CreateConversation(ctx context.Context, req CreateConversationRequest) (convdomain.Conversation, []convdomain.ConversationMember, error) {
	if err := validateConversationRequest(req); err != nil {
		return convdomain.Conversation{}, nil, err
	}

	uniqueUsernames := dedupeStrings(req.ParticipantUsernames)

	users := make([]auth.User, 0, len(uniqueUsernames))
	for _, username := range uniqueUsernames {
		user, err := s.Users.UpsertByUsername(ctx, username, "", username)
		if err != nil {
			return convdomain.Conversation{}, nil, err
		}
		users = append(users, user)
	}

	now := s.now()
	var conv convdomain.Conversation
	var err error

	if req.Type == convdomain.ConversationTypeDirect {
		if len(users) != 2 {
			return convdomain.Conversation{}, nil, shareddomain.ErrInvalidInput
		}
		pairKey := buildDirectPairKey(users[0].ID, users[1].ID)
		existing, findErr := s.Conversations.FindByDirectPairKey(ctx, pairKey)
		if findErr == nil {
			conv = existing
		} else {
			conv = convdomain.Conversation{
				Type:          convdomain.ConversationTypeDirect,
				DirectPairKey: pairKey,
				CreatedAt:     now,
				Participants:  toParticipants(users),
			}
			conv, err = s.Conversations.Create(ctx, conv)
			if err != nil {
				return convdomain.Conversation{}, nil, err
			}
		}
	} else {
		conv = convdomain.Conversation{
			Type:         convdomain.ConversationTypeGroup,
			Title:        strings.TrimSpace(req.Title),
			CreatedAt:    now,
			Participants: toParticipants(users),
		}
		conv, err = s.Conversations.Create(ctx, conv)
		if err != nil {
			return convdomain.Conversation{}, nil, err
		}
	}

	members := make([]convdomain.ConversationMember, 0, len(users))
	for _, user := range users {
		members = append(members, convdomain.ConversationMember{
			ConversationID: conv.ID,
			UserID:         user.ID,
			Role:           "member",
			JoinedAt:       conv.CreatedAt,
		})
	}
	if err := s.Memberships.Add(ctx, members); err != nil {
		return convdomain.Conversation{}, nil, err
	}
	return conv, members, nil
}

// ListConversations returns all conversations for a user.
func (s *ConversationService) ListConversations(ctx context.Context, userID uuid.UUID) ([]convdomain.Conversation, error) {
	if userID == uuid.Nil {
		return nil, shareddomain.ErrInvalidInput
	}
	return s.Conversations.ListByUser(ctx, userID)
}

// ListMembers returns conversation members for broadcasting/authorization helpers.
func (s *ConversationService) ListMembers(ctx context.Context, conversationID uuid.UUID) ([]convdomain.ConversationMember, error) {
	if conversationID == uuid.Nil {
		return nil, shareddomain.ErrInvalidInput
	}
	return s.Memberships.ListByConversation(ctx, conversationID)
}

func validateConversationRequest(req CreateConversationRequest) error {
	if req.Type != convdomain.ConversationTypeDirect && req.Type != convdomain.ConversationTypeGroup {
		return shareddomain.ErrInvalidInput
	}
	if len(req.ParticipantUsernames) == 0 {
		return shareddomain.ErrInvalidInput
	}
	return nil
}

func dedupeStrings(list []string) []string {
	seen := make(map[string]struct{})
	out := make([]string, 0, len(list))
	for _, v := range list {
		key := strings.ToLower(strings.TrimSpace(v))
		if key == "" {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, v)
	}
	return out
}

func buildDirectPairKey(a, b uuid.UUID) string {
	ids := []string{a.String(), b.String()}
	sort.Strings(ids)
	return strings.Join(ids, ":")
}

func toParticipants(users []auth.User) []convdomain.Participant {
	parts := make([]convdomain.Participant, 0, len(users))
	for _, u := range users {
		parts = append(parts, convdomain.Participant{
			ID:       u.ID,
			Username: u.Username,
		})
	}
	return parts
}
