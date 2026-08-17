package handlers

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/professional-connections/backend/services/gateway/internal/authclient"
	sharedjwt "github.com/professional-connections/backend/shared/jwt"
)

// fakeAuthClient is a hand-written test double implementing
// authclient.Client, so handler tests don't need a running auth service.
// Verification-related fields are optional — tests that don't exercise
// those routes simply leave them nil.
type fakeAuthClient struct {
	completeFn func(ctx context.Context, code, redirectURI string) (authclient.Session, error)
	refreshFn  func(ctx context.Context, refreshToken string) (authclient.Session, error)
	revokeFn   func(ctx context.Context, refreshToken string) error

	startPhoneFn         func(ctx context.Context, userID, phoneNumber string) (int32, error)
	verifyPhoneFn        func(ctx context.Context, userID, phoneNumber, code string) (authclient.Session, error)
	startPersonalEmailFn func(ctx context.Context, userID, email string) (int32, error)
	verifyPersonalFn     func(ctx context.Context, userID, email, code string) (authclient.Session, error)
	submitDetailsFn      func(ctx context.Context, userID, legalName, address string) (authclient.Session, error)
	startCorporateFn     func(ctx context.Context, userID, email string) (int32, error)
	verifyCorporateFn    func(ctx context.Context, userID, email, code string) (authclient.Session, error)
	getProfileFn         func(ctx context.Context, userID string) (authclient.Profile, error)
}

func (f *fakeAuthClient) CompleteLinkedInOnboarding(ctx context.Context, code, redirectURI string) (authclient.Session, error) {
	return f.completeFn(ctx, code, redirectURI)
}

func (f *fakeAuthClient) RefreshSession(ctx context.Context, refreshToken string) (authclient.Session, error) {
	return f.refreshFn(ctx, refreshToken)
}

func (f *fakeAuthClient) RevokeSession(ctx context.Context, refreshToken string) error {
	return f.revokeFn(ctx, refreshToken)
}

func (f *fakeAuthClient) StartPhoneVerification(ctx context.Context, userID, phoneNumber string) (int32, error) {
	return f.startPhoneFn(ctx, userID, phoneNumber)
}

func (f *fakeAuthClient) VerifyPhoneCode(ctx context.Context, userID, phoneNumber, code string) (authclient.Session, error) {
	return f.verifyPhoneFn(ctx, userID, phoneNumber, code)
}

func (f *fakeAuthClient) StartPersonalEmailVerification(ctx context.Context, userID, email string) (int32, error) {
	return f.startPersonalEmailFn(ctx, userID, email)
}

func (f *fakeAuthClient) VerifyPersonalEmailCode(ctx context.Context, userID, email, code string) (authclient.Session, error) {
	return f.verifyPersonalFn(ctx, userID, email, code)
}

func (f *fakeAuthClient) SubmitPersonalDetails(ctx context.Context, userID, legalName, address string) (authclient.Session, error) {
	return f.submitDetailsFn(ctx, userID, legalName, address)
}

func (f *fakeAuthClient) StartCorporateEmailVerification(ctx context.Context, userID, email string) (int32, error) {
	return f.startCorporateFn(ctx, userID, email)
}

func (f *fakeAuthClient) VerifyCorporateEmailCode(ctx context.Context, userID, email, code string) (authclient.Session, error) {
	return f.verifyCorporateFn(ctx, userID, email, code)
}

func (f *fakeAuthClient) GetProfile(ctx context.Context, userID string) (authclient.Profile, error) {
	return f.getProfileFn(ctx, userID)
}

func (f *fakeAuthClient) Close() error { return nil }

// testAuth constructs a jwt.Verifier (and a matching Signer to mint test
// tokens) backed by a fresh throwaway RSA keypair — every handler test
// needs a real *jwt.Verifier now that New requires one.
func testAuth(t *testing.T) (*sharedjwt.Signer, *sharedjwt.Verifier) {
	t.Helper()

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	dir := t.TempDir()

	privPath := filepath.Join(dir, "private.pem")
	privPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})
	if err := os.WriteFile(privPath, privPEM, 0o600); err != nil {
		t.Fatalf("write private key: %v", err)
	}

	pubBytes, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	if err != nil {
		t.Fatalf("marshal public key: %v", err)
	}
	pubPath := filepath.Join(dir, "public.pem")
	if err := os.WriteFile(pubPath, pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: pubBytes}), 0o600); err != nil {
		t.Fatalf("write public key: %v", err)
	}

	signer, err := sharedjwt.NewSigner(privPath)
	if err != nil {
		t.Fatalf("NewSigner: %v", err)
	}
	verifier, err := sharedjwt.NewVerifier(pubPath)
	if err != nil {
		t.Fatalf("NewVerifier: %v", err)
	}
	return signer, verifier
}

// bearer returns req with an Authorization header carrying a valid token
// for userID, signed by signer.
func bearer(t *testing.T, req *http.Request, signer *sharedjwt.Signer, userID string) *http.Request {
	t.Helper()
	token, err := signer.Sign(sharedjwt.Claims{UserID: userID, TrustLevel: 1})
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	return req
}

func TestLinkedInCallback(t *testing.T) {
	tests := []struct {
		name       string
		body       string
		completeFn func(ctx context.Context, code, redirectURI string) (authclient.Session, error)
		wantStatus int
		wantBody   map[string]any
	}{
		{
			name: "success",
			body: `{"authorization_code":"code","redirect_uri":"app://callback"}`,
			completeFn: func(_ context.Context, code, redirectURI string) (authclient.Session, error) {
				if code != "code" || redirectURI != "app://callback" {
					t.Errorf("unexpected args: %q %q", code, redirectURI)
				}
				return authclient.Session{
					UserID:                      "user-1",
					AccessToken:                 "access-token",
					RefreshToken:                "refresh-token",
					AccessTokenExpiresInSeconds: 900,
					IsNewUser:                   true,
					FullName:                    "Ada Lovelace",
					ProfilePhotoURL:             "https://example.com/p.jpg",
				}, nil
			},
			wantStatus: http.StatusOK,
			wantBody: map[string]any{
				"user_id":           "user-1",
				"access_token":      "access-token",
				"refresh_token":     "refresh-token",
				"expires_in":        float64(900),
				"is_new_user":       true,
				"full_name":         "Ada Lovelace",
				"profile_photo_url": "https://example.com/p.jpg",
			},
		},
		{
			name:       "malformed json body",
			body:       `not json`,
			completeFn: func(context.Context, string, string) (authclient.Session, error) { return authclient.Session{}, nil },
			wantStatus: http.StatusBadRequest,
		},
		{
			name: "invalid code maps to 400",
			body: `{"authorization_code":"bad","redirect_uri":"app://callback"}`,
			completeFn: func(context.Context, string, string) (authclient.Session, error) {
				return authclient.Session{}, status.Error(codes.InvalidArgument, "linkedin rejected the code")
			},
			wantStatus: http.StatusBadRequest,
			wantBody:   map[string]any{"error": "linkedin rejected the code"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, verifier := testAuth(t)
			h := New(&fakeAuthClient{completeFn: tt.completeFn}, verifier)
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
			_, verifier := testAuth(t)
			h := New(&fakeAuthClient{refreshFn: tt.refreshFn}, verifier)
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
			_, verifier := testAuth(t)
			h := New(&fakeAuthClient{revokeFn: tt.revokeFn}, verifier)
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
