package worker

import (
	"context"
	"log/slog"
	"sync"
	"time"
	"unsafe"

	"github.com/promptmeter/promptmeter/server/internal/domain"
	"github.com/promptmeter/promptmeter/server/internal/storage"
)

const (
	defaultBatchSize     = 10000
	defaultFlushInterval = 5 * time.Second
	defaultMaxMemoryMB   = 50
)

// BatchWriter accumulates events and flushes them to ClickHouse in batches.
// Flush triggers: batch size, time interval, or memory threshold.
type BatchWriter struct {
	writer        storage.EventWriter
	batchSize     int
	flushInterval time.Duration
	maxMemBytes   int64
	logger        *slog.Logger

	mu        sync.Mutex
	buffer    []domain.Event
	memBytes  int64
	firstAt   time.Time
	ackFuncs  []func() error // NATS ack functions for buffered events
}

// NewBatchWriter creates a new batch writer.
func NewBatchWriter(writer storage.EventWriter, batchSize int, flushInterval time.Duration, logger *slog.Logger) *BatchWriter {
	if batchSize <= 0 {
		batchSize = defaultBatchSize
	}
	if flushInterval <= 0 {
		flushInterval = defaultFlushInterval
	}

	return &BatchWriter{
		writer:        writer,
		batchSize:     batchSize,
		flushInterval: flushInterval,
		maxMemBytes:   defaultMaxMemoryMB * 1024 * 1024,
		logger:        logger,
		buffer:        make([]domain.Event, 0, batchSize),
		ackFuncs:      make([]func() error, 0, batchSize),
	}
}

// Add appends an event to the batch buffer and returns true if a flush was triggered.
func (bw *BatchWriter) Add(event domain.Event, ackFn func() error) bool {
	bw.mu.Lock()
	defer bw.mu.Unlock()

	if len(bw.buffer) == 0 {
		bw.firstAt = time.Now()
	}

	bw.buffer = append(bw.buffer, event)
	bw.ackFuncs = append(bw.ackFuncs, ackFn)
	bw.memBytes += estimateEventSize(&event)

	return bw.shouldFlush()
}

// Flush writes the current batch to ClickHouse and acks NATS messages.
func (bw *BatchWriter) Flush(ctx context.Context) error {
	bw.mu.Lock()
	if len(bw.buffer) == 0 {
		bw.mu.Unlock()
		return nil
	}

	events := bw.buffer
	ackFuncs := bw.ackFuncs
	bw.buffer = make([]domain.Event, 0, bw.batchSize)
	bw.ackFuncs = make([]func() error, 0, bw.batchSize)
	bw.memBytes = 0
	bw.mu.Unlock()

	bw.logger.Info("flushing batch", "events", len(events))

	if err := bw.writer.InsertEvents(ctx, events); err != nil {
		// Put events back in buffer for retry
		bw.mu.Lock()
		bw.buffer = append(events, bw.buffer...)
		bw.ackFuncs = append(ackFuncs, bw.ackFuncs...)
		for _, e := range events {
			bw.memBytes += estimateEventSize(&e)
		}
		bw.mu.Unlock()
		return err
	}

	// Ack all NATS messages after successful insert
	for _, ack := range ackFuncs {
		if ack != nil {
			if err := ack(); err != nil {
				bw.logger.Warn("failed to ack nats message", "error", err)
			}
		}
	}

	bw.logger.Info("batch flushed successfully", "events", len(events))
	return nil
}

// Start begins the periodic flush loop. It blocks until ctx is cancelled.
func (bw *BatchWriter) Start(ctx context.Context) {
	ticker := time.NewTicker(bw.flushInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			// Final flush on shutdown
			if err := bw.Flush(context.Background()); err != nil {
				bw.logger.Error("final flush failed", "error", err)
			}
			return
		case <-ticker.C:
			if err := bw.Flush(ctx); err != nil {
				bw.logger.Error("periodic flush failed", "error", err)
			}
		}
	}
}

// BufferLen returns the current number of events in the buffer.
func (bw *BatchWriter) BufferLen() int {
	bw.mu.Lock()
	defer bw.mu.Unlock()
	return len(bw.buffer)
}

func (bw *BatchWriter) shouldFlush() bool {
	if len(bw.buffer) >= bw.batchSize {
		return true
	}
	if bw.memBytes >= bw.maxMemBytes {
		return true
	}
	if !bw.firstAt.IsZero() && time.Since(bw.firstAt) >= bw.flushInterval {
		return true
	}
	return false
}

func estimateEventSize(e *domain.Event) int64 {
	base := int64(unsafe.Sizeof(*e))
	base += int64(len(e.EventID) + len(e.Model) + len(e.Provider) + len(e.Prompt) + len(e.Response) + len(e.S3Key) + len(e.PromptHash))
	for k, v := range e.Tags {
		base += int64(len(k) + len(v))
	}
	return base
}
