package worker

import (
	"context"
	"log/slog"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/promptmeter/promptmeter/server/internal/domain"
)

type mockEventWriter struct {
	mu     sync.Mutex
	events []domain.Event
	err    error
}

func (m *mockEventWriter) InsertEvents(_ context.Context, events []domain.Event) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.err != nil {
		return m.err
	}
	m.events = append(m.events, events...)
	return nil
}

func (m *mockEventWriter) getEvents() []domain.Event {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]domain.Event{}, m.events...)
}

func testEvent(id string) domain.Event {
	return domain.Event{
		OrgID:            1,
		EventID:          id,
		Model:            "gpt-4o",
		Provider:         "openai",
		PromptTokens:     100,
		CompletionTokens: 50,
		TotalTokens:      150,
		StatusCode:       200,
		Timestamp:        time.Now(),
		SchemaVersion:    1,
		S3Status:         domain.S3StatusNone,
	}
}

func TestBatchWriter_FlushBySize(t *testing.T) {
	writer := &mockEventWriter{}
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	bw := NewBatchWriter(writer, 5, 1*time.Hour, logger)

	for i := 0; i < 5; i++ {
		shouldFlush := bw.Add(testEvent("event-"+string(rune('a'+i))), nil)
		if i < 4 && shouldFlush {
			t.Errorf("should not trigger flush at %d events", i+1)
		}
		if i == 4 && !shouldFlush {
			t.Error("should trigger flush at 5 events")
		}
	}

	if err := bw.Flush(context.Background()); err != nil {
		t.Fatalf("flush error: %v", err)
	}

	events := writer.getEvents()
	if len(events) != 5 {
		t.Errorf("expected 5 events, got %d", len(events))
	}
}

func TestBatchWriter_FlushByTime(t *testing.T) {
	writer := &mockEventWriter{}
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	bw := NewBatchWriter(writer, 1000, 100*time.Millisecond, logger)

	bw.Add(testEvent("event-a"), nil)
	bw.Add(testEvent("event-b"), nil)

	ctx, cancel := context.WithCancel(context.Background())
	go bw.Start(ctx)

	time.Sleep(300 * time.Millisecond)
	cancel()
	time.Sleep(50 * time.Millisecond)

	events := writer.getEvents()
	if len(events) != 2 {
		t.Errorf("expected 2 events after time-based flush, got %d", len(events))
	}
}

func TestBatchWriter_AckOnFlush(t *testing.T) {
	writer := &mockEventWriter{}
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	bw := NewBatchWriter(writer, 100, 1*time.Hour, logger)

	var acked atomic.Int32
	ackFn := func() error {
		acked.Add(1)
		return nil
	}

	bw.Add(testEvent("event-a"), ackFn)
	bw.Add(testEvent("event-b"), ackFn)

	if err := bw.Flush(context.Background()); err != nil {
		t.Fatalf("flush error: %v", err)
	}

	if acked.Load() != 2 {
		t.Errorf("expected 2 acks, got %d", acked.Load())
	}
}

func TestBatchWriter_ConcurrentAdd(t *testing.T) {
	writer := &mockEventWriter{}
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	bw := NewBatchWriter(writer, 10000, 1*time.Hour, logger)

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			bw.Add(testEvent("event-"+string(rune(i))), nil)
		}(i)
	}
	wg.Wait()

	if bw.BufferLen() != 100 {
		t.Errorf("expected 100 events in buffer, got %d", bw.BufferLen())
	}
}
