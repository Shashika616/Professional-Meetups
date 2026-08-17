package email

import (
	"context"
	"log/slog"
)

// LoggingEmailSender logs the code instead of sending an email — the
// default sender for local dev and every automated test (backend/PLAN.md's
// addendum, Step A/C-D), used whenever RESEND_API_KEY is empty. Never logs
// the raw target address (self-review checklist: no raw phone/email in any
// log line) — a developer testing locally already knows which address they
// just entered, the code is the only thing they need from this log line.
type LoggingEmailSender struct{}

// NewLoggingEmailSender constructs a LoggingEmailSender.
func NewLoggingEmailSender() *LoggingEmailSender {
	return &LoggingEmailSender{}
}

func (s *LoggingEmailSender) SendVerificationCode(_ context.Context, _, code string, purpose Purpose) error {
	slog.Default().Info("verification code (LoggingEmailSender — not actually sent)",
		"purpose", purpose,
		"code", code,
	)
	return nil
}
