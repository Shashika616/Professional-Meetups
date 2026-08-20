package notifications

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/professional-connections/backend/services/meetup/internal/repository"
)

// fakeDeviceTokenRepository is a minimal in-package stand-in for
// repository.DeviceTokenRepository, just enough for FCMPushSender's tests.
type fakeDeviceTokenRepository struct {
	mu     sync.Mutex
	tokens map[string][]string
}

func (f *fakeDeviceTokenRepository) Upsert(_ context.Context, userID, token string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.tokens[userID] = append(f.tokens[userID], token)
	return nil
}

func (f *fakeDeviceTokenRepository) ListForUser(_ context.Context, userID string) ([]string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.tokens[userID], nil
}

var _ repository.DeviceTokenRepository = (*fakeDeviceTokenRepository)(nil)

// newTestFCMPushSender constructs an FCMPushSender pointed at a test
// server, bypassing NewFCMPushSender's real OAuth2/service-account flow
// entirely — this is a same-package test, so it can build the struct
// literal directly with a plain (unauthenticated) http.Client, which is all
// a fake FCM server needs.
func newTestFCMPushSender(baseURL string, tokens repository.DeviceTokenRepository) *FCMPushSender {
	return &FCMPushSender{
		projectID:  "test-project",
		httpClient: http.DefaultClient,
		tokens:     tokens,
		baseURL:    baseURL,
	}
}

func TestFCMPushSender_PayloadShape(t *testing.T) {
	var gotPath string
	var gotBody fcmMessage
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	tokens := &fakeDeviceTokenRepository{tokens: map[string][]string{"user-1": {"device-token-abc"}}}
	sender := newTestFCMPushSender(server.URL, tokens)

	if err := sender.SendPushNotification(context.Background(), "user-1", "New request", "Someone wants to join", map[string]string{"meetup_id": "meetup-1"}); err != nil {
		t.Fatalf("SendPushNotification() error: %v", err)
	}

	wantPath := "/v1/projects/test-project/messages:send"
	if gotPath != wantPath {
		t.Errorf("request path = %q, want %q", gotPath, wantPath)
	}
	if gotBody.Message.Token != "device-token-abc" {
		t.Errorf("message.token = %q, want %q", gotBody.Message.Token, "device-token-abc")
	}
	if gotBody.Message.Notification.Title != "New request" {
		t.Errorf("message.notification.title = %q, want %q", gotBody.Message.Notification.Title, "New request")
	}
	if gotBody.Message.Notification.Body != "Someone wants to join" {
		t.Errorf("message.notification.body = %q, want %q", gotBody.Message.Notification.Body, "Someone wants to join")
	}
	if gotBody.Message.Data["meetup_id"] != "meetup-1" {
		t.Errorf("message.data[meetup_id] = %q, want %q", gotBody.Message.Data["meetup_id"], "meetup-1")
	}
}

func TestFCMPushSender_NoRegisteredTokenIsANoOp(t *testing.T) {
	called := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	tokens := &fakeDeviceTokenRepository{tokens: map[string][]string{}}
	sender := newTestFCMPushSender(server.URL, tokens)

	if err := sender.SendPushNotification(context.Background(), "user-with-no-device", "Title", "Body", nil); err != nil {
		t.Fatalf("SendPushNotification() error: %v, want nil (no-op)", err)
	}
	if called {
		t.Errorf("FCM endpoint was called for a user with no registered device token")
	}
}

func TestFCMPushSender_SendsToEveryRegisteredDevice(t *testing.T) {
	var mu sync.Mutex
	var gotTokens []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body fcmMessage
		_ = json.NewDecoder(r.Body).Decode(&body)
		mu.Lock()
		gotTokens = append(gotTokens, body.Message.Token)
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	tokens := &fakeDeviceTokenRepository{tokens: map[string][]string{"user-1": {"device-a", "device-b"}}}
	sender := newTestFCMPushSender(server.URL, tokens)

	if err := sender.SendPushNotification(context.Background(), "user-1", "Title", "Body", nil); err != nil {
		t.Fatalf("SendPushNotification() error: %v", err)
	}

	if len(gotTokens) != 2 {
		t.Fatalf("FCM called %d times, want 2 (one per registered device)", len(gotTokens))
	}
}

// TestFCMPushSender_OneFailingTokenDoesNotBlockTheOthers confirms a bad
// token for one of a user's devices doesn't prevent their other devices
// from being notified.
func TestFCMPushSender_OneFailingTokenDoesNotBlockTheOthers(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body fcmMessage
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body.Message.Token == "bad-token" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	tokens := &fakeDeviceTokenRepository{tokens: map[string][]string{"user-1": {"bad-token", "good-token"}}}
	sender := newTestFCMPushSender(server.URL, tokens)

	if err := sender.SendPushNotification(context.Background(), "user-1", "Title", "Body", nil); err != nil {
		t.Fatalf("SendPushNotification() error: %v, want nil (one of two devices succeeded)", err)
	}
}

func TestFCMPushSender_AllTokensFailingReturnsError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	tokens := &fakeDeviceTokenRepository{tokens: map[string][]string{"user-1": {"token-a"}}}
	sender := newTestFCMPushSender(server.URL, tokens)

	if err := sender.SendPushNotification(context.Background(), "user-1", "Title", "Body", nil); err == nil {
		t.Fatalf("SendPushNotification() error = nil, want an error when every device fails")
	}
}

// TestFCMPushSender_NeverLogsRawToken guards the self-review checklist item
// directly — an error returned from a failed send must not embed the raw
// device token, since apperror.ToGRPCStatus logs codes.Internal errors
// verbatim server-side.
func TestFCMPushSender_NeverLogsRawToken(t *testing.T) {
	const secretToken = "super-secret-device-token-value"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	tokens := &fakeDeviceTokenRepository{tokens: map[string][]string{"user-1": {secretToken}}}
	sender := newTestFCMPushSender(server.URL, tokens)

	err := sender.SendPushNotification(context.Background(), "user-1", "Title", "Body", nil)
	if err == nil {
		t.Fatal("expected an error")
	}
	if got := fmt.Sprintf("%v", err); strings.Contains(got, secretToken) {
		t.Errorf("error message leaked the raw device token: %q", got)
	}
}
