package ingestion

import (
	"strings"
	"testing"
	"time"
)

func ptr[T any](v T) *T { return &v }

func validEvent() *EventRequest {
	return &EventRequest{
		IdempotencyKey:   "01965a3c-8b2f-7d4e-9f1a-2c3d4e5f6a7b",
		Timestamp:        time.Now().UTC().Format(time.RFC3339Nano),
		Model:            "gpt-4o",
		Provider:         "openai",
		PromptTokens:     ptr(uint32(1250)),
		CompletionTokens: ptr(uint32(380)),
		LatencyMs:        ptr(uint32(2340)),
		StatusCode:       ptr(uint32(200)),
		Tags:             map[string]string{"feature": "chat"},
	}
}

func TestValidateEvent_Valid(t *testing.T) {
	errs := ValidateEvent(validEvent())
	if len(errs) != 0 {
		t.Errorf("expected no errors, got %v", errs)
	}
}

func TestValidateEvent_MissingModel(t *testing.T) {
	e := validEvent()
	e.Model = ""
	errs := ValidateEvent(e)
	if !hasFieldError(errs, "model") {
		t.Error("expected model error")
	}
}

func TestValidateEvent_MissingProvider(t *testing.T) {
	e := validEvent()
	e.Provider = ""
	errs := ValidateEvent(e)
	if !hasFieldError(errs, "provider") {
		t.Error("expected provider error")
	}
}

func TestValidateEvent_InvalidProvider(t *testing.T) {
	e := validEvent()
	e.Provider = "invalid"
	errs := ValidateEvent(e)
	if !hasFieldError(errs, "provider") {
		t.Error("expected provider error")
	}
}

func TestValidateEvent_MissingIdempotencyKey(t *testing.T) {
	e := validEvent()
	e.IdempotencyKey = ""
	errs := ValidateEvent(e)
	if !hasFieldError(errs, "idempotency_key") {
		t.Error("expected idempotency_key error")
	}
}

func TestValidateEvent_InvalidUUID(t *testing.T) {
	e := validEvent()
	e.IdempotencyKey = "not-a-uuid"
	errs := ValidateEvent(e)
	if !hasFieldError(errs, "idempotency_key") {
		t.Error("expected idempotency_key error")
	}
}

func TestValidateEvent_MissingTimestamp(t *testing.T) {
	e := validEvent()
	e.Timestamp = ""
	errs := ValidateEvent(e)
	if !hasFieldError(errs, "timestamp") {
		t.Error("expected timestamp error")
	}
}

func TestValidateEvent_FutureTimestamp(t *testing.T) {
	e := validEvent()
	e.Timestamp = time.Now().Add(2 * time.Hour).UTC().Format(time.RFC3339Nano)
	errs := ValidateEvent(e)
	if !hasFieldError(errs, "timestamp") {
		t.Error("expected timestamp error for future timestamp")
	}
}

func TestValidateEvent_InvalidStatusCode(t *testing.T) {
	e := validEvent()
	e.StatusCode = ptr(uint32(0))
	errs := ValidateEvent(e)
	if !hasFieldError(errs, "status_code") {
		t.Error("expected status_code error")
	}

	e.StatusCode = ptr(uint32(600))
	errs = ValidateEvent(e)
	if !hasFieldError(errs, "status_code") {
		t.Error("expected status_code error for 600")
	}
}

func TestValidateEvent_TooManyTags(t *testing.T) {
	e := validEvent()
	e.Tags = make(map[string]string)
	for i := 0; i < 21; i++ {
		e.Tags[strings.Repeat("a", i+1)] = "v"
	}
	errs := ValidateEvent(e)
	if !hasFieldError(errs, "tags") {
		t.Error("expected tags error")
	}
}

func TestValidateEvent_TagKeyTooLong(t *testing.T) {
	e := validEvent()
	e.Tags = map[string]string{
		strings.Repeat("a", 51): "value",
	}
	errs := ValidateEvent(e)
	if !hasFieldError(errs, "tags") {
		t.Error("expected tags key too long error")
	}
}

func TestValidateEvent_TagKeyInvalidChars(t *testing.T) {
	e := validEvent()
	e.Tags = map[string]string{
		"invalid-key": "value",
	}
	errs := ValidateEvent(e)
	if !hasFieldError(errs, "tags") {
		t.Error("expected tags key invalid chars error")
	}
}

func TestValidateEvent_TagValueTooLong(t *testing.T) {
	e := validEvent()
	e.Tags = map[string]string{
		"key": strings.Repeat("v", 201),
	}
	errs := ValidateEvent(e)
	if !hasFieldError(errs, "tags") {
		t.Error("expected tags value too long error")
	}
}

func TestValidateEvent_PromptTruncation(t *testing.T) {
	longPrompt := strings.Repeat("a", 150*1024)
	e := validEvent()
	e.Prompt = &longPrompt
	errs := ValidateEvent(e)
	if len(errs) != 0 {
		t.Errorf("should not reject long prompt, got %v", errs)
	}
	if len(*e.Prompt) != maxPromptBytes {
		t.Errorf("prompt should be truncated to %d, got %d", maxPromptBytes, len(*e.Prompt))
	}
}

func TestValidateEvent_ResponseTruncation(t *testing.T) {
	longResp := strings.Repeat("b", 150*1024)
	e := validEvent()
	e.Response = &longResp
	_ = ValidateEvent(e)
	if len(*e.Response) != maxResponseBytes {
		t.Errorf("response should be truncated to %d, got %d", maxResponseBytes, len(*e.Response))
	}
}

func TestValidateEvent_ModelTooLong(t *testing.T) {
	e := validEvent()
	e.Model = strings.Repeat("m", 101)
	errs := ValidateEvent(e)
	if !hasFieldError(errs, "model") {
		t.Error("expected model too long error")
	}
}

func TestValidateBatch_Empty(t *testing.T) {
	errs := ValidateBatch(&BatchRequest{Events: nil})
	if len(errs) == 0 {
		t.Error("expected error for empty batch")
	}
}

func TestValidateBatch_TooMany(t *testing.T) {
	events := make([]EventRequest, 501)
	errs := ValidateBatch(&BatchRequest{Events: events})
	if len(errs) == 0 {
		t.Error("expected error for too many events")
	}
}

func TestValidateBatch_Valid(t *testing.T) {
	events := make([]EventRequest, 50)
	errs := ValidateBatch(&BatchRequest{Events: events})
	if len(errs) != 0 {
		t.Errorf("expected no errors, got %v", errs)
	}
}

func hasFieldError(errs []FieldError, field string) bool {
	for _, e := range errs {
		if e.Field == field {
			return true
		}
	}
	return false
}
