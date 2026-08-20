// Package service implements the generated MeetupServiceServer gRPC
// interface: host-initiated meetup scheduling, join requests, the Safety
// Gate sub-flow, and event publishing for push notifications (ADR-013).
package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/professional-connections/backend/services/meetup/internal/events"
	"github.com/professional-connections/backend/services/meetup/internal/notifications"
	"github.com/professional-connections/backend/services/meetup/internal/repository"
	"github.com/professional-connections/backend/shared/apperror"
	meetupv1 "github.com/professional-connections/backend/shared/proto/meetup/v1"
)

// windowStartGracePeriod tolerates clock skew and normal request latency
// between when a client picks "now" as a window start and when the request
// actually arrives — anything staler than this (e.g. a window left over
// from a stale form, or literally yesterday) is rejected rather than
// silently accepted (ADR-016).
const windowStartGracePeriod = 5 * time.Minute

// Service implements meetupv1.MeetupServiceServer. Every dependency is
// passed explicitly via New — no framework, no globals.
type Service struct {
	meetupv1.UnimplementedMeetupServiceServer

	meetups       repository.MeetupRepository
	requests      repository.MeetupRequestRepository
	safetyState   repository.SafetyStateRepository
	feedback      repository.FeedbackRepository
	ratings       repository.RatingRepository
	deviceTokens  repository.DeviceTokenRepository
	events        events.Publisher
	notifications notifications.Sender
}

// New constructs a Service.
func New(
	meetups repository.MeetupRepository,
	requests repository.MeetupRequestRepository,
	safetyState repository.SafetyStateRepository,
	feedback repository.FeedbackRepository,
	ratings repository.RatingRepository,
	deviceTokens repository.DeviceTokenRepository,
	publisher events.Publisher,
	notificationSender notifications.Sender,
) *Service {
	return &Service{
		meetups:       meetups,
		requests:      requests,
		safetyState:   safetyState,
		feedback:      feedback,
		ratings:       ratings,
		deviceTokens:  deviceTokens,
		events:        publisher,
		notifications: notificationSender,
	}
}

func (s *Service) CreateMeetup(ctx context.Context, req *meetupv1.CreateMeetupRequest) (*meetupv1.MeetupResponse, error) {
	intent, ok := intentFromProto[req.GetIntent()]
	if !ok {
		return nil, apperror.ToGRPCStatus(fmt.Errorf("meetup: unrecognized intent: %w", apperror.ErrInvalidInput))
	}

	if err := checkTrustLevel(intent, int(req.GetHostTrustLevel())); err != nil {
		return nil, apperror.ToGRPCStatus(err)
	}

	if req.GetCapacity() < 1 || req.GetCapacity() > 20 {
		return nil, apperror.ToGRPCStatus(fmt.Errorf("meetup: capacity must be between 1 and 20: %w", apperror.ErrInvalidInput))
	}

	windowStart := timeFromUnixSeconds(req.GetWindowStartUnixSeconds())
	windowEnd := timeFromUnixSeconds(req.GetWindowEndUnixSeconds())
	// Defense in depth — the DB's CHECK(window_end > window_start) is the
	// backstop, not the only check (ADR-016).
	if !windowEnd.After(windowStart) {
		return nil, apperror.ToGRPCStatus(fmt.Errorf("meetup: window_end must be after window_start: %w", apperror.ErrInvalidInput))
	}
	if windowStart.Before(time.Now().Add(-windowStartGracePeriod)) {
		return nil, apperror.ToGRPCStatus(fmt.Errorf("meetup: window_start can't be in the past: %w", apperror.ErrInvalidInput))
	}

	created, err := s.meetups.Create(ctx, repository.NewMeetup{
		HostUserID:    req.GetHostUserId(),
		Intent:        intent,
		WindowStart:   windowStart,
		WindowEnd:     windowEnd,
		LocationLat:   req.GetLocationLat(),
		LocationLng:   req.GetLocationLng(),
		LocationLabel: req.GetLocationLabel(),
		Capacity:      int(req.GetCapacity()),
	})
	if err != nil {
		return nil, apperror.ToGRPCStatus(err)
	}

	// Create doesn't join against users for host display info (nothing to
	// join against for a brand-new row's host — it's the caller
	// themselves) — GetByID re-fetches the fully-populated view for the
	// response.
	full, err := s.meetups.GetByID(ctx, created.ID, req.GetHostUserId())
	if err != nil {
		return nil, apperror.ToGRPCStatus(err)
	}

	return meetupToProto(full, req.GetHostUserId()), nil
}

