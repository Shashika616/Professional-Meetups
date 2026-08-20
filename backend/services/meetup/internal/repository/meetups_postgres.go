package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/professional-connections/backend/services/meetup/internal/repository/sqlcgen"
	"github.com/professional-connections/backend/shared/apperror"
)

// defaultPageSize/maxPageSize bound ListOpen's page_size — a client-supplied
// value outside this range is clamped, not rejected, since it's a UX
// parameter, not a security boundary.
const (
	defaultPageSize = 20
	maxPageSize     = 50
)

type postgresMeetupRepository struct {
	q *sqlcgen.Queries
}

// NewMeetupRepository constructs a MeetupRepository backed by pool.
func NewMeetupRepository(pool *pgxpool.Pool) MeetupRepository {
	return &postgresMeetupRepository{q: sqlcgen.New(pool)}
}

// Create does not populate HostFullName/HostProfilePhotoURL/HostTrustLevel/
// AcceptedCount on the returned Meetup — the underlying INSERT has nothing
// to join against yet. Callers that need the fully-populated view (e.g. the
// service layer building a CreateMeetup RPC response) should follow up with
// GetByID.
func (r *postgresMeetupRepository) Create(ctx context.Context, m NewMeetup) (Meetup, error) {
	hostID, err := parseUUID(m.HostUserID)
	if err != nil {
		return Meetup{}, fmt.Errorf("repository: invalid host user id %q: %w", m.HostUserID, apperror.ErrInvalidInput)
	}

	row, err := r.q.CreateMeetup(ctx, sqlcgen.CreateMeetupParams{
		HostUserID:    hostID,
		Intent:        sqlcgen.IntentType(m.Intent),
		WindowStart:   toTimestamptz(m.WindowStart),
		WindowEnd:     toTimestamptz(m.WindowEnd),
		LocationLat:   m.LocationLat,
		LocationLng:   m.LocationLng,
		LocationLabel: m.LocationLabel,
		Capacity:      int16(m.Capacity),
	})
	if err != nil {
		return Meetup{}, fmt.Errorf("repository: create meetup: %w", err)
	}

	return Meetup{
		ID:            row.ID.String(),
		HostUserID:    row.HostUserID.String(),
		Intent:        Intent(row.Intent),
		WindowStart:   timestamptzOrZero(row.WindowStart),
		WindowEnd:     timestamptzOrZero(row.WindowEnd),
		LocationLat:   row.LocationLat,
		LocationLng:   row.LocationLng,
		LocationLabel: row.LocationLabel,
		Capacity:      int(row.Capacity),
		Status:        MeetupStatus(row.Status),
		CreatedAt:     timestamptzOrZero(row.CreatedAt),
		CancelledAt:   timePtrOrNil(row.CancelledAt),
		ClosedAt:      timePtrOrNil(row.ClosedAt),
	}, nil
}

func (r *postgresMeetupRepository) GetByID(ctx context.Context, id, viewerID string) (Meetup, error) {
	meetupID, err := parseUUID(id)
	if err != nil {
		return Meetup{}, fmt.Errorf("repository: invalid meetup id %q: %w", id, apperror.ErrInvalidInput)
	}
	viewer, err := parseUUID(viewerID)
	if err != nil {
		return Meetup{}, fmt.Errorf("repository: invalid viewer id %q: %w", viewerID, apperror.ErrInvalidInput)
	}

	row, err := r.q.GetMeetupByID(ctx, sqlcgen.GetMeetupByIDParams{ID: meetupID, RequesterID: viewer})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Meetup{}, fmt.Errorf("repository: meetup %s: %w", id, apperror.ErrNotFound)
		}
		return Meetup{}, fmt.Errorf("repository: get meetup: %w", err)
	}

	return Meetup{
		ID:                  row.ID.String(),
		HostUserID:          row.HostUserID.String(),
		HostFullName:        row.HostFullName,
		HostProfilePhotoURL: textOrEmpty(row.HostProfilePhotoUrl),
		HostTrustLevel:      int(row.HostTrustLevel),
		HostRatingAverage:   numericToFloat64(row.HostRatingAverage),
		HostRatingCount:     int(row.HostRatingCount),
		Intent:              Intent(row.Intent),
		WindowStart:         timestamptzOrZero(row.WindowStart),
		WindowEnd:           timestamptzOrZero(row.WindowEnd),
		LocationLat:         row.LocationLat,
		LocationLng:         row.LocationLng,
		LocationLabel:       row.LocationLabel,
		Capacity:            int(row.Capacity),
		AcceptedCount:       int(row.AcceptedCount),
		Status:              MeetupStatus(row.Status),
		CreatedAt:           timestamptzOrZero(row.CreatedAt),
		CancelledAt:         timePtrOrNil(row.CancelledAt),
		ClosedAt:            timePtrOrNil(row.ClosedAt),
		MyRequestStatus:     requestStatusPtrOrNil(row.MyRequestStatus),
	}, nil
}

