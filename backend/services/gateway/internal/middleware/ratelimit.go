package middleware

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"time"

	"github.com/redis/go-redis/v9"
)

// requestsPerMinute is the fixed-window limit applied per key.
const requestsPerMinute = 20

// emailKeyedPaths are the two email+password routes (ADR-014 decision #2,
// backend/level0-federated-identity-PLAN.md Step 4) where a rotating set of
// IPs hammering ONE account's password would sail past the IP-keyed limit
// below — an extension of the existing limiter, not a second subsystem.
// Every other /v1/auth/* route stays IP+path-keyed only, unchanged.
var emailKeyedPaths = map[string]bool{
	"/v1/auth/email/login":  true,
	"/v1/auth/email/signup": true,
}

// RateLimit is a Redis-backed fixed-window rate limiter: INCR + EXPIRE 60
// on the first increment for a given key, reject with 429 above
// requestsPerMinute requests/minute. Deliberately the simplest correct
// algorithm (fixed window), not a token bucket or sliding window — upgrade
// later only if the fixed-window edge effect (bursts at window boundaries)
// actually becomes a measured problem (PLAN.md Step 5). Applied only to the
// /v1/auth/* routes registered with it — those are the most abuse-prone,
// pre-auth surface (brute-forcing refresh, hammering the LinkedIn
// callback).
//
// emailKeyedPaths get a SECOND, additional check on top of the IP+path one
// below — both must pass, either can reject — so a single IP can't hammer
// many accounts (the original IP+path limit) and a rotating set of IPs
// can't hammer one account's password either (the new email+path limit).
func RateLimit(client *redis.Client) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()

			ipKey := fmt.Sprintf("ratelimit:%s:%s", clientIP(r), r.URL.Path)
			if rateLimited(ctx, client, ipKey) {
				writeRateLimited(w)
				return
			}

			if emailKeyedPaths[r.URL.Path] {
				if email, ok := peekRequestEmail(r); ok && email != "" {
					emailKey := fmt.Sprintf("ratelimit:email:%s:%s", r.URL.Path, email)
					if rateLimited(ctx, client, emailKey) {
						writeRateLimited(w)
						return
					}
				}
			}

			next.ServeHTTP(w, r)
		})
	}
}

// rateLimited increments key's fixed-window counter and reports whether it
// now exceeds requestsPerMinute. Fails open on a Redis error — rate
// limiting is defense in depth, not the only line of defense.
func rateLimited(ctx context.Context, client *redis.Client, key string) bool {
	count, err := client.Incr(ctx, key).Result()
	if err != nil {
		return false
	}
	if count == 1 {
		client.Expire(ctx, key, time.Minute)
	}
	return count > requestsPerMinute
}

func writeRateLimited(w http.ResponseWriter) {
	w.Header().Set("Retry-After", "60")
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusTooManyRequests)
	_, _ = w.Write([]byte(`{"error":"rate limited"}`))
}

// peekRequestEmail reads r.Body to extract an "email" field (present on
// both emailLoginRequest's and completeEmailSignupRequest's wire shape in
// internal/handlers), then restores r.Body so the downstream handler can
// still decode the full request normally — this middleware runs before any
// handler-level json.Decode, and a request body can only be read once.
func peekRequestEmail(r *http.Request) (string, bool) {
	body, err := io.ReadAll(r.Body)
	_ = r.Body.Close()
	r.Body = io.NopCloser(bytes.NewReader(body))
	if err != nil {
		return "", false
	}

	var parsed struct {
		Email string `json:"email"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return "", false
	}
	return parsed.Email, true
}

func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
