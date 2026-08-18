package middleware

import (
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/redis/go-redis/v9"
)

// requestsPerMinute is the fixed-window limit applied per (IP, route).
const requestsPerMinute = 20

// RateLimit is a Redis-backed fixed-window rate limiter: INCR + EXPIRE 60
// on the first increment for a given key, reject with 429 above
// requestsPerMinute requests/minute. Deliberately the simplest correct
// algorithm (fixed window), not a token bucket or sliding window — upgrade
// later only if the fixed-window edge effect (bursts at window boundaries)
// actually becomes a measured problem (PLAN.md Step 5). Applied only to the
// /v1/auth/* routes registered with it — those are the most abuse-prone,
// pre-auth surface (brute-forcing refresh, hammering the LinkedIn
// callback).
func RateLimit(client *redis.Client) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			key := fmt.Sprintf("ratelimit:%s:%s", clientIP(r), r.URL.Path)
			ctx := r.Context()

			count, err := client.Incr(ctx, key).Result()
			if err != nil {
				// Fail open: a Redis outage shouldn't take the auth surface
				// down entirely — rate limiting is defense in depth, not
				// the only line of defense.
				next.ServeHTTP(w, r)
				return
			}
			if count == 1 {
				client.Expire(ctx, key, time.Minute)
			}

			if count > requestsPerMinute {
				w.Header().Set("Retry-After", "60")
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusTooManyRequests)
				_, _ = w.Write([]byte(`{"error":"rate limited"}`))
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