func (r *postgresMeetupRepository) ListOpen(
	ctx context.Context, intent Intent, viewerID string, cursor *Cursor, pageSize int,
) ([]Meetup, *Cursor, error) {
	viewer, err := parseUUID(viewerID)
	if err != nil {
		return nil, nil, fmt.Errorf("repository: invalid viewer id %q: %w", viewerID, apperror.ErrInvalidInput)
	}

	limit := int32(pageSize)
	if limit <= 0 {
		limit = defaultPageSize
	} else if limit > maxPageSize {
		limit = maxPageSize
	}
	// Fetch one extra row to know whether there's a next page, without a
	// separate count query.
	fetchLimit := limit + 1

	var meetups []Meetup
	if cursor == nil {
		rows, err := r.q.ListOpenMeetupsByIntentFirstPage(ctx, sqlcgen.ListOpenMeetupsByIntentFirstPageParams{
			Intent:      sqlcgen.IntentType(intent),
			RequesterID: viewer,
			Limit:       fetchLimit,
		})
		if err != nil {
			return nil, nil, fmt.Errorf("repository: list open meetups: %w", err)
		}
		for _, row := range rows {
			meetups = append(meetups, Meetup{
				ID:                  row.ID.String(),
				HostUserID:          row.HostUserID.String(),
				HostFullName:        row.HostFullName,
				HostProfilePhotoURL: textOrEmpty(row.HostProfilePhotoUrl),
				HostTrustLevel:      int(row.HostTrustLevel),
				HostRatingAverage:   numericToFloat64(row.HostRatingAverage),
				HostRatingCount:     int(row.HostRatingCount),
				Intent:              Intent(row.Intent),
				WindowStart:         timestamptzOrZero(row.WindowStart),
				WindowEnd:           timestamptzOrZero(row.WindowEnd),
				LocationLat:         row.LocationLat,
				LocationLng:         row.LocationLng,
				LocationLabel:       row.LocationLabel,
				Capacity:            int(row.Capacity),
				AcceptedCount:       int(row.AcceptedCount),
				Status:              MeetupStatus(row.Status),
				CreatedAt:           timestamptzOrZero(row.CreatedAt),
				CancelledAt:         timePtrOrNil(row.CancelledAt),
				ClosedAt:            timePtrOrNil(row.ClosedAt),
				MyRequestStatus:     requestStatusPtrOrNil(row.MyRequestStatus),
			})
		}
	} else {
		rows, err := r.q.ListOpenMeetupsByIntentAfterCursor(ctx, sqlcgen.ListOpenMeetupsByIntentAfterCursorParams{
			Intent:          sqlcgen.IntentType(intent),
			RequesterID:     viewer,
			Limit:           fetchLimit,
			CursorCreatedAt: toTimestamptz(cursor.CreatedAt),
			CursorID:        mustParseUUID(cursor.ID),
		})
		if err != nil {
			return nil, nil, fmt.Errorf("repository: list open meetups after cursor: %w", err)
		}
		for _, row := range rows {
			meetups = append(meetups, Meetup{
				ID:                  row.ID.String(),
				HostUserID:          row.HostUserID.String(),
				HostFullName:        row.HostFullName,
				HostProfilePhotoURL: textOrEmpty(row.HostProfilePhotoUrl),
				HostTrustLevel:      int(row.HostTrustLevel),
				HostRatingAverage:   numericToFloat64(row.HostRatingAverage),
				HostRatingCount:     int(row.HostRatingCount),
				Intent:              Intent(row.Intent),
				WindowStart:         timestamptzOrZero(row.WindowStart),
				WindowEnd:           timestamptzOrZero(row.WindowEnd),
				LocationLat:         row.LocationLat,
				LocationLng:         row.LocationLng,
				LocationLabel:       row.LocationLabel,
				Capacity:            int(row.Capacity),
				AcceptedCount:       int(row.AcceptedCount),
				Status:              MeetupStatus(row.Status),
				CreatedAt:           timestamptzOrZero(row.CreatedAt),
				CancelledAt:         timePtrOrNil(row.CancelledAt),
				ClosedAt:            timePtrOrNil(row.ClosedAt),
				MyRequestStatus:     requestStatusPtrOrNil(row.MyRequestStatus),
			})
		}
	}

	var next *Cursor
	if len(meetups) > int(limit) {
		last := meetups[limit-1]
		next = &Cursor{CreatedAt: last.CreatedAt, ID: last.ID}
		meetups = meetups[:limit]
	}

	return meetups, next, nil
}

