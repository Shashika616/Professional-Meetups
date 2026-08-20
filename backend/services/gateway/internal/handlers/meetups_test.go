package handlers

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	sharedjwt "github.com/professional-connections/backend/shared/jwt"

	"github.com/professional-connections/backend/services/gateway/internal/meetupclient"
)

// TestMeetupRoutesRequireAuth confirms every new /v1/meetups/* route is
// rejected with 401 when the Authorization header is missing, and that the
// underlying handler/meetupclient is never reached — same discipline as
// TestVerificationRoutesRequireAuth.
func TestMeetupRoutesRequireAuth(t *testing.T) {
	routes := []struct {
		method string
		path   string
		body   string
	}{
		{http.MethodPost, "/v1/meetups", `{"intent":"coffee","capacity":2}`},
		{http.MethodGet, "/v1/meetups?intent=coffee", ""},
		{http.MethodGet, "/v1/meetups/mine", ""},
		{http.MethodGet, "/v1/meetups/meetup-1", ""},
		{http.MethodPost, "/v1/meetups/meetup-1/close", ""},
		{http.MethodPost, "/v1/meetups/meetup-1/cancel", ""},
		{http.MethodGet, "/v1/meetups/meetup-1/requests", ""},
		{http.MethodPost, "/v1/meetups/meetup-1/requests", ""},
		{http.MethodPost, "/v1/meetups/requests/request-1/withdraw", ""},
		{http.MethodPost, "/v1/meetups/requests/request-1/respond", `{"accept":true}`},
		{http.MethodPost, "/v1/meetups/device-token", `{"fcm_token":"tok"}`},
		{http.MethodGet, "/v1/meetups/meetup-1/safety", ""},
		{http.MethodPost, "/v1/meetups/meetup-1/safety/checklist", ""},
		{http.MethodPost, "/v1/meetups/meetup-1/safety/live-location", `{"opt_in":true}`},
		{http.MethodPost, "/v1/meetups/meetup-1/safety/check-in", ""},
		{http.MethodPost, "/v1/meetups/meetup-1/feedback", `{"happened":true}`},
	}

	_, verifier := testAuth(t)

	for _, rt := range routes {
		t.Run(rt.method+" "+rt.path, func(t *testing.T) {
			h := New(&fakeAuthClient{}, newNeverCalledMeetupClient(), verifier)
			mux := http.NewServeMux()
			h.Register(mux)

			req := httptest.NewRequest(rt.method, rt.path, bytes.NewBufferString(rt.body))
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)

			if rec.Code != http.StatusUnauthorized {
				t.Errorf("status = %d, want %d (body: %s)", rec.Code, http.StatusUnauthorized, rec.Body.String())
			}
		})
	}
}

// bearerWithTrustLevel mirrors handlers_test.go's bearer() but with a
// caller-chosen trust level — needed to verify the gateway actually
// forwards the JWT's trust_level claim to the meetup client, since bearer()
// always signs trust level 1.
func bearerWithTrustLevel(t *testing.T, req *http.Request, signer *sharedjwt.Signer, userID string, trustLevel int) *http.Request {
	t.Helper()
	token, err := signer.Sign(sharedjwt.Claims{UserID: userID, TrustLevel: trustLevel})
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	return req
}

func TestCreateMeetup_ForwardsUserIDAndTrustLevelFromToken(t *testing.T) {
	signer, verifier := testAuth(t)

	var gotHostID string
	var gotTrustLevel int32
	fake := &fakeMeetupClient{
		createMeetupFn: func(_ context.Context, hostUserID string, hostTrustLevel int32, intent string, _, _ int64, lat, lng float64, label string, capacity int32) (meetupclient.Meetup, error) {
			gotHostID = hostUserID
			gotTrustLevel = hostTrustLevel
			return meetupclient.Meetup{
				ID: "meetup-1", HostUserID: hostUserID, HostFullName: "Ada Lovelace",
				HostTrustLevel: hostTrustLevel, Intent: intent, LocationLat: lat, LocationLng: lng,
				LocationLabel: label, Capacity: capacity, Status: "open",
			}, nil
		},
	}
	h := New(&fakeAuthClient{}, fake, verifier)
	mux := http.NewServeMux()
	h.Register(mux)

	body := `{"intent":"coffee","location_lat":6.9,"location_lng":79.8,"location_label":"Cafe","capacity":3}`
	req := bearerWithTrustLevel(t, httptest.NewRequest(http.MethodPost, "/v1/meetups", bytes.NewBufferString(body)), signer, "user-42", 2)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d (body: %s)", rec.Code, http.StatusOK, rec.Body.String())
	}
	if gotHostID != "user-42" {
		t.Errorf("host_user_id passed to meetup client = %q, want %q", gotHostID, "user-42")
	}
	if gotTrustLevel != 2 {
		t.Errorf("trust_level passed to meetup client = %d, want %d", gotTrustLevel, 2)
	}
	assertJSONBody(t, rec.Body.Bytes(), map[string]any{
		"id":           "meetup-1",
		"host_user_id": "user-42",
		"intent":       "coffee",
		"status":       "open",
	})
}

