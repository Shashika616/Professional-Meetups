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
	"strings"
	"testing"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/professional-connections/backend/services/auth/internal/identity"
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

// newFederatedTestService builds a Service with fakes for every dependency except
// the ones the caller overrides — a shared constructor so each test below
// only has to spell out what it actually cares about, mirroring the
// pattern this file already used pre-ADR-014 for New(...)'s previously
// shorter argument list.
func newFederatedTestService(t *testing.T, opts ...func(*testServiceDeps)) (*Service, *testServiceDeps) {
	t.Helper()

	li, closeLI := newTestLinkedInServer(t)
	t.Cleanup(closeLI)

	deps := &testServiceDeps{
		users:      newFakeUserRepository(),
		identities: newFakeUserIdentityRepository(),
		tokens:     newFakeRefreshTokenRepository(),
		codes:      newFakeVerificationCodeRepository(),
		linkedin:   li,
		apple:      &fakeIdentityProvider{validTokens: map[string]identity.VerifiedIdentity{}},
		google:     &fakeIdentityProvider{validTokens: map[string]identity.VerifiedIdentity{}},
		pub:        &fakePublisher{},
		emailer:    &fakeEmailSender{},
		smser:      &fakeSmsSender{},
	}
	for _, opt := range opts {
		opt(deps)
	}

	svc := New(
		deps.users, deps.identities, deps.tokens, deps.codes,
		deps.linkedin, deps.apple, deps.google,
		newTestSigner(t), deps.pub, deps.emailer, deps.smser,
	)
	return svc, deps
}

type testServiceDeps struct {
	users      *fakeUserRepository
	identities *fakeUserIdentityRepository
	tokens     *fakeRefreshTokenRepository
	codes      *fakeVerificationCodeRepository
	linkedin   *linkedin.Client
	apple      *fakeIdentityProvider
	google     *fakeIdentityProvider
	pub        *fakePublisher
	emailer    *fakeEmailSender
	smser      *fakeSmsSender
}

func TestCompleteFederatedSignup_NewUser(t *testing.T) {
	svc, deps := newFederatedTestService(t)
	deps.apple.validTokens["good-token"] = identity.VerifiedIdentity{
		Subject: "apple-sub-123", Email: "ada@example.com", Name: "Ada Lovelace",
	}

	resp, err := svc.CompleteFederatedSignup(context.Background(), &authv1.CompleteFederatedSignupRequest{
		Provider:            authv1.IdentityProviderProto_IDENTITY_PROVIDER_APPLE,
		IdToken:             "good-token",
		AgeConfirmedOver_18: true,
	})
	if err != nil {
		t.Fatalf("CompleteFederatedSignup() error: %v", err)
	}

	if !resp.GetIsNewUser() {
		t.Error("IsNewUser = false, want true")
	}
	if resp.GetAccessToken() == "" {
		t.Error("AccessToken is empty")
	}
	if resp.GetFullName() != "Ada Lovelace" {
		t.Errorf("FullName = %q, want %q", resp.GetFullName(), "Ada Lovelace")
	}

	created := deps.users.byID[resp.GetUserId()]
	if created.TrustLevel != 0 {
		t.Errorf("new federated user's TrustLevel = %d, want 0 (Level 0, no LinkedIn linked yet)", created.TrustLevel)
	}
	if !created.AgeConfirmedOver18 {
		t.Error("AgeConfirmedOver18 = false, want true")
	}
	if created.AgeConfirmedAt == nil {
		t.Error("AgeConfirmedAt is nil, want set")
	}

	identityRow, err := deps.identities.GetByProviderSubject(context.Background(), repository.IdentityProviderApple, "apple-sub-123")
	if err != nil {
		t.Fatalf("expected a user_identities row for the new apple identity, got error: %v", err)
	}
	if identityRow.UserID != resp.GetUserId() {
		t.Errorf("identity row UserID = %q, want %q", identityRow.UserID, resp.GetUserId())
	}
	if identityRow.Email != "ada@example.com" {
		t.Errorf("identity row Email = %q, want %q", identityRow.Email, "ada@example.com")
	}

	if len(deps.pub.published) != 1 {
		t.Fatalf("PublishUserOnboarded called %d times, want 1", len(deps.pub.published))
	}
	if deps.pub.published[0].trustLevel != 0 {
		t.Errorf("published trustLevel = %d, want 0", deps.pub.published[0].trustLevel)
	}
}

