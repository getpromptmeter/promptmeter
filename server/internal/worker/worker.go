package worker

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/promptmeter/promptmeter/server/internal/domain"
	pmqueue "github.com/promptmeter/promptmeter/server/internal/nats"
	eventsv1 "github.com/promptmeter/promptmeter/server/internal/proto/eventsv1"
)

const shutdownTimeout = 30 * time.Second

// Worker orchestrates NATS consumers, cost calculation, batch writing, and S3 uploads.
type Worker struct {
	consumer    *pmqueue.EventConsumer
	batchWriter *BatchWriter
	priceCache  *PriceCache
	s3Uploader  *S3Uploader
	workerCount int
	logger      *slog.Logger
}

// NewWorker creates a new Worker instance.
func NewWorker(
	consumer *pmqueue.EventConsumer,
	batchWriter *BatchWriter,
	priceCache *PriceCache,
	s3Uploader *S3Uploader,
	workerCount int,
	logger *slog.Logger,
) *Worker {
	if workerCount <= 0 {
		workerCount = 3
	}
	return &Worker{
		consumer:    consumer,
		batchWriter: batchWriter,
		priceCache:  priceCache,
		s3Uploader:  s3Uploader,
		workerCount: workerCount,
		logger:      logger,
	}
}

// Start launches all worker goroutines. It blocks until ctx is cancelled,
// then performs a graceful shutdown: flushes the batch writer and drains
// in-flight S3 uploads before returning.
func (w *Worker) Start(ctx context.Context) error {
	// Start price cache refresh
	go func() {
		if err := w.priceCache.Start(ctx); err != nil && ctx.Err() == nil {
			w.logger.Error("price cache stopped", "error", err)
		}
	}()

	// Start S3 uploader
	w.s3Uploader.Start(ctx)

	// Start batch writer flush loop
	var bwDone sync.WaitGroup
	bwDone.Add(1)
	go func() {
		defer bwDone.Done()
		w.batchWriter.Start(ctx)
	}()

	// Start NATS consumers
	errCh := make(chan error, w.workerCount)
	for i := 0; i < w.workerCount; i++ {
		workerID := i
		go func() {
			w.logger.Info("starting worker", "worker_id", workerID)
			err := w.consumer.Start(ctx, w.handleMessage)
			errCh <- err
		}()
	}

	// Wait for context cancellation or fatal consumer error
	var startErr error
	select {
	case <-ctx.Done():
		w.logger.Info("worker received shutdown signal")
	case err := <-errCh:
		if err != nil && ctx.Err() == nil {
			startErr = err
		}
	}

	// Graceful shutdown: wait for the batch writer flush loop to finish
	// (it performs a final flush on context cancellation).
	w.logger.Info("waiting for batch writer to drain")
	bwDone.Wait()

	// Flush any remaining events that arrived after the flush loop exited.
	flushCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	if err := w.batchWriter.Flush(flushCtx); err != nil {
		w.logger.Error("final batch flush failed", "error", err)
	}

	// Drain in-flight S3 uploads.
	w.logger.Info("waiting for in-flight s3 uploads to complete")
	w.s3Uploader.Drain()

	w.logger.Info("worker shutdown complete")
	return startErr
}

func (w *Worker) handleMessage(pbEvent *eventsv1.LLMEvent, ack func() error) error {
	event := protoToEvent(pbEvent)

	// Calculate cost
	event.CostUSD = w.priceCache.CalculateCost(&event)
	event.TotalTokens = event.PromptTokens + event.CompletionTokens

	// Determine S3 status
	if event.HasText() {
		event.S3Status = domain.S3StatusPending
		event.S3Key = event.S3ObjectKey()
	} else {
		event.S3Status = domain.S3StatusNone
	}

	// Add to batch writer
	shouldFlush := w.batchWriter.Add(event, ack)
	if shouldFlush {
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			if err := w.batchWriter.Flush(ctx); err != nil {
				w.logger.Error("flush after size trigger failed", "error", err)
			}
		}()
	}

	// Enqueue S3 upload asynchronously
	if event.HasText() {
		w.s3Uploader.Enqueue(event)
	}

	return nil
}

func protoToEvent(pb *eventsv1.LLMEvent) domain.Event {
	event := domain.Event{
		OrgID:            pb.OrgId,
		EventID:          pb.EventId,
		ProjectID:        pb.ProjectId,
		Model:            pb.Model,
		Provider:         pb.Provider,
		PromptTokens:     pb.PromptTokens,
		CompletionTokens: pb.CompletionTokens,
		TotalTokens:      pb.TotalTokens,
		CostUSD:          pb.CostUsd,
		LatencyMs:        pb.LatencyMs,
		StatusCode:       pb.StatusCode,
		Tags:             pb.Tags,
		Prompt:           pb.Prompt,
		Response:         pb.Response,
		PromptHash:       pb.PromptHash,
		S3Key:            pb.S3Key,
		SchemaVersion:    pb.SchemaVersion,
	}

	if !pb.Timestamp.IsZero() {
		event.Timestamp = pb.Timestamp
	}
	if !pb.InsertedAt.IsZero() {
		event.InsertedAt = pb.InsertedAt
	}

	switch pb.S3Status {
	case eventsv1.S3Status_S3_STATUS_PENDING:
		event.S3Status = domain.S3StatusPending
	case eventsv1.S3Status_S3_STATUS_UPLOADED:
		event.S3Status = domain.S3StatusUploaded
	default:
		event.S3Status = domain.S3StatusNone
	}

	if event.Tags == nil {
		event.Tags = make(map[string]string)
	}

	return event
}