// TestCreateMeetup_TrustGateRejectionMapsTo403 confirms a PermissionDenied
// from the service layer (the trust-level gate) surfaces as a real 403, not
// a generic error — distinct from the 401 an unauthenticated call gets.
func TestCreateMeetup_TrustGateRejectionMapsTo403(t *testing.T) {
	signer, verifier := testAuth(t)

	fake := &fakeMeetupClient{
		createMeetupFn: func(context.Context, string, int32, string, int64, int64, float64, float64, string, int32) (meetupclient.Meetup, error) {
			return meetupclient.Meetup{}, status.Error(codes.PermissionDenied, "intent \"coffee\" requires trust level 2, caller has 1")
		},
	}
	h := New(&fakeAuthClient{}, fake, verifier)
	mux := http.NewServeMux()
	h.Register(mux)

	body := `{"intent":"coffee","capacity":2}`
	req := bearerWithTrustLevel(t, httptest.NewRequest(http.MethodPost, "/v1/meetups", bytes.NewBufferString(body)), signer, "user-1", 1)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("status = %d, want %d (body: %s)", rec.Code, http.StatusForbidden, rec.Body.String())
	}
}

// TestCloseMeetup_ForwardsHostUserIDFromToken confirms host_user_id is
// taken from the verified JWT (middleware.UserIDFromContext), never from
// the request body or path — same discipline as every other meetup RPC
// (ADR-016).
func TestCloseMeetup_ForwardsHostUserIDFromToken(t *testing.T) {
	signer, verifier := testAuth(t)

	var gotMeetupID, gotHostID string
	fake := &fakeMeetupClient{
		closeMeetupFn: func(_ context.Context, meetupID, hostUserID string) (meetupclient.Meetup, error) {
			gotMeetupID = meetupID
			gotHostID = hostUserID
			return meetupclient.Meetup{
				ID: meetupID, HostUserID: hostUserID, Status: "completed",
			}, nil
		},
	}
	h := New(&fakeAuthClient{}, fake, verifier)
	mux := http.NewServeMux()
	h.Register(mux)

	req := bearer(t, httptest.NewRequest(http.MethodPost, "/v1/meetups/meetup-1/close", nil), signer, "host-1")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d (body: %s)", rec.Code, http.StatusOK, rec.Body.String())
	}
	if gotMeetupID != "meetup-1" {
		t.Errorf("meetup id passed to meetup client = %q, want %q", gotMeetupID, "meetup-1")
	}
	if gotHostID != "host-1" {
		t.Errorf("host_user_id passed to meetup client = %q, want %q", gotHostID, "host-1")
	}
	assertJSONBody(t, rec.Body.Bytes(), map[string]any{
		"id":     "meetup-1",
		"status": "completed",
	})
}

// TestCloseMeetup_ConflictMapsTo409 confirms an already-closed/cancelled
// meetup surfaces as a real 409, not a generic error.
func TestCloseMeetup_ConflictMapsTo409(t *testing.T) {
	signer, verifier := testAuth(t)

	fake := &fakeMeetupClient{
		closeMeetupFn: func(context.Context, string, string) (meetupclient.Meetup, error) {
			return meetupclient.Meetup{}, status.Error(codes.AlreadyExists, "meetup: already closed or cancelled")
		},
	}
	h := New(&fakeAuthClient{}, fake, verifier)
	mux := http.NewServeMux()
	h.Register(mux)

	req := bearer(t, httptest.NewRequest(http.MethodPost, "/v1/meetups/meetup-1/close", nil), signer, "host-1")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusConflict {
		t.Errorf("status = %d, want %d (body: %s)", rec.Code, http.StatusConflict, rec.Body.String())
	}
}

func TestRespondToRequest_AcceptFlag(t *testing.T) {
	signer, verifier := testAuth(t)

	var gotAccept bool
	fake := &fakeMeetupClient{
		respondToRequestFn: func(_ context.Context, requestID, hostUserID string, accept bool) (meetupclient.MeetupRequest, error) {
			gotAccept = accept
			return meetupclient.MeetupRequest{ID: requestID, Status: "accepted"}, nil
		},
	}
	h := New(&fakeAuthClient{}, fake, verifier)
	mux := http.NewServeMux()
	h.Register(mux)

	req := bearer(t, httptest.NewRequest(http.MethodPost, "/v1/meetups/requests/request-1/respond", bytes.NewBufferString(`{"accept":true}`)), signer, "host-1")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d (body: %s)", rec.Code, http.StatusOK, rec.Body.String())
	}
	if !gotAccept {
		t.Errorf("accept flag not forwarded to meetup client")
	}
}

// TestRespondToRequest_NonHostRejectionMapsTo403 confirms the gateway
// correctly surfaces the service layer's host-ownership check.
func TestRespondToRequest_NonHostRejectionMapsTo403(t *testing.T) {
	signer, verifier := testAuth(t)

	fake := &fakeMeetupClient{
		respondToRequestFn: func(context.Context, string, string, bool) (meetupclient.MeetupRequest, error) {
			return meetupclient.MeetupRequest{}, status.Error(codes.PermissionDenied, "caller does not host this meetup")
		},
	}
	h := New(&fakeAuthClient{}, fake, verifier)
	mux := http.NewServeMux()
	h.Register(mux)

	req := bearer(t, httptest.NewRequest(http.MethodPost, "/v1/meetups/requests/request-1/respond", bytes.NewBufferString(`{"accept":true}`)), signer, "not-the-host")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("status = %d, want %d (body: %s)", rec.Code, http.StatusForbidden, rec.Body.String())
	}
}
