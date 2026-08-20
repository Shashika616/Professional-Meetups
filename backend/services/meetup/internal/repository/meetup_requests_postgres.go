package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/professional-connections/backend/services/meetup/internal/repository/sqlcgen"
	"github.com/professional-connections/backend/shared/apperror"
)

// pgUniqueViolation is Postgres's error code for a UNIQUE constraint
// violation (23505) — mirrors services/auth's users_postgres.go conflict
// handling.
const pgUniqueViolation = "23505"

type postgresMeetupRequestRepository struct {
	// pool (not just *sqlcgen.Queries) — Accept needs to start its own
	// transaction spanning both meetup_requests and meetups, which a
	// plain Queries wrapping the pool alone can't do (backend/meetup-
	// scheduling-PLAN.md Step B).
	pool *pgxpool.Pool
	q    *sqlcgen.Queries
}

// NewMeetupRequestRepository constructs a MeetupRequestRepository backed by
// pool.
func NewMeetupRequestRepository(pool *pgxpool.Pool) MeetupRequestRepository {
	return &postgresMeetupRequestRepository{pool: pool, q: sqlcgen.New(pool)}
}

func (r *postgresMeetupRequestRepository) Create(ctx context.Context, meetupID, requesterID string) (MeetupRequest, error) {
	meetup, err := parseUUID(meetupID)
	if err != nil {
		return MeetupRequest{}, fmt.Errorf("repository: invalid meetup id %q: %w", meetupID, apperror.ErrInvalidInput)
	}
	requester, err := parseUUID(requesterID)
	if err != nil {
		return MeetupRequest{}, fmt.Errorf("repository: invalid requester id %q: %w", requesterID, apperror.ErrInvalidInput)
	}

	row, err := r.q.CreateMeetupRequest(ctx, sqlcgen.CreateMeetupRequestParams{MeetupID: meetup, RequesterID: requester})
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == pgUniqueViolation {
			return MeetupRequest{}, fmt.Errorf("repository: already requested to join this meetup: %w", apperror.ErrConflict)
		}
		return MeetupRequest{}, fmt.Errorf("repository: create meetup request: %w", err)
	}

	return meetupRequestFromRow(row), nil
}

func (r *postgresMeetupRequestRepository) GetByID(ctx context.Context, id string) (MeetupRequest, error) {
	parsed, err := parseUUID(id)
	if err != nil {
		return MeetupRequest{}, fmt.Errorf("repository: invalid request id %q: %w", id, apperror.ErrInvalidInput)
	}

	row, err := r.q.GetMeetupRequestWithRequesterInfoByID(ctx, parsed)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return MeetupRequest{}, fmt.Errorf("repository: meetup request %s: %w", id, apperror.ErrNotFound)
		}
		return MeetupRequest{}, fmt.Errorf("repository: get meetup request: %w", err)
	}
	return MeetupRequest{
		ID:                       row.ID.String(),
		MeetupID:                 row.MeetupID.String(),
		RequesterID:              row.RequesterID.String(),
		RequesterFullName:        row.RequesterFullName,
		RequesterProfilePhotoURL: textOrEmpty(row.RequesterProfilePhotoUrl),
		RequesterTrustLevel:      int(row.RequesterTrustLevel),
		RequesterRatingAverage:   numericToFloat64(row.RequesterRatingAverage),
		RequesterRatingCount:     int(row.RequesterRatingCount),
		Status:                   MeetupRequestStatus(row.Status),
		AutoRejected:             row.AutoRejected,
		CreatedAt:                timestamptzOrZero(row.CreatedAt),
		ResolvedAt:               timePtrOrNil(row.ResolvedAt),
	}, nil
}