func (s *Service) GetMeetup(ctx context.Context, req *meetupv1.GetMeetupRequest) (*meetupv1.MeetupResponse, error) {
	m, err := s.meetups.GetByID(ctx, req.GetMeetupId(), req.GetUserId())
	if err != nil {
		return nil, apperror.ToGRPCStatus(err)
	}
	return meetupToProto(m, req.GetUserId()), nil
}

func (s *Service) ListOpenMeetups(ctx context.Context, req *meetupv1.ListOpenMeetupsRequest) (*meetupv1.ListOpenMeetupsResponse, error) {
	intent, ok := intentFromProto[req.GetIntent()]
	if !ok {
		return nil, apperror.ToGRPCStatus(fmt.Errorf("meetup: unrecognized intent: %w", apperror.ErrInvalidInput))
	}

	cursor, err := decodeCursor(req.GetCursor())
	if err != nil {
		return nil, apperror.ToGRPCStatus(fmt.Errorf("meetup: invalid cursor: %w", apperror.ErrInvalidInput))
	}

	meetups, next, err := s.meetups.ListOpen(ctx, intent, req.GetUserId(), cursor, int(req.GetPageSize()))
	if err != nil {
		return nil, apperror.ToGRPCStatus(err)
	}

	return &meetupv1.ListOpenMeetupsResponse{
		Meetups:    meetupsToProto(meetups, req.GetUserId()),
		NextCursor: encodeCursor(next),
	}, nil
}

func (s *Service) ListMyMeetups(ctx context.Context, req *meetupv1.ListMyMeetupsRequest) (*meetupv1.ListMyMeetupsResponse, error) {
	hosted, err := s.meetups.ListByHost(ctx, req.GetUserId())
	if err != nil {
		return nil, apperror.ToGRPCStatus(err)
	}
	requested, err := s.meetups.ListRequestedByUser(ctx, req.GetUserId())
	if err != nil {
		return nil, apperror.ToGRPCStatus(err)
	}

	return &meetupv1.ListMyMeetupsResponse{
		Hosted:    meetupsToProto(hosted, req.GetUserId()),
		Requested: meetupsToProto(requested, req.GetUserId()),
	}, nil
}

func (s *Service) ListMeetupRequests(ctx context.Context, req *meetupv1.ListMeetupRequestsRequest) (*meetupv1.ListMeetupRequestsResponse, error) {
	m, err := s.meetups.GetByID(ctx, req.GetMeetupId(), req.GetHostUserId())
	if err != nil {
		return nil, apperror.ToGRPCStatus(err)
	}
	if m.HostUserID != req.GetHostUserId() {
		return nil, apperror.ToGRPCStatus(fmt.Errorf("meetup: caller does not host meetup %s: %w", req.GetMeetupId(), apperror.ErrForbidden))
	}

	requests, err := s.requests.ListForMeetup(ctx, req.GetMeetupId())
	if err != nil {
		return nil, apperror.ToGRPCStatus(err)
	}
	return &meetupv1.ListMeetupRequestsResponse{Requests: requestsToProto(requests)}, nil
}

