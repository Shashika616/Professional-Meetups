package service

import (
	"context"
	"fmt"

	"github.com/professional-connections/backend/shared/apperror"
	meetupv1 "github.com/professional-connections/backend/shared/proto/meetup/v1"
)

// GetSafetyState returns apperror.ErrNotFound (wrapped, surfaced as gRPC
// NotFound) when the meetup has no Safety Gate state yet — no accepted
// request means the gate hasn't started (ADR-013 § 3). Lets a client
// reopening this meetup distinguish "not started" from "started, nothing
// acknowledged yet."
func (s *Service) GetSafetyState(ctx context.Context, req *meetupv1.GetSafetyStateRequest) (*meetupv1.SafetyStateResponse, error) {
	state, err := s.safetyState.Get(ctx, req.GetMeetupId())
	if err != nil {
		return nil, apperror.ToGRPCStatus(err)
	}
	return safetyStateToProto(state), nil
}

func (s *Service) AcknowledgeSafetyChecklist(ctx context.Context, req *meetupv1.AcknowledgeSafetyChecklistRequest) (*meetupv1.SafetyStateResponse, error) {
	state, err := s.safetyState.AcknowledgeChecklist(ctx, req.GetMeetupId())
	if err != nil {
		return nil, apperror.ToGRPCStatus(err)
	}
	return safetyStateToProto(state), nil
}

func (s *Service) SetLiveLocationOptIn(ctx context.Context, req *meetupv1.SetLiveLocationOptInRequest) (*meetupv1.SafetyStateResponse, error) {
	state, err := s.safetyState.SetLiveLocationOptIn(ctx, req.GetMeetupId(), req.GetOptIn())
	if err != nil {
		return nil, apperror.ToGRPCStatus(err)
	}
	return safetyStateToProto(state), nil
}

// CheckIn enforces Safety UX Flows.md's step order server-side (checklist
// before check-in), not just relying on the client's own screen sequencing
// — a client that skips straight to check-in (a bug, or a modified client)
// must not be able to bypass the checklist step.
func (s *Service) CheckIn(ctx context.Context, req *meetupv1.CheckInRequest) (*meetupv1.SafetyStateResponse, error) {
	current, err := s.safetyState.Get(ctx, req.GetMeetupId())
	if err != nil {
		return nil, apperror.ToGRPCStatus(err)
	}
	if current.ChecklistAckAt == nil {
		return nil, apperror.ToGRPCStatus(fmt.Errorf(
			"meetup: safety checklist must be acknowledged before check-in: %w", apperror.ErrConflict,
		))
	}

	state, err := s.safetyState.CheckIn(ctx, req.GetMeetupId())
	if err != nil {
		return nil, apperror.ToGRPCStatus(err)
	}
	return safetyStateToProto(state), nil
}

func (s *Service) SubmitMeetupFeedback(ctx context.Context, req *meetupv1.SubmitMeetupFeedbackRequest) (*meetupv1.SubmitMeetupFeedbackResponse, error) {
	// felt_safe/profile_accurate/would_meet_again are optional proto fields
	// — read via the raw pointer, not the Get*() accessor, since the
	// accessor collapses "genuinely unset" to false, which would otherwise
	// get written as a real "false" answer rather than left NULL. Only
	// meaningful when the meetup actually happened; a "didn't happen"
	// report is never misread as "happened but felt unsafe."
	var feltSafe, profileAccurate, wouldMeetAgain *bool
	if req.GetHappened() {
		feltSafe = req.FeltSafe
		profileAccurate = req.ProfileAccurate
		wouldMeetAgain = req.WouldMeetAgain
	}

	// notes (ADR-016) is never gated on happened, unlike the three booleans
	// above — a note is meaningful either way (e.g. "never showed up" on a
	// didn't-happen report).
	if err := s.feedback.Upsert(ctx, req.GetMeetupId(), req.GetUserId(), req.GetHappened(), feltSafe, profileAccurate, wouldMeetAgain, req.Notes); err != nil {
		return nil, apperror.ToGRPCStatus(err)
	}
	return &meetupv1.SubmitMeetupFeedbackResponse{Success: true}, nil
}
