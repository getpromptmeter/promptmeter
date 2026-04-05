package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/promptmeter/promptmeter/server/internal/domain"
	"github.com/promptmeter/promptmeter/server/internal/storage"
)

// S3Uploader handles asynchronous upload of prompt/response text to S3
// and reconciliation of pending uploads.
type S3Uploader struct {
	objectStore  storage.ObjectStore
	pendingStore storage.PendingEventsStore
	logger       *slog.Logger
	uploadCh     chan uploadRequest
	inflightWg   sync.WaitGroup
}

type uploadRequest struct {
	event domain.Event
}

// s3Payload is the JSON structure stored in S3.
type s3Payload struct {
	Version          int    `json:"v"`
	EventID          string `json:"event_id"`
	Prompt           string `json:"prompt,omitempty"`
	Response         string `json:"response,omitempty"`
	PromptTokens     uint32 `json:"prompt_tokens"`
	CompletionTokens uint32 `json:"completion_tokens"`
}

// NewS3Uploader creates a new async S3 uploader.
func NewS3Uploader(objectStore storage.ObjectStore, pendingStore storage.PendingEventsStore, logger *slog.Logger) *S3Uploader {
	return &S3Uploader{
		objectStore:  objectStore,
		pendingStore: pendingStore,
		logger:       logger,
		uploadCh:     make(chan uploadRequest, 1000),
	}
}

// Enqueue adds an event to the async upload queue.
func (u *S3Uploader) Enqueue(event domain.Event) {
	select {
	case u.uploadCh <- uploadRequest{event: event}:
	default:
		u.logger.Warn("s3 upload queue full, event will be reconciled later",
			"event_id", event.EventID,
		)
	}
}

// Start begins processing upload requests and runs the reconciler.
// It blocks until ctx is cancelled.
func (u *S3Uploader) Start(ctx context.Context) {
	go u.processUploads(ctx)
	go u.reconciler(ctx)
}

// Drain waits for all in-flight S3 uploads to complete.
// Call this during graceful shutdown after the context has been cancelled.
func (u *S3Uploader) Drain() {
	u.inflightWg.Wait()
}

func (u *S3Uploader) processUploads(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			// Drain remaining items in the channel before returning.
			for {
				select {
				case req := <-u.uploadCh:
					u.doUpload(context.Background(), req)
				default:
					return
				}
			}
		case req := <-u.uploadCh:
			u.doUpload(ctx, req)
		}
	}
}

func (u *S3Uploader) doUpload(ctx context.Context, req uploadRequest) {
	u.inflightWg.Add(1)
	defer u.inflightWg.Done()

	if err := u.uploadEvent(ctx, req.event); err != nil {
		u.logger.Warn("s3 upload failed, will be reconciled",
			"event_id", req.event.EventID,
			"error", err,
		)
	}
}

func (u *S3Uploader) uploadEvent(ctx context.Context, event domain.Event) error {
	payload := s3Payload{
		Version:          1,
		EventID:          event.EventID,
		Prompt:           event.Prompt,
		Response:         event.Response,
		PromptTokens:     event.PromptTokens,
		CompletionTokens: event.CompletionTokens,
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("s3 uploader: marshal payload: %w", err)
	}

	s3Key := event.S3ObjectKey()
	if err := u.objectStore.Upload(ctx, s3Key, data); err != nil {
		return fmt.Errorf("s3 uploader: upload: %w", err)
	}

	// Update status in ClickHouse
	if u.pendingStore != nil {
		if err := u.pendingStore.UpdateS3Status(ctx, event.EventID, domain.S3StatusUploaded, s3Key); err != nil {
			u.logger.Warn("s3 uploader: failed to update status",
				"event_id", event.EventID,
				"error", err,
			)
		}
	}

	return nil
}

func (u *S3Uploader) reconciler(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			u.reconcile(ctx)
		}
	}
}

func (u *S3Uploader) reconcile(ctx context.Context) {
	if u.pendingStore == nil {
		return
	}

	events, err := u.pendingStore.GetPendingS3Events(ctx, 100)
	if err != nil {
		u.logger.Warn("reconciler: failed to get pending events", "error", err)
		return
	}

	if len(events) == 0 {
		return
	}

	u.logger.Info("reconciler: processing pending s3 uploads", "count", len(events))

	for _, event := range events {
		if err := u.uploadEvent(ctx, event); err != nil {
			u.logger.Warn("reconciler: upload failed",
				"event_id", event.EventID,
				"error", err,
			)
		}
	}
}
