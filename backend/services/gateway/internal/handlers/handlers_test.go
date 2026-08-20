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
	federatedSignupFn     func(ctx context.Context, provider, idToken string, ageConfirmedOver18 bool) (authclient.Session, error)
	linkedInSignupFn      func(ctx context.Context, authorizationCode, redirectURI string, ageConfirmedOver18 bool) (authclient.Session, error)
	linkIdentityFn        func(ctx context.Context, userID, provider, idToken, authorizationCode, redirectURI string) (authclient.Session, error)
	startEmailSignupFn    func(ctx context.Context, email string) (int32, error)
	completeEmailSignupFn func(ctx context.Context, email, code, password string, ageConfirmedOver18 bool) (authclient.Session, error)
	loginWithPasswordFn   func(ctx context.Context, email, password string) (authclient.Session, error)
	refreshFn             func(ctx context.Context, refreshToken string) (authclient.Session, error)
	revokeFn              func(ctx context.Context, refreshToken string) error

	startPhoneFn         func(ctx context.Context, userID, phoneNumber string) (int32, error)
	verifyPhoneFn        func(ctx context.Context, userID, phoneNumber, code string) (authclient.Session, error)
	startPersonalEmailFn func(ctx context.Context, userID, email string) (int32, error)
	verifyPersonalFn     func(ctx context.Context, userID, email, code string) (authclient.Session, error)
	submitDetailsFn      func(ctx context.Context, userID, legalName, address string) (authclient.Session, error)
	startCorporateFn     func(ctx context.Context, userID, email string) (int32, error)
	verifyCorporateFn    func(ctx context.Context, userID, email, code string) (authclient.Session, error)
	getProfileFn         func(ctx context.Context, userID string) (authclient.Profile, error)
}

func (f *fakeAuthClient) CompleteFederatedSignup(ctx context.Context, provider, idToken string, ageConfirmedOver18 bool) (authclient.Session, error) {
	return f.federatedSignupFn(ctx, provider, idToken, ageConfirmedOver18)
}

func (f *fakeAuthClient) CompleteLinkedInOnboarding(ctx context.Context, authorizationCode, redirectURI string, ageConfirmedOver18 bool) (authclient.Session, error) {
	return f.linkedInSignupFn(ctx, authorizationCode, redirectURI, ageConfirmedOver18)
}

func (f *fakeAuthClient) LinkIdentity(ctx context.Context, userID, provider, idToken, authorizationCode, redirectURI string) (authclient.Session, error) {
	return f.linkIdentityFn(ctx, userID, provider, idToken, authorizationCode, redirectURI)
}

func (f *fakeAuthClient) StartEmailSignup(ctx context.Context, email string) (int32, error) {
	return f.startEmailSignupFn(ctx, email)
}

func (f *fakeAuthClient) CompleteEmailSignup(ctx context.Context, email, code, password string, ageConfirmedOver18 bool) (authclient.Session, error) {
	return f.completeEmailSignupFn(ctx, email, code, password, ageConfirmedOver18)
}

