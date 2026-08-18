// Package events publishes domain events to Pub/Sub (ADR-008).
package events

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"cloud.google.com/go/pubsub/v2"
)

// userOnboardedTopic uses kebab-case, matching this project's Pub/Sub
// topic-naming convention.
const userOnboardedTopic = "user-onboarded"

// userOnboardedPayload is deliberately minimal — user_id, trust_level, and
// occurred_at only. No name, photo, or other PII, consistent with the
// data-minimization principle applied throughout this project (ADR-003,
// ADR-011).
type userOnboardedPayload struct {
	UserID     string    `json:"user_id"`
	TrustLevel int       `json:"trust_level"`
	OccurredAt time.Time `json:"occurred_at"`
}

// Publisher publishes domain events. internal/service depends on this
// interface, not the concrete Pub/Sub client — mirrors internal/repository's
// interfaces-first pattern, and lets tests substitute a fake without a
// running Pub/Sub emulator.
type Publisher interface {
	// PublishUserOnboarded publishes a user-onboarded event for a
	// newly-onboarded user.
	PublishUserOnboarded(ctx context.Context, userID string, trustLevel int) error
	// Close releases the publisher's resources. Call once at process
	// shutdown.
	Close() error
}

type pubsubPublisher struct {
	client    *pubsub.Client
	publisher *pubsub.Publisher
}

// NewPublisher constructs a Pub/Sub-backed Publisher for the user-onboarded
// topic. In local dev, PUBSUB_EMULATOR_HOST (set by docker-compose.yml)
// makes the underlying client library transparently talk to the emulator
// instead of real GCP Pub/Sub — no code branch needed here.
func NewPublisher(ctx context.Context, gcpProjectID string) (Publisher, error) {
	client, err := pubsub.NewClient(ctx, gcpProjectID)
	if err != nil {
		return nil, fmt.Errorf("events: create pubsub client: %w", err)
	}

	return &pubsubPublisher{client: client, publisher: client.Publisher(userOnboardedTopic)}, nil
}

func (p *pubsubPublisher) Close() error {
	p.publisher.Stop()
	return p.client.Close()
}

func (p *pubsubPublisher) PublishUserOnboarded(ctx context.Context, userID string, trustLevel int) error {
	payload := userOnboardedPayload{
		UserID:     userID,
		TrustLevel: trustLevel,
		OccurredAt: time.Now().UTC(),
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("events: marshal user-onboarded payload: %w", err)
	}

	result := p.publisher.Publish(ctx, &pubsub.Message{Data: data})
	if _, err := result.Get(ctx); err != nil {
		return fmt.Errorf("events: publish user-onboarded: %w", err)
	}

	return nil
}
