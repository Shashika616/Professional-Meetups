package service

import (
	"context"
	"fmt"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/professional-connections/backend/shared/apperror"
	meetupv1 "github.com/professional-connections/backend/shared/proto/meetup/v1"
)

func newTestServiceWithRatings() (*Service, *fakeRatingRepository) {
	meetups := newFakeMeetupRepository()
	requests := newFakeMeetupRequestRepository()
	safety := newFakeSafetyStateRepository()
	feedback := newFakeFeedbackRepository()
	ratings := newFakeRatingRepository()
	deviceTokens := newFakeDeviceTokenRepository()
	publisher := &fakePublisher{}
	sender := &fakeNotificationSender{}
	svc := New(meetups, requests, safety, feedback, ratings, deviceTokens, publisher, sender)
	return svc, ratings
}

func wantGRPCCode(t *testing.T, err error, want codes.Code) {
	t.Helper()
	if err == nil {
		t.Fatalf("want error with code %s, got nil", want)
	}
	st, ok := status.FromError(err)
	if !ok || st.Code() != want {
		t.Fatalf("want error with code %s, got %v", want, err)
	}
}

func TestSubmitRating_NotAParticipant(t *testing.T) {
	svc, _ := newTestServiceWithRatings()
	// Neither rater nor ratee registered as a participant.
	_, err := svc.SubmitRating(context.Background(), &meetupv1.SubmitRatingRequest{
		MeetupId: "meetup-1", RaterUserId: "rater-1", RatedUserId: "ratee-1", Score: 5,
	})
	wantGRPCCode(t, err, codes.PermissionDenied) // apperror.ErrForbidden
}

func TestSubmitRating_RatedUserNotAParticipant(t *testing.T) {
	svc, ratings := newTestServiceWithRatings()
	ratings.setParticipant("meetup-1", "rater-1")
	ratings.setConfirmed("meetup-1", "rater-1")
	// rated-1 never registered as a participant.
	_, err := svc.SubmitRating(context.Background(), &meetupv1.SubmitRatingRequest{
		MeetupId: "meetup-1", RaterUserId: "rater-1", RatedUserId: "ratee-1", Score: 5,
	})
	wantGRPCCode(t, err, codes.InvalidArgument) // apperror.ErrInvalidInput
}

func TestSubmitRating_RaterHasNotConfirmedAttendance(t *testing.T) {
	svc, ratings := newTestServiceWithRatings()
	ratings.setParticipant("meetup-1", "rater-1")
	ratings.setParticipant("meetup-1", "ratee-1")
	// Deliberately not calling setConfirmed for rater-1.
	_, err := svc.SubmitRating(context.Background(), &meetupv1.SubmitRatingRequest{
		MeetupId: "meetup-1", RaterUserId: "rater-1", RatedUserId: "ratee-1", Score: 5,
	})
	// Refinement over the original plan: "not yet eligible to rate" is
	// Forbidden, not Conflict — Conflict is reserved for a true duplicate
	// submission (ADR-015).
	wantGRPCCode(t, err, codes.PermissionDenied)
}

