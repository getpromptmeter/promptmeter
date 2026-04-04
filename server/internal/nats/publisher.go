package pmqueue

import (
	"context"
	"fmt"

	"github.com/nats-io/nats.go/jetstream"

	eventsv1 "github.com/promptmeter/promptmeter/server/internal/proto/eventsv1"
)

// EventPublisher publishes LLM events to NATS JetStream.
type EventPublisher interface {
	Publish(ctx context.Context, orgID uint64, event *eventsv1.LLMEvent) error
	Close() error
}

// Publisher implements EventPublisher using NATS JetStream.
type Publisher struct {
	js jetstream.JetStream
}

// NewPublisher creates a new NATS event publisher.
func NewPublisher(js jetstream.JetStream) *Publisher {
	return &Publisher{js: js}
}

// Publish serializes the event and publishes it to the EVENTS stream.
// The idempotency_key (event_id) is used as the NATS msg-id for deduplication.
func (p *Publisher) Publish(ctx context.Context, orgID uint64, event *eventsv1.LLMEvent) error {
	data, err := event.Marshal()
	if err != nil {
		return fmt.Errorf("nats publisher: marshal event: %w", err)
	}

	subject := fmt.Sprintf("%s%d", SubjectPrefix, orgID)
	_, err = p.js.Publish(ctx, subject, data, jetstream.WithMsgID(event.EventId))
	if err != nil {
		return fmt.Errorf("nats publisher: publish to %s: %w", subject, err)
	}
	return nil
}

// Close is a no-op; connection lifecycle is managed externally.
func (p *Publisher) Close() error {
	return nil
}
