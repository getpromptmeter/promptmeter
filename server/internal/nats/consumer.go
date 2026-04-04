package pmqueue

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/nats-io/nats.go/jetstream"

	eventsv1 "github.com/promptmeter/promptmeter/server/internal/proto/eventsv1"
)

// MessageHandler processes a single LLM event message.
// The ack function should be called after successful processing.
type MessageHandler func(event *eventsv1.LLMEvent, ack func() error) error

// EventConsumer consumes events from NATS JetStream.
type EventConsumer struct {
	consumer jetstream.Consumer
	js       jetstream.JetStream
	logger   *slog.Logger
}

// NewEventConsumer creates a new NATS event consumer.
func NewEventConsumer(js jetstream.JetStream, consumer jetstream.Consumer, logger *slog.Logger) *EventConsumer {
	return &EventConsumer{
		consumer: consumer,
		js:       js,
		logger:   logger,
	}
}

// Start begins consuming messages and passes them to the handler.
// It blocks until the context is cancelled.
func (c *EventConsumer) Start(ctx context.Context, handler MessageHandler) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		msgs, err := c.consumer.Fetch(10, jetstream.FetchMaxWait(5*time.Second))
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			c.logger.Warn("nats consumer: fetch error", "error", err)
			continue
		}

		for msg := range msgs.Messages() {
			var event eventsv1.LLMEvent
			if err := event.Unmarshal(msg.Data()); err != nil {
				c.logger.Error("nats consumer: unmarshal error, sending NAK",
					"error", err,
					"subject", msg.Subject(),
				)
				_ = msg.Nak()
				continue
			}

			ack := func() error {
				return msg.Ack()
			}

			if err := handler(&event, ack); err != nil {
				c.logger.Error("nats consumer: handler error",
					"error", err,
					"event_id", event.EventId,
				)
				_ = msg.NakWithDelay(5 * time.Second)
				continue
			}
		}

		if err := msgs.Error(); err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			c.logger.Warn("nats consumer: messages error", "error", err)
		}
	}
}

// PublishToDLQ publishes a failed message to the dead-letter queue.
func (c *EventConsumer) PublishToDLQ(ctx context.Context, data []byte, reason string) error {
	subject := "events_dlq.failed"
	_, err := c.js.Publish(ctx, subject, data)
	if err != nil {
		return fmt.Errorf("nats: publish to dlq: %w", err)
	}
	return nil
}
