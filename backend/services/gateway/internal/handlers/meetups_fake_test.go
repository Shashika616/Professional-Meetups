package handlers

import (
	"context"

	"github.com/professional-connections/backend/services/gateway/internal/meetupclient"
)

// fakeMeetupClient is a hand-written test double implementing
// meetupclient.Client, so handler tests don't need a running meetup
// service. Every field is optional — tests that don't exercise a given
// route simply leave the corresponding field nil (calling it panics with a
// nil-function-call error, same as fakeAuthClient's pattern).
type fakeMeetupClient struct {
	createMeetupFn               func(ctx context.Context, hostUserID string, hostTrustLevel int32, intent string, windowStartUnixSeconds, windowEndUnixSeconds int64, lat, lng float64, locationLabel string, capacity int32) (meetupclient.Meetup, error)
	listOpenMeetupsFn            func(ctx context.Context, userID, intent, cursor string, pageSize int32) ([]meetupclient.Meetup, string, error)
	getMeetupFn                  func(ctx context.Context, meetupID, userID string) (meetupclient.Meetup, error)
	listMyMeetupsFn              func(ctx context.Context, userID string) ([]meetupclient.Meetup, []meetupclient.Meetup, error)
	listMeetupRequestsFn         func(ctx context.Context, meetupID, hostUserID string) ([]meetupclient.MeetupRequest, error)
	requestToJoinFn              func(ctx context.Context, meetupID, requesterID string, requesterTrustLevel int32) (meetupclient.MeetupRequest, error)
	withdrawRequestFn            func(ctx context.Context, requestID, requesterID string) error
	respondToRequestFn           func(ctx context.Context, requestID, hostUserID string, accept bool) (meetupclient.MeetupRequest, error)
	registerDeviceTokenFn        func(ctx context.Context, userID, fcmToken string) error
	getSafetyStateFn             func(ctx context.Context, meetupID string) (meetupclient.SafetyState, error)
	acknowledgeSafetyChecklistFn func(ctx context.Context, meetupID, userID string) (meetupclient.SafetyState, error)
	setLiveLocationOptInFn       func(ctx context.Context, meetupID, userID string, optIn bool) (meetupclient.SafetyState, error)
	checkInFn                    func(ctx context.Context, meetupID, userID string) (meetupclient.SafetyState, error)
	submitMeetupFeedbackFn       func(ctx context.Context, meetupID, userID string, happened bool, feltSafe, profileAccurate, wouldMeetAgain *bool, notes *string) error
	listRatableParticipantsFn    func(ctx context.Context, meetupID, viewerID string) ([]meetupclient.RatableParticipant, error)
	submitRatingFn               func(ctx context.Context, meetupID, raterUserID, ratedUserID string, score int32) error
	closeMeetupFn                func(ctx context.Context, meetupID, hostUserID string) (meetupclient.Meetup, error)
	cancelMeetupFn               func(ctx context.Context, meetupID, hostUserID string) error
}

func (f *fakeMeetupClient) CreateMeetup(ctx context.Context, hostUserID string, hostTrustLevel int32, intent string, windowStartUnixSeconds, windowEndUnixSeconds int64, lat, lng float64, locationLabel string, capacity int32) (meetupclient.Meetup, error) {
	return f.createMeetupFn(ctx, hostUserID, hostTrustLevel, intent, windowStartUnixSeconds, windowEndUnixSeconds, lat, lng, locationLabel, capacity)
}

func (f *fakeMeetupClient) ListOpenMeetups(ctx context.Context, userID, intent, cursor string, pageSize int32) ([]meetupclient.Meetup, string, error) {
	return f.listOpenMeetupsFn(ctx, userID, intent, cursor, pageSize)
}

func (f *fakeMeetupClient) GetMeetup(ctx context.Context, meetupID, userID string) (meetupclient.Meetup, error) {
	return f.getMeetupFn(ctx, meetupID, userID)
}

func (f *fakeMeetupClient) ListMyMeetups(ctx context.Context, userID string) ([]meetupclient.Meetup, []meetupclient.Meetup, error) {
	return f.listMyMeetupsFn(ctx, userID)
}

func (f *fakeMeetupClient) ListMeetupRequests(ctx context.Context, meetupID, hostUserID string) ([]meetupclient.MeetupRequest, error) {
	return f.listMeetupRequestsFn(ctx, meetupID, hostUserID)
}

func (f *fakeMeetupClient) RequestToJoin(ctx context.Context, meetupID, requesterID string, requesterTrustLevel int32) (meetupclient.MeetupRequest, error) {
	return f.requestToJoinFn(ctx, meetupID, requesterID, requesterTrustLevel)
}

func (f *fakeMeetupClient) WithdrawRequest(ctx context.Context, requestID, requesterID string) error {
	return f.withdrawRequestFn(ctx, requestID, requesterID)
}

func (f *fakeMeetupClient) RespondToRequest(ctx context.Context, requestID, hostUserID string, accept bool) (meetupclient.MeetupRequest, error) {
	return f.respondToRequestFn(ctx, requestID, hostUserID, accept)
}