func TestCompleteFederatedSignup_ReturningUserLogsIn(t *testing.T) {
	svc, deps := newFederatedTestService(t)
	deps.google.validTokens["good-token"] = identity.VerifiedIdentity{
		Subject: "google-sub-123", Email: "ada@example.com", Name: "Ada Lovelace",
	}

	first, err := svc.CompleteFederatedSignup(context.Background(), &authv1.CompleteFederatedSignupRequest{
		Provider:            authv1.IdentityProviderProto_IDENTITY_PROVIDER_GOOGLE,
		IdToken:             "good-token",
		AgeConfirmedOver_18: true,
	})
	if err != nil {
		t.Fatalf("first CompleteFederatedSignup() error: %v", err)
	}

	second, err := svc.CompleteFederatedSignup(context.Background(), &authv1.CompleteFederatedSignupRequest{
		Provider:            authv1.IdentityProviderProto_IDENTITY_PROVIDER_GOOGLE,
		IdToken:             "good-token",
		AgeConfirmedOver_18: true,
	})
	if err != nil {
		t.Fatalf("second CompleteFederatedSignup() error: %v", err)
	}

	if second.GetIsNewUser() {
		t.Error("IsNewUser = true on the second sign-in, want false")
	}
	if second.GetUserId() != first.GetUserId() {
		t.Errorf("second sign-in UserId = %q, want the same user %q", second.GetUserId(), first.GetUserId())
	}
	if len(deps.pub.published) != 1 {
		t.Errorf("PublishUserOnboarded called %d times across both sign-ins, want 1 (only the first)", len(deps.pub.published))
	}
}

func TestCompleteFederatedSignup_RejectsAgeNotConfirmed(t *testing.T) {
	svc, deps := newFederatedTestService(t)
	deps.apple.validTokens["good-token"] = identity.VerifiedIdentity{Subject: "apple-sub-123", Name: "Ada Lovelace"}

	_, err := svc.CompleteFederatedSignup(context.Background(), &authv1.CompleteFederatedSignupRequest{
		Provider:            authv1.IdentityProviderProto_IDENTITY_PROVIDER_APPLE,
		IdToken:             "good-token",
		AgeConfirmedOver_18: false,
	})
	if err == nil {
		t.Fatal("CompleteFederatedSignup() returned nil error for age_confirmed_over_18=false, want error")
	}
	if got := status.Code(err); got != codes.InvalidArgument {
		t.Errorf("status code = %v, want %v", got, codes.InvalidArgument)
	}
	if len(deps.users.createCalls) != 0 {
		t.Errorf("Create called %d times, want 0 — no account should be created when age isn't confirmed", len(deps.users.createCalls))
	}
}

func TestCompleteFederatedSignup_RejectsFailedVerification(t *testing.T) {
	svc, _ := newFederatedTestService(t, func(d *testServiceDeps) {
		d.apple = &fakeIdentityProvider{err: context.DeadlineExceeded}
	})

	_, err := svc.CompleteFederatedSignup(context.Background(), &authv1.CompleteFederatedSignupRequest{
		Provider:            authv1.IdentityProviderProto_IDENTITY_PROVIDER_APPLE,
		IdToken:             "any-token",
		AgeConfirmedOver_18: true,
	})
	if err == nil {
		t.Fatal("CompleteFederatedSignup() returned nil error for a failed id_token verification, want error")
	}
	if got := status.Code(err); got != codes.InvalidArgument {
		t.Errorf("status code = %v, want %v", got, codes.InvalidArgument)
	}
}

