package service

import (
	"context"
	"fmt"

	"github.com/professional-connections/backend/services/meetup/internal/repository"
	"github.com/professional-connections/backend/shared/apperror"
	meetupv1 "github.com/professional-connections/backend/shared/proto/meetup/v1"
)

// ListRatableParticipants returns viewer_id's other participants on
// meetup_id (host + accepted requesters, excluding self), each flagged with
// whether viewer_id already rated them. Not an error if the caller isn't a
// participant — returns an empty list instead.
//
// Security review finding (self-review pass against
// docs/03-architecture/security-review-framework.md's Confidentiality
// property, ADR-015): ListRatable's own query only ever selects rows keyed
// off the meetup's actual host/accepted-requester set and excludes
// viewer_id from the result, but it does NOT check that viewer_id is
// itself a participant — called directly, that would let any authenticated
// user enumerate any meetup's participant names/photos/trust levels just
// by guessing a meetup_id, regardless of whether they were ever invited or
// accepted. The explicit IsParticipant check below is what actually closes
// that gap; ListRatable alone does not.
func (s *Service) ListRatableParticipants(ctx context.Context, req *meetupv1.ListRatableParticipantsRequest) (*meetupv1.ListRatableParticipantsResponse, error) {
	viewerIsParticipant, err := s.ratings.IsParticipant(ctx, req.GetMeetupId(), req.GetViewerId())
	if err != nil {
		return nil, apperror.ToGRPCStatus(err)
	}
	if !viewerIsParticipant {
		return &meetupv1.ListRatableParticipantsResponse{}, nil
	}

	participants, err := s.ratings.ListRatable(ctx, req.GetMeetupId(), req.GetViewerId())
	if err != nil {
		return nil, apperror.ToGRPCStatus(err)
	}
	return &meetupv1.ListRatableParticipantsResponse{Participants: ratableParticipantsToProto(participants)}, nil
}

// SubmitRating enforces every ADR-015 guard server-side, never trusting the
// client's own UI gating: rater_user_id and rated_user_id must both be
// participants of meetup_id, they must differ, the rater must have already
// confirmed (SubmitMeetupFeedback, happened=true) that the meetup happened,
// and the score must be 1-5. RatingRepository.Submit's own DB-level
// constraints are the final backstop (duplicate submission, self-rating)
// should any of these race past the checks below.
func (s *Service) SubmitRating(ctx context.Context, req *meetupv1.SubmitRatingRequest) (*meetupv1.SubmitRatingResponse, error) {
	if req.GetScore() < 1 || req.GetScore() > 5 {
		return nil, apperror.ToGRPCStatus(fmt.Errorf("meetup: score must be between 1 and 5: %w", apperror.ErrInvalidInput))
	}
	if req.GetRaterUserId() == req.GetRatedUserId() {
		return nil, apperror.ToGRPCStatus(fmt.Errorf("meetup: cannot rate yourself: %w", apperror.ErrInvalidInput))
	}

	raterIsParticipant, err := s.ratings.IsParticipant(ctx, req.GetMeetupId(), req.GetRaterUserId())
	if err != nil {
		return nil, apperror.ToGRPCStatus(err)
	}
	if !raterIsParticipant {
		return nil, apperror.ToGRPCStatus(fmt.Errorf("meetup: caller is not a participant of meetup %s: %w", req.GetMeetupId(), apperror.ErrForbidden))
	}

	ratedIsParticipant, err := s.ratings.IsParticipant(ctx, req.GetMeetupId(), req.GetRatedUserId())
	if err != nil {
		return nil, apperror.ToGRPCStatus(err)
	}
	if !ratedIsParticipant {
		return nil, apperror.ToGRPCStatus(fmt.Errorf("meetup: rated user is not a participant of meetup %s: %w", req.GetMeetupId(), apperror.ErrInvalidInput))
	}

	// Gated on the *rater's* confirmed attendance only — a no-show is
	// legitimately ratable by someone who did attend and confirm (ADR-015).
	confirmed, err := s.ratings.HasConfirmedHappened(ctx, req.GetMeetupId(), req.GetRaterUserId())
	if err != nil {
		return nil, apperror.ToGRPCStatus(err)
	}
	if !confirmed {
		return nil, apperror.ToGRPCStatus(fmt.Errorf(
			"meetup: confirm the meetup happened (SubmitMeetupFeedback) before rating participants: %w", apperror.ErrForbidden,
		))
	}

	if err := s.ratings.Submit(ctx, req.GetMeetupId(), req.GetRaterUserId(), req.GetRatedUserId(), int(req.GetScore())); err != nil {
		return nil, apperror.ToGRPCStatus(err)
	}
	return &meetupv1.SubmitRatingResponse{Success: true}, nil
}

func ratableParticipantsToProto(participants []repository.RatableParticipant) []*meetupv1.RatableParticipant {
	out := make([]*meetupv1.RatableParticipant, 0, len(participants))
	for _, p := range participants {
		out = append(out, &meetupv1.RatableParticipant{
			UserId:          p.UserID,
			FullName:        p.FullName,
			ProfilePhotoUrl: p.ProfilePhotoURL,
			TrustLevel:      int32(p.TrustLevel),
			AlreadyRated:    p.AlreadyRated,
		})
	}
	return out
}
