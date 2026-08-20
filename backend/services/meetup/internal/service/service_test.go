package service

import (
	"context"
	"strings"
	"testing"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/professional-connections/backend/services/meetup/internal/repository"
	meetupv1 "github.com/professional-connections/backend/shared/proto/meetup/v1"
)

// validWindow returns a (window_start, window_end) pair an hour from now,
// far enough in the future to clear both CreateMeetup's grace-period and
// window_end > window_start checks (ADR-016).
func validWindow() (int64, int64) {
	start := time.Now().Add(1 * time.Hour).Unix()
	end := time.Now().Add(3 * time.Hour).Unix()
	return start, end
}

func newTestService() (*Service, *fakeMeetupRepository, *fakeMeetupRequestRepository, *fakeSafetyStateRepository, *fakePublisher, *fakeNotificationSender) {
	meetups := newFakeMeetupRepository()
	requests := newFakeMeetupRequestRepository()
	safety := newFakeSafetyStateRepository()
	feedback := newFakeFeedbackRepository()
	ratings := newFakeRatingRepository()
	deviceTokens := newFakeDeviceTokenRepository()
	publisher := &fakePublisher{}
	sender := &fakeNotificationSender{}
	svc := New(meetups, requests, safety, feedback, ratings, deviceTokens, publisher, sender)
	return svc, meetups, requests, safety, publisher, sender
}

func TestCreateMeetup_TrustGate(t *testing.T) {
	svc, _, _, _, _, _ := newTestService()
	ctx := context.Background()

	t.Run("level 1 rejected for coffee (needs level 2)", func(t *testing.T) {
		_, err := svc.CreateMeetup(ctx, &meetupv1.CreateMeetupRequest{
			HostUserId: "host-1", HostTrustLevel: 1, Intent: meetupv1.Intent_INTENT_COFFEE,
			LocationLabel: "Cafe", Capacity: 2,
		})
		if status.Code(err) != codes.PermissionDenied {
			t.Fatalf("CreateMeetup() code = %v, want %v", status.Code(err), codes.PermissionDenied)
		}
	})

	t.Run("level 2 allowed for coffee", func(t *testing.T) {
		windowStart, windowEnd := validWindow()
		resp, err := svc.CreateMeetup(ctx, &meetupv1.CreateMeetupRequest{
			HostUserId: "host-1", HostTrustLevel: 2, Intent: meetupv1.Intent_INTENT_COFFEE,
			LocationLabel: "Cafe", Capacity: 2,
			WindowStartUnixSeconds: windowStart, WindowEndUnixSeconds: windowEnd,
		})
		if err != nil {
			t.Fatalf("CreateMeetup() error: %v", err)
		}
		if resp.GetHostUserId() != "host-1" || resp.GetStatus() != meetupv1.MeetupStatus_MEETUP_STATUS_OPEN {
			t.Errorf("unexpected response: %+v", resp)
		}
	})

	t.Run("level 2 rejected for ride_share (needs level 4)", func(t *testing.T) {
		_, err := svc.CreateMeetup(ctx, &meetupv1.CreateMeetupRequest{
			HostUserId: "host-1", HostTrustLevel: 2, Intent: meetupv1.Intent_INTENT_RIDE_SHARE,
			LocationLabel: "Station", Capacity: 2,
		})
		if status.Code(err) != codes.PermissionDenied {
			t.Fatalf("CreateMeetup() code = %v, want %v", status.Code(err), codes.PermissionDenied)
		}
	})
}

func TestCreateMeetup_CapacityBounds(t *testing.T) {
	svc, _, _, _, _, _ := newTestService()
	ctx := context.Background()

	for _, capacity := range []int32{0, 21, -1} {
		t.Run("rejects out-of-range capacity", func(t *testing.T) {
			_, err := svc.CreateMeetup(ctx, &meetupv1.CreateMeetupRequest{
				HostUserId: "host-1", HostTrustLevel: 2, Intent: meetupv1.Intent_INTENT_COFFEE,
				LocationLabel: "Cafe", Capacity: capacity,
			})
			if status.Code(err) != codes.InvalidArgument {
				t.Errorf("capacity %d: code = %v, want %v", capacity, status.Code(err), codes.InvalidArgument)
			}
		})
	}
}

