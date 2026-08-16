package middleware

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

// requireRedis skips the test if Redis isn't reachable within a short
// timeout, mirroring services/auth's requirePostgres — so a plain
// `go test ./...` stays fast when a developer hasn't run
// `docker compose up` (PLAN.md Step 5: "one rate-limiter test against a
// real local Redis").
func requireRedis(t *testing.T) *redis.Client {
	t.Helper()

	addr := os.Getenv("REDIS_ADDR")
	if addr == "" {
		addr = "localhost:6379"
	}

	client := redis.NewClient(&redis.Options{Addr: addr})

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	if err := client.Ping(ctx).Err(); err != nil {
		_ = client.Close()
		t.Skipf("redis not reachable at %s, skipping (run `docker compose up` first): %v", addr, err)
	}

	return client
}

func TestRateLimit(t *testing.T) {
	client := requireRedis(t)
	defer func() { _ = client.Close() }()

	// Unique path per test run so leftover keys from a previous run (or a
	// concurrent test) never leak into this one's window.
	path := fmt.Sprintf("/v1/auth/test-%d", time.Now().UnixNano())

	allowed := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := RateLimit(client)(allowed)

	for i := 1; i <= requestsPerMinute; i++ {
		req := httptest.NewRequest(http.MethodPost, path, nil)
		req.RemoteAddr = "203.0.113.1:12345"
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("request %d: status = %d, want %d (should still be under the limit)", i, rec.Code, http.StatusOK)
		}
	}

	// One more, over the limit.
	req := httptest.NewRequest(http.MethodPost, path, nil)
	req.RemoteAddr = "203.0.113.1:12345"
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusTooManyRequests {
		t.Errorf("request %d: status = %d, want %d", requestsPerMinute+1, rec.Code, http.StatusTooManyRequests)
	}
}

func TestRateLimitScopedPerIP(t *testing.T) {
	client := requireRedis(t)
	defer func() { _ = client.Close() }()

	path := fmt.Sprintf("/v1/auth/test-%d", time.Now().UnixNano())

	allowed := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := RateLimit(client)(allowed)

	// Exhaust the limit for one IP.
	for i := 0; i < requestsPerMinute; i++ {
		req := httptest.NewRequest(http.MethodPost, path, nil)
		req.RemoteAddr = "203.0.113.2:1"
		handler.ServeHTTP(httptest.NewRecorder(), req)
	}

	// A different IP hitting the same path should be unaffected.
	req := httptest.NewRequest(http.MethodPost, path, nil)
	req.RemoteAddr = "203.0.113.3:1"
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("a different IP was rate limited by another IP's usage: status = %d, want %d", rec.Code, http.StatusOK)
	}
}
