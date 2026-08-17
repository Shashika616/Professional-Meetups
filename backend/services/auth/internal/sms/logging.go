package sms

import (
	"context"
	"log/slog"
)

// LoggingSmsSender logs the code instead of sending an SMS — the default
// sender for local dev and every automated test, used whenever Twilio's
// env vars are empty. Never logs the raw target number (self-review
// checklist: no raw phone/email in any log line) — a developer testing
// locally already knows which number they just entered, the code is the
// only thing they need from this log line.
type LoggingSmsSender struct{}

// NewLoggingSmsSender constructs a LoggingSmsSender.
func NewLoggingSmsSender() *LoggingSmsSender {
	return &LoggingSmsSender{}
}

func (s *LoggingSmsSender) SendVerificationCode(_ context.Context, _, code string) error {
	slog.Default().Info("verification code (LoggingSmsSender — not actually sent)",
		"purpose", "phone",
		"code", code,
	)
	return nil
}