func TestRequestToJoin_CannotJoinOwnMeetup(t *testing.T) {
	svc, meetups, _, _, _, _ := newTestService()
	ctx := context.Background()

	meetups.put(repository.Meetup{ID: "meetup-1", HostUserID: "host-1", Intent: repository.IntentCoffee, Status: repository.MeetupStatusOpen, Capacity: 2})

	_, err := svc.RequestToJoin(ctx, &meetupv1.RequestToJoinRequest{
		MeetupId: "meetup-1", RequesterId: "host-1", RequesterTrustLevel: 2,
	})
	if status.Code(err) != codes.PermissionDenied {
		t.Fatalf("RequestToJoin() code = %v, want %v", status.Code(err), codes.PermissionDenied)
	}
}

func TestRequestToJoin_TrustGate(t *testing.T) {
	svc, meetups, _, _, _, _ := newTestService()
	ctx := context.Background()

	meetups.put(repository.Meetup{ID: "meetup-1", HostUserID: "host-1", Intent: repository.IntentDating, Status: repository.MeetupStatusOpen, Capacity: 2})

	_, err := svc.RequestToJoin(ctx, &meetupv1.RequestToJoinRequest{
		MeetupId: "meetup-1", RequesterId: "requester-1", RequesterTrustLevel: 2,
	})
	if status.Code(err) != codes.PermissionDenied {
		t.Fatalf("RequestToJoin() code = %v, want %v (dating needs level 4)", status.Code(err), codes.PermissionDenied)
	}
}

func TestRequestToJoin_RejectsOnNonOpenMeetup(t *testing.T) {
	svc, meetups, _, _, _, _ := newTestService()
	ctx := context.Background()

	meetups.put(repository.Meetup{ID: "meetup-1", HostUserID: "host-1", Intent: repository.IntentCoffee, Status: repository.MeetupStatusFull, Capacity: 2})

	_, err := svc.RequestToJoin(ctx, &meetupv1.RequestToJoinRequest{
		MeetupId: "meetup-1", RequesterId: "requester-1", RequesterTrustLevel: 2,
	})
	if status.Code(err) != codes.AlreadyExists {
		t.Fatalf("RequestToJoin() code = %v, want %v (Conflict)", status.Code(err), codes.AlreadyExists)
	}
}

func TestRequestToJoin_DoubleRequestRejected(t *testing.T) {
	svc, meetups, _, _, publisher, sender := newTestService()
	ctx := context.Background()

	meetups.put(repository.Meetup{ID: "meetup-1", HostUserID: "host-1", Intent: repository.IntentCoffee, Status: repository.MeetupStatusOpen, Capacity: 2})

	req := &meetupv1.RequestToJoinRequest{MeetupId: "meetup-1", RequesterId: "requester-1", RequesterTrustLevel: 2}
	if _, err := svc.RequestToJoin(ctx, req); err != nil {
		t.Fatalf("first RequestToJoin() error: %v", err)
	}
	if _, err := svc.RequestToJoin(ctx, req); status.Code(err) != codes.AlreadyExists {
		t.Fatalf("second RequestToJoin() code = %v, want %v", status.Code(err), codes.AlreadyExists)
	}

	if publisher.createdCalls != 1 {
		t.Errorf("PublishRequestCreated calls = %d, want 1 (only the first, successful request)", publisher.createdCalls)
	}
	if sender.calls != 1 {
		t.Errorf("push notification calls = %d, want 1", sender.calls)
	}
}

func TestRespondToRequest_OnlyHostCanRespond(t *testing.T) {
	svc, meetups, requests, _, _, _ := newTestService()
	ctx := context.Background()

	meetups.put(repository.Meetup{ID: "meetup-1", HostUserID: "host-1", Intent: repository.IntentCoffee, Status: repository.MeetupStatusOpen, Capacity: 2})
	requests.put(repository.MeetupRequest{ID: "request-1", MeetupID: "meetup-1", RequesterID: "requester-1", Status: repository.RequestStatusPending})

	_, err := svc.RespondToRequest(ctx, &meetupv1.RespondToRequestRequest{RequestId: "request-1", HostUserId: "not-the-host", Accept: true})
	if status.Code(err) != codes.PermissionDenied {
		t.Fatalf("RespondToRequest() code = %v, want %v", status.Code(err), codes.PermissionDenied)
	}
}

