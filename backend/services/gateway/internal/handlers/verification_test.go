package handlers

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/professional-connections/backend/services/gateway/internal/authclient"
)

// newNeverCalledAuthClient panics if any method is invoked — used to prove
// the auth middleware rejects a request before the handler (and therefore
// the authclient) is ever reached.
func newNeverCalledAuthClient() *fakeAuthClient {
	f := &fakeAuthClient{}
	f.startPhoneFn = func(context.Context, string, string) (int32, error) {
		panic("should not be called: request should have been rejected by the auth middleware")
	}
	f.verifyPhoneFn = func(context.Context, string, string, string) (authclient.Session, error) {
		panic("should not be called")
	}
	f.startPersonalEmailFn = func(context.Context, string, string) (int32, error) {
		panic("should not be called")
	}
	f.verifyPersonalFn = func(context.Context, string, string, string) (authclient.Session, error) {
		panic("should not be called")
	}
	f.submitDetailsFn = func(context.Context, string, string, string) (authclient.Session, error) {
		panic("should not be called")
	}
	f.startCorporateFn = func(context.Context, string, string) (int32, error) {
		panic("should not be called")
	}
	f.verifyCorporateFn = func(context.Context, string, string, string) (authclient.Session, error) {
		panic("should not be called")
	}
	f.getProfileFn = func(context.Context, string) (authclient.Profile, error) {
		panic("should not be called")
	}
	return f
}

// TestVerificationRoutesRequireAuth confirms every new /v1/verification/*
// and /v1/users/me route is rejected with 401 when the Authorization
// header is missing, and — critically — that the underlying handler/
// authclient is never reached (backend/PLAN.md's Level 2/3 addendum, Step
// F's self-review checklist item).
func TestVerificationRoutesRequireAuth(t *testing.T) {
	routes := []struct {
		method string
		path   string
		body   string
	}{
		{http.MethodPost, "/v1/verification/phone/start", `{"phone_number":"+94771234567"}`},
		{http.MethodPost, "/v1/verification/phone/verify", `{"phone_number":"+94771234567","code":"123456"}`},
		{http.MethodPost, "/v1/verification/personal-email/start", `{"email":"a@example.com"}`},
		{http.MethodPost, "/v1/verification/personal-email/verify", `{"email":"a@example.com","code":"123456"}`},
		{http.MethodPost, "/v1/verification/personal-details", `{"legal_name":"A B","address":"1 Main St"}`},
		{http.MethodPost, "/v1/verification/corporate-email/start", `{"email":"a@company.com"}`},
		{http.MethodPost, "/v1/verification/corporate-email/verify", `{"email":"a@company.com","code":"123456"}`},
		{http.MethodGet, "/v1/users/me", ""},
	}

	_, verifier := testAuth(t)

	for _, rt := range routes {
		t.Run(rt.path, func(t *testing.T) {
			h := New(newNeverCalledAuthClient(), verifier)
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

func TestGetProfile(t *testing.T) {
	signer, verifier := testAuth(t)

	t.Run("valid token returns the profile for that user, no raw contact info", func(t *testing.T) {
		var gotUserID string
		fake := &fakeAuthClient{
			getProfileFn: func(_ context.Context, userID string) (authclient.Profile, error) {
				gotUserID = userID
				return authclient.Profile{
					UserID:                  userID,
					FullName:                "Ada Lovelace",
					TrustLevel:              2,
					PhoneVerified:           true,
					PersonalEmailVerified:   true,
					PersonalDetailsComplete: true,
					WorkEmailVerified:       false,
				}, nil
			},
		}
		h := New(fake, verifier)
		mux := http.NewServeMux()
		h.Register(mux)

		req := bearer(t, httptest.NewRequest(http.MethodGet, "/v1/users/me", nil), signer, "user-42")
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d (body: %s)", rec.Code, http.StatusOK, rec.Body.String())
		}
		if gotUserID != "user-42" {
			t.Errorf("authclient.GetProfile called with userID %q, want %q", gotUserID, "user-42")
		}
		assertJSONBody(t, rec.Body.Bytes(), map[string]any{
			"user_id":                   "user-42",
			"full_name":                 "Ada Lovelace",
			"trust_level":               float64(2),
			"phone_verified":            true,
			"personal_email_verified":   true,
			"personal_details_complete": true,
			"work_email_verified":       false,
		})
		// No raw phone number or email address field anywhere in the
		// response body (Verification Model § 1).
		if bytes.Contains(rec.Body.Bytes(), []byte("phone_number")) || bytes.Contains(rec.Body.Bytes(), []byte("\"email\"")) {
			t.Errorf("response body contains a raw contact field: %s", rec.Body.String())
		}
	})
}

func TestStartPhoneVerification_ResendCooldownMapsTo429(t *testing.T) {
	signer, verifier := testAuth(t)

	fake := &fakeAuthClient{
		startPhoneFn: func(context.Context, string, string) (int32, error) {
			return 0, status.Error(codes.ResourceExhausted, "please wait before requesting another code")
		},
	}
	h := New(fake, verifier)
	mux := http.NewServeMux()
	h.Register(mux)

	req := bearer(t, httptest.NewRequest(http.MethodPost, "/v1/verification/phone/start", bytes.NewBufferString(`{"phone_number":"+94771234567"}`)), signer, "user-1")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusTooManyRequests {
		t.Errorf("status = %d, want %d (body: %s)", rec.Code, http.StatusTooManyRequests, rec.Body.String())
	}
}

func TestStartCorporateEmailVerification_DomainRejectionMapsTo400(t *testing.T) {
	signer, verifier := testAuth(t)

	fake := &fakeAuthClient{
		startCorporateFn: func(context.Context, string, string) (int32, error) {
			return 0, status.Error(codes.InvalidArgument, "please use your work email, not a personal address")
		},
	}
	h := New(fake, verifier)
	mux := http.NewServeMux()
	h.Register(mux)

	req := bearer(t, httptest.NewRequest(http.MethodPost, "/v1/verification/corporate-email/start", bytes.NewBufferString(`{"email":"someone@gmail.com"}`)), signer, "user-1")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d (body: %s)", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
	assertJSONBody(t, rec.Body.Bytes(), map[string]any{"error": "please use your work email, not a personal address"})
}
