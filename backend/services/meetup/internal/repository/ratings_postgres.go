package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/professional-connections/backend/services/meetup/internal/repository/sqlcgen"
	"github.com/professional-connections/backend/shared/apperror"
)

// pgCheckViolation is Postgres's error code for a CHECK constraint
// violation (23514) — meetup_ratings' self-rating guard
// (CHECK(rater_user_id <> rated_user_id)) is the only one this repository
// expects to hit in practice (ADR-015).
const pgCheckViolation = "23514"

type postgresRatingRepository struct {
	// pool (not just *sqlcgen.Queries) — Submit needs a transaction spanning
	// both meetup_ratings and users, same reasoning as
	// postgresMeetupRequestRepository.Accept.
	pool *pgxpool.Pool
	q    *sqlcgen.Queries
}

// NewRatingRepository constructs a RatingRepository backed by pool.
func NewRatingRepository(pool *pgxpool.Pool) RatingRepository {
	return &postgresRatingRepository{pool: pool, q: sqlcgen.New(pool)}
}

func (r *postgresRatingRepository) IsParticipant(ctx context.Context, meetupID, userID string) (bool, error) {
	meetup, err := parseUUID(meetupID)
	if err != nil {
		return false, fmt.Errorf("repository: invalid meetup id %q: %w", meetupID, apperror.ErrInvalidInput)
	}
	user, err := parseUUID(userID)
	if err != nil {
		return false, fmt.Errorf("repository: invalid user id %q: %w", userID, apperror.ErrInvalidInput)
	}

	ok, err := r.q.IsMeetupParticipant(ctx, sqlcgen.IsMeetupParticipantParams{MeetupID: meetup, UserID: user})
	if err != nil {
		return false, fmt.Errorf("repository: check meetup participant: %w", err)
	}
	return ok, nil
}

func (r *postgresRatingRepository) HasConfirmedHappened(ctx context.Context, meetupID, userID string) (bool, error) {
	meetup, err := parseUUID(meetupID)
	if err != nil {
		return false, fmt.Errorf("repository: invalid meetup id %q: %w", meetupID, apperror.ErrInvalidInput)
	}
	user, err := parseUUID(userID)
	if err != nil {
		return false, fmt.Errorf("repository: invalid user id %q: %w", userID, apperror.ErrInvalidInput)
	}

	ok, err := r.q.HasConfirmedMeetupHappened(ctx, sqlcgen.HasConfirmedMeetupHappenedParams{MeetupID: meetup, UserID: user})
	if err != nil {
		return false, fmt.Errorf("repository: check confirmed attendance: %w", err)
	}
	return ok, nil
}

func (r *postgresRatingRepository) ListRatable(ctx context.Context, meetupID, viewerID string) ([]RatableParticipant, error) {
	meetup, err := parseUUID(meetupID)
	if err != nil {
		return nil, fmt.Errorf("repository: invalid meetup id %q: %w", meetupID, apperror.ErrInvalidInput)
	}
	viewer, err := parseUUID(viewerID)
	if err != nil {
		return nil, fmt.Errorf("repository: invalid viewer id %q: %w", viewerID, apperror.ErrInvalidInput)
	}

	rows, err := r.q.ListRatableParticipants(ctx, sqlcgen.ListRatableParticipantsParams{MeetupID: meetup, ViewerID: viewer})
	if err != nil {
		return nil, fmt.Errorf("repository: list ratable participants: %w", err)
	}

	participants := make([]RatableParticipant, 0, len(rows))
	for _, row := range rows {
		participants = append(participants, RatableParticipant{
			UserID:          row.ID.String(),
			FullName:        row.FullName,
			ProfilePhotoURL: textOrEmpty(row.ProfilePhotoUrl),
			TrustLevel:      int(row.TrustLevel),
			AlreadyRated:    row.AlreadyRated,
		})
	}
	return participants, nil
}

func (r *postgresRatingRepository) Submit(ctx context.Context, meetupID, raterID, ratedID string, score int) error {
	meetup, err := parseUUID(meetupID)
	if err != nil {
		return fmt.Errorf("repository: invalid meetup id %q: %w", meetupID, apperror.ErrInvalidInput)
	}
	rater, err := parseUUID(raterID)
	if err != nil {
		return fmt.Errorf("repository: invalid rater id %q: %w", raterID, apperror.ErrInvalidInput)
	}
	rated, err := parseUUID(ratedID)
	if err != nil {
		return fmt.Errorf("repository: invalid rated id %q: %w", ratedID, apperror.ErrInvalidInput)
	}

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("repository: begin submit rating transaction: %w: %w", apperror.ErrInternal, err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	q := r.q.WithTx(tx)

	if _, err := q.CreateMeetupRating(ctx, sqlcgen.CreateMeetupRatingParams{
		MeetupID:    meetup,
		RaterUserID: rater,
		RatedUserID: rated,
		Score:       int16(score),
	}); err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			switch pgErr.Code {
			case pgUniqueViolation:
				return fmt.Errorf("repository: already rated this participant for this meetup: %w", apperror.ErrConflict)
			case pgCheckViolation:
				return fmt.Errorf("repository: invalid rating: %w", apperror.ErrInvalidInput)
			}
		}
		return fmt.Errorf("repository: create meetup rating: %w", err)
	}

	if err := q.RecomputeUserRating(ctx, rated); err != nil {
		return fmt.Errorf("repository: recompute user rating: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("repository: commit submit rating transaction: %w: %w", apperror.ErrInternal, err)
	}
	return nil
}
