package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/fakhry/share-location/backend/internal/features/conversations/domain"
	"github.com/fakhry/share-location/backend/internal/repo/pg"
)

// PgMembershipRepository implements repo.MembershipRepository using Postgres.
type PgMembershipRepository struct {
	DB  pg.PgxPool
	now func() time.Time
}

func NewPgMembershipRepository(db pg.PgxPool, now func() time.Time) *PgMembershipRepository {
	return &PgMembershipRepository{DB: db, now: now}
}

func (r *PgMembershipRepository) Add(ctx context.Context, members []domain.ConversationMember) error {
	if len(members) == 0 {
		return nil
	}
	args := make([]any, 0, len(members)*5)
	var sb strings.Builder
	sb.WriteString("INSERT INTO conversation_members (conversation_id, user_id, role, joined_at, last_read_message_id) VALUES ")

	for i, m := range members {
		if m.JoinedAt.IsZero() {
			m.JoinedAt = nowOr(r.now, time.Now())
		}
		placeholderOffset := i*5 + 1
		sb.WriteString(fmt.Sprintf("($%d,$%d,$%d,$%d,$%d)", placeholderOffset, placeholderOffset+1, placeholderOffset+2, placeholderOffset+3, placeholderOffset+4))
		if i < len(members)-1 {
			sb.WriteString(",")
		}
		args = append(args, m.ConversationID, m.UserID, m.Role, m.JoinedAt, m.LastReadMessageID)
	}
	sb.WriteString(" ON CONFLICT (conversation_id, user_id) DO NOTHING")

	_, err := r.DB.Exec(ctx, sb.String(), args...)
	return err
}

func (r *PgMembershipRepository) IsMember(ctx context.Context, conversationID, userID uuid.UUID) (bool, error) {
	const query = `SELECT 1 FROM conversation_members WHERE conversation_id=$1 AND user_id=$2 LIMIT 1`
	var dummy int
	if err := r.DB.QueryRow(ctx, query, conversationID, userID).Scan(&dummy); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (r *PgMembershipRepository) ListByConversation(ctx context.Context, conversationID uuid.UUID) ([]domain.ConversationMember, error) {
	const query = `SELECT conversation_id, user_id, role, joined_at, last_read_message_id FROM conversation_members WHERE conversation_id=$1 ORDER BY joined_at`
	rows, err := r.DB.Query(ctx, query, conversationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var members []domain.ConversationMember
	for rows.Next() {
		var m domain.ConversationMember
		var lastRead uuid.NullUUID
		if err := rows.Scan(&m.ConversationID, &m.UserID, &m.Role, &m.JoinedAt, &lastRead); err != nil {
			return nil, err
		}
		if lastRead.Valid {
			m.LastReadMessageID = &lastRead.UUID
		}
		members = append(members, m)
	}
	return members, rows.Err()
}
