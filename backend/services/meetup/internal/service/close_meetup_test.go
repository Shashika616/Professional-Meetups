package service

import (
	"context"
	"testing"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/professional-connections/backend/services/meetup/internal/repository"
	meetupv1 "github.com/professional-connections/backend/shared/proto/meetup/v1"
)

func TestCloseMeetup_Success(t *testing.T) {
	svc, meetups, _, _, _, _ := newTestService()
	meetups.put(repository.Meetup{
		ID: "meetup-1", HostUserID: "host-1", Intent: repository.IntentCoffee,
		Status:      repository.MeetupStatusOpen,
		WindowStart: time.Now().Add(-30 * time.Minute), // already started
		WindowEnd:   time.Now().Add(30 * time.Minute),
		Capacity:    2,
	})

	resp, err := svc.CloseMeetup(context.Background(), &meetupv1.CloseMeetupRequest{
		MeetupId: "meetup-1", HostUserId: "host-1",
	})
	if err != nil {
		t.Fatalf("CloseMeetup() error: %v", err)
	}
	if resp.GetMeetup().GetStatus() != meetupv1.MeetupStatus_MEETUP_STATUS_COMPLETED {
		t.Errorf("status = %v, want COMPLETED", resp.GetMeetup().GetStatus())
	}
}

func TestCloseMeetup_NonHostForbidden(t *testing.T) {
	svc, meetups, _, _, _, _ := newTestService()
	meetups.put(repository.Meetup{
		ID: "meetup-1", HostUserID: "host-1", Intent: repository.IntentCoffee,
		Status:      repository.MeetupStatusOpen,
		WindowStart: time.Now().Add(-30 * time.Minute),
		WindowEnd:   time.Now().Add(30 * time.Minute),
		Capacity:    2,
	})

	_, err := svc.CloseMeetup(context.Background(), &meetupv1.CloseMeetupRequest{
		MeetupId: "meetup-1", HostUserId: "someone-else",
	})
	if got := status.Code(err); got != codes.PermissionDenied {
		t.Errorf("status code = %v, want %v", got, codes.PermissionDenied)
	}
}

func TestCloseMeetup_WindowNotStartedYet(t *testing.T) {
	svc, meetups, _, _, _, _ := newTestService()
	meetups.put(repository.Meetup{
		ID: "meetup-1", HostUserID: "host-1", Intent: repository.IntentCoffee,
		Status:      repository.MeetupStatusOpen,
		WindowStart: time.Now().Add(1 * time.Hour), // hasn't started
		WindowEnd:   time.Now().Add(3 * time.Hour),
		Capacity:    2,
	})

	_, err := svc.CloseMeetup(context.Background(), &meetupv1.CloseMeetupRequest{
		MeetupId: "meetup-1", HostUserId: "host-1",
	})
	if got := status.Code(err); got != codes.PermissionDenied {
		t.Errorf("status code = %v, want %v (can't close before the window starts)", got, codes.PermissionDenied)
	}
}

func TestCloseMeetup_AlreadyClosedConflicts(t *testing.T) {
	svc, meetups, _, _, _, _ := newTestService()
	meetups.put(repository.Meetup{
		ID: "meetup-1", HostUserID: "host-1", Intent: repository.IntentCoffee,
		Status:      repository.MeetupStatusCompleted,
		WindowStart: time.Now().Add(-30 * time.Minute),
		WindowEnd:   time.Now().Add(30 * time.Minute),
		Capacity:    2,
	})

	_, err := svc.CloseMeetup(context.Background(), &meetupv1.CloseMeetupRequest{
		MeetupId: "meetup-1", HostUserId: "host-1",
	})
	if got := status.Code(err); got != codes.AlreadyExists { // apperror.ErrConflict
		t.Errorf("status code = %v, want %v", got, codes.AlreadyExists)
	}
}

// TestCloseMeetup_DoesNotAffectRatingEligibility guards ADR-016's explicit
// decoupling: closing a meetup must have zero effect on SubmitRating's own
// gate (each participant's meetup_feedback.happened, ADR-015) — a rater who
// already confirmed attendance can still rate, whether or not the host has
// since closed the meetup, and closing itself never touches
// meetup_feedback or meetup_ratings.
func TestCloseMeetup_DoesNotAffectRatingEligibility(t *testing.T) {
	svc, meetups, _, _, _, _ := newTestService()
	meetups.put(repository.Meetup{
		ID: "meetup-1", HostUserID: "host-1", Intent: repository.IntentCoffee,
		Status:      repository.MeetupStatusOpen,
		WindowStart: time.Now().Add(-30 * time.Minute),
		WindowEnd:   time.Now().Add(30 * time.Minute),
		Capacity:    2,
	})

	ratings := newFakeRatingRepository()
	ratings.setParticipant("meetup-1", "host-1")
	ratings.setParticipant("meetup-1", "attendee-1")
	ratings.setConfirmed("meetup-1", "attendee-1")
	svc.ratings = ratings

	if _, err := svc.CloseMeetup(context.Background(), &meetupv1.CloseMeetupRequest{
		MeetupId: "meetup-1", HostUserId: "host-1",
	}); err != nil {
		t.Fatalf("CloseMeetup() error: %v", err)
	}

	if _, err := svc.SubmitRating(context.Background(), &meetupv1.SubmitRatingRequest{
		MeetupId: "meetup-1", RaterUserId: "attendee-1", RatedUserId: "host-1", Score: 5,
	}); err != nil {
		t.Fatalf("SubmitRating() after close unexpectedly failed: %v — closing must not affect rating eligibility (ADR-016)", err)
	}
}