func (r *postgresMeetupRepository) ListByHost(ctx context.Context, hostID string) ([]Meetup, error) {
	host, err := parseUUID(hostID)
	if err != nil {
		return nil, fmt.Errorf("repository: invalid host id %q: %w", hostID, apperror.ErrInvalidInput)
	}

	rows, err := r.q.ListMeetupsByHost(ctx, host)
	if err != nil {
		return nil, fmt.Errorf("repository: list meetups by host: %w", err)
	}

	meetups := make([]Meetup, 0, len(rows))
	for _, row := range rows {
		meetups = append(meetups, Meetup{
			ID:                  row.ID.String(),
			HostUserID:          row.HostUserID.String(),
			HostFullName:        row.HostFullName,
			HostProfilePhotoURL: textOrEmpty(row.HostProfilePhotoUrl),
			HostTrustLevel:      int(row.HostTrustLevel),
			HostRatingAverage:   numericToFloat64(row.HostRatingAverage),
			HostRatingCount:     int(row.HostRatingCount),
			Intent:              Intent(row.Intent),
			WindowStart:         timestamptzOrZero(row.WindowStart),
			WindowEnd:           timestamptzOrZero(row.WindowEnd),
			LocationLat:         row.LocationLat,
			LocationLng:         row.LocationLng,
			LocationLabel:       row.LocationLabel,
			Capacity:            int(row.Capacity),
			AcceptedCount:       int(row.AcceptedCount),
			Status:              MeetupStatus(row.Status),
			CreatedAt:           timestamptzOrZero(row.CreatedAt),
			CancelledAt:         timePtrOrNil(row.CancelledAt),
			ClosedAt:            timePtrOrNil(row.ClosedAt),
		})
	}
	return meetups, nil
}

func (r *postgresMeetupRepository) ListRequestedByUser(ctx context.Context, userID string) ([]Meetup, error) {
	requester, err := parseUUID(userID)
	if err != nil {
		return nil, fmt.Errorf("repository: invalid user id %q: %w", userID, apperror.ErrInvalidInput)
	}

	rows, err := r.q.ListMeetupsRequestedByUser(ctx, requester)
	if err != nil {
		return nil, fmt.Errorf("repository: list meetups requested by user: %w", err)
	}

	meetups := make([]Meetup, 0, len(rows))
	for _, row := range rows {
		status := MeetupRequestStatus(row.MyRequestStatus)
		meetups = append(meetups, Meetup{
			ID:                    row.ID.String(),
			HostUserID:            row.HostUserID.String(),
			HostFullName:          row.HostFullName,
			HostProfilePhotoURL:   textOrEmpty(row.HostProfilePhotoUrl),
			HostTrustLevel:        int(row.HostTrustLevel),
			HostRatingAverage:     numericToFloat64(row.HostRatingAverage),
			HostRatingCount:       int(row.HostRatingCount),
			Intent:                Intent(row.Intent),
			WindowStart:           timestamptzOrZero(row.WindowStart),
			WindowEnd:             timestamptzOrZero(row.WindowEnd),
			LocationLat:           row.LocationLat,
			LocationLng:           row.LocationLng,
			LocationLabel:         row.LocationLabel,
			Capacity:              int(row.Capacity),
			AcceptedCount:         int(row.AcceptedCount),
			Status:                MeetupStatus(row.Status),
			CreatedAt:             timestamptzOrZero(row.CreatedAt),
			CancelledAt:           timePtrOrNil(row.CancelledAt),
			ClosedAt:              timePtrOrNil(row.ClosedAt),
			MyRequestStatus:       &status,
			MyRequestAutoRejected: row.MyRequestAutoRejected,
		})
	}
	return meetups, nil
}

