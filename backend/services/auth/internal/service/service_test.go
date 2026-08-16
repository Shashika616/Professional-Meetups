package service

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/professional-connections/backend/services/auth/internal/linkedin"
	"github.com/professional-connections/backend/services/auth/internal/repository"
	sharedjwt "github.com/professional-connections/backend/shared/jwt"
	authv1 "github.com/professional-connections/backend/shared/proto/auth/v1"
)

func newTestSigner(t *testing.T) *sharedjwt.Signer {
	t.Helper()

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}

	path := filepath.Join(t.TempDir(), "private.pem")
	pemBytes := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(key),
	})
	if err := os.WriteFile(path, pemBytes, 0o600); err != nil {
		t.Fatalf("write private key: %v", err)
	}

	signer, err := sharedjwt.NewSigner(path)
	if err != nil {
		t.Fatalf("NewSigner: %v", err)
	}
	return signer
}

// newTestLinkedInServer stands in for LinkedIn's token + userinfo endpoints.
func newTestLinkedInServer(t *testing.T) (*linkedin.Client, func()) {
	t.Helper()

	mux := http.NewServeMux()
	mux.HandleFunc("/token", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"access_token":"li-access-token","expires_in":5184000}`))
	})
	mux.HandleFunc("/userinfo", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"sub":"li-sub-123","name":"Ada Lovelace","picture":"https://example.com/p.jpg"}`))
	})
	server := httptest.NewServer(mux)

	client := linkedin.New(
		linkedin.Config{ClientID: "cid", ClientSecret: "csecret"},
		linkedin.WithTokenURL(server.URL+"/token"),
		linkedin.WithUserInfoURL(server.URL+"/userinfo"),
	)

	return client, server.Close
}

func TestCompleteLinkedInOnboarding_NewUser(t *testing.T) {
	li, closeServer := newTestLinkedInServer(t)
	defer closeServer()

	users := newFakeUserRepository()
	tokens := newFakeRefreshTokenRepository()
	pub := &fakePublisher{}
	svc := New(users, tokens, li, newTestSigner(t), pub)

	resp, err := svc.CompleteLinkedInOnboarding(context.Background(), &authv1.CompleteLinkedInOnboardingRequest{
		AuthorizationCode: "auth-code",
		PkceVerifier:      "verifier",
		RedirectUri:       "app://callback",
	})
	if err != nil {
		t.Fatalf("CompleteLinkedInOnboarding() error: %v", err)
	}

	if !resp.GetIsNewUser() {
		t.Error("IsNewUser = false, want true")
	}
	if resp.GetUserId() == "" {
		t.Error("UserId is empty")
	}
	if resp.GetAccessToken() == "" {
		t.Error("AccessToken is empty")
	}
	if resp.GetRefreshToken() == "" {
		t.Error("RefreshToken is empty")
	}
	if resp.GetAccessTokenExpiresInSeconds() != int64(sharedjwt.AccessTokenTTL.Seconds()) {
		t.Errorf("AccessTokenExpiresInSeconds = %d, want %d", resp.GetAccessTokenExpiresInSeconds(), int64(sharedjwt.AccessTokenTTL.Seconds()))
	}

	if len(users.createCalls) != 1 {
		t.Fatalf("Create called %d times, want 1", len(users.createCalls))
	}
	if users.createCalls[0].LinkedInSub != "li-sub-123" {
		t.Errorf("created user LinkedInSub = %q, want %q", users.createCalls[0].LinkedInSub, "li-sub-123")
	}

	if len(pub.published) != 1 {
		t.Fatalf("PublishUserOnboarded called %d times, want 1", len(pub.published))
	}
	if pub.published[0].userID != resp.GetUserId() {
		t.Errorf("published userID = %q, want %q", pub.published[0].userID, resp.GetUserId())
	}
}

