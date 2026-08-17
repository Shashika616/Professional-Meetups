package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/professional-connections/backend/services/auth/internal/repository/sqlcgen"
	"github.com/professional-connections/backend/shared/apperror"
)

// postgresVerificationCodeRepository implements VerificationCodeRepository
// against Postgres via pgx/sqlc.
type postgresVerificationCodeRepository struct {
	q *sqlcgen.Queries
}

// NewVerificationCodeRepository constructs a VerificationCodeRepository
// backed by pool.
func NewVerificationCodeRepository(pool *pgxpool.Pool) VerificationCodeRepository {
	return &postgresVerificationCodeRepository{q: sqlcgen.New(pool)}
}

func (r *postgresVerificationCodeRepository) Upsert(
	ctx context.Context, userID string, purpose VerificationPurpose, target, codeHash string, expiresAt time.Time,
) (VerificationCode, error) {
	parsed, err := uuid.Parse(userID)
	if err != nil {
		return VerificationCode{}, fmt.Errorf("repository: invalid user id %q: %w", userID, apperror.ErrInvalidInput)
	}

	row, err := r.q.UpsertVerificationCode(ctx, sqlcgen.UpsertVerificationCodeParams{
		UserID:    parsed,
		Purpose:   sqlcgen.VerificationPurpose(purpose),
		Target:    target,
		CodeHash:  codeHash,
		ExpiresAt: toTimestamptz(expiresAt),
	})
	if err != nil {
		return VerificationCode{}, fmt.Errorf("repository: upsert verification code: %w", err)
	}
	return verificationCodeFromRow(row), nil
}

func (r *postgresVerificationCodeRepository) Get(ctx context.Context, userID string, purpose VerificationPurpose) (VerificationCode, error) {
	parsed, err := uuid.Parse(userID)
	if err != nil {
		return VerificationCode{}, fmt.Errorf("repository: invalid user id %q: %w", userID, apperror.ErrInvalidInput)
	}

	row, err := r.q.GetVerificationCode(ctx, sqlcgen.GetVerificationCodeParams{
		UserID:  parsed,
		Purpose: sqlcgen.VerificationPurpose(purpose),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return VerificationCode{}, fmt.Errorf("repository: no pending %s verification code: %w", purpose, apperror.ErrNotFound)
		}
		return VerificationCode{}, fmt.Errorf("repository: get verification code: %w", err)
	}
	return verificationCodeFromRow(row), nil
}

func (r *postgresVerificationCodeRepository) IncrementAttempts(ctx context.Context, userID string, purpose VerificationPurpose) (VerificationCode, error) {
	parsed, err := uuid.Parse(userID)
	if err != nil {
		return VerificationCode{}, fmt.Errorf("repository: invalid user id %q: %w", userID, apperror.ErrInvalidInput)
	}

	row, err := r.q.IncrementVerificationAttempts(ctx, sqlcgen.IncrementVerificationAttemptsParams{
		UserID:  parsed,
		Purpose: sqlcgen.VerificationPurpose(purpose),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return VerificationCode{}, fmt.Errorf("repository: no pending %s verification code: %w", purpose, apperror.ErrNotFound)
		}
		return VerificationCode{}, fmt.Errorf("repository: increment verification attempts: %w", err)
	}
	return verificationCodeFromRow(row), nil
}

func (r *postgresVerificationCodeRepository) Delete(ctx context.Context, userID string, purpose VerificationPurpose) error {
	parsed, err := uuid.Parse(userID)
	if err != nil {
		return fmt.Errorf("repository: invalid user id %q: %w", userID, apperror.ErrInvalidInput)
	}

	if err := r.q.DeleteVerificationCode(ctx, sqlcgen.DeleteVerificationCodeParams{
		UserID:  parsed,
		Purpose: sqlcgen.VerificationPurpose(purpose),
	}); err != nil {
		return fmt.Errorf("repository: delete verification code: %w", err)
	}
	return nil
}

func verificationCodeFromRow(row sqlcgen.VerificationCode) VerificationCode {
	return VerificationCode{
		ID:        row.ID.String(),
		UserID:    row.UserID.String(),
		Purpose:   VerificationPurpose(row.Purpose),
		Target:    row.Target,
		CodeHash:  row.CodeHash,
		Attempts:  int(row.Attempts),
		ExpiresAt: timestamptzOrZero(row.ExpiresAt),
		CreatedAt: timestamptzOrZero(row.CreatedAt),
	}
}
