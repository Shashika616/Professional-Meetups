package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/professional-connections/backend/services/meetup/internal/repository/sqlcgen"
	"github.com/professional-connections/backend/shared/apperror"
)

type postgresSafetyStateRepository struct {
	q *sqlcgen.Queries
}

// NewSafetyStateRepository constructs a SafetyStateRepository backed by
// pool.
func NewSafetyStateRepository(pool *pgxpool.Pool) SafetyStateRepository {
	return &postgresSafetyStateRepository{q: sqlcgen.New(pool)}
}

func (r *postgresSafetyStateRepository) EnsureExists(ctx context.Context, meetupID string) error {
	id, err := parseUUID(meetupID)
	if err != nil {
		return fmt.Errorf("repository: invalid meetup id %q: %w", meetupID, apperror.ErrInvalidInput)
	}
	if err := r.q.EnsureSafetyState(ctx, id); err != nil {
		return fmt.Errorf("repository: ensure safety state: %w", err)
	}
	return nil
}

func (r *postgresSafetyStateRepository) Get(ctx context.Context, meetupID string) (SafetyState, error) {
	id, err := parseUUID(meetupID)
	if err != nil {
		return SafetyState{}, fmt.Errorf("repository: invalid meetup id %q: %w", meetupID, apperror.ErrInvalidInput)
	}
	row, err := r.q.GetSafetyState(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return SafetyState{}, fmt.Errorf("repository: no safety state for meetup %s: %w", meetupID, apperror.ErrNotFound)
		}
		return SafetyState{}, fmt.Errorf("repository: get safety state: %w", err)
	}
	return safetyStateFromRow(row), nil
}

func (r *postgresSafetyStateRepository) AcknowledgeChecklist(ctx context.Context, meetupID string) (SafetyState, error) {
	id, err := parseUUID(meetupID)
	if err != nil {
		return SafetyState{}, fmt.Errorf("repository: invalid meetup id %q: %w", meetupID, apperror.ErrInvalidInput)
	}
	row, err := r.q.SetChecklistAck(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return SafetyState{}, fmt.Errorf("repository: no safety state for meetup %s: %w", meetupID, apperror.ErrNotFound)
		}
		return SafetyState{}, fmt.Errorf("repository: acknowledge checklist: %w", err)
	}
	return safetyStateFromRow(row), nil
}

func (r *postgresSafetyStateRepository) SetLiveLocationOptIn(ctx context.Context, meetupID string, optIn bool) (SafetyState, error) {
	id, err := parseUUID(meetupID)
	if err != nil {
		return SafetyState{}, fmt.Errorf("repository: invalid meetup id %q: %w", meetupID, apperror.ErrInvalidInput)
	}
	row, err := r.q.SetLiveLocationOptIn(ctx, sqlcgen.SetLiveLocationOptInParams{MeetupID: id, LiveLocationOptIn: optIn})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return SafetyState{}, fmt.Errorf("repository: no safety state for meetup %s: %w", meetupID, apperror.ErrNotFound)
		}
		return SafetyState{}, fmt.Errorf("repository: set live location opt-in: %w", err)
	}
	return safetyStateFromRow(row), nil
}

func (r *postgresSafetyStateRepository) CheckIn(ctx context.Context, meetupID string) (SafetyState, error) {
	id, err := parseUUID(meetupID)
	if err != nil {
		return SafetyState{}, fmt.Errorf("repository: invalid meetup id %q: %w", meetupID, apperror.ErrInvalidInput)
	}
	row, err := r.q.SetCheckedIn(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return SafetyState{}, fmt.Errorf("repository: no safety state for meetup %s: %w", meetupID, apperror.ErrNotFound)
		}
		return SafetyState{}, fmt.Errorf("repository: check in: %w", err)
	}
	return safetyStateFromRow(row), nil
}

func safetyStateFromRow(row sqlcgen.MeetupSafetyState) SafetyState {
	return SafetyState{
		MeetupID:          row.MeetupID.String(),
		ChecklistAckAt:    timePtrOrNil(row.ChecklistAckAt),
		LiveLocationOptIn: row.LiveLocationOptIn,
		CheckedInAt:       timePtrOrNil(row.CheckedInAt),
	}
}
