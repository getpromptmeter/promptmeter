// Package ingestion implements the HTTP handlers, validation, and middleware
// for the Promptmeter Ingestion API.
package ingestion

import (
	"fmt"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	maxTags          = 20
	maxTagKeyLen     = 50
	maxTagValueLen   = 200
	maxModelLen      = 100
	maxPromptBytes   = 100 * 1024 // 100 KB
	maxResponseBytes = 100 * 1024 // 100 KB
	maxBatchSize     = 500
	maxBodySingle    = 1 << 20 // 1 MB
	maxBodyBatch     = 5 << 20 // 5 MB
)

var (
	allowedProviders = map[string]bool{
		"openai":    true,
		"anthropic": true,
		"google":    true,
		"other":     true,
	}
	tagKeyRegex = regexp.MustCompile(`^[a-zA-Z0-9_]+$`)
	// Simple UUID v7 format check: 8-4-4-4-12 hex with version nibble = 7
	uuidV7Regex = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-7[0-9a-f]{3}-[0-9a-f]{4}-[0-9a-f]{12}$`)
)

// FieldError represents a validation error on a specific field.
type FieldError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

// EventRequest is the JSON request body for a single event.
type EventRequest struct {
	IdempotencyKey   string            `json:"idempotency_key"`
	Timestamp        string            `json:"timestamp"`
	Model            string            `json:"model"`
	Provider         string            `json:"provider"`
	PromptTokens     *uint32           `json:"prompt_tokens"`
	CompletionTokens *uint32           `json:"completion_tokens"`
	LatencyMs        *uint32           `json:"latency_ms"`
	StatusCode       *uint32           `json:"status_code"`
	Tags             map[string]string `json:"tags"`
	Prompt           *string           `json:"prompt"`
	Response         *string           `json:"response"`
}

// BatchRequest is the JSON request body for a batch of events.
type BatchRequest struct {
	Events []EventRequest `json:"events"`
}

// ValidateEvent validates a single event request and returns field errors.
// It also truncates prompt/response if they exceed 100 KB.
func ValidateEvent(req *EventRequest) []FieldError {
	var errs []FieldError

	// idempotency_key
	if req.IdempotencyKey == "" {
		errs = append(errs, FieldError{Field: "idempotency_key", Message: "required field is missing"})
	} else if !uuidV7Regex.MatchString(strings.ToLower(req.IdempotencyKey)) {
		errs = append(errs, FieldError{Field: "idempotency_key", Message: "invalid UUID format"})
	}

	// timestamp
	if req.Timestamp == "" {
		errs = append(errs, FieldError{Field: "timestamp", Message: "required field is missing"})
	} else {
		ts, err := time.Parse(time.RFC3339Nano, req.Timestamp)
		if err != nil {
			errs = append(errs, FieldError{Field: "timestamp", Message: "invalid timestamp format"})
		} else if ts.After(time.Now().Add(1 * time.Hour)) {
			errs = append(errs, FieldError{Field: "timestamp", Message: "timestamp cannot be more than 1 hour in the future"})
		}
	}

	// model
	if req.Model == "" {
		errs = append(errs, FieldError{Field: "model", Message: "required field is missing"})
	} else if utf8.RuneCountInString(req.Model) > maxModelLen {
		errs = append(errs, FieldError{Field: "model", Message: fmt.Sprintf("max %d characters", maxModelLen)})
	}

	// provider
	if req.Provider == "" {
		errs = append(errs, FieldError{Field: "provider", Message: "required field is missing"})
	} else if !allowedProviders[req.Provider] {
		errs = append(errs, FieldError{Field: "provider", Message: "must be one of: openai, anthropic, google, other"})
	}

	// prompt_tokens
	if req.PromptTokens == nil {
		errs = append(errs, FieldError{Field: "prompt_tokens", Message: "required field is missing"})
	}

	// completion_tokens
	if req.CompletionTokens == nil {
		errs = append(errs, FieldError{Field: "completion_tokens", Message: "required field is missing"})
	}

	// latency_ms
	if req.LatencyMs == nil {
		errs = append(errs, FieldError{Field: "latency_ms", Message: "required field is missing"})
	}

	// status_code
	if req.StatusCode == nil {
		errs = append(errs, FieldError{Field: "status_code", Message: "required field is missing"})
	} else if *req.StatusCode < 100 || *req.StatusCode > 599 {
		errs = append(errs, FieldError{Field: "status_code", Message: "must be between 100 and 599"})
	}

	// tags
	if req.Tags != nil {
		if len(req.Tags) > maxTags {
			errs = append(errs, FieldError{Field: "tags", Message: fmt.Sprintf("max %d tags", maxTags)})
		} else {
			for k, v := range req.Tags {
				if len(k) > maxTagKeyLen {
					errs = append(errs, FieldError{Field: "tags", Message: fmt.Sprintf("key %q too long (max %d)", k, maxTagKeyLen)})
				}
				if !tagKeyRegex.MatchString(k) {
					errs = append(errs, FieldError{Field: "tags", Message: fmt.Sprintf("key %q must be alphanumeric with underscores", k)})
				}
				if len(v) > maxTagValueLen {
					errs = append(errs, FieldError{Field: "tags", Message: fmt.Sprintf("value for %q too long (max %d)", k, maxTagValueLen)})
				}
			}
		}
	}

	// prompt/response: truncate, do NOT reject
	if req.Prompt != nil && len(*req.Prompt) > maxPromptBytes {
		truncated := (*req.Prompt)[:maxPromptBytes]
		req.Prompt = &truncated
	}
	if req.Response != nil && len(*req.Response) > maxResponseBytes {
		truncated := (*req.Response)[:maxResponseBytes]
		req.Response = &truncated
	}

	return errs
}

// ValidateBatch validates a batch request.
func ValidateBatch(req *BatchRequest) []FieldError {
	if len(req.Events) == 0 {
		return []FieldError{{Field: "events", Message: "at least one event is required"}}
	}
	if len(req.Events) > maxBatchSize {
		return []FieldError{{Field: "events", Message: fmt.Sprintf("max %d events per batch", maxBatchSize)}}
	}
	return nil
}
