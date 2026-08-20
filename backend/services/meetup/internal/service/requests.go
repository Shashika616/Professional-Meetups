package service

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/professional-connections/backend/services/meetup/internal/repository"
	"github.com/professional-connections/backend/shared/apperror"
	meetupv1 "github.com/professional-connections/backend/shared/proto/meetup/v1"
)

func (s *Service) RequestToJoin(ctx context.Context, req *meetupv1.RequestToJoinRequest) (*meetupv1.MeetupRequestResponse, error) {
	m, err := s.meetups.GetByID(ctx, req.GetMeetupId(), req.GetRequesterId())
	if err != nil {
		return nil, apperror.ToGRPCStatus(err)
	}

	if m.HostUserID == req.GetRequesterId() {
		return nil, apperror.ToGRPCStatus(fmt.Errorf("meetup: cannot request to join your own meetup: %w", apperror.ErrForbidden))
	}
	if err := checkTrustLevel(m.Intent, int(req.GetRequesterTrustLevel())); err != nil {
		return nil, apperror.ToGRPCStatus(err)
	}
	if m.Status != "open" {
		return nil, apperror.ToGRPCStatus(fmt.Errorf("meetup: meetup %s is not open: %w", req.GetMeetupId(), apperror.ErrConflict))
	}

	created, err := s.requests.Create(ctx, req.GetMeetupId(), req.GetRequesterId())
	if err != nil {
		return nil, apperror.ToGRPCStatus(err)
	}

	if err := s.events.PublishRequestCreated(ctx, created.ID, req.GetMeetupId(), req.GetRequesterId(), m.HostUserID); err != nil {
		return nil, apperror.ToGRPCStatus(fmt.Errorf("publish meetup.request.created: %w: %w", apperror.ErrInternal, err))
	}

	// Create's return value has no requester display info (see
	// meetupRequestFromRow's doc comment) — GetByID re-fetches the joined
	// view before it's needed for the notification body or the response.
	full, err := s.requests.GetByID(ctx, created.ID)
	if err != nil {
		return nil, apperror.ToGRPCStatus(err)
	}

	// A push-send failure must not roll back the request that already
	// succeeded (backend/meetup-scheduling-PLAN.md's Test strategy) —
	// logged, not propagated as an RPC error.
	if err := s.notifications.SendPushNotification(ctx, m.HostUserID,
		"New join request", fmt.Sprintf("%s wants to join your %s meetup", full.RequesterFullName, m.Intent),
		map[string]string{"meetup_id": m.ID, "request_id": created.ID},
	); err != nil {
		slog.Default().Error("send push notification for new join request", "error", err)
	}

	return requestToProto(full), nil
}

func (s *Service) WithdrawRequest(ctx context.Context, req *meetupv1.WithdrawRequestRequest) (*meetupv1.WithdrawRequestResponse, error) {
	existing, err := s.requests.GetByID(ctx, req.GetRequestId())
	if err != nil {
		return nil, apperror.ToGRPCStatus(err)
	}
	if existing.RequesterID != req.GetRequesterId() {
		return nil, apperror.ToGRPCStatus(fmt.Errorf("meetup: caller did not make request %s: %w", req.GetRequestId(), apperror.ErrForbidden))
	}

	if _, err := s.requests.Withdraw(ctx, req.GetRequestId()); err != nil {
		return nil, apperror.ToGRPCStatus(err)
	}
	return &meetupv1.WithdrawRequestResponse{Success: true}, nil
}

func (s *Service) RespondToRequest(ctx context.Context, req *meetupv1.RespondToRequestRequest) (*meetupv1.MeetupRequestResponse, error) {
	existing, err := s.requests.GetByID(ctx, req.GetRequestId())
	if err != nil {
		return nil, apperror.ToGRPCStatus(err)
	}

	m, err := s.meetups.GetByID(ctx, existing.MeetupID, req.GetHostUserId())
	if err != nil {
		return nil, apperror.ToGRPCStatus(err)
	}
	if m.HostUserID != req.GetHostUserId() {
		return nil, apperror.ToGRPCStatus(fmt.Errorf("meetup: caller does not host meetup %s: %w", existing.MeetupID, apperror.ErrForbidden))
	}

	if req.GetAccept() {
		return s.acceptRequest(ctx, req.GetRequestId(), m)
	}
	return s.rejectRequest(ctx, req.GetRequestId(), m)
}