func (f *fakeMeetupClient) RegisterDeviceToken(ctx context.Context, userID, fcmToken string) error {
	return f.registerDeviceTokenFn(ctx, userID, fcmToken)
}

func (f *fakeMeetupClient) GetSafetyState(ctx context.Context, meetupID string) (meetupclient.SafetyState, error) {
	return f.getSafetyStateFn(ctx, meetupID)
}

func (f *fakeMeetupClient) AcknowledgeSafetyChecklist(ctx context.Context, meetupID, userID string) (meetupclient.SafetyState, error) {
	return f.acknowledgeSafetyChecklistFn(ctx, meetupID, userID)
}

func (f *fakeMeetupClient) SetLiveLocationOptIn(ctx context.Context, meetupID, userID string, optIn bool) (meetupclient.SafetyState, error) {
	return f.setLiveLocationOptInFn(ctx, meetupID, userID, optIn)
}

func (f *fakeMeetupClient) CheckIn(ctx context.Context, meetupID, userID string) (meetupclient.SafetyState, error) {
	return f.checkInFn(ctx, meetupID, userID)
}

func (f *fakeMeetupClient) SubmitMeetupFeedback(ctx context.Context, meetupID, userID string, happened bool, feltSafe, profileAccurate, wouldMeetAgain *bool, notes *string) error {
	return f.submitMeetupFeedbackFn(ctx, meetupID, userID, happened, feltSafe, profileAccurate, wouldMeetAgain, notes)
}

func (f *fakeMeetupClient) ListRatableParticipants(ctx context.Context, meetupID, viewerID string) ([]meetupclient.RatableParticipant, error) {
	return f.listRatableParticipantsFn(ctx, meetupID, viewerID)
}

func (f *fakeMeetupClient) SubmitRating(ctx context.Context, meetupID, raterUserID, ratedUserID string, score int32) error {
	return f.submitRatingFn(ctx, meetupID, raterUserID, ratedUserID, score)
}

func (f *fakeMeetupClient) CloseMeetup(ctx context.Context, meetupID, hostUserID string) (meetupclient.Meetup, error) {
	return f.closeMeetupFn(ctx, meetupID, hostUserID)
}

func (f *fakeMeetupClient) CancelMeetup(ctx context.Context, meetupID, hostUserID string) error {
	return f.cancelMeetupFn(ctx, meetupID, hostUserID)
}

func (f *fakeMeetupClient) Close() error { return nil }

// newNeverCalledMeetupClient panics if any method is invoked — used by
// tests that don't exercise any /v1/meetups/* route, mirroring
// newNeverCalledAuthClient's pattern in verification_test.go.
func newNeverCalledMeetupClient() *fakeMeetupClient {
	panicMsg := "should not be called: this test doesn't exercise any meetup route"
	return &fakeMeetupClient{
		createMeetupFn: func(context.Context, string, int32, string, int64, int64, float64, float64, string, int32) (meetupclient.Meetup, error) {
			panic(panicMsg)
		},
		listOpenMeetupsFn: func(context.Context, string, string, string, int32) ([]meetupclient.Meetup, string, error) {
			panic(panicMsg)
		},
		getMeetupFn: func(context.Context, string, string) (meetupclient.Meetup, error) { panic(panicMsg) },
		listMyMeetupsFn: func(context.Context, string) ([]meetupclient.Meetup, []meetupclient.Meetup, error) {
			panic(panicMsg)
		},
		listMeetupRequestsFn: func(context.Context, string, string) ([]meetupclient.MeetupRequest, error) {
			panic(panicMsg)
		},
		requestToJoinFn: func(context.Context, string, string, int32) (meetupclient.MeetupRequest, error) {
			panic(panicMsg)
		},
		withdrawRequestFn: func(context.Context, string, string) error { panic(panicMsg) },
		respondToRequestFn: func(context.Context, string, string, bool) (meetupclient.MeetupRequest, error) {
			panic(panicMsg)
		},
		registerDeviceTokenFn: func(context.Context, string, string) error { panic(panicMsg) },
		getSafetyStateFn: func(context.Context, string) (meetupclient.SafetyState, error) {
			panic(panicMsg)
		},
		acknowledgeSafetyChecklistFn: func(context.Context, string, string) (meetupclient.SafetyState, error) {
			panic(panicMsg)
		},
		setLiveLocationOptInFn: func(context.Context, string, string, bool) (meetupclient.SafetyState, error) {
			panic(panicMsg)
		},
		checkInFn: func(context.Context, string, string) (meetupclient.SafetyState, error) { panic(panicMsg) },
		submitMeetupFeedbackFn: func(context.Context, string, string, bool, *bool, *bool, *bool, *string) error {
			panic(panicMsg)
		},
		listRatableParticipantsFn: func(context.Context, string, string) ([]meetupclient.RatableParticipant, error) {
			panic(panicMsg)
		},
		submitRatingFn: func(context.Context, string, string, string, int32) error { panic(panicMsg) },
		closeMeetupFn: func(context.Context, string, string) (meetupclient.Meetup, error) {
			panic(panicMsg)
		},
		cancelMeetupFn: func(context.Context, string, string) error { panic(panicMsg) },
	}
}