func TestLinkIdentity_LinksLinkedInToExistingFederatedUser(t *testing.T) {
	svc, deps := newFederatedTestService(t)

	user, err := deps.users.Create(context.Background(), repository.NewUser{
		FullName: "Ada Lovelace", TrustLevel: 0, AgeConfirmedOver18: true,
	})
	if err != nil {
		t.Fatalf("seed federated user: %v", err)
	}

	resp, err := svc.LinkIdentity(context.Background(), &authv1.LinkIdentityRequest{
		UserId:            user.ID,
		Provider:          authv1.IdentityProviderProto_IDENTITY_PROVIDER_LINKEDIN,
		AuthorizationCode: "auth-code",
		RedirectUri:       "app://callback",
	})
	if err != nil {
		t.Fatalf("LinkIdentity() error: %v", err)
	}

	if resp.GetUserId() != user.ID {
		t.Errorf("UserId = %q, want %q", resp.GetUserId(), user.ID)
	}
	linked := deps.users.byID[user.ID]
	if linked.LinkedInSub != "li-sub-123" {
		t.Errorf("LinkedInSub = %q, want %q", linked.LinkedInSub, "li-sub-123")
	}
	if linked.TrustLevel != 1 {
		t.Errorf("TrustLevel after linking LinkedIn = %d, want 1", linked.TrustLevel)
	}
}

// TestLinkIdentity_RejectsLinkedInSubjectAlreadyLinkedToADifferentUser is
// the explicit abuse-case test the backend plan calls out by name: linking
// must never silently merge two accounts.
func TestLinkIdentity_RejectsLinkedInSubjectAlreadyLinkedToADifferentUser(t *testing.T) {
	svc, deps := newFederatedTestService(t)

	// A different user already has this exact LinkedIn subject linked
	// (li-sub-123, per newTestLinkedInServer's fixed response).
	if _, err := deps.users.Create(context.Background(), fakeNewUser("li-sub-123")); err != nil {
		t.Fatalf("seed existing linkedin-linked user: %v", err)
	}

	victim, err := deps.users.Create(context.Background(), repository.NewUser{
		FullName: "Bob", TrustLevel: 0, AgeConfirmedOver18: true,
	})
	if err != nil {
		t.Fatalf("seed second federated user: %v", err)
	}

	_, err = svc.LinkIdentity(context.Background(), &authv1.LinkIdentityRequest{
		UserId:            victim.ID,
		Provider:          authv1.IdentityProviderProto_IDENTITY_PROVIDER_LINKEDIN,
		AuthorizationCode: "auth-code",
		RedirectUri:       "app://callback",
	})
	if err == nil {
		t.Fatal("LinkIdentity() returned nil error when the LinkedIn subject already belongs to a different user, want error")
	}
	if got := status.Code(err); got != codes.AlreadyExists {
		t.Errorf("status code = %v, want %v (ErrConflict)", got, codes.AlreadyExists)
	}

	// The victim must not have been silently merged/updated.
	unchanged := deps.users.byID[victim.ID]
	if unchanged.LinkedInSub != "" {
		t.Errorf("victim's LinkedInSub = %q, want unchanged (empty)", unchanged.LinkedInSub)
	}
}