func (s *Service) acceptRequest(ctx context.Context, requestID string, m repository.Meetup) (*meetupv1.MeetupRequestResponse, error) {
	accepted, _, autoRejected, err := s.requests.Accept(ctx, requestID)
	if err != nil {
		return nil, apperror.ToGRPCStatus(err)
	}

	if err := s.events.PublishRequestAccepted(ctx, accepted.ID, m.ID, accepted.RequesterID, m.HostUserID); err != nil {
		return nil, apperror.ToGRPCStatus(fmt.Errorf("publish meetup.request.accepted: %w: %w", apperror.ErrInternal, err))
	}
	if err := s.notifications.SendPushNotification(ctx, accepted.RequesterID,
		"Request accepted", fmt.Sprintf("The host accepted your request to join their %s meetup", m.Intent),
		map[string]string{"meetup_id": m.ID, "request_id": accepted.ID},
	); err != nil {
		slog.Default().Error("send push notification for accepted request", "error", err)
	}

	for _, rejected := range autoRejected {
		if err := s.events.PublishRequestRejected(ctx, rejected.ID, m.ID, rejected.RequesterID, m.HostUserID, true); err != nil {
			slog.Default().Error("publish meetup.request.rejected (auto)", "error", err)
			continue
		}
		if err := s.notifications.SendPushNotification(ctx, rejected.RequesterID,
			"Meetup is full", fmt.Sprintf("This %s meetup reached capacity before your request was accepted", m.Intent),
			map[string]string{"meetup_id": m.ID, "request_id": rejected.ID},
		); err != nil {
			slog.Default().Error("send push notification for auto-rejected request", "error", err)
		}
	}
	// Every successful accept gets Safety Gate state — a meetup starts the
	// Safety Gate the moment it has its *first* accepted request (ADR-013
	// § 3), not only once it's completely full. EnsureExists is idempotent
	// (ON CONFLICT DO NOTHING), so calling it on every accept rather than
	// only the first is simpler than tracking "was this the first" and
	// costs nothing extra. Logged, not propagated — the Safety Gate row
	// matters, but a transient failure here shouldn't undo a real,
	// already-committed accept.
	if err := s.safetyState.EnsureExists(ctx, m.ID); err != nil {
		slog.Default().Error("ensure safety state after accepted request", "error", err)
	}

	full, err := s.requests.GetByID(ctx, accepted.ID)
	if err != nil {
		return nil, apperror.ToGRPCStatus(err)
	}
	return requestToProto(full), nil
}

func (s *Service) rejectRequest(ctx context.Context, requestID string, m repository.Meetup) (*meetupv1.MeetupRequestResponse, error) {
	rejected, err := s.requests.Reject(ctx, requestID)
	if err != nil {
		return nil, apperror.ToGRPCStatus(err)
	}

	if err := s.events.PublishRequestRejected(ctx, rejected.ID, m.ID, rejected.RequesterID, m.HostUserID, false); err != nil {
		return nil, apperror.ToGRPCStatus(fmt.Errorf("publish meetup.request.rejected: %w: %w", apperror.ErrInternal, err))
	}
	if err := s.notifications.SendPushNotification(ctx, rejected.RequesterID,
		"Request declined", fmt.Sprintf("The host declined your request to join their %s meetup", m.Intent),
		map[string]string{"meetup_id": m.ID, "request_id": rejected.ID},
	); err != nil {
		slog.Default().Error("send push notification for rejected request", "error", err)
	}

	full, err := s.requests.GetByID(ctx, rejected.ID)
	if err != nil {
		return nil, apperror.ToGRPCStatus(err)
	}
	return requestToProto(full), nil
}