func (f *fakeAuthClient) LoginWithPassword(ctx context.Context, email, password string) (authclient.Session, error) {
	return f.loginWithPasswordFn(ctx, email, password)
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

func TestFederatedSignup(t *testing.T) {
	tests := []struct {
		name       string
		body       string
		signupFn   func(ctx context.Context, provider, idToken string, ageConfirmedOver18 bool) (authclient.Session, error)
		wantStatus int
		wantBody   map[string]any
	}{
		{
			name: "success",
			body: `{"provider":"apple","id_token":"a-token","age_confirmed_over_18":true}`,
			signupFn: func(_ context.Context, provider, idToken string, ageConfirmedOver18 bool) (authclient.Session, error) {
				if provider != "apple" || idToken != "a-token" || !ageConfirmedOver18 {
					t.Errorf("unexpected args: %q %q %v", provider, idToken, ageConfirmedOver18)
				}
				return authclient.Session{
					UserID:                      "user-1",
					AccessToken:                 "access-token",
					RefreshToken:                "refresh-token",
					AccessTokenExpiresInSeconds: 900,
					IsNewUser:                   true,
					FullName:                    "Ada Lovelace",
				}, nil
			},
			wantStatus: http.StatusOK,
			wantBody: map[string]any{
				"user_id":       "user-1",
				"access_token":  "access-token",
				"refresh_token": "refresh-token",
				"expires_in":    float64(900),
				"is_new_user":   true,
				"full_name":     "Ada Lovelace",
			},
		},
		{
			name: "malformed json body",
			body: `not json`,
			signupFn: func(context.Context, string, string, bool) (authclient.Session, error) {
				return authclient.Session{}, nil
			},
			wantStatus: http.StatusBadRequest,
		},
		{
			name: "age not confirmed maps to 400",
			body: `{"provider":"apple","id_token":"a-token","age_confirmed_over_18":false}`,
			signupFn: func(context.Context, string, string, bool) (authclient.Session, error) {
				return authclient.Session{}, status.Error(codes.InvalidArgument, "you must confirm you are 18 or older to create an account")
			},
			wantStatus: http.StatusBadRequest,
			wantBody:   map[string]any{"error": "you must confirm you are 18 or older to create an account"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, verifier := testAuth(t)
			h := New(&fakeAuthClient{federatedSignupFn: tt.signupFn}, newNeverCalledMeetupClient(), verifier)
			mux := http.NewServeMux()
			h.Register(mux)

			// Deliberately unauthenticated — no bearer() call — this is
			// how a caller gets their first token at all (ADR-014).
			req := httptest.NewRequest(http.MethodPost, "/v1/auth/federated/signup", bytes.NewBufferString(tt.body))
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

// TestLinkedInCallback_Unauthenticated confirms /v1/auth/linkedin/callback
// is genuinely unchanged from ADR-011 (unauthenticated, account-creating) —
// unlike an earlier same-day draft of ADR-014, the final design never made
// this route authenticated (backend/level0-federated-identity-PLAN.md Step
// 6: "UNCHANGED — stays unauthenticated").
func TestLinkedInCallback_Unauthenticated(t *testing.T) {
	_, verifier := testAuth(t)
	h := New(&fakeAuthClient{
		linkedInSignupFn: func(_ context.Context, code, redirectURI string, ageConfirmedOver18 bool) (authclient.Session, error) {
			if code != "code" || redirectURI != "app://callback" || !ageConfirmedOver18 {
				t.Errorf("unexpected args: %q %q %v", code, redirectURI, ageConfirmedOver18)
			}
			return authclient.Session{UserID: "user-1", AccessToken: "access-token", IsNewUser: true}, nil
		},
	}, newNeverCalledMeetupClient(), verifier)
	mux := http.NewServeMux()
	h.Register(mux)

	req := httptest.NewRequest(http.MethodPost, "/v1/auth/linkedin/callback",
		bytes.NewBufferString(`{"authorization_code":"code","redirect_uri":"app://callback","age_confirmed_over_18":true}`))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d (body: %s)", rec.Code, http.StatusOK, rec.Body.String())
	}
	assertJSONBody(t, rec.Body.Bytes(), map[string]any{"user_id": "user-1", "is_new_user": true})
}

// TestLinkIdentity_RequiresAuth guards ADR-014's new authenticated route —
// unlike /v1/auth/linkedin/callback above, /v1/auth/identities/link must
// reject a request with no bearer token at all, before ever reaching
// authclient.
func TestLinkIdentity_RequiresAuth(t *testing.T) {
	_, verifier := testAuth(t)
	h := New(&fakeAuthClient{
		linkIdentityFn: func(context.Context, string, string, string, string, string) (authclient.Session, error) {
			t.Fatal("LinkIdentity should not have been called for an unauthenticated request")
			return authclient.Session{}, nil
		},
	}, newNeverCalledMeetupClient(), verifier)
	mux := http.NewServeMux()
	h.Register(mux)

	req := httptest.NewRequest(http.MethodPost, "/v1/auth/identities/link", bytes.NewBufferString(`{"provider":"linkedin","authorization_code":"code","redirect_uri":"app://callback"}`))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestLinkIdentity(t *testing.T) {
	tests := []struct {
		name           string
		body           string
		linkIdentityFn func(ctx context.Context, userID, provider, idToken, authorizationCode, redirectURI string) (authclient.Session, error)
		wantStatus     int
		wantBody       map[string]any
	}{
		{
			name: "success: linkedin (Profile Connect LinkedIn)",
			body: `{"provider":"linkedin","authorization_code":"code","redirect_uri":"app://callback"}`,
			linkIdentityFn: func(_ context.Context, userID, provider, idToken, code, redirectURI string) (authclient.Session, error) {
				if userID != "user-1" {
					t.Errorf("userID = %q, want %q (from verified JWT, not client-supplied)", userID, "user-1")
				}
				if provider != "linkedin" || code != "code" || redirectURI != "app://callback" {
					t.Errorf("unexpected args: %q %q %q", provider, code, redirectURI)
				}
				return authclient.Session{
					UserID:                      "user-1",
					AccessToken:                 "access-token",
					RefreshToken:                "refresh-token",
					AccessTokenExpiresInSeconds: 900,
					IsNewUser:                   false,
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
				"is_new_user":       false,
				"full_name":         "Ada Lovelace",
				"profile_photo_url": "https://example.com/p.jpg",
			},
		},
		{
			name: "success: apple/google id_token linking",
			body: `{"provider":"apple","id_token":"a-token"}`,
			linkIdentityFn: func(_ context.Context, userID, provider, idToken, code, redirectURI string) (authclient.Session, error) {
				if provider != "apple" || idToken != "a-token" {
					t.Errorf("unexpected args: %q %q", provider, idToken)
				}
				return authclient.Session{UserID: "user-1", AccessToken: "access-token"}, nil
			},
			wantStatus: http.StatusOK,
			wantBody:   map[string]any{"user_id": "user-1"},
		},
		{
			name: "malformed json body",
			body: `not json`,
			linkIdentityFn: func(context.Context, string, string, string, string, string) (authclient.Session, error) {
				return authclient.Session{}, nil
			},
			wantStatus: http.StatusBadRequest,
		},
		{
			name: "invalid code maps to 400",
			body: `{"provider":"linkedin","authorization_code":"bad","redirect_uri":"app://callback"}`,
			linkIdentityFn: func(context.Context, string, string, string, string, string) (authclient.Session, error) {
				return authclient.Session{}, status.Error(codes.InvalidArgument, "linkedin rejected the code")
			},
			wantStatus: http.StatusBadRequest,
			wantBody:   map[string]any{"error": "linkedin rejected the code"},
		},
		{
			name: "identity already linked to a different user maps to 409",
			body: `{"provider":"linkedin","authorization_code":"code","redirect_uri":"app://callback"}`,
			linkIdentityFn: func(context.Context, string, string, string, string, string) (authclient.Session, error) {
				return authclient.Session{}, status.Error(codes.AlreadyExists, "linkedin account already linked to a different user")
			},
			wantStatus: http.StatusConflict,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			signer, verifier := testAuth(t)
			h := New(&fakeAuthClient{linkIdentityFn: tt.linkIdentityFn}, newNeverCalledMeetupClient(), verifier)
			mux := http.NewServeMux()
			h.Register(mux)

			req := httptest.NewRequest(http.MethodPost, "/v1/auth/identities/link", bytes.NewBufferString(tt.body))
			req = bearer(t, req, signer, "user-1")
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

func TestEmailSignupAndLogin_Unauthenticated(t *testing.T) {
	_, verifier := testAuth(t)
	h := New(&fakeAuthClient{
		startEmailSignupFn: func(_ context.Context, email string) (int32, error) {
			if email != "ada@example.com" {
				t.Errorf("email = %q, want %q", email, "ada@example.com")
			}
			return 60, nil
		},
		completeEmailSignupFn: func(_ context.Context, email, code, password string, ageConfirmedOver18 bool) (authclient.Session, error) {
			if email != "ada@example.com" || code != "111111" || password != "hunter22" || !ageConfirmedOver18 {
				t.Errorf("unexpected args: %q %q %q %v", email, code, password, ageConfirmedOver18)
			}
			return authclient.Session{UserID: "user-1", AccessToken: "access-token", IsNewUser: true}, nil
		},
		loginWithPasswordFn: func(_ context.Context, email, password string) (authclient.Session, error) {
			if email != "ada@example.com" || password != "hunter22" {
				t.Errorf("unexpected args: %q %q", email, password)
			}
			return authclient.Session{UserID: "user-1", AccessToken: "access-token-2"}, nil
		},
	}, newNeverCalledMeetupClient(), verifier)
	mux := http.NewServeMux()
	h.Register(mux)

	startReq := httptest.NewRequest(http.MethodPost, "/v1/auth/email/signup/start", bytes.NewBufferString(`{"email":"ada@example.com"}`))
	startRec := httptest.NewRecorder()
	mux.ServeHTTP(startRec, startReq)
	if startRec.Code != http.StatusOK {
		t.Fatalf("start: status = %d, want %d (body: %s)", startRec.Code, http.StatusOK, startRec.Body.String())
	}
	assertJSONBody(t, startRec.Body.Bytes(), map[string]any{"resend_after_seconds": float64(60)})

	completeReq := httptest.NewRequest(http.MethodPost, "/v1/auth/email/signup",
		bytes.NewBufferString(`{"email":"ada@example.com","code":"111111","password":"hunter22","age_confirmed_over_18":true}`))
	completeRec := httptest.NewRecorder()
	mux.ServeHTTP(completeRec, completeReq)
	if completeRec.Code != http.StatusOK {
		t.Fatalf("complete: status = %d, want %d (body: %s)", completeRec.Code, http.StatusOK, completeRec.Body.String())
	}
	assertJSONBody(t, completeRec.Body.Bytes(), map[string]any{"user_id": "user-1", "is_new_user": true})

	loginReq := httptest.NewRequest(http.MethodPost, "/v1/auth/email/login", bytes.NewBufferString(`{"email":"ada@example.com","password":"hunter22"}`))
	loginRec := httptest.NewRecorder()
	mux.ServeHTTP(loginRec, loginReq)
	if loginRec.Code != http.StatusOK {
		t.Fatalf("login: status = %d, want %d (body: %s)", loginRec.Code, http.StatusOK, loginRec.Body.String())
	}
	assertJSONBody(t, loginRec.Body.Bytes(), map[string]any{"user_id": "user-1"})
}

func TestLoginWithPassword_InvalidCredentialsMapsTo401(t *testing.T) {
	_, verifier := testAuth(t)
	h := New(&fakeAuthClient{
		loginWithPasswordFn: func(context.Context, string, string) (authclient.Session, error) {
			return authclient.Session{}, status.Error(codes.Unauthenticated, "invalid email or password")
		},
	}, newNeverCalledMeetupClient(), verifier)
	mux := http.NewServeMux()
	h.Register(mux)

	req := httptest.NewRequest(http.MethodPost, "/v1/auth/email/login", bytes.NewBufferString(`{"email":"ada@example.com","password":"wrong"}`))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d (body: %s)", rec.Code, http.StatusUnauthorized, rec.Body.String())
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
			h := New(&fakeAuthClient{refreshFn: tt.refreshFn}, newNeverCalledMeetupClient(), verifier)
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
			h := New(&fakeAuthClient{revokeFn: tt.revokeFn}, newNeverCalledMeetupClient(), verifier)
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