func TestCompleteLinkedInOnboarding_ExistingUser(t *testing.T) {
	li, closeServer := newTestLinkedInServer(t)
	defer closeServer()

	users := newFakeUserRepository()
	tokens := newFakeRefreshTokenRepository()
	pub := &fakePublisher{}
	svc := New(users, tokens, li, newTestSigner(t), pub)

	// Pre-seed the user as if they'd onboarded before.
	if _, err := users.Create(context.Background(), fakeNewUser("li-sub-123")); err != nil {
		t.Fatalf("seed user: %v", err)
	}

	resp, err := svc.CompleteLinkedInOnboarding(context.Background(), &authv1.CompleteLinkedInOnboardingRequest{
		AuthorizationCode: "auth-code",
		PkceVerifier:      "verifier",
		RedirectUri:       "app://callback",
	})
	if err != nil {
		t.Fatalf("CompleteLinkedInOnboarding() error: %v", err)
	}

	if resp.GetIsNewUser() {
		t.Error("IsNewUser = true, want false")
	}
	if len(users.createCalls) != 1 { // only the seed call, not a second one
		t.Errorf("Create called %d times, want 1 (seed only)", len(users.createCalls))
	}
	if len(pub.published) != 0 {
		t.Errorf("PublishUserOnboarded called %d times, want 0 for a returning user", len(pub.published))
	}
}

func TestCompleteLinkedInOnboarding_LinkedInExchangeFails(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":"invalid_grant"}`))
	}))
	defer server.Close()

	li := linkedin.New(linkedin.Config{ClientID: "cid", ClientSecret: "csecret"}, linkedin.WithTokenURL(server.URL))
	svc := New(newFakeUserRepository(), newFakeRefreshTokenRepository(), li, newTestSigner(t), &fakePublisher{})

	_, err := svc.CompleteLinkedInOnboarding(context.Background(), &authv1.CompleteLinkedInOnboardingRequest{
		AuthorizationCode: "bad-code",
		PkceVerifier:      "verifier",
		RedirectUri:       "app://callback",
	})
	if err == nil {
		t.Fatal("CompleteLinkedInOnboarding() returned nil error, want error")
	}
	if got := status.Code(err); got != codes.InvalidArgument {
		t.Errorf("status code = %v, want %v", got, codes.InvalidArgument)
	}
}

func TestRefreshSession(t *testing.T) {
	li, closeServer := newTestLinkedInServer(t)
	defer closeServer()

	users := newFakeUserRepository()
	tokens := newFakeRefreshTokenRepository()
	svc := New(users, tokens, li, newTestSigner(t), &fakePublisher{})

	user, err := users.Create(context.Background(), fakeNewUser("li-sub-123"))
	if err != nil {
		t.Fatalf("seed user: %v", err)
	}

	rawToken, hash, err := newRefreshToken()
	if err != nil {
		t.Fatalf("newRefreshToken: %v", err)
	}
	seeded, err := tokens.Create(context.Background(), user.ID, hash, time.Now().Add(RefreshTokenTTL))
	if err != nil {
		t.Fatalf("seed refresh token: %v", err)
	}

	resp, err := svc.RefreshSession(context.Background(), &authv1.RefreshSessionRequest{RefreshToken: rawToken})
	if err != nil {
		t.Fatalf("RefreshSession() error: %v", err)
	}
	if resp.GetRefreshToken() == rawToken {
		t.Error("RefreshSession returned the same raw refresh token, want a new one")
	}
	if resp.GetUserId() != user.ID {
		t.Errorf("UserId = %q, want %q", resp.GetUserId(), user.ID)
	}

	old := tokens.byID[seeded.ID]
	if old.ReplacedBy == nil {
		t.Error("old refresh token row has no ReplacedBy set after rotation")
	}
}

