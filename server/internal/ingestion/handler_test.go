package ingestion

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	eventsv1 "github.com/promptmeter/promptmeter/server/internal/proto/eventsv1"
)

// mockPublisher is a test double for the NATS publisher.
type mockPublisher struct {
	events []*eventsv1.LLMEvent
	err    error
}

func (m *mockPublisher) Publish(_ context.Context, _ uint64, event *eventsv1.LLMEvent) error {
	if m.err != nil {
		return m.err
	}
	m.events = append(m.events, event)
	return nil
}

func (m *mockPublisher) Close() error { return nil }

func TestHandleEvent_HappyPath(t *testing.T) {
	pub := &mockPublisher{}
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	h := NewHandler(pub, logger)

	body := `{
		"idempotency_key": "01965a3c-8b2f-7d4e-9f1a-2c3d4e5f6a7b",
		"timestamp": "2026-03-26T14:22:01.345Z",
		"model": "gpt-4o",
		"provider": "openai",
		"prompt_tokens": 1250,
		"completion_tokens": 380,
		"latency_ms": 2340,
		"status_code": 200,
		"tags": {"feature": "chat"}
	}`

	req := httptest.NewRequest(http.MethodPost, "/v1/events", bytes.NewBufferString(body))
	req = req.WithContext(context.WithValue(req.Context(), ctxOrgID, "550e8400-e29b-41d4-a716-446655440000"))
	req = req.WithContext(context.WithValue(req.Context(), ctxReqID, "req_test1"))

	w := httptest.NewRecorder()
	h.HandleEvent(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusAccepted {
		bodyBytes, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 202, got %d: %s", resp.StatusCode, string(bodyBytes))
	}

	if len(pub.events) != 1 {
		t.Fatalf("expected 1 published event, got %d", len(pub.events))
	}

	event := pub.events[0]
	if event.Model != "gpt-4o" {
		t.Errorf("expected model gpt-4o, got %s", event.Model)
	}
	if event.PromptTokens != 1250 {
		t.Errorf("expected prompt_tokens 1250, got %d", event.PromptTokens)
	}
	if event.TotalTokens != 1630 {
		t.Errorf("expected total_tokens 1630, got %d", event.TotalTokens)
	}
}

func TestHandleEvent_ValidationError(t *testing.T) {
	pub := &mockPublisher{}
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	h := NewHandler(pub, logger)

	body := `{"model": ""}`
	req := httptest.NewRequest(http.MethodPost, "/v1/events", bytes.NewBufferString(body))
	req = req.WithContext(context.WithValue(req.Context(), ctxOrgID, "550e8400-e29b-41d4-a716-446655440000"))

	w := httptest.NewRecorder()
	h.HandleEvent(w, req)

	if w.Result().StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Result().StatusCode)
	}
}

func TestHandleEvent_PublishError(t *testing.T) {
	pub := &mockPublisher{err: fmt.Errorf("nats down")}
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	h := NewHandler(pub, logger)

	body := `{
		"idempotency_key": "01965a3c-8b2f-7d4e-9f1a-2c3d4e5f6a7b",
		"timestamp": "2026-03-26T14:22:01.345Z",
		"model": "gpt-4o",
		"provider": "openai",
		"prompt_tokens": 100,
		"completion_tokens": 50,
		"latency_ms": 100,
		"status_code": 200
	}`

	req := httptest.NewRequest(http.MethodPost, "/v1/events", bytes.NewBufferString(body))
	req = req.WithContext(context.WithValue(req.Context(), ctxOrgID, "550e8400-e29b-41d4-a716-446655440000"))

	w := httptest.NewRecorder()
	h.HandleEvent(w, req)

	if w.Result().StatusCode != http.StatusServiceUnavailable {
		t.Errorf("expected 503, got %d", w.Result().StatusCode)
	}
}

func TestHandleBatch_PartialAccept(t *testing.T) {
	pub := &mockPublisher{}
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	h := NewHandler(pub, logger)

	body := `{
		"events": [
			{
				"idempotency_key": "01965a3c-8b2f-7d4e-9f1a-2c3d4e5f6a7b",
				"timestamp": "2026-03-26T14:22:01.345Z",
				"model": "gpt-4o",
				"provider": "openai",
				"prompt_tokens": 100,
				"completion_tokens": 50,
				"latency_ms": 100,
				"status_code": 200
			},
			{
				"idempotency_key": "",
				"timestamp": "",
				"model": "",
				"provider": ""
			}
		]
	}`

	req := httptest.NewRequest(http.MethodPost, "/v1/events/batch", bytes.NewBufferString(body))
	req = req.WithContext(context.WithValue(req.Context(), ctxOrgID, "550e8400-e29b-41d4-a716-446655440000"))
	req = req.WithContext(context.WithValue(req.Context(), ctxReqID, "req_test2"))

	w := httptest.NewRecorder()
	h.HandleBatch(w, req)

	if w.Result().StatusCode != http.StatusAccepted {
		bodyBytes, _ := io.ReadAll(w.Result().Body)
		t.Fatalf("expected 202, got %d: %s", w.Result().StatusCode, string(bodyBytes))
	}

	var resp map[string]any
	json.NewDecoder(w.Result().Body).Decode(&resp)
	data := resp["data"].(map[string]any)
	if data["accepted"].(float64) != 1 {
		t.Errorf("expected 1 accepted, got %v", data["accepted"])
	}
	if data["rejected"].(float64) != 1 {
		t.Errorf("expected 1 rejected, got %v", data["rejected"])
	}
}

func TestHandleHealth(t *testing.T) {
	pub := &mockPublisher{}
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	h := NewHandler(pub, logger)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()
	h.HandleHealth(w, req)

	if w.Result().StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Result().StatusCode)
	}
}
