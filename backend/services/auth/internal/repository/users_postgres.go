package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/professional-connections/backend/services/auth/internal/repository/sqlcgen"
	"github.com/professional-connections/backend/shared/apperror"
)

// postgresUserRepository implements UserRepository against Postgres via
// pgx/sqlc.
type postgresUserRepository struct {
	q *sqlcgen.Queries
}

// NewUserRepository constructs a UserRepository backed by pool.
func NewUserRepository(pool *pgxpool.Pool) UserRepository {
	return &postgresUserRepository{q: sqlcgen.New(pool)}
}

func (r *postgresUserRepository) GetByLinkedInSub(ctx context.Context, linkedInSub string) (User, error) {
	row, err := r.q.GetUserByLinkedInSub(ctx, textOrNull(linkedInSub))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return User{}, fmt.Errorf("repository: user with linkedin_sub %q: %w", linkedInSub, apperror.ErrNotFound)
		}
		return User{}, fmt.Errorf("repository: get user by linkedin_sub: %w", err)
	}
	return userFromRow(row), nil
}

func (r *postgresUserRepository) GetByID(ctx context.Context, id string) (User, error) {
	parsed, err := uuid.Parse(id)
	if err != nil {
		return User{}, fmt.Errorf("repository: invalid user id %q: %w", id, apperror.ErrInvalidInput)
	}

	row, err := r.q.GetUserByID(ctx, parsed)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return User{}, fmt.Errorf("repository: user %q: %w", id, apperror.ErrNotFound)
		}
		return User{}, fmt.Errorf("repository: get user by id: %w", err)
	}
	return userFromRow(row), nil
}

func (r *postgresUserRepository) Create(ctx context.Context, u NewUser) (User, error) {
	row, err := r.q.CreateUser(ctx, sqlcgen.CreateUserParams{
		LinkedinSub:     textOrNull(u.LinkedInSub),
		FullName:        u.FullName,
		ProfilePhotoUrl: textOrNull(u.ProfilePhotoURL),
		Headline:        textOrNull(u.Headline),
		TrustLevel:      int16(u.TrustLevel),
	})
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" { // unique_violation
			return User{}, fmt.Errorf("repository: user with linkedin_sub %q already exists: %w", u.LinkedInSub, apperror.ErrConflict)
		}
		return User{}, fmt.Errorf("repository: create user: %w", err)
	}
	return userFromRow(row), nil
}

func userFromRow(row sqlcgen.User) User {
	return User{
		ID:              row.ID.String(),
		LinkedInSub:     textOrEmpty(row.LinkedinSub),
		FullName:        row.FullName,
		ProfilePhotoURL: textOrEmpty(row.ProfilePhotoUrl),
		Headline:        textOrEmpty(row.Headline),
		TrustLevel:      int(row.TrustLevel),
		AccountStatus:   AccountStatus(row.AccountStatus),
		CreatedAt:       timestamptzOrZero(row.CreatedAt),
		UpdatedAt:       timestamptzOrZero(row.UpdatedAt),
	}
}