// CloseMeetup is the host-only "meetup is done" action (ADR-016), reviving
// meetup_status's previously-unused 'completed' value. The repository's
// Close does the entire authorization/precondition check (right host,
// currently open-ish, window started) as a single UPDATE ... WHERE — no
// separate check-then-act that a concurrent request could race. Zero rows
// updated surfaces here as apperror.ErrNotFound; re-fetching via GetByID
// distinguishes *why* for a useful client message, without a second
// authoritative check duplicating the query's own logic.
//
// Explicitly does not touch rating eligibility — SubmitRating's gate is
// each participant's own meetup_feedback.happened (ADR-015), independent
// of meetups.status. Closing is a display/organizational move (My
// Meetups' Open vs. History split), not a new security gate.
func (s *Service) CloseMeetup(ctx context.Context, req *meetupv1.CloseMeetupRequest) (*meetupv1.CloseMeetupResponse, error) {
	if _, err := s.meetups.Close(ctx, req.GetMeetupId(), req.GetHostUserId()); err != nil {
		if errors.Is(err, apperror.ErrNotFound) {
			current, getErr := s.meetups.GetByID(ctx, req.GetMeetupId(), req.GetHostUserId())
			if getErr != nil {
				return nil, apperror.ToGRPCStatus(getErr)
			}
			switch {
			case current.HostUserID != req.GetHostUserId():
				return nil, apperror.ToGRPCStatus(fmt.Errorf("meetup: only the host can close this meetup: %w", apperror.ErrForbidden))
			case current.Status != repository.MeetupStatusOpen && current.Status != repository.MeetupStatusFull:
				return nil, apperror.ToGRPCStatus(fmt.Errorf("meetup: already closed or cancelled: %w", apperror.ErrConflict))
			case time.Now().Before(current.WindowStart):
				return nil, apperror.ToGRPCStatus(fmt.Errorf("meetup: can't close before the meetup's window has started: %w", apperror.ErrForbidden))
			}
		}
		return nil, apperror.ToGRPCStatus(err)
	}

	// Close's own RETURNING * has no host display info to join against
	// (same limitation as Cancel) — re-fetch the fully-populated view for
	// the response, same "Create doesn't join, GetByID does" precedent
	// used right above.
	full, err := s.meetups.GetByID(ctx, req.GetMeetupId(), req.GetHostUserId())
	if err != nil {
		return nil, apperror.ToGRPCStatus(err)
	}
	return &meetupv1.CloseMeetupResponse{Meetup: meetupToProto(full, req.GetHostUserId())}, nil
}

func (s *Service) CancelMeetup(ctx context.Context, req *meetupv1.CancelMeetupRequest) (*meetupv1.CancelMeetupResponse, error) {
	m, err := s.meetups.GetByID(ctx, req.GetMeetupId(), req.GetHostUserId())
	if err != nil {
		return nil, apperror.ToGRPCStatus(err)
	}
	if m.HostUserID != req.GetHostUserId() {
		return nil, apperror.ToGRPCStatus(fmt.Errorf("meetup: caller does not host meetup %s: %w", req.GetMeetupId(), apperror.ErrForbidden))
	}
	if m.AcceptedCount > 0 {
		// Cancelling a meetup with confirmed participants needs its own
		// confirm-and-notify path (backend/meetup-scheduling-PLAN.md Step
		// C) — not built in this slice, so this is where it would plug in.
		// Rejecting rather than silently vanishing on people who already
		// said yes. ErrConflict (not ErrForbidden) — this is a state
		// precondition, not a permissions question; the caller is
		// definitely the host, cancelling just isn't allowed right now.
		return nil, apperror.ToGRPCStatus(fmt.Errorf(
			"meetup: cannot cancel a meetup with accepted participants: %w", apperror.ErrConflict,
		))
	}

	if _, err := s.meetups.Cancel(ctx, req.GetMeetupId()); err != nil {
		return nil, apperror.ToGRPCStatus(err)
	}
	return &meetupv1.CancelMeetupResponse{Success: true}, nil
}

func (s *Service) RegisterDeviceToken(ctx context.Context, req *meetupv1.RegisterDeviceTokenRequest) (*meetupv1.RegisterDeviceTokenResponse, error) {
	if req.GetFcmToken() == "" {
		return nil, apperror.ToGRPCStatus(fmt.Errorf("meetup: fcm_token is required: %w", apperror.ErrInvalidInput))
	}
	if err := s.deviceTokens.Upsert(ctx, req.GetUserId(), req.GetFcmToken()); err != nil {
		return nil, apperror.ToGRPCStatus(err)
	}
	return &meetupv1.RegisterDeviceTokenResponse{Success: true}, nil
}