// TestLinkIdentity_SupportsAppleAndGoogleToo confirms LinkIdentity dispatches
// the Apple/Google branch (id_token verification, no server-to-server
// exchange) just as validly as the LinkedIn branch — ADR-014's Profile
// "Connect LinkedIn" flow is the only one built out end-to-end in the
// frontend for v1, but the RPC itself is provider-generic (a future "add
// Apple/Google as backup sign-in" reuses this same path).
func TestLinkIdentity_SupportsAppleAndGoogleToo(t *testing.T) {
	svc, deps := newFederatedTestService(t)
	deps.apple.validTokens["good-token"] = identity.VerifiedIdentity{Subject: "apple-sub-77"}
	user, _ := deps.users.Create(context.Background(), repository.NewUser{FullName: "Ada", TrustLevel: 0, AgeConfirmedOver18: true})

	resp, err := svc.LinkIdentity(context.Background(), &authv1.LinkIdentityRequest{
		UserId:   user.ID,
		Provider: authv1.IdentityProviderProto_IDENTITY_PROVIDER_APPLE,
		IdToken:  "good-token",
	})
	if err != nil {
		t.Fatalf("LinkIdentity() error: %v", err)
	}
	if resp.GetUserId() != user.ID {
		t.Errorf("UserId = %q, want %q", resp.GetUserId(), user.ID)
	}

	row, err := deps.identities.GetByProviderSubject(context.Background(), repository.IdentityProviderApple, "apple-sub-77")
	if err != nil {
		t.Fatalf("expected a user_identities row, got error: %v", err)
	}
	if row.UserID != user.ID {
		t.Errorf("identity row UserID = %q, want %q", row.UserID, user.ID)
	}
}

func TestLinkIdentity_RejectsUnspecifiedProvider(t *testing.T) {
	svc, deps := newFederatedTestService(t)
	user, _ := deps.users.Create(context.Background(), repository.NewUser{FullName: "Ada", TrustLevel: 0, AgeConfirmedOver18: true})

	_, err := svc.LinkIdentity(context.Background(), &authv1.LinkIdentityRequest{
		UserId:   user.ID,
		Provider: authv1.IdentityProviderProto_IDENTITY_PROVIDER_UNSPECIFIED,
	})
	if err == nil {
		t.Fatal("LinkIdentity() returned nil error for provider=UNSPECIFIED, want error")
	}
	if got := status.Code(err); got != codes.InvalidArgument {
		t.Errorf("status code = %v, want %v", got, codes.InvalidArgument)
	}
}

func TestCompleteLinkedInOnboarding_NewUserGrantsLevel1Immediately(t *testing.T) {
	svc, deps := newFederatedTestService(t)

	resp, err := svc.CompleteLinkedInOnboarding(context.Background(), &authv1.CompleteLinkedInOnboardingRequest{
		AuthorizationCode:   "auth-code",
		RedirectUri:         "app://callback",
		AgeConfirmedOver_18: true,
	})
	if err != nil {
		t.Fatalf("CompleteLinkedInOnboarding() error: %v", err)
	}
	if !resp.GetIsNewUser() {
		t.Error("IsNewUser = false, want true")
	}

	created := deps.users.byID[resp.GetUserId()]
	if created.LinkedInSub != "li-sub-123" {
		t.Errorf("LinkedInSub = %q, want %q", created.LinkedInSub, "li-sub-123")
	}
	if created.TrustLevel != 1 {
		t.Errorf("TrustLevel = %d, want 1 (LinkedIn direct signup, unchanged from ADR-011)", created.TrustLevel)
	}
	if created.ProfilePhotoURL == "" {
		t.Error("ProfilePhotoURL is empty, want the photo from LinkedIn's userinfo response (no regression vs. ADR-011)")
	}
}

func TestCompleteLinkedInOnboarding_ReturningUserLogsIn(t *testing.T) {
	svc, _ := newFederatedTestService(t)

	first, err := svc.CompleteLinkedInOnboarding(context.Background(), &authv1.CompleteLinkedInOnboardingRequest{
		AuthorizationCode:   "auth-code",
		RedirectUri:         "app://callback",
		AgeConfirmedOver_18: true,
	})
	if err != nil {
		t.Fatalf("first CompleteLinkedInOnboarding() error: %v", err)
	}

	second, err := svc.CompleteLinkedInOnboarding(context.Background(), &authv1.CompleteLinkedInOnboardingRequest{
		AuthorizationCode:   "auth-code-2",
		RedirectUri:         "app://callback",
		AgeConfirmedOver_18: true,
	})
	if err != nil {
		t.Fatalf("second CompleteLinkedInOnboarding() error: %v", err)
	}
	if second.GetIsNewUser() {
		t.Error("IsNewUser = true on the second sign-in, want false")
	}
	if second.GetUserId() != first.GetUserId() {
		t.Errorf("second sign-in UserId = %q, want the same user %q", second.GetUserId(), first.GetUserId())
	}
}

