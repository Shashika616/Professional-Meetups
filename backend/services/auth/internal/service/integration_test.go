package service_test

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"database/sql"
	"encoding/pem"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib" // database/sql driver, needed only to drive migrate

	"github.com/professional-connections/backend/services/auth/internal/linkedin"
	"github.com/professional-connections/backend/services/auth/internal/repository"
	"github.com/professional-connections/backend/services/auth/internal/service"
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
		li,
		signer,
		noopPublisher{},
	)

	resp, err := svc.CompleteLinkedInOnboarding(ctx, &authv1.CompleteLinkedInOnboardingRequest{
		AuthorizationCode: "auth-code",
		PkceVerifier:      "verifier",
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
