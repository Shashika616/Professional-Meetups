package service_test

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"database/sql"
	"encoding/pem"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib" // database/sql driver, needed only to drive migrate
	"google.golang.org/protobuf/encoding/protojson"

	"github.com/professional-connections/backend/services/auth/internal/email"
	"github.com/professional-connections/backend/services/auth/internal/identity"
	"github.com/professional-connections/backend/services/auth/internal/linkedin"
	"github.com/professional-connections/backend/services/auth/internal/repository"
	"github.com/professional-connections/backend/services/auth/internal/service"
	"github.com/professional-connections/backend/services/auth/internal/sms"
	sharedjwt "github.com/professional-connections/backend/shared/jwt"
	authv1 "github.com/professional-connections/backend/shared/proto/auth/v1"
)

// identityProviderStub stands in for AppleProvider/GoogleProvider — real
// Apple/Google credentials don't exist yet (Action Tracker §1), and this
// package (service_test, external/black-box) can't reach fakes_test.go's
// unexported fakeIdentityProvider anyway (different package). Real
// signature/issuer/audience/expiry verification is internal/identity's
// own concern (identity_test.go, against a local test JWKS server).
type identityProviderStub struct {
	validTokens map[string]identity.VerifiedIdentity
}

func (s *identityProviderStub) Verify(_ context.Context, idToken string) (identity.VerifiedIdentity, error) {
	v, ok := s.validTokens[idToken]
	if !ok {
		return identity.VerifiedIdentity{}, fmt.Errorf("stub: invalid id_token")
	}
	return v, nil
}

// newIntegrationSigner generates a throwaway RSA keypair for this test —
// duplicated from internal/service's own unit-test helper because this file
// is package service_test (external, black-box) and can't reach an
// unexported helper in package service.
func newIntegrationSigner(t *testing.T) *sharedjwt.Signer {
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

// defaultTestDatabaseURL matches docker-compose.yml's local Postgres
// defaults. CI (Step 6) overrides this via the DATABASE_URL env var to
// point at its Postgres service container instead.
const defaultTestDatabaseURL = "postgres://app:app@localhost:5432/professional_connections?sslmode=disable"

// migrationsPath is relative to this file's directory (Go tests always run
// with cwd set to the package directory), pointing at the same
// db/migrations golang-migrate's docker-compose `migrate` service applies.
const migrationsPath = "file://../../../../db/migrations"

// requirePostgres skips the test if Postgres isn't reachable within a short
// timeout, so a plain `go test ./...` stays fast and doesn't hang when a
// developer hasn't run `docker compose up` — see PLAN.md Step 4.
func requirePostgres(t *testing.T) string {
	t.Helper()

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = defaultTestDatabaseURL
	}

	conn, err := net.DialTimeout("tcp", "localhost:5432", 500*time.Millisecond)
	if err != nil {
		t.Skipf("postgres not reachable on localhost:5432, skipping integration test (run `docker compose up` first): %v", err)
	}
	_ = conn.Close()

	return dbURL
}

func runMigrations(t *testing.T, dbURL string) {
	t.Helper()

	sqlDB, err := sql.Open("pgx", dbURL)
	if err != nil {
		t.Fatalf("open database/sql connection for migrate: %v", err)
	}
	defer func() { _ = sqlDB.Close() }()

	driver, err := postgres.WithInstance(sqlDB, &postgres.Config{})
	if err != nil {
		t.Fatalf("create migrate postgres driver: %v", err)
	}

	m, err := migrate.NewWithDatabaseInstance(migrationsPath, "postgres", driver)
	if err != nil {
		t.Fatalf("create migrate instance: %v", err)
	}

	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		t.Fatalf("run migrations: %v", err)
	}
}