// TestCompleteLinkedInOnboarding_RejectsAgeNotConfirmed is the one addition
// to this RPC's behavior vs. ADR-011 (backend/level0-federated-identity-
// PLAN.md Step 6) — LinkedIn direct signup didn't have an age gate before
// ADR-014 and needed one, since it's a standalone account-creation path
// just like the other three.
func TestCompleteLinkedInOnboarding_RejectsAgeNotConfirmed(t *testing.T) {
	svc, deps := newFederatedTestService(t)

	_, err := svc.CompleteLinkedInOnboarding(context.Background(), &authv1.CompleteLinkedInOnboardingRequest{
		AuthorizationCode:   "auth-code",
		RedirectUri:         "app://callback",
		AgeConfirmedOver_18: false,
	})
	if err == nil {
		t.Fatal("CompleteLinkedInOnboarding() returned nil error for age_confirmed_over_18=false, want error")
	}
	if got := status.Code(err); got != codes.InvalidArgument {
		t.Errorf("status code = %v, want %v", got, codes.InvalidArgument)
	}
	if len(deps.users.createCalls) != 0 {
		t.Errorf("Create called %d times, want 0 — no account should be created when age isn't confirmed", len(deps.users.createCalls))
	}
}

func TestEmailSignupAndLogin_FullRoundTrip(t *testing.T) {
	svc, deps := newFederatedTestService(t)

	startResp, err := svc.StartEmailSignup(context.Background(), &authv1.StartVerificationRequest{
		Purpose: authv1.VerificationPurpose_VERIFICATION_PURPOSE_EMAIL_SIGNUP,
		Target:  "ada@example.com",
	})
	if err != nil {
		t.Fatalf("StartEmailSignup() error: %v", err)
	}
	if startResp.GetResendAfterSeconds() <= 0 {
		t.Error("ResendAfterSeconds <= 0, want positive")
	}
	code := deps.emailer.lastCode()
	if code == "" {
		t.Fatal("no OTP was dispatched")
	}

	signupResp, err := svc.CompleteEmailSignup(context.Background(), &authv1.CompleteEmailSignupRequest{
		Email:               "ada@example.com",
		Code:                code,
		Password:            "correct horse battery staple",
		AgeConfirmedOver_18: true,
	})
	if err != nil {
		t.Fatalf("CompleteEmailSignup() error: %v", err)
	}
	if !signupResp.GetIsNewUser() {
		t.Error("IsNewUser = false, want true")
	}

	created := deps.users.byID[signupResp.GetUserId()]
	if created.TrustLevel != 0 {
		t.Errorf("TrustLevel = %d, want 0 (email+password alone never grants Level 1)", created.TrustLevel)
	}
	if created.PasswordHash == "" || created.PasswordHash == "correct horse battery staple" {
		t.Errorf("PasswordHash = %q, want a hashed value, never the raw password", created.PasswordHash)
	}

	// The same code must not be usable twice.
	if _, err := svc.CompleteEmailSignup(context.Background(), &authv1.CompleteEmailSignupRequest{
		Email: "ada@example.com", Code: code, Password: "another password", AgeConfirmedOver_18: true,
	}); err == nil {
		t.Error("CompleteEmailSignup() with an already-consumed code returned nil error, want error")
	}

	loginResp, err := svc.LoginWithPassword(context.Background(), &authv1.LoginWithPasswordRequest{
		Email: "ada@example.com", Password: "correct horse battery staple",
	})
	if err != nil {
		t.Fatalf("LoginWithPassword() error: %v", err)
	}
	if loginResp.GetUserId() != signupResp.GetUserId() {
		t.Errorf("LoginWithPassword UserId = %q, want %q", loginResp.GetUserId(), signupResp.GetUserId())
	}
}

