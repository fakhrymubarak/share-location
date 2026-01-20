package service

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/fakhry/share-location/backend/internal/domain"
	"github.com/fakhry/share-location/backend/internal/repo"
)

type LiveLocationService struct {
	Sessions    repo.LiveLocationRepository
	Memberships repo.MembershipRepository
	Publisher   repo.LiveLocationPublisher
	MaxTTL      time.Duration
	now         func() time.Time
}

func NewLiveLocationService(sessions repo.LiveLocationRepository, memberships repo.MembershipRepository, publisher repo.LiveLocationPublisher, maxTTL time.Duration, now func() time.Time) *LiveLocationService {
	if maxTTL <= 0 {
		maxTTL = 15 * time.Minute
	}
	if now == nil {
		now = time.Now
	}
	return &LiveLocationService{
		Sessions:    sessions,
		Memberships: memberships,
		Publisher:   publisher,
		MaxTTL:      maxTTL,
		now:         now,
	}
}

type StartSessionRequest struct {
	UserID         uuid.UUID
	ConversationID uuid.UUID
	RequestedTTL   time.Duration
}

func (s *LiveLocationService) StartSession(ctx context.Context, req StartSessionRequest) (domain.LiveLocationSession, error) {
	if req.UserID == uuid.Nil {
		return domain.LiveLocationSession{}, domain.ErrInvalidInput
	}
	if req.ConversationID != uuid.Nil {
		ok, err := s.Memberships.IsMember(ctx, req.ConversationID, req.UserID)
		if err != nil {
			return domain.LiveLocationSession{}, err
		}
		if !ok {
			return domain.LiveLocationSession{}, domain.ErrForbidden
		}
	}
	now := s.now()
	ttl := req.RequestedTTL
	if ttl <= 0 || ttl > s.MaxTTL {
		ttl = s.MaxTTL
	}
	session := domain.LiveLocationSession{
		UserID:         req.UserID,
		ConversationID: req.ConversationID,
		ExpiresAt:      now.Add(ttl),
		CreatedAt:      now,
	}
	return s.Sessions.UpsertSession(ctx, session)
}

type PublishLocationRequest struct {
	StreamID       uuid.UUID
	UserID         uuid.UUID
	ConversationID uuid.UUID
	Lat            float64
	Lng            float64
	Accuracy       float64
	Speed          float64
	Heading        float64
	ExpiresAt      time.Time
}

func (s *LiveLocationService) PublishLocation(ctx context.Context, req PublishLocationRequest) (domain.LiveLocationPoint, error) {
	if req.StreamID == uuid.Nil || req.UserID == uuid.Nil {
		return domain.LiveLocationPoint{}, domain.ErrInvalidInput
	}

	session, err := s.Sessions.GetSession(ctx, req.StreamID)
	if err != nil {
		return domain.LiveLocationPoint{}, err
	}
	now := s.now()
	if !session.ExpiresAt.IsZero() && now.After(session.ExpiresAt) {
		return domain.LiveLocationPoint{}, domain.ErrForbidden
	}
	if session.UserID != req.UserID {
		return domain.LiveLocationPoint{}, domain.ErrForbidden
	}
	if session.ConversationID != uuid.Nil && session.ConversationID != req.ConversationID {
		return domain.LiveLocationPoint{}, domain.ErrForbidden
	}
	if session.ConversationID != uuid.Nil {
		ok, err := s.Memberships.IsMember(ctx, session.ConversationID, req.UserID)
		if err != nil {
			return domain.LiveLocationPoint{}, err
		}
		if !ok {
			return domain.LiveLocationPoint{}, domain.ErrForbidden
		}
	}

	effectiveExpiry := session.ExpiresAt
	maxExpiry := now.Add(s.MaxTTL)
	if effectiveExpiry.IsZero() || effectiveExpiry.After(maxExpiry) {
		effectiveExpiry = maxExpiry
	}
	if !req.ExpiresAt.IsZero() && req.ExpiresAt.Before(effectiveExpiry) {
		effectiveExpiry = req.ExpiresAt
	}

	point := domain.LiveLocationPoint{
		StreamID:       req.StreamID,
		UserID:         req.UserID,
		ConversationID: req.ConversationID,
		Lat:            req.Lat,
		Lng:            req.Lng,
		Accuracy:       req.Accuracy,
		Speed:          req.Speed,
		Heading:        req.Heading,
		UpdatedAt:      now,
		ExpiresAt:      effectiveExpiry,
	}

	if err := s.Sessions.SavePoint(ctx, point); err != nil {
		return domain.LiveLocationPoint{}, err
	}
	if s.Publisher != nil {
		if err := s.Publisher.PublishLiveLocation(ctx, point); err != nil {
			return domain.LiveLocationPoint{}, err
		}
	}
	return point, nil
}