func TestRespondToRequest_AcceptAndReject(t *testing.T) {
	t.Run("accept publishes accepted event, notifies requester, ensures safety state", func(t *testing.T) {
		svc, meetups, requests, safety, publisher, sender := newTestService()
		ctx := context.Background()

		meetups.put(repository.Meetup{ID: "meetup-1", HostUserID: "host-1", Intent: repository.IntentCoffee, Status: repository.MeetupStatusOpen, Capacity: 2})
		requests.put(repository.MeetupRequest{ID: "request-1", MeetupID: "meetup-1", RequesterID: "requester-1", Status: repository.RequestStatusPending})

		resp, err := svc.RespondToRequest(ctx, &meetupv1.RespondToRequestRequest{RequestId: "request-1", HostUserId: "host-1", Accept: true})
		if err != nil {
			t.Fatalf("RespondToRequest() error: %v", err)
		}
		if resp.GetStatus() != meetupv1.MeetupRequestStatus_MEETUP_REQUEST_STATUS_ACCEPTED {
			t.Errorf("status = %v, want ACCEPTED", resp.GetStatus())
		}
		if publisher.acceptedCalls != 1 {
			t.Errorf("PublishRequestAccepted calls = %d, want 1", publisher.acceptedCalls)
		}
		if sender.calls != 1 {
			t.Errorf("push notification calls = %d, want 1", sender.calls)
		}
		if _, err := safety.Get(ctx, "meetup-1"); err != nil {
			t.Errorf("safety state was not created after accept: %v", err)
		}
	})

	t.Run("reject publishes rejected event with auto_rejected=false, notifies requester", func(t *testing.T) {
		svc, meetups, requests, _, publisher, sender := newTestService()
		ctx := context.Background()

		meetups.put(repository.Meetup{ID: "meetup-1", HostUserID: "host-1", Intent: repository.IntentCoffee, Status: repository.MeetupStatusOpen, Capacity: 2})
		requests.put(repository.MeetupRequest{ID: "request-1", MeetupID: "meetup-1", RequesterID: "requester-1", Status: repository.RequestStatusPending})

		resp, err := svc.RespondToRequest(ctx, &meetupv1.RespondToRequestRequest{RequestId: "request-1", HostUserId: "host-1", Accept: false})
		if err != nil {
			t.Fatalf("RespondToRequest() error: %v", err)
		}
		if resp.GetStatus() != meetupv1.MeetupRequestStatus_MEETUP_REQUEST_STATUS_REJECTED {
			t.Errorf("status = %v, want REJECTED", resp.GetStatus())
		}
		if publisher.rejectedCalls != 1 {
			t.Errorf("PublishRequestRejected calls = %d, want 1", publisher.rejectedCalls)
		}
		if publisher.lastRejectedAutoReject {
			t.Errorf("auto_rejected = true, want false (host explicit rejection)")
		}
		if sender.calls != 1 {
			t.Errorf("push notification calls = %d, want 1", sender.calls)
		}
	})

	t.Run("cannot respond to an already-resolved request", func(t *testing.T) {
		svc, meetups, requests, _, _, _ := newTestService()
		ctx := context.Background()

		meetups.put(repository.Meetup{ID: "meetup-1", HostUserID: "host-1", Intent: repository.IntentCoffee, Status: repository.MeetupStatusOpen, Capacity: 2})
		requests.put(repository.MeetupRequest{ID: "request-1", MeetupID: "meetup-1", RequesterID: "requester-1", Status: repository.RequestStatusAccepted})

		_, err := svc.RespondToRequest(ctx, &meetupv1.RespondToRequestRequest{RequestId: "request-1", HostUserId: "host-1", Accept: true})
		if status.Code(err) != codes.AlreadyExists {
			t.Fatalf("RespondToRequest() code = %v, want %v (Conflict)", status.Code(err), codes.AlreadyExists)
		}
	})
}

