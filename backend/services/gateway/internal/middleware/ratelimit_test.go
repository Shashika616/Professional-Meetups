package middleware

import (
	"bytes"
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

// TestRateLimitEmailKeyedAcrossRotatingIPs is the required test for Step
// 4's rate-limiter extension (backend/level0-federated-identity-PLAN.md) —
// proves a rotating set of IPs hammering ONE account's password on
// /v1/auth/email/login is still caught, even though each individual IP
// stays well under the plain IP+path limit.
func TestRateLimitEmailKeyedAcrossRotatingIPs(t *testing.T) {
	client := requireRedis(t)
	defer func() { _ = client.Close() }()

	// email/login is always evaluated at this exact path — use it directly
	// (unlike the other tests' unique-per-run paths) since emailKeyedPaths
	// is keyed by literal path, and confirm cleanup below doesn't leak
	// across runs by using a unique email instead.
	email := fmt.Sprintf("victim-%d@example.com", time.Now().UnixNano())
	body := fmt.Sprintf(`{"email":%q,"password":"guess"}`, email)

	var handlerCalls int
	allowed := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCalls++
		w.WriteHeader(http.StatusOK)
	})
	handler := RateLimit(client)(allowed)

	// Each request comes from a DIFFERENT IP — would sail past the plain
	// IP+path limiter entirely, since no single IP ever gets close to
	// requestsPerMinute on its own.
	for i := 0; i < requestsPerMinute; i++ {
		req := httptest.NewRequest(http.MethodPost, "/v1/auth/email/login", bytes.NewBufferString(body))
		req.RemoteAddr = fmt.Sprintf("198.51.100.%d:1", i%254+1)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("request %d (from a fresh IP): status = %d, want %d (should still be under the email-keyed limit)", i, rec.Code, http.StatusOK)
		}
	}

	// One more, from yet another new IP, same email — should trip the
	// email-keyed limit even though this specific IP has never been seen.
	req := httptest.NewRequest(http.MethodPost, "/v1/auth/email/login", bytes.NewBufferString(body))
	req.RemoteAddr = "198.51.100.250:1"
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusTooManyRequests {
		t.Errorf("status = %d, want %d — a rotating set of IPs hammering one email must still be caught", rec.Code, http.StatusTooManyRequests)
	}
	if handlerCalls != requestsPerMinute {
		t.Errorf("handler was called %d times, want exactly %d (the final, over-limit request must never reach the handler)", handlerCalls, requestsPerMinute)
	}

	// A DIFFERENT email from a fresh IP must be unaffected — the email-keyed
	// limit is per-email, not a blanket lockout of the whole route.
	otherBody := fmt.Sprintf(`{"email":"someone-else-%d@example.com","password":"x"}`, time.Now().UnixNano())
	otherReq := httptest.NewRequest(http.MethodPost, "/v1/auth/email/login", bytes.NewBufferString(otherBody))
	otherReq.RemoteAddr = "198.51.100.251:1"
	otherRec := httptest.NewRecorder()
	handler.ServeHTTP(otherRec, otherReq)
	if otherRec.Code != http.StatusOK {
		t.Errorf("a different email was rate limited by another email's usage: status = %d, want %d", otherRec.Code, http.StatusOK)
	}
}
