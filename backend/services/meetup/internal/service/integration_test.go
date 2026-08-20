package service_test

import (
	"context"
	"database/sql"
	"fmt"
	"net"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib" // database/sql driver, needed only to drive migrate
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/professional-connections/backend/services/meetup/internal/repository"
	"github.com/professional-connections/backend/services/meetup/internal/service"
	meetupv1 "github.com/professional-connections/backend/shared/proto/meetup/v1"
)

// defaultTestDatabaseURL matches docker-compose.yml's local Postgres
// defaults — same pattern as services/auth/internal/service/
// integration_test.go.
const defaultTestDatabaseURL = "postgres://app:app@localhost:5432/professional_connections?sslmode=disable"

// migrationsPath is relative to this file's directory (Go tests always run
// with cwd set to the package directory) — same depth as
// services/auth/internal/service, so the same relative path resolves to
// the same shared db/migrations.
const migrationsPath = "file://../../../../db/migrations"

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

// noopPublisher/noopNotificationSender stand in for the real Pub/Sub
// publisher and FCM sender — these tests assert against Postgres state and
// service-layer behavior, not delivery, and don't require the Pub/Sub
// emulator or a real Firebase project (same reasoning as
// services/auth/internal/service/integration_test.go's noopPublisher).
type noopPublisher struct{}

func (noopPublisher) PublishRequestCreated(context.Context, string, string, string, string) error {
	return nil
}
func (noopPublisher) PublishRequestAccepted(context.Context, string, string, string, string) error {
	return nil
}
func (noopPublisher) PublishRequestRejected(context.Context, string, string, string, string, bool) error {
	return nil
}
func (noopPublisher) Close() error { return nil }

type noopNotificationSender struct{}

func (noopNotificationSender) SendPushNotification(context.Context, string, string, string, map[string]string) error {
	return nil
}

// newIntegrationService constructs a real, Postgres-backed Service plus the
// raw pool (for direct assertions the repository interfaces don't expose,
// like counting rows by hand) and creates hostID/two requester IDs as real
// user rows — the meetup service owns no user-creation RPC of its own, so
// integration tests seed users directly, same boundary-crossing-read
// reasoning as the postgres queries themselves.
func newIntegrationService(t *testing.T) (svc *service.Service, pool *pgxpool.Pool, hostID string, requesterIDs []string) {
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

	nonce := time.Now().UnixNano()
	hostID = seedUser(t, ctx, pool, fmt.Sprintf("meetup-integration-host-%d", nonce))
	requesterIDs = []string{
		seedUser(t, ctx, pool, fmt.Sprintf("meetup-integration-requester-a-%d", nonce)),
		seedUser(t, ctx, pool, fmt.Sprintf("meetup-integration-requester-b-%d", nonce)),
	}

	svc = service.New(
		repository.NewMeetupRepository(pool),
		repository.NewMeetupRequestRepository(pool),
		repository.NewSafetyStateRepository(pool),
		repository.NewFeedbackRepository(pool),
		repository.NewRatingRepository(pool),
		repository.NewDeviceTokenRepository(pool),
		noopPublisher{},
		noopNotificationSender{},
	)
	return svc, pool, hostID, requesterIDs
}

// seedUser inserts a minimal user row at trust level 2 (enough to pass
// every intent gate this test package exercises) and returns its id.
func seedUser(t *testing.T, ctx context.Context, pool *pgxpool.Pool, linkedInSub string) string {
	t.Helper()

	var id string
	err := pool.QueryRow(ctx,
		`INSERT INTO users (linkedin_sub, full_name, trust_level) VALUES ($1, $2, 2) RETURNING id`,
		linkedInSub, "Integration Test User",
	).Scan(&id)
	if err != nil {
		t.Fatalf("seed user: %v", err)
	}
	return id
}

