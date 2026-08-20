// Command server wires up and runs the gateway's HTTP server. This file is
// wiring only: load config, construct dependencies, start serving, handle
// SIGTERM gracefully. Route logic lives in internal/handlers.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/professional-connections/backend/services/gateway/internal/authclient"
	"github.com/professional-connections/backend/services/gateway/internal/config"
	"github.com/professional-connections/backend/services/gateway/internal/handlers"
	"github.com/professional-connections/backend/services/gateway/internal/meetupclient"
	"github.com/professional-connections/backend/services/gateway/internal/middleware"
	sharedjwt "github.com/professional-connections/backend/shared/jwt"
	"github.com/professional-connections/backend/shared/logging"
)

// shutdownTimeout bounds how long graceful shutdown waits for in-flight
// requests to finish before forcing a close.
const shutdownTimeout = 10 * time.Second

func main() {
	logger := logging.New()

	if err := run(logger); err != nil {
		logger.Error("gateway exited with error", "error", err)
		os.Exit(1)
	}
}

func run(logger *slog.Logger) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	auth, err := authclient.New(cfg.AuthServiceAddr)
	if err != nil {
		return fmt.Errorf("connect to auth service: %w", err)
	}
	defer func() {
		if err := auth.Close(); err != nil {
			logger.Error("closing auth client", "error", err)
		}
	}()

	meetup, err := meetupclient.New(cfg.MeetupServiceAddr)
	if err != nil {
		return fmt.Errorf("connect to meetup service: %w", err)
	}
	defer func() {
		if err := meetup.Close(); err != nil {
			logger.Error("closing meetup client", "error", err)
		}
	}()

	redisClient := redis.NewClient(&redis.Options{Addr: cfg.RedisAddr})
	defer func() {
		if err := redisClient.Close(); err != nil {
			logger.Error("closing redis client", "error", err)
		}
	}()
	if err := redisClient.Ping(ctx).Err(); err != nil {
		return fmt.Errorf("connect to redis: %w", err)
	}

	// First time the gateway constructs a jwt.Verifier — everything before
	// the Level 2/3 addendum was unauthenticated at this layer (confirmed
	// by reading the actual pre-addendum code, not assumed). Fails fast at
	// startup if the public key is missing/unreadable, same discipline as
	// every other required config value.
	verifier, err := sharedjwt.NewVerifier(cfg.JWTPublicKeyPath)
	if err != nil {
		return fmt.Errorf("load jwt public key: %w", err)
	}

	mux := http.NewServeMux()
	handlers.New(auth, meetup, verifier).Register(mux)

	// middleware.RateLimit keys its fixed window on (IP, route path), so
	// wrapping the whole mux gives every route — /v1/auth/*,
	// /v1/verification/*, /v1/users/me, and now /v1/meetups/* — its own
	// independent 20-req/min-per-IP limit, not one shared budget across all
	// of them. CreateMeetup/RequestToJoin are meant to share this same
	// mechanism (backend/meetup-scheduling-PLAN.md Step D: a spam-created-
	// meetups or spam-join-requests vector is the same shape of abuse as
	// spam-OTP-sends), so no second limiter was needed when those routes
	// were added.
	var handler http.Handler = mux
	handler = middleware.RateLimit(redisClient)(handler)
	handler = middleware.RequestLogging(logger)(handler)
	handler = logging.HTTPMiddleware(handler)
	handler = middleware.Recover(logger)(handler)

	server := &http.Server{
		Addr:    ":" + cfg.Port,
		Handler: handler,
	}

	serveErrCh := make(chan error, 1)
	go func() {
		logger.Info("gateway listening", "port", cfg.Port)
		serveErrCh <- server.ListenAndServe()
	}()

	select {
	case err := <-serveErrCh:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("http server: %w", err)
		}
		return nil
	case <-ctx.Done():
		logger.Info("shutting down gateway")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("graceful shutdown: %w", err)
		}
		return nil
	}
}
