package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/professional-connections/backend/services/gateway/internal/authclient"
)

// fakeAuthClient is a hand-written test double implementing
// authclient.Client, so handler tests don't need a running auth service.
type fakeAuthClient struct {
	completeFn func(ctx context.Context, code, verifier, redirectURI string) (authclient.Session, error)
	refreshFn  func(ctx context.Context, refreshToken string) (authclient.Session, error)
	revokeFn   func(ctx context.Context, refreshToken string) error
}

func (f *fakeAuthClient) CompleteLinkedInOnboarding(ctx context.Context, code, verifier, redirectURI string) (authclient.Session, error) {
	return f.completeFn(ctx, code, verifier, redirectURI)
}

func (f *fakeAuthClient) RefreshSession(ctx context.Context, refreshToken string) (authclient.Session, error) {
	return f.refreshFn(ctx, refreshToken)
}

func (f *fakeAuthClient) RevokeSession(ctx context.Context, refreshToken string) error {
	return f.revokeFn(ctx, refreshToken)
}

func (f *fakeAuthClient) Close() error { return nil }

func TestLinkedInCallback(t *testing.T) {
	tests := []struct {
		name       string
		body       string
		completeFn func(ctx context.Context, code, verifier, redirectURI string) (authclient.Session, error)
		wantStatus int
		wantBody   map[string]any
	}{
		{
			name: "success",
			body: `{"authorization_code":"code","code_verifier":"verifier","redirect_uri":"app://callback"}`,
			completeFn: func(_ context.Context, code, verifier, redirectURI string) (authclient.Session, error) {
				if code != "code" || verifier != "verifier" || redirectURI != "app://callback" {
					t.Errorf("unexpected args: %q %q %q", code, verifier, redirectURI)
				}
				return authclient.Session{
					UserID:                      "user-1",
					AccessToken:                 "access-token",
					RefreshToken:                "refresh-token",
					AccessTokenExpiresInSeconds: 900,
					IsNewUser:                   true,
				}, nil
			},
			wantStatus: http.StatusOK,
			wantBody: map[string]any{
				"user_id":       "user-1",
				"access_token":  "access-token",
				"refresh_token": "refresh-token",
				"expires_in":    float64(900),
				"is_new_user":   true,
			},
		},
		{
			name:       "malformed json body",
			body:       `not json`,
			completeFn: func(context.Context, string, string, string) (authclient.Session, error) { return authclient.Session{}, nil },
			wantStatus: http.StatusBadRequest,
		},
		{
			name: "invalid code maps to 400",
			body: `{"authorization_code":"bad","code_verifier":"v","redirect_uri":"app://callback"}`,
			completeFn: func(context.Context, string, string, string) (authclient.Session, error) {
				return authclient.Session{}, status.Error(codes.InvalidArgument, "linkedin rejected the code")
			},
			wantStatus: http.StatusBadRequest,
			wantBody:   map[string]any{"error": "linkedin rejected the code"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := New(&fakeAuthClient{completeFn: tt.completeFn})
			mux := http.NewServeMux()
			h.Register(mux)

			req := httptest.NewRequest(http.MethodPost, "/v1/auth/linkedin/callback", bytes.NewBufferString(tt.body))
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d (body: %s)", rec.Code, tt.wantStatus, rec.Body.String())
			}
			if tt.wantBody != nil {
				assertJSONBody(t, rec.Body.Bytes(), tt.wantBody)
			}
		})
	}
}

func TestRefresh(t *testing.T) {
	tests := []struct {
		name       string
		refreshFn  func(ctx context.Context, refreshToken string) (authclient.Session, error)
		wantStatus int
	}{
		{
			name: "success",
			refreshFn: func(_ context.Context, refreshToken string) (authclient.Session, error) {
				if refreshToken != "old-token" {
					t.Errorf("refreshToken = %q, want %q", refreshToken, "old-token")
				}
				return authclient.Session{UserID: "user-1", AccessToken: "new-access", RefreshToken: "new-refresh"}, nil
			},
			wantStatus: http.StatusOK,
		},
		{
			name: "already rotated token maps to 401",
			refreshFn: func(context.Context, string) (authclient.Session, error) {
				return authclient.Session{}, status.Error(codes.Unauthenticated, "refresh token already used")
			},
			wantStatus: http.StatusUnauthorized,
		},
		{
			name: "unknown token maps to 404",
			refreshFn: func(context.Context, string) (authclient.Session, error) {
				return authclient.Session{}, status.Error(codes.NotFound, "refresh token")
			},
			wantStatus: http.StatusNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := New(&fakeAuthClient{refreshFn: tt.refreshFn})
			mux := http.NewServeMux()
			h.Register(mux)

			req := httptest.NewRequest(http.MethodPost, "/v1/auth/refresh", bytes.NewBufferString(`{"refresh_token":"old-token"}`))
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d (body: %s)", rec.Code, tt.wantStatus, rec.Body.String())
			}
		})
	}
}

func TestLogout(t *testing.T) {
	tests := []struct {
		name       string
		revokeFn   func(ctx context.Context, refreshToken string) error
		wantStatus int
		wantBody   map[string]any
	}{
		{
			name:       "known token",
			revokeFn:   func(context.Context, string) error { return nil },
			wantStatus: http.StatusOK,
			wantBody:   map[string]any{"success": true},
		},
		{
			name:       "unknown token is still 200 (idempotent)",
			revokeFn:   func(context.Context, string) error { return nil }, // repository/service layer already makes this a no-op success
			wantStatus: http.StatusOK,
			wantBody:   map[string]any{"success": true},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := New(&fakeAuthClient{revokeFn: tt.revokeFn})
			mux := http.NewServeMux()
			h.Register(mux)

			req := httptest.NewRequest(http.MethodPost, "/v1/auth/logout", bytes.NewBufferString(`{"refresh_token":"some-token"}`))
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", rec.Code, tt.wantStatus)
			}
			assertJSONBody(t, rec.Body.Bytes(), tt.wantBody)
		})
	}
}

func assertJSONBody(t *testing.T, body []byte, want map[string]any) {
	t.Helper()

	var got map[string]any
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("unmarshal response body: %v (raw: %s)", err, body)
	}
	for k, wantV := range want {
		if gotV := got[k]; gotV != wantV {
			t.Errorf("body[%q] = %v, want %v", k, gotV, wantV)
		}
	}
}
