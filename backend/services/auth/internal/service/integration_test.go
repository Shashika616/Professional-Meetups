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
	"github.com/professional-connections/backend/services/auth/internal/linkedin"
	"github.com/professional-connections/backend/services/auth/internal/repository"
	"github.com/professional-connections/backend/services/auth/internal/service"
	"github.com/professional-connections/backend/services/auth/internal/sms"
	sharedjwt "github.com/professional-connections/backend/shared/jwt"
	authv1 "github.com/professional-connections/backend/shared/proto/auth/v1"
)

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

// TestCompleteLinkedInOnboarding_Integration exercises the full path this
// unit tests can't: a real Postgres, the actual migrations, and pgx/sqlc
// query execution — the class of bug a bad migration or a bad SQL query
// wouldn't be caught by mocked-repository unit tests (PLAN.md Step 4).
func TestCompleteLinkedInOnboarding_Integration(t *testing.T) {
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
	// database don't collide on the linkedin_sub unique constraint.
	sub := fmt.Sprintf("integration-test-sub-%d", time.Now().UnixNano())
	li, closeServer := newIntegrationLinkedInServer(t, sub)
	defer closeServer()

	signer := newIntegrationSigner(t)

	svc := service.New(
		repository.NewUserRepository(pool),
		repository.NewRefreshTokenRepository(pool),
		repository.NewVerificationCodeRepository(pool),
		li,
		signer,
		noopPublisher{},
		email.NewLoggingEmailSender(),
		sms.NewLoggingSmsSender(),
	)

	resp, err := svc.CompleteLinkedInOnboarding(ctx, &authv1.CompleteLinkedInOnboardingRequest{
		AuthorizationCode: "auth-code",
		RedirectUri:       "app://callback",
	})
	if err != nil {
		t.Fatalf("CompleteLinkedInOnboarding() error: %v", err)
	}
	if !resp.GetIsNewUser() {
		t.Error("IsNewUser = false, want true for a first-time sub")
	}

	// Assert a user row actually exists — this is the check a mocked
	// repository test can't make.
	var fullName string
	err = pool.QueryRow(ctx, "SELECT full_name FROM users WHERE linkedin_sub = $1", sub).Scan(&fullName)
	if err != nil {
		t.Fatalf("query user row: %v", err)
	}
	if fullName != "Integration Test User" {
		t.Errorf("full_name = %q, want %q", fullName, "Integration Test User")
	}

	// Assert a refresh-token row exists and only the hash was stored, never
	// the raw token (ADR-009).
	var tokenCount int
	err = pool.QueryRow(ctx,
		`SELECT count(*) FROM refresh_tokens rt JOIN users u ON u.id = rt.user_id WHERE u.linkedin_sub = $1`,
		sub,
	).Scan(&tokenCount)
	if err != nil {
		t.Fatalf("query refresh_tokens row: %v", err)
	}
	if tokenCount != 1 {
		t.Errorf("refresh_tokens rows for this user = %d, want 1", tokenCount)
	}

	var storedHash string
	err = pool.QueryRow(ctx,
		`SELECT token_hash FROM refresh_tokens rt JOIN users u ON u.id = rt.user_id WHERE u.linkedin_sub = $1`,
		sub,
	).Scan(&storedHash)
	if err != nil {
		t.Fatalf("query token_hash: %v", err)
	}
	if storedHash == resp.GetRefreshToken() {
		t.Error("refresh_tokens.token_hash stores the raw refresh token verbatim, want a SHA-256 hash")
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

// newIntegrationUser creates a fresh user via the real LinkedIn onboarding
// path (same as TestCompleteLinkedInOnboarding_Integration) and returns a
// Service wired against real Postgres repositories and the real
// LoggingSmsSender/LoggingEmailSender — the actual fallback implementations
// local dev/CI use, not test-only fakes, per the addendum's explicit ask
// that phone be "held to the same bar as the email screens," not special-
// cased.
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

	sub := fmt.Sprintf("integration-verification-sub-%d", time.Now().UnixNano())
	li, closeServer := newIntegrationLinkedInServer(t, sub)
	t.Cleanup(closeServer)

	capture = &codeCapturingHandler{}
	prevLogger := slog.Default()
	slog.SetDefault(slog.New(capture))
	t.Cleanup(func() { slog.SetDefault(prevLogger) })

	svc = service.New(
		repository.NewUserRepository(pool),
		repository.NewRefreshTokenRepository(pool),
		repository.NewVerificationCodeRepository(pool),
		li,
		newIntegrationSigner(t),
		noopPublisher{},
		email.NewLoggingEmailSender(),
		sms.NewLoggingSmsSender(),
	)

	resp, err := svc.CompleteLinkedInOnboarding(ctx, &authv1.CompleteLinkedInOnboardingRequest{
		AuthorizationCode: "auth-code",
		RedirectUri:       "app://callback",
	})
	if err != nil {
		t.Fatalf("seed user via CompleteLinkedInOnboarding() error: %v", err)
	}

	return svc, pool, resp.GetUserId(), capture
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
