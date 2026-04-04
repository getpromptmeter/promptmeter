// Package eventsv1 provides the LLM event types for NATS wire format.
//
// These types mirror the protobuf schema in events.proto.
// For production, regenerate with protoc. This file provides
// JSON-serializable types that work without protoc code generation.
package eventsv1

import (
	"encoding/json"
	"time"
)

// S3Status represents the S3 upload status of an event.
type S3Status int32

const (
	S3Status_S3_STATUS_NONE     S3Status = 0
	S3Status_S3_STATUS_PENDING  S3Status = 1
	S3Status_S3_STATUS_UPLOADED S3Status = 2
)

func (x S3Status) String() string {
	switch x {
	case S3Status_S3_STATUS_NONE:
		return "S3_STATUS_NONE"
	case S3Status_S3_STATUS_PENDING:
		return "S3_STATUS_PENDING"
	case S3Status_S3_STATUS_UPLOADED:
		return "S3_STATUS_UPLOADED"
	default:
		return "UNKNOWN"
	}
}

// LLMEvent represents a single LLM API call in the NATS wire format.
type LLMEvent struct {
	OrgId            uint64            `json:"org_id"`
	EventId          string            `json:"event_id"`
	ProjectId        string            `json:"project_id,omitempty"`
	Timestamp        time.Time         `json:"timestamp"`
	InsertedAt       time.Time         `json:"inserted_at"`
	Model            string            `json:"model"`
	Provider         string            `json:"provider"`
	PromptTokens     uint32            `json:"prompt_tokens"`
	CompletionTokens uint32            `json:"completion_tokens"`
	TotalTokens      uint32            `json:"total_tokens"`
	CostUsd          float64           `json:"cost_usd"`
	LatencyMs        uint32            `json:"latency_ms"`
	StatusCode       uint32            `json:"status_code"`
	Tags             map[string]string `json:"tags,omitempty"`
	Prompt           string            `json:"prompt,omitempty"`
	Response         string            `json:"response,omitempty"`
	PromptHash       string            `json:"prompt_hash,omitempty"`
	S3Key            string            `json:"s3_key,omitempty"`
	S3Status         S3Status          `json:"s3_status"`
	SchemaVersion    uint32            `json:"schema_version"`
}

// Marshal serializes the event to JSON bytes.
func (x *LLMEvent) Marshal() ([]byte, error) {
	return json.Marshal(x)
}

// Unmarshal deserializes JSON bytes into the event.
func (x *LLMEvent) Unmarshal(data []byte) error {
	return json.Unmarshal(data, x)
}

func (x *LLMEvent) GetOrgId() uint64              { if x != nil { return x.OrgId }; return 0 }
func (x *LLMEvent) GetEventId() string            { if x != nil { return x.EventId }; return "" }
func (x *LLMEvent) GetProjectId() string           { if x != nil { return x.ProjectId }; return "" }
func (x *LLMEvent) GetModel() string              { if x != nil { return x.Model }; return "" }
func (x *LLMEvent) GetProvider() string            { if x != nil { return x.Provider }; return "" }
func (x *LLMEvent) GetPromptTokens() uint32        { if x != nil { return x.PromptTokens }; return 0 }
func (x *LLMEvent) GetCompletionTokens() uint32    { if x != nil { return x.CompletionTokens }; return 0 }
func (x *LLMEvent) GetTotalTokens() uint32         { if x != nil { return x.TotalTokens }; return 0 }
func (x *LLMEvent) GetCostUsd() float64            { if x != nil { return x.CostUsd }; return 0 }
func (x *LLMEvent) GetLatencyMs() uint32           { if x != nil { return x.LatencyMs }; return 0 }
func (x *LLMEvent) GetStatusCode() uint32          { if x != nil { return x.StatusCode }; return 0 }
func (x *LLMEvent) GetTags() map[string]string     { if x != nil { return x.Tags }; return nil }
func (x *LLMEvent) GetPrompt() string              { if x != nil { return x.Prompt }; return "" }
func (x *LLMEvent) GetResponse() string            { if x != nil { return x.Response }; return "" }
func (x *LLMEvent) GetPromptHash() string          { if x != nil { return x.PromptHash }; return "" }
func (x *LLMEvent) GetS3Key() string               { if x != nil { return x.S3Key }; return "" }
func (x *LLMEvent) GetS3Status() S3Status          { if x != nil { return x.S3Status }; return S3Status_S3_STATUS_NONE }
func (x *LLMEvent) GetSchemaVersion() uint32        { if x != nil { return x.SchemaVersion }; return 0 }

// EventBatch is a collection of LLM events.
type EventBatch struct {
	Events []*LLMEvent `json:"events"`
}

func (x *EventBatch) GetEvents() []*LLMEvent {
	if x != nil {
		return x.Events
	}
	return nil
}
