package repository

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/professional-connections/backend/services/meetup/internal/repository/sqlcgen"
	"github.com/professional-connections/backend/shared/apperror"
)

type postgresDeviceTokenRepository struct {
	q *sqlcgen.Queries
}

// NewDeviceTokenRepository constructs a DeviceTokenRepository backed by
// pool.
func NewDeviceTokenRepository(pool *pgxpool.Pool) DeviceTokenRepository {
	return &postgresDeviceTokenRepository{q: sqlcgen.New(pool)}
}

func (r *postgresDeviceTokenRepository) Upsert(ctx context.Context, userID, fcmToken string) error {
	user, err := parseUUID(userID)
	if err != nil {
		return fmt.Errorf("repository: invalid user id %q: %w", userID, apperror.ErrInvalidInput)
	}
	if _, err := r.q.UpsertDeviceToken(ctx, sqlcgen.UpsertDeviceTokenParams{UserID: user, FcmToken: fcmToken}); err != nil {
		return fmt.Errorf("repository: upsert device token: %w", err)
	}
	return nil
}

func (r *postgresDeviceTokenRepository) ListForUser(ctx context.Context, userID string) ([]string, error) {
	user, err := parseUUID(userID)
	if err != nil {
		return nil, fmt.Errorf("repository: invalid user id %q: %w", userID, apperror.ErrInvalidInput)
	}
	rows, err := r.q.ListDeviceTokensForUser(ctx, user)
	if err != nil {
		return nil, fmt.Errorf("repository: list device tokens for user: %w", err)
	}
	tokens := make([]string, 0, len(rows))
	for _, row := range rows {
		tokens = append(tokens, row.FcmToken)
	}
	return tokens, nil
}