func TestLoginWithPassword_RejectsWrongPasswordAndUnknownEmailIdentically(t *testing.T) {
	svc, deps := newFederatedTestService(t)
	hash, err := hashPassword("correct horse battery staple")
	if err != nil {
		t.Fatalf("hashPassword: %v", err)
	}
	existing, _ := deps.users.Create(context.Background(), repository.NewUser{FullName: "Ada", TrustLevel: 0, AgeConfirmedOver18: true})
	_, _ = deps.users.UpdatePersonalEmail(context.Background(), existing.ID, "ada@example.com", 0)
	_, _ = deps.users.SetPasswordHash(context.Background(), existing.ID, hash)

	_, wrongPasswordErr := svc.LoginWithPassword(context.Background(), &authv1.LoginWithPasswordRequest{
		Email: "ada@example.com", Password: "wrong password",
	})
	_, unknownEmailErr := svc.LoginWithPassword(context.Background(), &authv1.LoginWithPasswordRequest{
		Email: "nobody@example.com", Password: "anything",
	})

	if wrongPasswordErr == nil || unknownEmailErr == nil {
		t.Fatal("expected both wrong-password and unknown-email login attempts to fail")
	}
	if status.Convert(wrongPasswordErr).Message() != status.Convert(unknownEmailErr).Message() {
		t.Errorf("error messages differ (%q vs %q) — must be identical to avoid leaking account existence",
			status.Convert(wrongPasswordErr).Message(), status.Convert(unknownEmailErr).Message())
	}
	if got := status.Code(wrongPasswordErr); got != codes.Unauthenticated {
		t.Errorf("status code = %v, want %v", got, codes.Unauthenticated)
	}
}

// TestLoginWithPassword_AlwaysCallsVerifyPassword is ADR-016's timing-
// side-channel regression test: a prior version short-circuited past
// verifyPassword entirely for a federated-only account (empty
// PasswordHash), so it returned near-instantly while a real wrong-password
// attempt paid the full argon2id cost — same error, different timing. A
// literal timing measurement would be flaky in CI, so this asserts the
// underlying cause instead: verifyPassword is actually invoked exactly
// once on both paths, via a counting wrapper swapped in for the package
// var.
func TestLoginWithPassword_AlwaysCallsVerifyPassword(t *testing.T) {
	svc, deps := newFederatedTestService(t)

	hash, err := hashPassword("correct horse battery staple")
	if err != nil {
		t.Fatalf("hashPassword: %v", err)
	}
	withPassword, _ := deps.users.Create(context.Background(), repository.NewUser{FullName: "Ada", TrustLevel: 0, AgeConfirmedOver18: true})
	_, _ = deps.users.UpdatePersonalEmail(context.Background(), withPassword.ID, "ada@example.com", 0)
	_, _ = deps.users.SetPasswordHash(context.Background(), withPassword.ID, hash)

	federatedOnly, _ := deps.users.Create(context.Background(), repository.NewUser{FullName: "Grace", TrustLevel: 1, AgeConfirmedOver18: true})
	_, _ = deps.users.UpdatePersonalEmail(context.Background(), federatedOnly.ID, "grace@example.com", 0)
	// federatedOnly deliberately never gets SetPasswordHash — mirrors a
	// LinkedIn/Apple/Google-only account that never used this login path.

	original := verifyPassword
	t.Cleanup(func() { verifyPassword = original })
	var calls int
	verifyPassword = func(encoded, raw string) bool {
		calls++
		return original(encoded, raw)
	}

	if _, err := svc.LoginWithPassword(context.Background(), &authv1.LoginWithPasswordRequest{
		Email: "ada@example.com", Password: "wrong password",
	}); err == nil {
		t.Fatal("expected wrong-password login to fail")
	}
	if _, err := svc.LoginWithPassword(context.Background(), &authv1.LoginWithPasswordRequest{
		Email: "grace@example.com", Password: "anything",
	}); err == nil {
		t.Fatal("expected federated-only-account login to fail")
	}

	if calls != 2 {
		t.Errorf("verifyPassword called %d times, want 2 — the federated-only path must pay the same argon2id cost as a real wrong-password attempt, not short-circuit past it", calls)
	}
}