func TestWithdrawRequest_OnlyRequesterCanWithdraw(t *testing.T) {
	svc, _, requests, _, _, _ := newTestService()
	ctx := context.Background()

	requests.put(repository.MeetupRequest{ID: "request-1", MeetupID: "meetup-1", RequesterID: "requester-1", Status: repository.RequestStatusPending})

	_, err := svc.WithdrawRequest(ctx, &meetupv1.WithdrawRequestRequest{RequestId: "request-1", RequesterId: "someone-else"})
	if status.Code(err) != codes.PermissionDenied {
		t.Fatalf("WithdrawRequest() code = %v, want %v", status.Code(err), codes.PermissionDenied)
	}
}

func TestCancelMeetup(t *testing.T) {
	t.Run("only the host can cancel", func(t *testing.T) {
		svc, meetups, _, _, _, _ := newTestService()
		ctx := context.Background()
		meetups.put(repository.Meetup{ID: "meetup-1", HostUserID: "host-1", Status: repository.MeetupStatusOpen})

		_, err := svc.CancelMeetup(ctx, &meetupv1.CancelMeetupRequest{MeetupId: "meetup-1", HostUserId: "not-the-host"})
		if status.Code(err) != codes.PermissionDenied {
			t.Fatalf("CancelMeetup() code = %v, want %v", status.Code(err), codes.PermissionDenied)
		}
	})

	t.Run("cannot cancel a meetup with accepted participants", func(t *testing.T) {
		svc, meetups, _, _, _, _ := newTestService()
		ctx := context.Background()
		meetups.put(repository.Meetup{ID: "meetup-1", HostUserID: "host-1", Status: repository.MeetupStatusOpen, AcceptedCount: 1})

		_, err := svc.CancelMeetup(ctx, &meetupv1.CancelMeetupRequest{MeetupId: "meetup-1", HostUserId: "host-1"})
		if status.Code(err) != codes.AlreadyExists {
			t.Fatalf("CancelMeetup() code = %v, want %v (Conflict)", status.Code(err), codes.AlreadyExists)
		}
	})

	t.Run("host can cancel a meetup with no accepted participants", func(t *testing.T) {
		svc, meetups, _, _, _, _ := newTestService()
		ctx := context.Background()
		meetups.put(repository.Meetup{ID: "meetup-1", HostUserID: "host-1", Status: repository.MeetupStatusOpen})

		resp, err := svc.CancelMeetup(ctx, &meetupv1.CancelMeetupRequest{MeetupId: "meetup-1", HostUserId: "host-1"})
		if err != nil {
			t.Fatalf("CancelMeetup() error: %v", err)
		}
		if !resp.GetSuccess() {
			t.Errorf("success = false, want true")
		}
	})
}

func TestCheckIn_RequiresChecklistAcknowledgedFirst(t *testing.T) {
	svc, _, _, safety, _, _ := newTestService()
	ctx := context.Background()

	if err := safety.EnsureExists(ctx, "meetup-1"); err != nil {
		t.Fatalf("EnsureExists() error: %v", err)
	}

	_, err := svc.CheckIn(ctx, &meetupv1.CheckInRequest{MeetupId: "meetup-1", UserId: "user-1"})
	if status.Code(err) != codes.AlreadyExists {
		t.Fatalf("CheckIn() before checklist ack: code = %v, want %v (Conflict)", status.Code(err), codes.AlreadyExists)
	}

	if _, err := svc.AcknowledgeSafetyChecklist(ctx, &meetupv1.AcknowledgeSafetyChecklistRequest{MeetupId: "meetup-1", UserId: "user-1"}); err != nil {
		t.Fatalf("AcknowledgeSafetyChecklist() error: %v", err)
	}

	resp, err := svc.CheckIn(ctx, &meetupv1.CheckInRequest{MeetupId: "meetup-1", UserId: "user-1"})
	if err != nil {
		t.Fatalf("CheckIn() after checklist ack: error: %v", err)
	}
	if resp.GetCheckedInAtUnixSeconds() == 0 {
		t.Errorf("checked_in_at_unix_seconds not set")
	}
}

func TestGetSafetyState_NotFoundBeforeFirstAcceptedRequest(t *testing.T) {
	svc, _, _, _, _, _ := newTestService()
	ctx := context.Background()

	_, err := svc.GetSafetyState(ctx, &meetupv1.GetSafetyStateRequest{MeetupId: "meetup-1"})
	if status.Code(err) != codes.NotFound {
		t.Fatalf("GetSafetyState() code = %v, want %v", status.Code(err), codes.NotFound)
	}
}