func TestRefreshSession_RejectsAlreadyRotatedToken(t *testing.T) {
	li, closeServer := newTestLinkedInServer(t)
	defer closeServer()

	users := newFakeUserRepository()
	tokens := newFakeRefreshTokenRepository()
	svc := New(users, tokens, li, newTestSigner(t), &fakePublisher{})

	user, _ := users.Create(context.Background(), fakeNewUser("li-sub-123"))
	rawToken, hash, _ := newRefreshToken()
	seeded, _ := tokens.Create(context.Background(), user.ID, hash, time.Now().Add(RefreshTokenTTL))

	replacedID := "already-replaced"
	seeded.ReplacedBy = &replacedID
	tokens.byID[seeded.ID] = seeded
	tokens.byHash[hash] = seeded

	_, err := svc.RefreshSession(context.Background(), &authv1.RefreshSessionRequest{RefreshToken: rawToken})
	if err == nil {
		t.Fatal("RefreshSession() returned nil error for an already-rotated token, want error")
	}
	if got := status.Code(err); got != codes.Unauthenticated {
		t.Errorf("status code = %v, want %v", got, codes.Unauthenticated)
	}
}

func TestRefreshSession_RejectsExpiredToken(t *testing.T) {
	li, closeServer := newTestLinkedInServer(t)
	defer closeServer()

	users := newFakeUserRepository()
	tokens := newFakeRefreshTokenRepository()
	svc := New(users, tokens, li, newTestSigner(t), &fakePublisher{})

	user, _ := users.Create(context.Background(), fakeNewUser("li-sub-123"))
	rawToken, hash, _ := newRefreshToken()
	_, _ = tokens.Create(context.Background(), user.ID, hash, time.Now().Add(-time.Hour))

	_, err := svc.RefreshSession(context.Background(), &authv1.RefreshSessionRequest{RefreshToken: rawToken})
	if err == nil {
		t.Fatal("RefreshSession() returned nil error for an expired token, want error")
	}
	if got := status.Code(err); got != codes.Unauthenticated {
		t.Errorf("status code = %v, want %v", got, codes.Unauthenticated)
	}
}

func TestRefreshSession_UnknownTokenNotFound(t *testing.T) {
	li, closeServer := newTestLinkedInServer(t)
	defer closeServer()

	svc := New(newFakeUserRepository(), newFakeRefreshTokenRepository(), li, newTestSigner(t), &fakePublisher{})

	_, err := svc.RefreshSession(context.Background(), &authv1.RefreshSessionRequest{RefreshToken: "never-issued"})
	if err == nil {
		t.Fatal("RefreshSession() returned nil error for an unknown token, want error")
	}
	if got := status.Code(err); got != codes.NotFound {
		t.Errorf("status code = %v, want %v", got, codes.NotFound)
	}
}

func TestRevokeSession_IdempotentOnUnknownToken(t *testing.T) {
	li, closeServer := newTestLinkedInServer(t)
	defer closeServer()

	svc := New(newFakeUserRepository(), newFakeRefreshTokenRepository(), li, newTestSigner(t), &fakePublisher{})

	resp, err := svc.RevokeSession(context.Background(), &authv1.RevokeSessionRequest{RefreshToken: "never-issued"})
	if err != nil {
		t.Fatalf("RevokeSession() on an unknown token returned error: %v", err)
	}
	if !resp.GetSuccess() {
		t.Error("Success = false, want true (revoke is idempotent)")
	}
}

func TestRevokeSession_KnownToken(t *testing.T) {
	li, closeServer := newTestLinkedInServer(t)
	defer closeServer()

	users := newFakeUserRepository()
	tokens := newFakeRefreshTokenRepository()
	svc := New(users, tokens, li, newTestSigner(t), &fakePublisher{})

	user, _ := users.Create(context.Background(), fakeNewUser("li-sub-123"))
	rawToken, hash, _ := newRefreshToken()
	_, _ = tokens.Create(context.Background(), user.ID, hash, time.Now().Add(RefreshTokenTTL))

	resp, err := svc.RevokeSession(context.Background(), &authv1.RevokeSessionRequest{RefreshToken: rawToken})
	if err != nil {
		t.Fatalf("RevokeSession() error: %v", err)
	}
	if !resp.GetSuccess() {
		t.Error("Success = false, want true")
	}
	if tokens.byHash[hash].RevokedAt == nil {
		t.Error("token was not marked revoked")
	}
}

func fakeNewUser(linkedInSub string) repository.NewUser {
	return repository.NewUser{LinkedInSub: linkedInSub, FullName: "Ada Lovelace", TrustLevel: 1}
}