func (r *postgresMeetupRequestRepository) ListForMeetup(ctx context.Context, meetupID string) ([]MeetupRequest, error) {
	meetup, err := parseUUID(meetupID)
	if err != nil {
		return nil, fmt.Errorf("repository: invalid meetup id %q: %w", meetupID, apperror.ErrInvalidInput)
	}

	rows, err := r.q.ListRequestsForMeetup(ctx, meetup)
	if err != nil {
		return nil, fmt.Errorf("repository: list requests for meetup: %w", err)
	}

	requests := make([]MeetupRequest, 0, len(rows))
	for _, row := range rows {
		requests = append(requests, MeetupRequest{
			ID:                       row.ID.String(),
			MeetupID:                 row.MeetupID.String(),
			RequesterID:              row.RequesterID.String(),
			RequesterFullName:        row.RequesterFullName,
			RequesterProfilePhotoURL: textOrEmpty(row.RequesterProfilePhotoUrl),
			RequesterTrustLevel:      int(row.RequesterTrustLevel),
			RequesterRatingAverage:   numericToFloat64(row.RequesterRatingAverage),
			RequesterRatingCount:     int(row.RequesterRatingCount),
			Status:                   MeetupRequestStatus(row.Status),
			AutoRejected:             row.AutoRejected,
			CreatedAt:                timestamptzOrZero(row.CreatedAt),
			ResolvedAt:               timePtrOrNil(row.ResolvedAt),
		})
	}
	return requests, nil
}

func (r *postgresMeetupRequestRepository) Withdraw(ctx context.Context, id string) (MeetupRequest, error) {
	parsed, err := parseUUID(id)
	if err != nil {
		return MeetupRequest{}, fmt.Errorf("repository: invalid request id %q: %w", id, apperror.ErrInvalidInput)
	}

	row, err := r.q.WithdrawMeetupRequest(ctx, parsed)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return MeetupRequest{}, fmt.Errorf("repository: request %s is not pending: %w", id, apperror.ErrConflict)
		}
		return MeetupRequest{}, fmt.Errorf("repository: withdraw meetup request: %w", err)
	}
	return meetupRequestFromRow(row), nil
}

func (r *postgresMeetupRequestRepository) Reject(ctx context.Context, id string) (MeetupRequest, error) {
	parsed, err := parseUUID(id)
	if err != nil {
		return MeetupRequest{}, fmt.Errorf("repository: invalid request id %q: %w", id, apperror.ErrInvalidInput)
	}

	row, err := r.q.RejectMeetupRequest(ctx, parsed)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return MeetupRequest{}, fmt.Errorf("repository: request %s is not pending: %w", id, apperror.ErrConflict)
		}
		return MeetupRequest{}, fmt.Errorf("repository: reject meetup request: %w", err)
	}
	return meetupRequestFromRow(row), nil
}

