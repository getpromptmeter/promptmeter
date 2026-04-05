// Package pmqueue provides NATS JetStream publisher and consumer for the events stream.
package pmqueue

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

const (
	// StreamName is the NATS JetStream stream name for LLM events.
	StreamName = "EVENTS"
	// DLQStreamName is the dead-letter queue stream.
	DLQStreamName = "EVENTS_DLQ"
	// ConsumerName is the durable consumer name for workers.
	ConsumerName = "event-workers"
	// SubjectPrefix is the subject prefix for event messages.
	SubjectPrefix = "events."
)

// Connect establishes a NATS connection.
func Connect(url string) (*nats.Conn, error) {
	nc, err := nats.Connect(url,
		nats.MaxReconnects(-1),
		nats.ReconnectWait(time.Second),
		nats.DisconnectErrHandler(func(_ *nats.Conn, err error) {
			if err != nil {
				slog.Warn("nats: disconnected", "error", err)
			}
		}),
		nats.ReconnectHandler(func(_ *nats.Conn) {
			slog.Info("nats: reconnected")
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("nats: connect: %w", err)
	}
	return nc, nil
}

// EnsureStream creates or updates the EVENTS stream.
func EnsureStream(js jetstream.JetStream) (jetstream.Stream, error) {
	cfg := jetstream.StreamConfig{
		Name:        StreamName,
		Subjects:    []string{SubjectPrefix + ">"},
		Storage:     jetstream.FileStorage,
		Retention:   jetstream.WorkQueuePolicy,
		MaxAge:      72 * time.Hour,
		MaxBytes:    1 << 30, // 1 GB
		Replicas:    1,
		Discard:     jetstream.DiscardOld,
		Duplicates:  10 * time.Minute,
		Description: "LLM event ingestion stream",
	}

	stream, err := js.CreateOrUpdateStream(context.Background(), cfg)
	if err != nil {
		return nil, fmt.Errorf("nats: ensure stream %s: %w", StreamName, err)
	}
	return stream, nil
}

// EnsureDLQStream creates or updates the dead-letter queue stream.
func EnsureDLQStream(js jetstream.JetStream) (jetstream.Stream, error) {
	cfg := jetstream.StreamConfig{
		Name:        DLQStreamName,
		Subjects:    []string{"events_dlq.>"},
		Storage:     jetstream.FileStorage,
		Retention:   jetstream.LimitsPolicy,
		MaxAge:      7 * 24 * time.Hour,
		MaxBytes:    100 << 20, // 100 MB
		Replicas:    1,
		Description: "Dead letter queue for failed event processing",
	}

	stream, err := js.CreateOrUpdateStream(context.Background(), cfg)
	if err != nil {
		return nil, fmt.Errorf("nats: ensure dlq stream: %w", err)
	}
	return stream, nil
}

// EnsureConsumer creates or updates the event-workers consumer.
func EnsureConsumer(js jetstream.JetStream) (jetstream.Consumer, error) {
	cfg := jetstream.ConsumerConfig{
		Name:          ConsumerName,
		Durable:       ConsumerName,
		AckPolicy:     jetstream.AckExplicitPolicy,
		AckWait:       30 * time.Second,
		MaxDeliver:    5,
		MaxAckPending: 1000,
		FilterSubject: SubjectPrefix + ">",
	}

	consumer, err := js.CreateOrUpdateConsumer(context.Background(), StreamName, cfg)
	if err != nil {
		return nil, fmt.Errorf("nats: ensure consumer %s: %w", ConsumerName, err)
	}
	return consumer, nil
}