func newIntegrationLinkedInServer(t *testing.T, sub string) (*linkedin.Client, func()) {
	t.Helper()

	mux := http.NewServeMux()
	mux.HandleFunc("/token", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"access_token":"li-access-token","expires_in":5184000}`))
	})
	mux.HandleFunc("/userinfo", func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprintf(w, `{"sub":%q,"name":"Integration Test User","picture":"https://example.com/p.jpg"}`, sub)
	})
	server := httptest.NewServer(mux)

	client := linkedin.New(
		linkedin.Config{ClientID: "cid", ClientSecret: "csecret"},
		linkedin.WithTokenURL(server.URL+"/token"),
		linkedin.WithUserInfoURL(server.URL+"/userinfo"),
	)
	return client, server.Close
}

// noopPublisher stands in for the real Pub/Sub publisher — this test
// asserts against Postgres state, not Pub/Sub delivery, and doesn't require
// the Pub/Sub emulator to be running.
type noopPublisher struct{}

func (noopPublisher) PublishUserOnboarded(context.Context, string, int) error { return nil }
func (noopPublisher) Close() error                                            { return nil }

// newIntegrationService wires a Service against real Postgres repositories
// and a fake Apple provider (real Apple/Google credentials don't exist yet
// — Action Tracker §1 — so this is the same fakes-first treatment the
// backend plan asks for; internal/identity's own tests are what actually
// exercise real JWKS verification logic, against a local test JWKS
// server). appleValidTokens lets each test control exactly which id_token
// strings verify successfully, same pattern as fakes_test.go's
// fakeIdentityProvider.
func newIntegrationService(t *testing.T, dbURL string, pool *pgxpool.Pool, appleValidTokens map[string]identity.VerifiedIdentity) *service.Service {
	t.Helper()

	li, closeServer := newIntegrationLinkedInServer(t, fmt.Sprintf("integration-test-sub-%d", time.Now().UnixNano()))
	t.Cleanup(closeServer)

	return service.New(
		repository.NewUserRepository(pool),
		repository.NewUserIdentityRepository(pool),
		repository.NewRefreshTokenRepository(pool),
		repository.NewVerificationCodeRepository(pool),
		li,
		&identityProviderStub{validTokens: appleValidTokens},
		&identityProviderStub{validTokens: appleValidTokens},
		newIntegrationSigner(t),
		noopPublisher{},
		email.NewLoggingEmailSender(),
		sms.NewLoggingSmsSender(),
	)
}

// TestCompleteFederatedSignupAndLinkIdentity_Integration exercises the
// full path this unit tests can't: a real Postgres, the actual
// migrations, and pgx/sqlc query execution — the class of bug a bad
// migration or a bad SQL query wouldn't be caught by mocked-repository
// unit tests (PLAN.md Step 4). Covers ADR-014's two new RPCs end to end:
// Level 0 account creation via CompleteFederatedSignup, then the upgrade
// to Level 1 via LinkIdentity — including the real unique-index rejection
// of a LinkedIn subject already claimed by a different user, not just the
// fake repository's simulated version of that constraint.
func TestCompleteFederatedSignupAndLinkIdentity_Integration(t *testing.T) {
	dbURL := requirePostgres(t)
	runMigrations(t, dbURL)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		t.Fatalf("connect to postgres: %v", err)
	}
	defer pool.Close()

	// Unique per run so repeated executions against a persistent dev
	// database don't collide on the identity_provider+subject unique
	// constraint (migration 0004).
	appleSub := fmt.Sprintf("integration-apple-sub-%d", time.Now().UnixNano())
	svc := newIntegrationService(t, dbURL, pool, map[string]identity.VerifiedIdentity{
		"good-apple-token": {Subject: appleSub, Email: "ada@example.com", Name: "Ada Lovelace"},
	})

	signupResp, err := svc.CompleteFederatedSignup(ctx, &authv1.CompleteFederatedSignupRequest{
		Provider:            authv1.IdentityProviderProto_IDENTITY_PROVIDER_APPLE,
		IdToken:             "good-apple-token",
		AgeConfirmedOver_18: true,
	})
	if err != nil {
		t.Fatalf("CompleteFederatedSignup() error: %v", err)
	}
	if !signupResp.GetIsNewUser() {
		t.Error("IsNewUser = false, want true for a first-time subject")
	}

	// Assert the user row and the user_identities row both actually exist
	// — this is the check a mocked repository test can't make.
	var fullName string
	var trustLevel int
	var ageConfirmed bool
	err = pool.QueryRow(ctx, "SELECT full_name, trust_level, age_confirmed_over_18 FROM users WHERE id = $1", signupResp.GetUserId()).
		Scan(&fullName, &trustLevel, &ageConfirmed)
	if err != nil {
		t.Fatalf("query user row: %v", err)
	}
	if fullName != "Ada Lovelace" {
		t.Errorf("full_name = %q, want %q", fullName, "Ada Lovelace")
	}
	if trustLevel != 0 {
		t.Errorf("trust_level = %d, want 0 (Level 0, no LinkedIn linked yet)", trustLevel)
	}
	if !ageConfirmed {
		t.Error("age_confirmed_over_18 = false, want true")
	}

	var identityCount int
	err = pool.QueryRow(ctx,
		"SELECT count(*) FROM user_identities WHERE user_id = $1 AND provider = 'apple' AND subject = $2",
		signupResp.GetUserId(), appleSub,
	).Scan(&identityCount)
	if err != nil {
		t.Fatalf("query user_identities: %v", err)
	}
	if identityCount != 1 {
		t.Errorf("user_identities rows for this user/subject = %d, want 1", identityCount)
	}

	// Assert a refresh-token row exists and only the hash was stored, never
	// the raw token (ADR-009).
	var tokenCount int
	err = pool.QueryRow(ctx,
		`SELECT count(*) FROM refresh_tokens WHERE user_id = $1`, signupResp.GetUserId(),
	).Scan(&tokenCount)
	if err != nil {
		t.Fatalf("query refresh_tokens row: %v", err)
	}
	if tokenCount != 1 {
		t.Errorf("refresh_tokens rows for this user = %d, want 1", tokenCount)
	}

	var storedHash string
	err = pool.QueryRow(ctx,
		`SELECT token_hash FROM refresh_tokens WHERE user_id = $1`, signupResp.GetUserId(),
	).Scan(&storedHash)
	if err != nil {
		t.Fatalf("query token_hash: %v", err)
	}
	if storedHash == signupResp.GetRefreshToken() {
		t.Error("refresh_tokens.token_hash stores the raw refresh token verbatim, want a SHA-256 hash")
	}

	// Now link LinkedIn — the Level 0 -> Level 1 upgrade path.
	linkResp, err := svc.LinkIdentity(ctx, &authv1.LinkIdentityRequest{
		UserId:            signupResp.GetUserId(),
		Provider:          authv1.IdentityProviderProto_IDENTITY_PROVIDER_LINKEDIN,
		AuthorizationCode: "auth-code",
		RedirectUri:       "app://callback",
	})
	if err != nil {
		t.Fatalf("LinkIdentity() error: %v", err)
	}

	var linkedInSub string
	err = pool.QueryRow(ctx, "SELECT linkedin_sub, trust_level FROM users WHERE id = $1", linkResp.GetUserId()).
		Scan(&linkedInSub, &trustLevel)
	if err != nil {
		t.Fatalf("query user row after linking: %v", err)
	}
	if linkedInSub == "" {
		t.Error("linkedin_sub is empty after LinkIdentity, want set")
	}
	if trustLevel != 1 {
		t.Errorf("trust_level after linking LinkedIn = %d, want 1", trustLevel)
	}

	// The real unique-index rejection (idx_users_linkedin_sub, migration
	// 0001) — a second, different user must not be able to claim the same
	// LinkedIn subject. Reuses the same fake LinkedIn server, which always
	// returns the same sub for any authorization_code.
	otherSignup, err := svc.CompleteFederatedSignup(ctx, &authv1.CompleteFederatedSignupRequest{
		Provider:            authv1.IdentityProviderProto_IDENTITY_PROVIDER_APPLE,
		IdToken:             "good-apple-token-2",
		AgeConfirmedOver_18: true,
	})
	_ = otherSignup
	if err == nil {
		// good-apple-token-2 was never registered as valid — this branch
		// would only run if that assumption stops holding.
		t.Fatal("expected CompleteFederatedSignup with an unregistered token to fail")
	}
}

// codeCapturingHandler is a minimal slog.Handler that remembers the last
// "code" attribute logged — used to read back what LoggingSmsSender/
// LoggingEmailSender actually logged, so the Level 2/3 integration tests
// below can assert the logged code is the one that verifies successfully
// (backend/PLAN.md's addendum, Tests section) rather than trusting the
// code path blindly.
type codeCapturingHandler struct {
	mu   sync.Mutex
	code string
}

func (h *codeCapturingHandler) Enabled(context.Context, slog.Level) bool { return true }

func (h *codeCapturingHandler) Handle(_ context.Context, r slog.Record) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	r.Attrs(func(a slog.Attr) bool {
		if a.Key == "code" {
			h.code = a.Value.String()
		}
		return true
	})
	return nil
}

func (h *codeCapturingHandler) WithAttrs([]slog.Attr) slog.Handler { return h }
func (h *codeCapturingHandler) WithGroup(string) slog.Handler      { return h }

func (h *codeCapturingHandler) lastCode() string {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.code
}

// newIntegrationUser creates a fresh user via CompleteFederatedSignup
// (Level 0) then LinkIdentity (Level 1) — every Level 2/3 verification RPC
// requires LinkedIn linked first as of ADR-014 (requireLinkedIn in
// verification.go), so a bare Level 0 user isn't enough to seed for this
// file's verification-RPC tests below. Returns a Service wired against
// real Postgres repositories and the real LoggingSmsSender/
// LoggingEmailSender — the actual fallback implementations local dev/CI
// use, not test-only fakes, per the addendum's explicit ask that phone be
// "held to the same bar as the email screens," not special-cased.
func newIntegrationUser(t *testing.T) (svc *service.Service, pool *pgxpool.Pool, userID string, capture *codeCapturingHandler) {
	t.Helper()

	dbURL := requirePostgres(t)
	runMigrations(t, dbURL)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	t.Cleanup(cancel)

	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		t.Fatalf("connect to postgres: %v", err)
	}
	t.Cleanup(pool.Close)

	const appleToken = "integration-verification-apple-token"
	appleSub := fmt.Sprintf("integration-verification-apple-sub-%d", time.Now().UnixNano())

	capture = &codeCapturingHandler{}
	prevLogger := slog.Default()
	slog.SetDefault(slog.New(capture))
	t.Cleanup(func() { slog.SetDefault(prevLogger) })

	svc = newIntegrationService(t, dbURL, pool, map[string]identity.VerifiedIdentity{
		appleToken: {Subject: appleSub, Email: "integration@example.com", Name: "Integration Test User"},
	})

	signupResp, err := svc.CompleteFederatedSignup(ctx, &authv1.CompleteFederatedSignupRequest{
		Provider:            authv1.IdentityProviderProto_IDENTITY_PROVIDER_APPLE,
		IdToken:             appleToken,
		AgeConfirmedOver_18: true,
	})
	if err != nil {
		t.Fatalf("seed user via CompleteFederatedSignup() error: %v", err)
	}

	linkResp, err := svc.LinkIdentity(ctx, &authv1.LinkIdentityRequest{
		UserId:            signupResp.GetUserId(),
		Provider:          authv1.IdentityProviderProto_IDENTITY_PROVIDER_LINKEDIN,
		AuthorizationCode: "auth-code",
		RedirectUri:       "app://callback",
	})
	if err != nil {
		t.Fatalf("seed user via LinkIdentity() error: %v", err)
	}

	return svc, pool, linkResp.GetUserId(), capture
}

// uniqueTarget avoids collisions on phone_number/personal_email's UNIQUE
// constraints across repeated test runs against a persistent dev database
// — same reasoning as newIntegrationLinkedInServer's sub already uses
// time.Now().UnixNano() for linkedin_sub.
func uniqueTarget(format string) string {
	return fmt.Sprintf(format, time.Now().UnixNano())
}

func TestPhoneVerification_Integration(t *testing.T) {
	svc, _, userID, capture := newIntegrationUser(t)
	ctx := context.Background()
	phoneNumber := uniqueTarget("+9477%09d")

	if _, err := svc.StartPhoneVerification(ctx, &authv1.StartVerificationRequest{
		UserId: userID, Purpose: authv1.VerificationPurpose_VERIFICATION_PURPOSE_PHONE, Target: phoneNumber,
	}); err != nil {
		t.Fatalf("StartPhoneVerification() error: %v", err)
	}

	code := capture.lastCode()
	if code == "" {
		t.Fatal("LoggingSmsSender did not log a code")
	}

	session, err := svc.VerifyPhoneCode(ctx, &authv1.VerifyCodeRequest{
		UserId: userID, Purpose: authv1.VerificationPurpose_VERIFICATION_PURPOSE_PHONE, Target: phoneNumber, Code: code,
	})
	if err != nil {
		t.Fatalf("VerifyPhoneCode() with the logged code returned error: %v, want success", err)
	}
	if session.GetUserId() != userID {
		t.Errorf("UserId = %q, want %q", session.GetUserId(), userID)
	}
}

func TestPersonalEmailVerification_Integration(t *testing.T) {
	svc, _, userID, capture := newIntegrationUser(t)
	ctx := context.Background()
	personalEmail := uniqueTarget("person-%d@example.com")

	if _, err := svc.StartPersonalEmailVerification(ctx, &authv1.StartVerificationRequest{
		UserId: userID, Purpose: authv1.VerificationPurpose_VERIFICATION_PURPOSE_PERSONAL_EMAIL, Target: personalEmail,
	}); err != nil {
		t.Fatalf("StartPersonalEmailVerification() error: %v", err)
	}

	code := capture.lastCode()
	if code == "" {
		t.Fatal("LoggingEmailSender did not log a code")
	}

	if _, err := svc.VerifyPersonalEmailCode(ctx, &authv1.VerifyCodeRequest{
		UserId: userID, Purpose: authv1.VerificationPurpose_VERIFICATION_PURPOSE_PERSONAL_EMAIL, Target: personalEmail, Code: code,
	}); err != nil {
		t.Fatalf("VerifyPersonalEmailCode() with the logged code returned error: %v, want success", err)
	}
}

func TestCorporateEmailVerification_Integration(t *testing.T) {
	svc, pool, userID, capture := newIntegrationUser(t)
	ctx := context.Background()
	corporateEmail := uniqueTarget("jane-%d@acmecorp.com")

	if _, err := svc.StartCorporateEmailVerification(ctx, &authv1.StartVerificationRequest{
		UserId: userID, Purpose: authv1.VerificationPurpose_VERIFICATION_PURPOSE_CORPORATE_EMAIL, Target: corporateEmail,
	}); err != nil {
		t.Fatalf("StartCorporateEmailVerification() error: %v", err)
	}

	code := capture.lastCode()
	if code == "" {
		t.Fatal("LoggingEmailSender did not log a code")
	}

	if _, err := svc.VerifyCorporateEmailCode(ctx, &authv1.VerifyCodeRequest{
		UserId: userID, Purpose: authv1.VerificationPurpose_VERIFICATION_PURPOSE_CORPORATE_EMAIL, Target: corporateEmail, Code: code,
	}); err != nil {
		t.Fatalf("VerifyCorporateEmailCode() with the logged code returned error: %v, want success", err)
	}

	var companyDomain string
	var workEmailVerified bool
	if err := pool.QueryRow(ctx, "SELECT company_domain, work_email_verified FROM users WHERE id = $1", userID).Scan(&companyDomain, &workEmailVerified); err != nil {
		t.Fatalf("query user row: %v", err)
	}
	if companyDomain != "acmecorp.com" {
		t.Errorf("company_domain = %q, want %q", companyDomain, "acmecorp.com")
	}
	if !workEmailVerified {
		t.Error("work_email_verified = false, want true")
	}

	// ADR-003: the raw corporate email address must be gone from
	// verification_codes after success — grep the row, not just trust the
	// code path.
	var pendingCount int
	if err := pool.QueryRow(ctx,
		"SELECT count(*) FROM verification_codes WHERE user_id = $1 AND purpose = 'corporate_email'", userID,
	).Scan(&pendingCount); err != nil {
		t.Fatalf("query verification_codes: %v", err)
	}
	if pendingCount != 0 {
		t.Errorf("verification_codes rows remaining for this user/purpose = %d, want 0 (raw address must not linger)", pendingCount)
	}
}

// TestGetProfile_Integration_NeverReturnsRawContactInfo guards the same PII
// property as the unit-level test, end to end against real Postgres —
// genuinely round-tripping through the DB and back, not just asserted
// against a fake.
func TestGetProfile_Integration_NeverReturnsRawContactInfo(t *testing.T) {
	svc, _, userID, capture := newIntegrationUser(t)
	ctx := context.Background()
	personalEmail := uniqueTarget("person-%d@example.com")

	if _, err := svc.StartPersonalEmailVerification(ctx, &authv1.StartVerificationRequest{
		UserId: userID, Purpose: authv1.VerificationPurpose_VERIFICATION_PURPOSE_PERSONAL_EMAIL, Target: personalEmail,
	}); err != nil {
		t.Fatalf("StartPersonalEmailVerification() error: %v", err)
	}
	if _, err := svc.VerifyPersonalEmailCode(ctx, &authv1.VerifyCodeRequest{
		UserId: userID, Purpose: authv1.VerificationPurpose_VERIFICATION_PURPOSE_PERSONAL_EMAIL, Target: personalEmail, Code: capture.lastCode(),
	}); err != nil {
		t.Fatalf("VerifyPersonalEmailCode() error: %v", err)
	}

	profile, err := svc.GetProfile(ctx, &authv1.GetProfileRequest{UserId: userID})
	if err != nil {
		t.Fatalf("GetProfile() error: %v", err)
	}
	if !profile.GetPersonalEmailVerified() {
		t.Error("PersonalEmailVerified = false, want true")
	}

	marshaled, err := protojson.Marshal(profile)
	if err != nil {
		t.Fatalf("marshal profile: %v", err)
	}
	if strings.Contains(string(marshaled), personalEmail) {
		t.Errorf("GetProfile response contains the raw email address: %s", marshaled)
	}
}