// TestCapacityRace_Integration exercises the concurrency bug a mocked-
// repository unit test structurally cannot catch: two RespondToRequest
// accept calls, fired genuinely concurrently, against a capacity-1 meetup
// with two pending requests. Exactly one must succeed; the other must be
// rejected with a clear "meetup full"/conflict reason; there must never be
// a double-accept past capacity (backend/meetup-scheduling-PLAN.md's Test
// strategy, § 4).
func TestCapacityRace_Integration(t *testing.T) {
	svc, pool, hostID, requesterIDs := newIntegrationService(t)
	ctx := context.Background()

	created, err := svc.CreateMeetup(ctx, &meetupv1.CreateMeetupRequest{
		HostUserId: hostID, HostTrustLevel: 2, Intent: meetupv1.Intent_INTENT_COFFEE,
		LocationLabel: "Race Test Cafe", LocationLat: 6.9, LocationLng: 79.8, Capacity: 1,
		WindowStartUnixSeconds: time.Now().Add(time.Hour).Unix(),
		WindowEndUnixSeconds:   time.Now().Add(3 * time.Hour).Unix(),
	})
	if err != nil {
		t.Fatalf("CreateMeetup() error: %v", err)
	}
	meetupID := created.GetId()

	var requestIDs [2]string
	for i, requesterID := range requesterIDs {
		req, err := svc.RequestToJoin(ctx, &meetupv1.RequestToJoinRequest{
			MeetupId: meetupID, RequesterId: requesterID, RequesterTrustLevel: 2,
		})
		if err != nil {
			t.Fatalf("RequestToJoin() error: %v", err)
		}
		requestIDs[i] = req.GetId()
	}

	// Fire both accept calls as close to simultaneously as possible —
	// a start barrier (the WaitGroup below) rather than launching them
	// sequentially, so this actually exercises the row lock rather than
	// two calls that happen to be near each other in test code.
	var wg sync.WaitGroup
	start := make(chan struct{})
	results := make([]error, 2)
	for i, requestID := range requestIDs {
		wg.Add(1)
		go func(i int, requestID string) {
			defer wg.Done()
			<-start
			_, err := svc.RespondToRequest(ctx, &meetupv1.RespondToRequestRequest{
				RequestId: requestID, HostUserId: hostID, Accept: true,
			})
			results[i] = err
		}(i, requestID)
	}
	close(start)
	wg.Wait()

	succeeded, failed := 0, 0
	var failureCode codes.Code
	for _, err := range results {
		if err == nil {
			succeeded++
		} else {
			failed++
			failureCode = status.Code(err)
		}
	}
	if succeeded != 1 {
		t.Errorf("succeeded accepts = %d, want exactly 1", succeeded)
	}
	if failed != 1 {
		t.Errorf("failed accepts = %d, want exactly 1", failed)
	}
	if failed == 1 && failureCode != codes.AlreadyExists {
		t.Errorf("failed accept's code = %v, want %v (Conflict — meetup full)", failureCode, codes.AlreadyExists)
	}

	// Verify actual DB state directly, not just the RPC results — the real
	// point of this test.
	var acceptedCount, rejectedCount int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM meetup_requests WHERE meetup_id = $1 AND status = 'accepted'`, meetupID).Scan(&acceptedCount); err != nil {
		t.Fatalf("count accepted requests: %v", err)
	}
	if acceptedCount != 1 {
		t.Errorf("accepted requests in DB = %d, want exactly 1 (no double-accept past capacity)", acceptedCount)
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM meetup_requests WHERE meetup_id = $1 AND status = 'rejected' AND auto_rejected = true`, meetupID).Scan(&rejectedCount); err != nil {
		t.Fatalf("count auto-rejected requests: %v", err)
	}
	if rejectedCount != 1 {
		t.Errorf("auto-rejected requests in DB = %d, want exactly 1", rejectedCount)
	}

	var meetupStatus string
	if err := pool.QueryRow(ctx, `SELECT status FROM meetups WHERE id = $1`, meetupID).Scan(&meetupStatus); err != nil {
		t.Fatalf("get meetup status: %v", err)
	}
	if meetupStatus != "full" {
		t.Errorf("meetup status = %q, want %q", meetupStatus, "full")
	}
}

// TestCreateAndGetMeetup_Integration is a basic real-Postgres round trip —
// catches the class of bug (a bad migration, a bad SQL query) mocked-
// repository unit tests structurally can't.
func TestCreateAndGetMeetup_Integration(t *testing.T) {
	svc, _, hostID, _ := newIntegrationService(t)
	ctx := context.Background()

	created, err := svc.CreateMeetup(ctx, &meetupv1.CreateMeetupRequest{
		HostUserId: hostID, HostTrustLevel: 2, Intent: meetupv1.Intent_INTENT_LUNCH,
		LocationLabel: "Test Restaurant", LocationLat: 6.91, LocationLng: 79.85, Capacity: 4,
		WindowStartUnixSeconds: time.Now().Add(time.Hour).Unix(),
		WindowEndUnixSeconds:   time.Now().Add(3 * time.Hour).Unix(),
	})
	if err != nil {
		t.Fatalf("CreateMeetup() error: %v", err)
	}
	if created.GetHostFullName() == "" {
		t.Errorf("CreateMeetup() response has no host_full_name — the follow-up GetByID join didn't populate it")
	}

	fetched, err := svc.GetMeetup(ctx, &meetupv1.GetMeetupRequest{MeetupId: created.GetId(), UserId: hostID})
	if err != nil {
		t.Fatalf("GetMeetup() error: %v", err)
	}
	if !fetched.GetIsHostedByMe() {
		t.Errorf("GetMeetup() is_hosted_by_me = false, want true for the host viewing their own meetup")
	}
	if fetched.GetStatus() != meetupv1.MeetupStatus_MEETUP_STATUS_OPEN {
		t.Errorf("status = %v, want OPEN", fetched.GetStatus())
	}
}
