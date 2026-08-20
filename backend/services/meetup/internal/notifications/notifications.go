// Package notifications sends push notifications for meetup request
// events (ADR-013 § 6) — mirrors services/auth/internal/email and
// internal/sms's Sender-interface-plus-Logging-fallback pattern.
package notifications

import "context"

// Sender pushes a notification to every device userID has registered
// (internal/repository.DeviceTokenRepository) — the interface takes a user,
// not a device token directly, since resolving "which device(s) does this
// user have" is the sender's job (backend/meetup-scheduling-PLAN.md Step
// B). A user with no registered device token is not an error — there's
// simply nothing to push to yet (e.g. they haven't opened the app since
// this feature shipped).
type Sender interface {
	SendPushNotification(ctx context.Context, userID, title, body string, data map[string]string) error
}
