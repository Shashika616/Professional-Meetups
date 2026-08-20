// Command server wires up and runs the meetup service's gRPC server. This
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

	"github.com/professional-connections/backend/services/meetup/internal/config"
	"github.com/professional-connections/backend/services/meetup/internal/events"
	"github.com/professional-connections/backend/services/meetup/internal/notifications"
	"github.com/professional-connections/backend/services/meetup/internal/repository"
	"github.com/professional-connections/backend/services/meetup/internal/service"
	"github.com/professional-connections/backend/shared/logging"
	meetupv1 "github.com/professional-connections/backend/shared/proto/meetup/v1"
)

func main() {
	logger := logging.New()
	// apperror.ToGRPCStatus and internal/service log unclassified/redacted
	// error detail via slog.Default() rather than threading a *slog.Logger
	// through every call site — matches services/auth/cmd/server/main.go.
	slog.SetDefault(logger)

	if err := run(logger); err != nil {
		logger.Error("meetup service exited with error", "error", err)
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

	publisher, err := events.NewPublisher(ctx, cfg.GCPProjectID)
	if err != nil {
		return fmt.Errorf("create pubsub publisher: %w", err)
	}
	defer func() {
		if err := publisher.Close(); err != nil {
			logger.Error("closing pubsub publisher", "error", err)
		}
	}()

	deviceTokens := repository.NewDeviceTokenRepository(pool)

	notificationSender, err := newPushSender(ctx, cfg, deviceTokens, logger)
	if err != nil {
		return fmt.Errorf("create push sender: %w", err)
	}

	svc := service.New(
		repository.NewMeetupRepository(pool),
		repository.NewMeetupRequestRepository(pool),
		repository.NewSafetyStateRepository(pool),
		repository.NewFeedbackRepository(pool),
		repository.NewRatingRepository(pool),
		deviceTokens,
		publisher,
		notificationSender,
	)

	listener, err := net.Listen("tcp", ":"+cfg.GRPCPort)
	if err != nil {
		return fmt.Errorf("listen on port %s: %w", cfg.GRPCPort, err)
	}

	grpcServer := grpc.NewServer(grpc.UnaryInterceptor(logging.UnaryServerInterceptor()))
	meetupv1.RegisterMeetupServiceServer(grpcServer, svc)

	serveErrCh := make(chan error, 1)
	go func() {
		logger.Info("meetup service listening", "port", cfg.GRPCPort)
		serveErrCh <- grpcServer.Serve(listener)
	}()

	select {
	case err := <-serveErrCh:
		if err != nil && !errors.Is(err, grpc.ErrServerStopped) {
			return fmt.Errorf("grpc server: %w", err)
		}
		return nil
	case <-ctx.Done():
		logger.Info("shutting down meetup service")
		grpcServer.GracefulStop()
		return nil
	}
}

// newPushSender uses FCMPushSender only once FIREBASE_SERVICE_ACCOUNT_JSON
// is non-empty — LoggingPushSender otherwise, so local dev/tests keep
// working before Shashika provides real Firebase credentials
// (backend/meetup-scheduling-PLAN.md's prerequisite).
func newPushSender(
	ctx context.Context, cfg config.Config, deviceTokens repository.DeviceTokenRepository, logger *slog.Logger,
) (notifications.Sender, error) {
	if cfg.FirebaseServiceAccountJSON == "" {
		logger.Info("push notification delivery: LoggingPushSender (FIREBASE_SERVICE_ACCOUNT_JSON not set)")
		return notifications.NewLoggingPushSender(), nil
	}

	sender, err := notifications.NewFCMPushSender(ctx, []byte(cfg.FirebaseServiceAccountJSON), deviceTokens)
	if err != nil {
		return nil, fmt.Errorf("construct FCM push sender: %w", err)
	}
	logger.Info("push notification delivery: FCM")
	return sender, nil
}
