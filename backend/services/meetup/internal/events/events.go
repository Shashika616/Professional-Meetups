// Package events publishes domain events to Pub/Sub (ADR-008), mirroring
// services/auth/internal/events' Publisher interface shape.
package events

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"cloud.google.com/go/pubsub/v2"
)

// Topic names use kebab-case, matching this project's Pub/Sub
// topic-naming convention (services/auth's user-onboarded).
const (
	requestCreatedTopic  = "meetup-request-created"
	requestAcceptedTopic = "meetup-request-accepted"
	requestRejectedTopic = "meetup-request-rejected"
)

// requestEventPayload is deliberately minimal — IDs and the fact of the
// transition only, no meetup location/timing or requester name, consistent
// with the data-minimization principle applied throughout this project
// (ADR-003, ADR-011). The notification-dispatch consumer looks up whatever
// display text it needs itself rather than the event payload carrying it.
type requestEventPayload struct {
	RequestID    string    `json:"request_id"`
	MeetupID     string    `json:"meetup_id"`
	RequesterID  string    `json:"requester_id"`
	HostUserID   string    `json:"host_user_id"`
	AutoRejected bool      `json:"auto_rejected,omitempty"`
	OccurredAt   time.Time `json:"occurred_at"`
}

// Publisher publishes domain events. internal/service depends on this
// interface, not the concrete Pub/Sub client — mirrors
// internal/repository's interfaces-first pattern, and lets tests substitute
// a fake without a running Pub/Sub emulator.
type Publisher interface {
	PublishRequestCreated(ctx context.Context, requestID, meetupID, requesterID, hostUserID string) error
	PublishRequestAccepted(ctx context.Context, requestID, meetupID, requesterID, hostUserID string) error
	PublishRequestRejected(ctx context.Context, requestID, meetupID, requesterID, hostUserID string, autoRejected bool) error
	// Close releases the publisher's resources. Call once at process
	// shutdown.
	Close() error
}

type pubsubPublisher struct {
	client            *pubsub.Client
	createdPublisher  *pubsub.Publisher
	acceptedPublisher *pubsub.Publisher
	rejectedPublisher *pubsub.Publisher
}

// NewPublisher constructs a Pub/Sub-backed Publisher for this service's
// three topics. In local dev, PUBSUB_EMULATOR_HOST (set by
// docker-compose.yml) makes the underlying client library transparently
// talk to the emulator instead of real GCP Pub/Sub — no code branch needed
// here.
func NewPublisher(ctx context.Context, gcpProjectID string) (Publisher, error) {
	client, err := pubsub.NewClient(ctx, gcpProjectID)
	if err != nil {
		return nil, fmt.Errorf("events: create pubsub client: %w", err)
	}

	return &pubsubPublisher{
		client:            client,
		createdPublisher:  client.Publisher(requestCreatedTopic),
		acceptedPublisher: client.Publisher(requestAcceptedTopic),
		rejectedPublisher: client.Publisher(requestRejectedTopic),
	}, nil
}

func (p *pubsubPublisher) Close() error {
	p.createdPublisher.Stop()
	p.acceptedPublisher.Stop()
	p.rejectedPublisher.Stop()
	return p.client.Close()
}

func (p *pubsubPublisher) PublishRequestCreated(ctx context.Context, requestID, meetupID, requesterID, hostUserID string) error {
	return publish(ctx, p.createdPublisher, requestEventPayload{
		RequestID:   requestID,
		MeetupID:    meetupID,
		RequesterID: requesterID,
		HostUserID:  hostUserID,
		OccurredAt:  time.Now().UTC(),
	})
}

func (p *pubsubPublisher) PublishRequestAccepted(ctx context.Context, requestID, meetupID, requesterID, hostUserID string) error {
	return publish(ctx, p.acceptedPublisher, requestEventPayload{
		RequestID:   requestID,
		MeetupID:    meetupID,
		RequesterID: requesterID,
		HostUserID:  hostUserID,
		OccurredAt:  time.Now().UTC(),
	})
}

func (p *pubsubPublisher) PublishRequestRejected(
	ctx context.Context, requestID, meetupID, requesterID, hostUserID string, autoRejected bool,
) error {
	return publish(ctx, p.rejectedPublisher, requestEventPayload{
		RequestID:    requestID,
		MeetupID:     meetupID,
		RequesterID:  requesterID,
		HostUserID:   hostUserID,
		AutoRejected: autoRejected,
		OccurredAt:   time.Now().UTC(),
	})
}

func publish(ctx context.Context, publisher *pubsub.Publisher, payload requestEventPayload) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("events: marshal payload: %w", err)
	}

	result := publisher.Publish(ctx, &pubsub.Message{Data: data})
	if _, err := result.Get(ctx); err != nil {
		return fmt.Errorf("events: publish: %w", err)
	}
	return nil
}
