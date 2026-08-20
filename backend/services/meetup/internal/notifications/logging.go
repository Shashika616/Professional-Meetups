package notifications

import (
	"context"
	"log/slog"
)

// LoggingPushSender logs the notification payload instead of calling FCM —
// the default sender for local dev and every automated test, used whenever
// FIREBASE_SERVICE_ACCOUNT_JSON is empty (backend/meetup-scheduling-PLAN.md
// Step B). Never logs a raw device token (self-review checklist) — there is
// none to log at this layer, since the interface takes a userID and any
// token lookup happens only inside FCMPushSender.
type LoggingPushSender struct{}

// NewLoggingPushSender constructs a LoggingPushSender.
func NewLoggingPushSender() *LoggingPushSender {
	return &LoggingPushSender{}
}

func (s *LoggingPushSender) SendPushNotification(_ context.Context, userID, title, body string, data map[string]string) error {
	slog.Default().Info("push notification (LoggingPushSender — not actually sent)",
		"user_id", userID,
		"title", title,
		"body", body,
		"data", data,
	)
	return nil
}
