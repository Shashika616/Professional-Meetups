package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

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

func (r *postgresUserRepository) GetByPersonalEmail(ctx context.Context, email string) (User, error) {
	row, err := r.q.GetUserByPersonalEmail(ctx, textOrNull(email))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return User{}, fmt.Errorf("repository: user with personal_email %q: %w", email, apperror.ErrNotFound)
		}
		return User{}, fmt.Errorf("repository: get user by personal_email: %w", err)
	}
	return userFromRow(row), nil
}

func (r *postgresUserRepository) SetPasswordHash(ctx context.Context, userID, hash string) (User, error) {
	parsed, err := uuid.Parse(userID)
	if err != nil {
		return User{}, fmt.Errorf("repository: invalid user id %q: %w", userID, apperror.ErrInvalidInput)
	}

	row, err := r.q.SetUserPasswordHash(ctx, sqlcgen.SetUserPasswordHashParams{
		ID:           parsed,
		PasswordHash: textOrNull(hash),
	})
	if err != nil {
		return User{}, fmt.Errorf("repository: set user password hash: %w", err)
	}
	return userFromRow(row), nil
}

func (r *postgresUserRepository) Create(ctx context.Context, u NewUser) (User, error) {
	row, err := r.q.CreateUser(ctx, sqlcgen.CreateUserParams{
		LinkedinSub:        textOrNull(u.LinkedInSub),
		FullName:           u.FullName,
		ProfilePhotoUrl:    textOrNull(u.ProfilePhotoURL),
		Headline:           textOrNull(u.Headline),
		TrustLevel:         int16(u.TrustLevel),
		AgeConfirmedOver18: u.AgeConfirmedOver18,
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

func (r *postgresUserRepository) UpdatePhoneNumber(ctx context.Context, userID, phoneNumber string, trustLevel int) (User, error) {
	parsed, err := uuid.Parse(userID)
	if err != nil {
		return User{}, fmt.Errorf("repository: invalid user id %q: %w", userID, apperror.ErrInvalidInput)
	}

	row, err := r.q.UpdateUserPhoneNumber(ctx, sqlcgen.UpdateUserPhoneNumberParams{
		ID:          parsed,
		PhoneNumber: textOrNull(phoneNumber),
		TrustLevel:  int16(trustLevel),
	})
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" { // unique_violation
			return User{}, fmt.Errorf("repository: phone number already verified on a different account: %w", apperror.ErrConflict)
		}
		return User{}, fmt.Errorf("repository: update user phone number: %w", err)
	}
	return userFromRow(row), nil
}

func (r *postgresUserRepository) UpdatePersonalEmail(ctx context.Context, userID, personalEmail string, trustLevel int) (User, error) {
	parsed, err := uuid.Parse(userID)
	if err != nil {
		return User{}, fmt.Errorf("repository: invalid user id %q: %w", userID, apperror.ErrInvalidInput)
	}

	row, err := r.q.UpdateUserPersonalEmail(ctx, sqlcgen.UpdateUserPersonalEmailParams{
		ID:            parsed,
		PersonalEmail: textOrNull(personalEmail),
		TrustLevel:    int16(trustLevel),
	})
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" { // unique_violation
			return User{}, fmt.Errorf("repository: personal email already verified on a different account: %w", apperror.ErrConflict)
		}
		return User{}, fmt.Errorf("repository: update user personal email: %w", err)
	}
	return userFromRow(row), nil
}

func (r *postgresUserRepository) UpdatePersonalDetails(ctx context.Context, userID, legalName, address string, trustLevel int) (User, error) {
	parsed, err := uuid.Parse(userID)
	if err != nil {
		return User{}, fmt.Errorf("repository: invalid user id %q: %w", userID, apperror.ErrInvalidInput)
	}

	row, err := r.q.UpdateUserPersonalDetails(ctx, sqlcgen.UpdateUserPersonalDetailsParams{
		ID:         parsed,
		LegalName:  textOrNull(legalName),
		Address:    textOrNull(address),
		TrustLevel: int16(trustLevel),
	})
	if err != nil {
		return User{}, fmt.Errorf("repository: update user personal details: %w", err)
	}
	return userFromRow(row), nil
}

func (r *postgresUserRepository) UpdateLinkedInSub(ctx context.Context, userID, linkedInSub string, trustLevel int) (User, error) {
	parsed, err := uuid.Parse(userID)
	if err != nil {
		return User{}, fmt.Errorf("repository: invalid user id %q: %w", userID, apperror.ErrInvalidInput)
	}

	row, err := r.q.UpdateUserLinkedInSub(ctx, sqlcgen.UpdateUserLinkedInSubParams{
		ID:          parsed,
		LinkedinSub: textOrNull(linkedInSub),
		TrustLevel:  int16(trustLevel),
	})
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" { // unique_violation
			return User{}, fmt.Errorf("repository: linkedin account already linked to a different user: %w", apperror.ErrConflict)
		}
		return User{}, fmt.Errorf("repository: update user linkedin sub: %w", err)
	}
	return userFromRow(row), nil
}

func (r *postgresUserRepository) UpdateWorkEmailVerified(ctx context.Context, userID, companyDomain string, verified bool, verifiedAt time.Time, trustLevel int) (User, error) {
	parsed, err := uuid.Parse(userID)
	if err != nil {
		return User{}, fmt.Errorf("repository: invalid user id %q: %w", userID, apperror.ErrInvalidInput)
	}

	row, err := r.q.UpdateUserWorkEmailVerified(ctx, sqlcgen.UpdateUserWorkEmailVerifiedParams{
		ID:                  parsed,
		CompanyDomain:       textOrNull(companyDomain),
		WorkEmailVerified:   verified,
		WorkEmailVerifiedAt: toTimestamptz(verifiedAt),
		TrustLevel:          int16(trustLevel),
	})
	if err != nil {
		return User{}, fmt.Errorf("repository: update user work email verified: %w", err)
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

		PhoneNumber:         textOrEmpty(row.PhoneNumber),
		PersonalEmail:       textOrEmpty(row.PersonalEmail),
		LegalName:           textOrEmpty(row.LegalName),
		Address:             textOrEmpty(row.Address),
		CompanyDomain:       textOrEmpty(row.CompanyDomain),
		WorkEmailVerified:   row.WorkEmailVerified,
		WorkEmailVerifiedAt: timePtrOrNil(row.WorkEmailVerifiedAt),

		AgeConfirmedOver18: row.AgeConfirmedOver18,
		AgeConfirmedAt:     timePtrOrNil(row.AgeConfirmedAt),
		PasswordHash:       textOrEmpty(row.PasswordHash),

		RatingAverage: numericToFloat64(row.RatingAverage),
		RatingCount:   int(row.RatingCount),
	}
}