func TestLinkIdentity_LinkedInExchangeFails(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":"invalid_grant"}`))
	}))
	defer server.Close()

	li := linkedin.New(linkedin.Config{ClientID: "cid", ClientSecret: "csecret"}, linkedin.WithTokenURL(server.URL))
	svc, deps := newFederatedTestService(t, func(d *testServiceDeps) { d.linkedin = li })
	user, _ := deps.users.Create(context.Background(), repository.NewUser{FullName: "Ada", TrustLevel: 0, AgeConfirmedOver18: true})

	_, err := svc.LinkIdentity(context.Background(), &authv1.LinkIdentityRequest{
		UserId:            user.ID,
		Provider:          authv1.IdentityProviderProto_IDENTITY_PROVIDER_LINKEDIN,
		AuthorizationCode: "bad-code",
		RedirectUri:       "app://callback",
	})
	if err == nil {
		t.Fatal("LinkIdentity() returned nil error, want error")
	}
	if got := status.Code(err); got != codes.InvalidArgument {
		t.Errorf("status code = %v, want %v", got, codes.InvalidArgument)
	}
}

// TestLinkIdentity_LinkedInExchangeFailureDoesNotLeakUpstreamBody guards
// the security-review fix carried over from before ADR-014: LinkedIn's raw
// token-exchange error body must never reach the client-facing error
// message, even though it's embedded in the error linkedin.Client returns
// internally.
func TestLinkIdentity_LinkedInExchangeFailureDoesNotLeakUpstreamBody(t *testing.T) {
	const upstreamBody = `{"error":"invalid_grant","error_description":"the provided authorization grant is invalid, expired, revoked"}`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(upstreamBody))
	}))
	defer server.Close()

	li := linkedin.New(linkedin.Config{ClientID: "cid", ClientSecret: "csecret"}, linkedin.WithTokenURL(server.URL))
	svc, deps := newFederatedTestService(t, func(d *testServiceDeps) { d.linkedin = li })
	user, _ := deps.users.Create(context.Background(), repository.NewUser{FullName: "Ada", TrustLevel: 0, AgeConfirmedOver18: true})

	_, err := svc.LinkIdentity(context.Background(), &authv1.LinkIdentityRequest{
		UserId:            user.ID,
		Provider:          authv1.IdentityProviderProto_IDENTITY_PROVIDER_LINKEDIN,
		AuthorizationCode: "bad-code",
		RedirectUri:       "app://callback",
	})
	if err == nil {
		t.Fatal("LinkIdentity() returned nil error, want error")
	}

	msg := status.Convert(err).Message()
	if strings.Contains(msg, "invalid_grant") || strings.Contains(msg, "error_description") {
		t.Errorf("client-facing message leaked LinkedIn's raw response body: %q", msg)
	}
	const wantMsg = "linkedin sign-in failed, please try again: invalid input"
	if msg != wantMsg {
		t.Errorf("client-facing message = %q, want %q", msg, wantMsg)
	}
}

func TestRefreshSession(t *testing.T) {
	svc, deps := newFederatedTestService(t)

	user, err := deps.users.Create(context.Background(), fakeNewUser("li-sub-123"))
	if err != nil {
		t.Fatalf("seed user: %v", err)
	}

	rawToken, hash, err := newRefreshToken()
	if err != nil {
		t.Fatalf("newRefreshToken: %v", err)
	}
	seeded, err := deps.tokens.Create(context.Background(), user.ID, hash, time.Now().Add(RefreshTokenTTL))
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
	if resp.GetFullName() != user.FullName {
		t.Errorf("FullName = %q, want %q", resp.GetFullName(), user.FullName)
	}

	old := deps.tokens.byID[seeded.ID]
	if old.ReplacedBy == nil {
		t.Error("old refresh token row has no ReplacedBy set after rotation")
	}
}

