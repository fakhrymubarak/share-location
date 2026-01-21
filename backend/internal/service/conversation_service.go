package service

import (
	"context"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/fakhry/share-location/backend/internal/domain"
	"github.com/fakhry/share-location/backend/internal/repo"
)

type ConversationService struct {
	Users         repo.UserRepository
	Conversations repo.ConversationRepository
	Memberships   repo.MembershipRepository
	now           func() time.Time
}

func NewConversationService(users repo.UserRepository, convs repo.ConversationRepository, members repo.MembershipRepository, now func() time.Time) *ConversationService {
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
	Type                 domain.ConversationType
	Title                string
	ParticipantUsernames []string
}

func (s *ConversationService) CreateConversation(ctx context.Context, req CreateConversationRequest) (domain.Conversation, []domain.ConversationMember, error) {
	if err := validateConversationRequest(req); err != nil {
		return domain.Conversation{}, nil, err
	}

	uniqueUsernames := dedupeStrings(req.ParticipantUsernames)

	users := make([]domain.User, 0, len(uniqueUsernames))
	for _, username := range uniqueUsernames {
		user, err := s.Users.UpsertByUsername(ctx, username, username)
		if err != nil {
			return domain.Conversation{}, nil, err
		}
		users = append(users, user)
	}

	now := s.now()
	var conv domain.Conversation
	var err error

	if req.Type == domain.ConversationTypeDirect {
		if len(users) != 2 {
			return domain.Conversation{}, nil, domain.ErrInvalidInput
		}
		pairKey := buildDirectPairKey(users[0].ID, users[1].ID)
		existing, findErr := s.Conversations.FindByDirectPairKey(ctx, pairKey)
		if findErr == nil {
			conv = existing
		} else {
			conv = domain.Conversation{
				Type:          domain.ConversationTypeDirect,
				DirectPairKey: pairKey,
				CreatedAt:     now,
				Participants:  []uuid.UUID{users[0].ID, users[1].ID},
			}
			conv, err = s.Conversations.Create(ctx, conv)
			if err != nil {
				return domain.Conversation{}, nil, err
			}
		}
	} else {
		conv = domain.Conversation{
			Type:         domain.ConversationTypeGroup,
			Title:        strings.TrimSpace(req.Title),
			CreatedAt:    now,
			Participants: extractUserIDs(users),
		}
		conv, err = s.Conversations.Create(ctx, conv)
		if err != nil {
			return domain.Conversation{}, nil, err
		}
	}

	members := make([]domain.ConversationMember, 0, len(users))
	for _, user := range users {
		members = append(members, domain.ConversationMember{
			ConversationID: conv.ID,
			UserID:         user.ID,
			Role:           "member",
			JoinedAt:       conv.CreatedAt,
		})
	}
	if err := s.Memberships.Add(ctx, members); err != nil {
		return domain.Conversation{}, nil, err
	}
	return conv, members, nil
}

// ListConversations returns all conversations for a user.
func (s *ConversationService) ListConversations(ctx context.Context, userID uuid.UUID) ([]domain.Conversation, error) {
	return s.Conversations.ListByUser(ctx, userID)
}

// ListMembers returns conversation members for broadcasting/authorization helpers.
func (s *ConversationService) ListMembers(ctx context.Context, conversationID uuid.UUID) ([]domain.ConversationMember, error) {
	if conversationID == uuid.Nil {
		return nil, domain.ErrInvalidInput
	}
	return s.Memberships.ListByConversation(ctx, conversationID)
}

func validateConversationRequest(req CreateConversationRequest) error {
	if req.Type != domain.ConversationTypeDirect && req.Type != domain.ConversationTypeGroup {
		return domain.ErrInvalidInput
	}
	if len(req.ParticipantUsernames) == 0 {
		return domain.ErrInvalidInput
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

func extractUserIDs(users []domain.User) []uuid.UUID {
	ids := make([]uuid.UUID, 0, len(users))
	for _, u := range users {
		ids = append(ids, u.ID)
	}
	return ids
}
