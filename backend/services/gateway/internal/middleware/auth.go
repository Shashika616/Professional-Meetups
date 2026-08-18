package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/professional-connections/backend/shared/jwt"
)

type contextKey int

const userIDKey contextKey = iota

// WithUserID returns a context carrying the authenticated user's ID —
// exported mainly for tests; Auth is the only production caller.
func WithUserID(ctx context.Context, userID string) context.Context {
	return context.WithValue(ctx, userIDKey, userID)
}

// UserIDFromContext returns the authenticated user's ID that Auth attached
// to the request context, or "" if none is present (an unauthenticated
// route, or Auth wasn't applied).
func UserIDFromContext(ctx context.Context) string {
	id, _ := ctx.Value(userIDKey).(string)
	return id
}

// Auth verifies the "Authorization: Bearer <access_token>" header using
// verifier, rejecting with 401 if the header is missing or the token is
// invalid/expired, and attaches the verified user_id to the request
// context (retrievable via UserIDFromContext) before calling next.
//
// This is genuinely new wiring, not a reuse of something that already
// runs — confirmed by reading the actual gateway code: nothing here
// constructed a jwt.Verifier before this (backend/PLAN.md's Level 2/3
// addendum, Step F). Applied per-route at registration time (see
// internal/handlers.Register), not globally — the LinkedIn/refresh/logout
// routes stay unauthenticated at the gateway layer, as they are today.
func Auth(verifier *jwt.Verifier) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token, ok := bearerToken(r)
			if !ok {
				writeUnauthorized(w)
				return
			}

			claims, err := verifier.Verify(token)
			if err != nil {
				writeUnauthorized(w)
				return
			}

			next.ServeHTTP(w, r.WithContext(WithUserID(r.Context(), claims.UserID)))
		})
	}
}

func bearerToken(r *http.Request) (string, bool) {
	const prefix = "Bearer "
	header := r.Header.Get("Authorization")
	if !strings.HasPrefix(header, prefix) {
		return "", false
	}
	token := strings.TrimPrefix(header, prefix)
	if token == "" {
		return "", false
	}
	return token, true
}

func writeUnauthorized(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	_, _ = w.Write([]byte(`{"error":"unauthorized"}`))
}