func TestGetSafetyState_ReflectsPriorAcknowledgment(t *testing.T) {
	svc, _, _, safety, _, _ := newTestService()
	ctx := context.Background()

	if err := safety.EnsureExists(ctx, "meetup-1"); err != nil {
		t.Fatalf("EnsureExists() error: %v", err)
	}
	if _, err := svc.AcknowledgeSafetyChecklist(ctx, &meetupv1.AcknowledgeSafetyChecklistRequest{MeetupId: "meetup-1", UserId: "user-1"}); err != nil {
		t.Fatalf("AcknowledgeSafetyChecklist() error: %v", err)
	}

	resp, err := svc.GetSafetyState(ctx, &meetupv1.GetSafetyStateRequest{MeetupId: "meetup-1"})
	if err != nil {
		t.Fatalf("GetSafetyState() error: %v", err)
	}
	if resp.GetChecklistAckAtUnixSeconds() == 0 {
		t.Errorf("checklist_ack_at_unix_seconds not reflected in GetSafetyState()")
	}
}

func TestSubmitMeetupFeedback_NilsOutOptionalFieldsWhenNotHappened(t *testing.T) {
	svc, _, _, _, _, _ := newTestService()
	ctx := context.Background()

	feltSafe := true
	resp, err := svc.SubmitMeetupFeedback(ctx, &meetupv1.SubmitMeetupFeedbackRequest{
		MeetupId: "meetup-1", UserId: "user-1", Happened: false, FeltSafe: &feltSafe,
	})
	if err != nil {
		t.Fatalf("SubmitMeetupFeedback() error: %v", err)
	}
	if !resp.GetSuccess() {
		t.Errorf("success = false, want true")
	}
}

// TestRequestToJoin_NotificationUsesRequesterFullName is a regression test:
// RequestToJoin once built its "wants to join" notification body straight
// from Create()'s return value, which — like the real Postgres repository
// — carries no requester display info, producing a body like " wants to
// join your coffee meetup" with the name silently blank. Caught during a
// live end-to-end pass against the real stack, fixed by re-fetching the
// joined view via GetByID before building the notification.
func TestRequestToJoin_NotificationUsesRequesterFullName(t *testing.T) {
	svc, meetups, _, _, _, sender := newTestService()
	ctx := context.Background()
	sender.lastBody = ""

	meetups.put(repository.Meetup{ID: "meetup-1", HostUserID: "host-1", Intent: repository.IntentCoffee, Status: repository.MeetupStatusOpen, Capacity: 2})

	if _, err := svc.RequestToJoin(ctx, &meetupv1.RequestToJoinRequest{MeetupId: "meetup-1", RequesterId: "requester-1", RequesterTrustLevel: 2}); err != nil {
		t.Fatalf("RequestToJoin() error: %v", err)
	}

	if !strings.Contains(sender.lastBody, "Test Requester") {
		t.Errorf("notification body = %q, want it to contain the requester's name", sender.lastBody)
	}
}

// A failing push notification must not fail the RPC that already
// succeeded — backend/meetup-scheduling-PLAN.md's Test strategy.
func TestRequestToJoin_NotificationFailureDoesNotFailTheRequest(t *testing.T) {
	svc, meetups, _, _, _, sender := newTestService()
	ctx := context.Background()
	sender.err = context.DeadlineExceeded

	meetups.put(repository.Meetup{ID: "meetup-1", HostUserID: "host-1", Intent: repository.IntentCoffee, Status: repository.MeetupStatusOpen, Capacity: 2})

	resp, err := svc.RequestToJoin(ctx, &meetupv1.RequestToJoinRequest{MeetupId: "meetup-1", RequesterId: "requester-1", RequesterTrustLevel: 2})
	if err != nil {
		t.Fatalf("RequestToJoin() error: %v, want success despite the notification failure", err)
	}
	if resp.GetStatus() != meetupv1.MeetupRequestStatus_MEETUP_REQUEST_STATUS_PENDING {
		t.Errorf("status = %v, want PENDING", resp.GetStatus())
	}
}
