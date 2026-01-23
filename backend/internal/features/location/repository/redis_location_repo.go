package repository

import (
	"context"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"

	"github.com/fakhry/share-location/backend/internal/features/location/domain"
)

// LiveLocationRepo stores sessions and points in Redis hashes/keys with TTL.
type LiveLocationRepo struct {
	Client *redis.Client
	MaxTTL time.Duration
}

func NewLiveLocationRepo(client *redis.Client, maxTTL time.Duration) *LiveLocationRepo {
	return &LiveLocationRepo{Client: client, MaxTTL: maxTTL}
}

func (r *LiveLocationRepo) UpsertSession(ctx context.Context, session domain.LiveLocationSession) (domain.LiveLocationSession, error) {
	if session.ID == uuid.Nil {
		session.ID = uuid.New()
	}
	key := r.sessionKey(session.ID)
	ttl := time.Until(session.ExpiresAt)
	if ttl <= 0 || (r.MaxTTL > 0 && ttl > r.MaxTTL) {
		ttl = r.MaxTTL
		session.ExpiresAt = time.Now().Add(ttl)
	}
	fields := map[string]any{
		"user_id":         session.UserID.String(),
		"conversation_id": session.ConversationID.String(),
		"expires_at":      session.ExpiresAt.Unix(),
	}
	if err := r.Client.HSet(ctx, key, fields).Err(); err != nil {
		return domain.LiveLocationSession{}, err
	}
	if ttl > 0 {
		_ = r.Client.Expire(ctx, key, ttl).Err()
	}
	return session, nil
}

func (r *LiveLocationRepo) GetSession(ctx context.Context, streamID uuid.UUID) (domain.LiveLocationSession, error) {
	key := r.sessionKey(streamID)
	data, err := r.Client.HGetAll(ctx, key).Result()
	if err != nil {
		return domain.LiveLocationSession{}, err
	}
	if len(data) == 0 {
		return domain.LiveLocationSession{}, domain.ErrNotFound
	}
	userID, _ := uuid.Parse(data["user_id"])
	convID, _ := uuid.Parse(data["conversation_id"])
	expUnix, _ := strconv.ParseInt(data["expires_at"], 10, 64)
	return domain.LiveLocationSession{
		ID:             streamID,
		UserID:         userID,
		ConversationID: convID,
		ExpiresAt:      time.Unix(expUnix, 0),
	}, nil
}

func (r *LiveLocationRepo) SavePoint(ctx context.Context, point domain.LiveLocationPoint) error {
	key := r.pointKey(point.StreamID)
	fields := map[string]any{
		"user_id":         point.UserID.String(),
		"conversation_id": point.ConversationID.String(),
		"lat":             point.Lat,
		"lng":             point.Lng,
		"accuracy":        point.Accuracy,
		"speed":           point.Speed,
		"heading":         point.Heading,
		"updated_at":      point.UpdatedAt.UnixNano(),
		"expires_at":      point.ExpiresAt.Unix(),
	}
	if err := r.Client.HSet(ctx, key, fields).Err(); err != nil {
		return err
	}
	ttl := time.Until(point.ExpiresAt)
	if ttl <= 0 || (r.MaxTTL > 0 && ttl > r.MaxTTL) {
		ttl = r.MaxTTL
	}
	if ttl > 0 {
		_ = r.Client.Expire(ctx, key, ttl).Err()
	}
	return nil
}

func (r *LiveLocationRepo) GetPoint(ctx context.Context, streamID uuid.UUID) (domain.LiveLocationPoint, error) {
	key := r.pointKey(streamID)
	data, err := r.Client.HGetAll(ctx, key).Result()
	if err != nil {
		return domain.LiveLocationPoint{}, err
	}
	if len(data) == 0 {
		return domain.LiveLocationPoint{}, domain.ErrNotFound
	}
	userID, _ := uuid.Parse(data["user_id"])
	convID, _ := uuid.Parse(data["conversation_id"])
	lat, _ := strconv.ParseFloat(data["lat"], 64)
	lng, _ := strconv.ParseFloat(data["lng"], 64)
	acc, _ := strconv.ParseFloat(data["accuracy"], 64)
	speed, _ := strconv.ParseFloat(data["speed"], 64)
	heading, _ := strconv.ParseFloat(data["heading"], 64)
	updated, _ := strconv.ParseInt(data["updated_at"], 10, 64)
	expires, _ := strconv.ParseInt(data["expires_at"], 10, 64)
	return domain.LiveLocationPoint{
		StreamID:       streamID,
		UserID:         userID,
		ConversationID: convID,
		Lat:            lat,
		Lng:            lng,
		Accuracy:       acc,
		Speed:          speed,
		Heading:        heading,
		UpdatedAt:      time.Unix(0, updated),
		ExpiresAt:      time.Unix(expires, 0),
	}, nil
}

func (r *LiveLocationRepo) sessionKey(streamID uuid.UUID) string {
	return "live_session:" + streamID.String()
}

func (r *LiveLocationRepo) pointKey(streamID uuid.UUID) string {
	return "live_point:" + streamID.String()
}