func TestSubmitRating_RateeDoesNotNeedOwnConfirmedAttendance(t *testing.T) {
	// A no-show is legitimately ratable by someone who did attend and
	// confirm — the ratee's own feedback state is irrelevant (ADR-015).
	svc, ratings := newTestServiceWithRatings()
	ratings.setParticipant("meetup-1", "rater-1")
	ratings.setParticipant("meetup-1", "ratee-1")
	ratings.setConfirmed("meetup-1", "rater-1")
	// ratee-1 never confirmed anything.

	_, err := svc.SubmitRating(context.Background(), &meetupv1.SubmitRatingRequest{
		MeetupId: "meetup-1", RaterUserId: "rater-1", RatedUserId: "ratee-1", Score: 4,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(ratings.submitted) != 1 || ratings.submitted[0].Score != 4 {
		t.Fatalf("expected one submitted rating with score 4, got %+v", ratings.submitted)
	}
}

func TestSubmitRating_SelfRatingRejectedBeforeHittingTheRepository(t *testing.T) {
	svc, ratings := newTestServiceWithRatings()
	ratings.setParticipant("meetup-1", "user-1")
	ratings.setConfirmed("meetup-1", "user-1")

	_, err := svc.SubmitRating(context.Background(), &meetupv1.SubmitRatingRequest{
		MeetupId: "meetup-1", RaterUserId: "user-1", RatedUserId: "user-1", Score: 5,
	})
	wantGRPCCode(t, err, codes.InvalidArgument)
	if len(ratings.submitted) != 0 {
		t.Fatalf("Submit should never have been called for a self-rating, got %+v", ratings.submitted)
	}
}

func TestSubmitRating_ScoreOutOfRange(t *testing.T) {
	svc, ratings := newTestServiceWithRatings()
	ratings.setParticipant("meetup-1", "rater-1")
	ratings.setParticipant("meetup-1", "ratee-1")
	ratings.setConfirmed("meetup-1", "rater-1")

	for _, score := range []int32{0, 6, -1} {
		_, err := svc.SubmitRating(context.Background(), &meetupv1.SubmitRatingRequest{
			MeetupId: "meetup-1", RaterUserId: "rater-1", RatedUserId: "ratee-1", Score: score,
		})
		wantGRPCCode(t, err, codes.InvalidArgument)
	}
}

func TestSubmitRating_DuplicateSubmissionMapsToConflict(t *testing.T) {
	svc, ratings := newTestServiceWithRatings()
	ratings.setParticipant("meetup-1", "rater-1")
	ratings.setParticipant("meetup-1", "ratee-1")
	ratings.setConfirmed("meetup-1", "rater-1")
	ratings.submitErr = errConflictf("repository: already rated this participant for this meetup")

	_, err := svc.SubmitRating(context.Background(), &meetupv1.SubmitRatingRequest{
		MeetupId: "meetup-1", RaterUserId: "rater-1", RatedUserId: "ratee-1", Score: 3,
	})
	wantGRPCCode(t, err, codes.AlreadyExists) // apperror.ErrConflict
}

func TestListRatableParticipants_ViewerNotAParticipantReturnsEmpty(t *testing.T) {
	// Security review finding: a non-participant must not be able to
	// enumerate a meetup's participants by guessing a meetup_id.
	svc, ratings := newTestServiceWithRatings()
	ratings.setParticipant("meetup-1", "host-1")
	ratings.setParticipant("meetup-1", "attendee-1")
	// "outsider-1" is deliberately never registered as a participant.

	resp, err := svc.ListRatableParticipants(context.Background(), &meetupv1.ListRatableParticipantsRequest{
		MeetupId: "meetup-1", ViewerId: "outsider-1",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.GetParticipants()) != 0 {
		t.Fatalf("a non-participant must get an empty list, got %+v", resp.GetParticipants())
	}
}

func TestListRatableParticipants_PassesThrough(t *testing.T) {
	svc, ratings := newTestServiceWithRatings()
	ratings.setParticipant("meetup-1", "viewer-1")
	resp, err := svc.ListRatableParticipants(context.Background(), &meetupv1.ListRatableParticipantsRequest{
		MeetupId: "meetup-1", ViewerId: "viewer-1",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.GetParticipants()) != 0 {
		t.Fatalf("expected no participants from the empty fake, got %+v", resp.GetParticipants())
	}
}

// errConflictf mirrors the "...: %w"-wrapped-sentinel shape every real
// repository error in this codebase uses, so ToGRPCStatus's errors.Is
// check behaves the same as it would against a real Postgres error.
func errConflictf(msg string) error {
	return fmt.Errorf("%s: %w", msg, apperror.ErrConflict)
}
