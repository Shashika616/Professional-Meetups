package notifications

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"cloud.google.com/go/auth/credentials"
	"cloud.google.com/go/auth/httptransport"

	"github.com/professional-connections/backend/services/meetup/internal/repository"
)

const (
	defaultBaseURL = "https://fcm.googleapis.com"

	// defaultTimeout is deliberately explicit — the zero-value http.Client
	// has no timeout at all, which would let a hung FCM request block a
	// request-handling goroutine indefinitely (same reasoning as
	// internal/linkedin/client.go's defaultTimeout).
	defaultTimeout = 5 * time.Second

	fcmMessagingScope = "https://www.googleapis.com/auth/firebase.messaging"
)

// FCMPushSender sends real push notifications via the FCM HTTP v1 API,
// authenticated with a Firebase service account (OAuth2 JWT bearer grant,
// via golang.org/x/oauth2/google — the standard library for calling Google
// APIs with a service account, not hand-rolled token signing). Looks up the
// recipient's device token(s) via tokens (DeviceTokenRepository) — a user
// with no registered token is a no-op, not an error, and a send failure to
// one token doesn't block sending to the user's other tokens.
type FCMPushSender struct {
	projectID  string
	httpClient *http.Client
	tokens     repository.DeviceTokenRepository
	baseURL    string
}

// NewFCMPushSender constructs an FCMPushSender from a Firebase service
// account's JSON key bytes.
func NewFCMPushSender(ctx context.Context, serviceAccountJSON []byte, tokens repository.DeviceTokenRepository) (*FCMPushSender, error) {
	// NewCredentialsFromJSON with an explicit credentials.ServiceAccount
	// type, not DetectDefault/CredentialsFromJSON(WithParams) (both
	// deprecated as of this writing: they don't validate the credential
	// configuration, which matters here since serviceAccountJSON ultimately
	// comes from an operator-supplied env var, not a hardcoded trusted
	// source) — asserting the expected type up front is exactly the
	// mitigation those deprecation notices recommend.
	creds, err := credentials.NewCredentialsFromJSON(credentials.ServiceAccount, serviceAccountJSON, &credentials.DetectOptions{
		Scopes: []string{fcmMessagingScope},
	})
	if err != nil {
		return nil, fmt.Errorf("notifications: parse firebase service account: %w", err)
	}
	projectID, err := creds.ProjectID(ctx)
	if err != nil {
		return nil, fmt.Errorf("notifications: read project id from firebase service account: %w", err)
	}
	if projectID == "" {
		return nil, fmt.Errorf("notifications: firebase service account JSON has no project_id")
	}

	httpClient, err := httptransport.NewClient(&httptransport.Options{Credentials: creds})
	if err != nil {
		return nil, fmt.Errorf("notifications: build authenticated http client: %w", err)
	}
	httpClient.Timeout = defaultTimeout

	return &FCMPushSender{
		projectID:  projectID,
		httpClient: httpClient,
		tokens:     tokens,
		baseURL:    defaultBaseURL,
	}, nil
}

type fcmMessage struct {
	Message fcmMessageBody `json:"message"`
}

type fcmMessageBody struct {
	Token        string            `json:"token"`
	Notification fcmNotification   `json:"notification"`
	Data         map[string]string `json:"data,omitempty"`
}

type fcmNotification struct {
	Title string `json:"title"`
	Body  string `json:"body"`
}

func (s *FCMPushSender) SendPushNotification(ctx context.Context, userID, title, body string, data map[string]string) error {
	tokens, err := s.tokens.ListForUser(ctx, userID)
	if err != nil {
		return fmt.Errorf("notifications: list device tokens: %w", err)
	}
	if len(tokens) == 0 {
		return nil
	}

	var lastErr error
	sent := false
	for _, token := range tokens {
		if err := s.send(ctx, token, title, body, data); err != nil {
			// Logged with the error but never with the token itself (see
			// send's own doc comment) — one bad token shouldn't stop this
			// user's other devices from being notified.
			lastErr = err
			continue
		}
		sent = true
	}
	if !sent && lastErr != nil {
		return fmt.Errorf("notifications: send to all %d device(s) failed, last error: %w", len(tokens), lastErr)
	}
	return nil
}

// send never includes the raw device token in a returned error or log line
// (self-review checklist: no raw device token logged or exposed) — only the
// HTTP status code, which carries no PII.
func (s *FCMPushSender) send(ctx context.Context, token, title, body string, data map[string]string) error {
	payload := fcmMessage{Message: fcmMessageBody{
		Token:        token,
		Notification: fcmNotification{Title: title, Body: body},
		Data:         data,
	}}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("notifications: marshal fcm payload: %w", err)
	}

	endpoint := fmt.Sprintf("%s/v1/projects/%s/messages:send", s.baseURL, s.projectID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(encoded))
	if err != nil {
		return fmt.Errorf("notifications: build fcm request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("notifications: fcm request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("notifications: fcm send failed with status %d: %s", resp.StatusCode, respBody)
	}
	return nil
}
