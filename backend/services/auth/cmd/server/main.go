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
	"github.com/professional-connections/backend/services/auth/internal/email"
	"github.com/professional-connections/backend/services/auth/internal/events"
	"github.com/professional-connections/backend/services/auth/internal/identity"
	"github.com/professional-connections/backend/services/auth/internal/linkedin"
	"github.com/professional-connections/backend/services/auth/internal/repository"
	"github.com/professional-connections/backend/services/auth/internal/service"
	"github.com/professional-connections/backend/services/auth/internal/sms"
	sharedjwt "github.com/professional-connections/backend/shared/jwt"
	"github.com/professional-connections/backend/shared/logging"
	authv1 "github.com/professional-connections/backend/shared/proto/auth/v1"
)

func main() {
	logger := logging.New()
	// apperror.ToGRPCStatus and internal/service log unclassified/redacted
	// error detail via slog.Default() rather than threading a *slog.Logger
	// through every call site — this makes that output match the rest of
	// the service's Cloud-Logging JSON format instead of slog's plain-text
	// default.
	slog.SetDefault(logger)

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

	// AppleServicesID/GoogleClientID may legitimately be "" here (real
	// credentials not yet issued, Action Tracker §1) — both providers
	// still construct successfully and fetch their real JWKS at startup;
	// Verify simply rejects every token until a real audience is
	// configured (identity.Verify's own doc comment).
	appleProvider, err := identity.NewAppleProvider(ctx, cfg.AppleServicesID)
	if err != nil {
		return fmt.Errorf("construct apple identity provider: %w", err)
	}
	googleProvider, err := identity.NewGoogleProvider(ctx, cfg.GoogleClientID)
	if err != nil {
		return fmt.Errorf("construct google identity provider: %w", err)
	}

	emailSender := newEmailSender(cfg, logger)
	smsSender := newSmsSender(cfg, logger)

	svc := service.New(
		repository.NewUserRepository(pool),
		repository.NewUserIdentityRepository(pool),
		repository.NewRefreshTokenRepository(pool),
		repository.NewVerificationCodeRepository(pool),
		linkedInClient,
		appleProvider,
		googleProvider,
		signer,
		publisher,
		emailSender,
		smsSender,
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

// newEmailSender prefers GmailSMTPEmailSender when GMAIL_ADDRESS/
// GMAIL_APP_PASSWORD are both set — a Gmail account can send to any
// recipient immediately, unlike a Resend sandbox account (limited to the
// account owner's own inbox until a domain is verified there), which makes
// it the better default for testing signup with arbitrary addresses. Falls
// back to ResendEmailSender if only Resend is configured, then to
// LoggingEmailSender if neither is, so local dev/tests keep working before
// either exists (backend/PLAN.md's Level 2/3 addendum, Step A).
func newEmailSender(cfg config.Config, logger *slog.Logger) email.EmailSender {
	if cfg.GmailAddress != "" && cfg.GmailAppPassword != "" {
		logger.Info("verification email delivery: Gmail SMTP")
		return email.NewGmailSMTPEmailSender(cfg.GmailAddress, cfg.GmailAppPassword)
	}
	if cfg.ResendAPIKey != "" && cfg.ResendFromEmail != "" {
		logger.Info("verification email delivery: Resend")
		return email.NewResendEmailSender(cfg.ResendAPIKey, cfg.ResendFromEmail)
	}
	logger.Info("verification email delivery: LoggingEmailSender (GMAIL_ADDRESS/GMAIL_APP_PASSWORD and RESEND_API_KEY/RESEND_FROM_EMAIL not set)")
	return email.NewLoggingEmailSender()
}

// newSmsSender uses TwilioSmsSender only once all three TWILIO_* vars are
// non-empty — LoggingSmsSender otherwise, same fallback pattern as email.
func newSmsSender(cfg config.Config, logger *slog.Logger) sms.SmsSender {
	if cfg.TwilioAccountSID != "" && cfg.TwilioAuthToken != "" && cfg.TwilioPhoneNumber != "" {
		logger.Info("verification SMS delivery: Twilio")
		return sms.NewTwilioSmsSender(cfg.TwilioAccountSID, cfg.TwilioAuthToken, cfg.TwilioPhoneNumber)
	}
	logger.Info("verification SMS delivery: LoggingSmsSender (TWILIO_* not fully set)")
	return sms.NewLoggingSmsSender()
}