func (r *postgresMeetupRepository) Cancel(ctx context.Context, id string) (Meetup, error) {
	meetupID, err := parseUUID(id)
	if err != nil {
		return Meetup{}, fmt.Errorf("repository: invalid meetup id %q: %w", id, apperror.ErrInvalidInput)
	}

	row, err := r.q.CancelMeetup(ctx, meetupID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Meetup{}, fmt.Errorf("repository: meetup %s: %w", id, apperror.ErrNotFound)
		}
		return Meetup{}, fmt.Errorf("repository: cancel meetup: %w", err)
	}

	return Meetup{
		ID:            row.ID.String(),
		HostUserID:    row.HostUserID.String(),
		Intent:        Intent(row.Intent),
		WindowStart:   timestamptzOrZero(row.WindowStart),
		WindowEnd:     timestamptzOrZero(row.WindowEnd),
		LocationLat:   row.LocationLat,
		LocationLng:   row.LocationLng,
		LocationLabel: row.LocationLabel,
		Capacity:      int(row.Capacity),
		Status:        MeetupStatus(row.Status),
		CreatedAt:     timestamptzOrZero(row.CreatedAt),
		CancelledAt:   timePtrOrNil(row.CancelledAt),
		ClosedAt:      timePtrOrNil(row.ClosedAt),
	}, nil
}

// Close returns apperror.ErrNotFound (wrapped) if zero rows matched — the
// service layer re-fetches via GetByID to distinguish *why* (wrong host,
// already closed/cancelled, window not started yet) for a useful error
// message, same "query encodes the whole precondition check" shape as
// RejectMeetupRequest's zero-rows-means-ErrConflict pattern elsewhere in
// this package (ADR-016).
func (r *postgresMeetupRepository) Close(ctx context.Context, id, hostUserID string) (Meetup, error) {
	meetupID, err := parseUUID(id)
	if err != nil {
		return Meetup{}, fmt.Errorf("repository: invalid meetup id %q: %w", id, apperror.ErrInvalidInput)
	}
	host, err := parseUUID(hostUserID)
	if err != nil {
		return Meetup{}, fmt.Errorf("repository: invalid host user id %q: %w", hostUserID, apperror.ErrInvalidInput)
	}

	row, err := r.q.CloseMeetup(ctx, sqlcgen.CloseMeetupParams{ID: meetupID, HostUserID: host})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Meetup{}, fmt.Errorf("repository: meetup %s: %w", id, apperror.ErrNotFound)
		}
		return Meetup{}, fmt.Errorf("repository: close meetup: %w", err)
	}

	return Meetup{
		ID:            row.ID.String(),
		HostUserID:    row.HostUserID.String(),
		Intent:        Intent(row.Intent),
		WindowStart:   timestamptzOrZero(row.WindowStart),
		WindowEnd:     timestamptzOrZero(row.WindowEnd),
		LocationLat:   row.LocationLat,
		LocationLng:   row.LocationLng,
		LocationLabel: row.LocationLabel,
		Capacity:      int(row.Capacity),
		Status:        MeetupStatus(row.Status),
		CreatedAt:     timestamptzOrZero(row.CreatedAt),
		CancelledAt:   timePtrOrNil(row.CancelledAt),
		ClosedAt:      timePtrOrNil(row.ClosedAt),
	}, nil
}

func mustParseUUID(id string) uuid.UUID {
	parsed, err := parseUUID(id)
	if err != nil {
		// Cursor.ID is only ever constructed by this package from a real
		// database row's id (see ListOpen above) — a parse failure here
		// means a caller fabricated a cursor by hand, which is a
		// programming error, not a runtime condition to recover from.
		panic(fmt.Sprintf("repository: cursor id %q is not a valid uuid: %v", id, err))
	}
	return parsed
}
