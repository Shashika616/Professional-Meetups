// Command server wires up and runs the auth service's gRPC server. This
// file is wiring only: load config, construct dependencies, start serving,
// handle SIGTERM gracefully. Business logic lives in internal/service.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/grpc"

	"github.com/professional-connections/backend/services/auth/internal/config"
	"github.com/professional-connections/backend/services/auth/internal/events"
	"github.com/professional-connections/backend/services/auth/internal/linkedin"
	"github.com/professional-connections/backend/services/auth/internal/repository"
	"github.com/professional-connections/backend/services/auth/internal/service"
	sharedjwt "github.com/professional-connections/backend/shared/jwt"
	"github.com/professional-connections/backend/shared/logging"
	authv1 "github.com/professional-connections/backend/shared/proto/auth/v1"
)

func main() {
	logger := logging.New()

	if err := run(logger); err != nil {
		logger.Error("auth service exited with error", "error", err)
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

	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("connect to postgres: %w", err)
	}
	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		return fmt.Errorf("ping postgres: %w", err)
	}

	// Only the auth service ever constructs a Signer — the gateway holds
	// just the public key via a Verifier (ADR-009).
	signer, err := sharedjwt.NewSigner(cfg.JWTPrivateKeyPath)
	if err != nil {
		return fmt.Errorf("load jwt signing key: %w", err)
	}

	publisher, err := events.NewPublisher(ctx, cfg.GCPProjectID)
	if err != nil {
		return fmt.Errorf("create pubsub publisher: %w", err)
	}
	defer func() {
		if err := publisher.Close(); err != nil {
			logger.Error("closing pubsub publisher", "error", err)
		}
	}()

	linkedInClient := linkedin.New(linkedin.Config{
		ClientID:     cfg.LinkedInClientID,
		ClientSecret: cfg.LinkedInClientSecret,
	})

	svc := service.New(
		repository.NewUserRepository(pool),
		repository.NewRefreshTokenRepository(pool),
		linkedInClient,
		signer,
		publisher,
	)

	listener, err := net.Listen("tcp", ":"+cfg.GRPCPort)
	if err != nil {
		return fmt.Errorf("listen on port %s: %w", cfg.GRPCPort, err)
	}

	grpcServer := grpc.NewServer(grpc.UnaryInterceptor(logging.UnaryServerInterceptor()))
	authv1.RegisterAuthServiceServer(grpcServer, svc)

	serveErrCh := make(chan error, 1)
	go func() {
		logger.Info("auth service listening", "port", cfg.GRPCPort)
		serveErrCh <- grpcServer.Serve(listener)
	}()

	select {
	case err := <-serveErrCh:
		if err != nil && !errors.Is(err, grpc.ErrServerStopped) {
			return fmt.Errorf("grpc server: %w", err)
		}
		return nil
	case <-ctx.Done():
		logger.Info("shutting down auth service")
		grpcServer.GracefulStop()
		return nil
	}
}
