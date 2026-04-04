package worker

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/promptmeter/promptmeter/server/internal/domain"
)

// mockObjectStore records uploaded objects for verification.
type mockObjectStore struct {
	mu      sync.Mutex
	uploads map[string][]byte
	err     error
}

func newMockObjectStore() *mockObjectStore {
	return &mockObjectStore{uploads: make(map[string][]byte)}
}

func (m *mockObjectStore) Upload(_ context.Context, key string, data []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.err != nil {
		return m.err
	}
	m.uploads[key] = data
	return nil
}

func (m *mockObjectStore) getUploads() map[string][]byte {
	m.mu.Lock()
	defer m.mu.Unlock()
	cp := make(map[string][]byte, len(m.uploads))
	for k, v := range m.uploads {
		cp[k] = v
	}
	return cp
}

// mockPendingEventsStore tracks S3 status updates.
type mockPendingEventsStore struct {
	mu       sync.Mutex
	pending  []domain.Event
	statuses map[string]string
}

func newMockPendingStore() *mockPendingEventsStore {
	return &mockPendingEventsStore{statuses: make(map[string]string)}
}

func (m *mockPendingEventsStore) GetPendingS3Events(_ context.Context, limit int) ([]domain.Event, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.pending) == 0 {
		return nil, nil
	}
	n := len(m.pending)
	if n > limit {
		n = limit
	}
	result := m.pending[:n]
	m.pending = m.pending[n:]
	return result, nil
}

func (m *mockPendingEventsStore) UpdateS3Status(_ context.Context, eventID string, status string, _ string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.statuses[eventID] = status
	return nil
}

func (m *mockPendingEventsStore) getStatus(eventID string) string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.statuses[eventID]
}

func TestS3Uploader_EnqueueAndUpload(t *testing.T) {
	objStore := newMockObjectStore()
	pendStore := newMockPendingStore()
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	uploader := NewS3Uploader(objStore, pendStore, logger)

	ctx, cancel := context.WithCancel(context.Background())
	uploader.Start(ctx)

	event := domain.Event{
		OrgID:            1,
		EventID:          "01965a3c-8b2f-7d4e-9f1a-2c3d4e5f6a7b",
		Model:            "gpt-4o",
		Provider:         "openai",
		PromptTokens:     100,
		CompletionTokens: 50,
		Prompt:           "Hello world",
		Response:         "Hi there",
		Timestamp:        time.Date(2026, 4, 1, 14, 0, 0, 0, time.UTC),
	}

	uploader.Enqueue(event)
	time.Sleep(200 * time.Millisecond)
	cancel()

	uploads := objStore.getUploads()
	if len(uploads) != 1 {
		t.Fatalf("expected 1 upload, got %d", len(uploads))
	}

	expectedKey := event.S3ObjectKey()
	if _, ok := uploads[expectedKey]; !ok {
		t.Errorf("expected upload with key %s", expectedKey)
	}

	status := pendStore.getStatus(event.EventID)
	if status != domain.S3StatusUploaded {
		t.Errorf("expected status %s, got %s", domain.S3StatusUploaded, status)
	}
}

func TestS3Uploader_QueueFull(t *testing.T) {
	objStore := newMockObjectStore()
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	uploader := NewS3Uploader(objStore, nil, logger)

	// Fill the upload channel (capacity 1000)
	for i := 0; i < 1000; i++ {
		uploader.Enqueue(domain.Event{
			EventID:   fmt.Sprintf("event-%d", i),
			Timestamp: time.Now(),
		})
	}

	// This should not block; the event is just dropped
	uploader.Enqueue(domain.Event{
		EventID:   "overflow-event",
		Timestamp: time.Now(),
	})
}

func TestS3Uploader_UploadFailure(t *testing.T) {
	objStore := newMockObjectStore()
	objStore.err = fmt.Errorf("s3 unavailable")
	pendStore := newMockPendingStore()
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	uploader := NewS3Uploader(objStore, pendStore, logger)

	ctx, cancel := context.WithCancel(context.Background())
	uploader.Start(ctx)

	event := domain.Event{
		OrgID:   1,
		EventID: "01965a3c-8b2f-7d4e-9f1a-2c3d4e5f6a7b",
		Prompt:  "test",
		Timestamp: time.Now(),
	}

	uploader.Enqueue(event)
	time.Sleep(200 * time.Millisecond)
	cancel()

	// Status should NOT be updated since upload failed
	status := pendStore.getStatus(event.EventID)
	if status == domain.S3StatusUploaded {
		t.Error("status should not be 'uploaded' after failed upload")
	}
}