func TestRefreshSession_RejectsAlreadyRotatedToken(t *testing.T) {
	svc, deps := newFederatedTestService(t)

	user, _ := deps.users.Create(context.Background(), fakeNewUser("li-sub-123"))
	rawToken, hash, _ := newRefreshToken()
	seeded, _ := deps.tokens.Create(context.Background(), user.ID, hash, time.Now().Add(RefreshTokenTTL))

	replacedID := "already-replaced"
	seeded.ReplacedBy = &replacedID
	deps.tokens.byID[seeded.ID] = seeded
	deps.tokens.byHash[hash] = seeded

	_, err := svc.RefreshSession(context.Background(), &authv1.RefreshSessionRequest{RefreshToken: rawToken})
	if err == nil {
		t.Fatal("RefreshSession() returned nil error for an already-rotated token, want error")
	}
	if got := status.Code(err); got != codes.Unauthenticated {
		t.Errorf("status code = %v, want %v", got, codes.Unauthenticated)
	}
}

func TestRefreshSession_RejectsExpiredToken(t *testing.T) {
	svc, deps := newFederatedTestService(t)

	user, _ := deps.users.Create(context.Background(), fakeNewUser("li-sub-123"))
	rawToken, hash, _ := newRefreshToken()
	_, _ = deps.tokens.Create(context.Background(), user.ID, hash, time.Now().Add(-time.Hour))

	_, err := svc.RefreshSession(context.Background(), &authv1.RefreshSessionRequest{RefreshToken: rawToken})
	if err == nil {
		t.Fatal("RefreshSession() returned nil error for an expired token, want error")
	}
	if got := status.Code(err); got != codes.Unauthenticated {
		t.Errorf("status code = %v, want %v", got, codes.Unauthenticated)
	}
}

func TestRefreshSession_UnknownTokenNotFound(t *testing.T) {
	svc, _ := newFederatedTestService(t)

	_, err := svc.RefreshSession(context.Background(), &authv1.RefreshSessionRequest{RefreshToken: "never-issued"})
	if err == nil {
		t.Fatal("RefreshSession() returned nil error for an unknown token, want error")
	}
	if got := status.Code(err); got != codes.NotFound {
		t.Errorf("status code = %v, want %v", got, codes.NotFound)
	}
}

func TestRevokeSession_IdempotentOnUnknownToken(t *testing.T) {
	svc, _ := newFederatedTestService(t)

	resp, err := svc.RevokeSession(context.Background(), &authv1.RevokeSessionRequest{RefreshToken: "never-issued"})
	if err != nil {
		t.Fatalf("RevokeSession() on an unknown token returned error: %v", err)
	}
	if !resp.GetSuccess() {
		t.Error("Success = false, want true (revoke is idempotent)")
	}
}

func TestRevokeSession_KnownToken(t *testing.T) {
	svc, deps := newFederatedTestService(t)

	user, _ := deps.users.Create(context.Background(), fakeNewUser("li-sub-123"))
	rawToken, hash, _ := newRefreshToken()
	_, _ = deps.tokens.Create(context.Background(), user.ID, hash, time.Now().Add(RefreshTokenTTL))

	resp, err := svc.RevokeSession(context.Background(), &authv1.RevokeSessionRequest{RefreshToken: rawToken})
	if err != nil {
		t.Fatalf("RevokeSession() error: %v", err)
	}
	if !resp.GetSuccess() {
		t.Error("Success = false, want true")
	}
	if deps.tokens.byHash[hash].RevokedAt == nil {
		t.Error("token was not marked revoked")
	}
}

func fakeNewUser(linkedInSub string) repository.NewUser {
	return repository.NewUser{LinkedInSub: linkedInSub, FullName: "Ada Lovelace", TrustLevel: 1}
}
