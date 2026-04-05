package ingestion

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/promptmeter/promptmeter/server/internal/domain"
	pmqueue "github.com/promptmeter/promptmeter/server/internal/nats"
	eventsv1 "github.com/promptmeter/promptmeter/server/internal/proto/eventsv1"
)

// Handler implements the Ingestion API HTTP endpoints.
type Handler struct {
	publisher pmqueue.EventPublisher
	logger    *slog.Logger
}

// NewHandler creates a new ingestion handler.
func NewHandler(publisher pmqueue.EventPublisher, logger *slog.Logger) *Handler {
	return &Handler{
		publisher: publisher,
		logger:    logger,
	}
}

// HandleEvent handles POST /v1/events (single event).
func (h *Handler) HandleEvent(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Method not allowed", nil)
		return
	}

	var req EventRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid request body", nil)
		return
	}

	fieldErrors := ValidateEvent(&req)
	if len(fieldErrors) > 0 {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid request body", map[string]any{
			"fields": fieldErrors,
		})
		return
	}

	orgID := OrgIDFromContext(r.Context())
	orgIDUint := domain.OrgIDToUint64(orgID)

	event := h.buildProtoEvent(orgIDUint, &req)

	if err := h.publisher.Publish(r.Context(), orgIDUint, event); err != nil {
		h.logger.Error("failed to publish event", "error", err, "event_id", req.IdempotencyKey)
		writeError(w, http.StatusServiceUnavailable, "SERVICE_UNAVAILABLE",
			"Ingestion temporarily unavailable. Retry later.",
			map[string]any{"retry_after_ms": 5000})
		return
	}

	writeJSON(w, http.StatusAccepted, map[string]any{
		"data": map[string]any{"accepted": 1},
		"meta": map[string]any{
			"request_id": RequestIDFromContext(r.Context()),
			"timestamp":  time.Now().UTC().Format(time.RFC3339),
		},
	})
}

// HandleBatch handles POST /v1/events/batch.
func (h *Handler) HandleBatch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Method not allowed", nil)
		return
	}

	var req BatchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid request body", nil)
		return
	}

	batchErrors := ValidateBatch(&req)
	if len(batchErrors) > 0 {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid request body", map[string]any{
			"fields": batchErrors,
		})
		return
	}

	orgID := OrgIDFromContext(r.Context())
	orgIDUint := domain.OrgIDToUint64(orgID)

	accepted := 0
	rejected := 0
	var eventErrors []map[string]any

	for i, eventReq := range req.Events {
		fieldErrors := ValidateEvent(&eventReq)
		if len(fieldErrors) > 0 {
			rejected++
			eventErrors = append(eventErrors, map[string]any{
				"index":          i,
				"idempotency_key": eventReq.IdempotencyKey,
				"error":          fieldErrors[0], // first error
			})
			continue
		}

		event := h.buildProtoEvent(orgIDUint, &eventReq)
		if err := h.publisher.Publish(r.Context(), orgIDUint, event); err != nil {
			h.logger.Error("failed to publish event in batch",
				"error", err,
				"event_id", eventReq.IdempotencyKey,
				"index", i,
			)
			rejected++
			eventErrors = append(eventErrors, map[string]any{
				"index":          i,
				"idempotency_key": eventReq.IdempotencyKey,
				"error":          map[string]string{"message": "failed to enqueue"},
			})
			continue
		}
		accepted++
	}

	data := map[string]any{
		"accepted": accepted,
		"rejected": rejected,
	}
	if len(eventErrors) > 0 {
		data["errors"] = eventErrors
	}

	writeJSON(w, http.StatusAccepted, map[string]any{
		"data": data,
		"meta": map[string]any{
			"request_id": RequestIDFromContext(r.Context()),
			"timestamp":  time.Now().UTC().Format(time.RFC3339),
		},
	})
}

// HandleHealth handles GET /health.
func (h *Handler) HandleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok"})
}

func (h *Handler) buildProtoEvent(orgID uint64, req *EventRequest) *eventsv1.LLMEvent {
	ts, _ := time.Parse(time.RFC3339Nano, req.Timestamp)

	totalTokens := uint32(0)
	if req.PromptTokens != nil {
		totalTokens += *req.PromptTokens
	}
	if req.CompletionTokens != nil {
		totalTokens += *req.CompletionTokens
	}

	event := &eventsv1.LLMEvent{
		OrgId:         orgID,
		EventId:       req.IdempotencyKey,
		Timestamp:     ts,
		InsertedAt:    time.Now().UTC(),
		Model:         req.Model,
		Provider:      req.Provider,
		TotalTokens:   totalTokens,
		SchemaVersion: 1,
		Tags:          req.Tags,
	}

	if req.PromptTokens != nil {
		event.PromptTokens = *req.PromptTokens
	}
	if req.CompletionTokens != nil {
		event.CompletionTokens = *req.CompletionTokens
	}
	if req.LatencyMs != nil {
		event.LatencyMs = *req.LatencyMs
	}
	if req.StatusCode != nil {
		event.StatusCode = *req.StatusCode
	}

	if req.Prompt != nil {
		event.Prompt = *req.Prompt
		event.PromptHash = sha256Hex(*req.Prompt)
		event.S3Status = eventsv1.S3Status_S3_STATUS_PENDING
	}
	if req.Response != nil {
		event.Response = *req.Response
		if event.S3Status == eventsv1.S3Status_S3_STATUS_NONE {
			event.S3Status = eventsv1.S3Status_S3_STATUS_PENDING
		}
	}

	if event.Tags == nil {
		event.Tags = make(map[string]string)
	}

	return event
}

func sha256Hex(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		slog.Error("ingestion: write response error", "error", err)
	}
}