func (r *postgresMeetupRequestRepository) Accept(
	ctx context.Context, id string,
) (accepted MeetupRequest, meetupNowFull bool, autoRejected []MeetupRequest, err error) {
	requestID, err := parseUUID(id)
	if err != nil {
		return MeetupRequest{}, false, nil, fmt.Errorf("repository: invalid request id %q: %w", id, apperror.ErrInvalidInput)
	}

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return MeetupRequest{}, false, nil, fmt.Errorf("repository: begin accept transaction: %w: %w", apperror.ErrInternal, err)
	}
	// Rollback is a no-op if Commit already succeeded — this is just the
	// safety net for every early-return error path below.
	defer func() { _ = tx.Rollback(ctx) }()

	q := r.q.WithTx(tx)

	reqRow, err := q.GetMeetupRequestByID(ctx, requestID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return MeetupRequest{}, false, nil, fmt.Errorf("repository: meetup request %s: %w", id, apperror.ErrNotFound)
		}
		return MeetupRequest{}, false, nil, fmt.Errorf("repository: get request for accept: %w", err)
	}
	if reqRow.Status != sqlcgen.MeetupRequestStatusPending {
		return MeetupRequest{}, false, nil, fmt.Errorf("repository: request %s is not pending: %w", id, apperror.ErrConflict)
	}

	// SELECT ... FOR UPDATE locks this meetup row until the transaction
	// commits or rolls back — a second, concurrent Accept call against the
	// same meetup blocks here until this one finishes, then re-reads the
	// now-current status/capacity rather than racing against a stale read
	// (backend/meetup-scheduling-PLAN.md Step B, the capacity-race
	// integration test exercises exactly this).
	meetupRow, err := q.GetMeetupByIDForUpdate(ctx, reqRow.MeetupID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return MeetupRequest{}, false, nil, fmt.Errorf("repository: meetup %s: %w", reqRow.MeetupID, apperror.ErrNotFound)
		}
		return MeetupRequest{}, false, nil, fmt.Errorf("repository: lock meetup for accept: %w", err)
	}
	if meetupRow.Status != sqlcgen.MeetupStatusOpen {
		return MeetupRequest{}, false, nil, fmt.Errorf("repository: meetup %s is not open: %w", reqRow.MeetupID, apperror.ErrConflict)
	}

	acceptedCount, err := q.CountAcceptedRequests(ctx, reqRow.MeetupID)
	if err != nil {
		return MeetupRequest{}, false, nil, fmt.Errorf("repository: count accepted requests: %w", err)
	}
	if acceptedCount >= int64(meetupRow.Capacity) {
		// Shouldn't normally be reachable — the meetup's status should have
		// already flipped to 'full' the moment capacity was reached — but
		// checked defensively under the same lock rather than trusting that
		// invariant blindly.
		return MeetupRequest{}, false, nil, fmt.Errorf("repository: meetup %s is full: %w", reqRow.MeetupID, apperror.ErrConflict)
	}

	acceptedRow, err := q.AcceptMeetupRequest(ctx, requestID)
	if err != nil {
		return MeetupRequest{}, false, nil, fmt.Errorf("repository: accept meetup request: %w", err)
	}

	newCount := acceptedCount + 1
	var autoRejectedRows []sqlcgen.MeetupRequest
	if newCount >= int64(meetupRow.Capacity) {
		if err := q.MarkMeetupFull(ctx, meetupRow.ID); err != nil {
			return MeetupRequest{}, false, nil, fmt.Errorf("repository: mark meetup full: %w", err)
		}
		autoRejectedRows, err = q.AutoRejectPendingRequestsForMeetup(ctx, meetupRow.ID)
		if err != nil {
			return MeetupRequest{}, false, nil, fmt.Errorf("repository: auto-reject pending requests: %w", err)
		}
		meetupNowFull = true
	}

	if err := tx.Commit(ctx); err != nil {
		return MeetupRequest{}, false, nil, fmt.Errorf("repository: commit accept transaction: %w: %w", apperror.ErrInternal, err)
	}

	autoRejected = make([]MeetupRequest, 0, len(autoRejectedRows))
	for _, row := range autoRejectedRows {
		autoRejected = append(autoRejected, meetupRequestFromRow(row))
	}

	return meetupRequestFromRow(acceptedRow), meetupNowFull, autoRejected, nil
}

// meetupRequestFromRow converts a plain (non-joined) sqlcgen.MeetupRequest —
// the shape every write query in this file returns via RETURNING — leaving
// RequesterFullName/RequesterProfilePhotoURL/RequesterTrustLevel blank.
// Callers that need those populated (an RPC response) should re-fetch via
// ListForMeetup or the caller's own join, same "write queries don't join"
// pattern as meetups_postgres.go's Create.
func meetupRequestFromRow(row sqlcgen.MeetupRequest) MeetupRequest {
	return MeetupRequest{
		ID:           row.ID.String(),
		MeetupID:     row.MeetupID.String(),
		RequesterID:  row.RequesterID.String(),
		Status:       MeetupRequestStatus(row.Status),
		AutoRejected: row.AutoRejected,
		CreatedAt:    timestamptzOrZero(row.CreatedAt),
		ResolvedAt:   timePtrOrNil(row.ResolvedAt),
	}
}
