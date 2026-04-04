// Package domain contains core business types for Promptmeter.
package domain

import "time"

// Event represents a single LLM API call tracked by the system.
type Event struct {
	OrgID            uint64            `json:"org_id"`
	EventID          string            `json:"event_id"`
	ProjectID        string            `json:"project_id,omitempty"`
	Timestamp        time.Time         `json:"timestamp"`
	InsertedAt       time.Time         `json:"inserted_at"`
	Model            string            `json:"model"`
	Provider         string            `json:"provider"`
	PromptTokens     uint32            `json:"prompt_tokens"`
	CompletionTokens uint32            `json:"completion_tokens"`
	TotalTokens      uint32            `json:"total_tokens"`
	CostUSD          float64           `json:"cost_usd"`
	LatencyMs        uint32            `json:"latency_ms"`
	StatusCode       uint32            `json:"status_code"`
	Tags             map[string]string `json:"tags,omitempty"`
	Prompt           string            `json:"prompt,omitempty"`
	Response         string            `json:"response,omitempty"`
	PromptHash       string            `json:"prompt_hash,omitempty"`
	S3Key            string            `json:"s3_key,omitempty"`
	S3Status         string            `json:"s3_status"`
	SchemaVersion    uint32            `json:"schema_version"`
}

// S3 status constants.
const (
	S3StatusNone     = "none"
	S3StatusPending  = "pending"
	S3StatusUploaded = "uploaded"
)

// HasText returns true if the event contains prompt or response text
// that should be offloaded to S3.
func (e *Event) HasText() bool {
	return e.Prompt != "" || e.Response != ""
}

// S3ObjectKey returns the S3 key for storing prompt/response text.
// Format: v1/{org_id}/{YYYY}/{MM}/{DD}/{event_id}.zst
func (e *Event) S3ObjectKey() string {
	t := e.Timestamp
	return "v1/" +
		uintToStr(e.OrgID) + "/" +
		t.Format("2006") + "/" +
		t.Format("01") + "/" +
		t.Format("02") + "/" +
		e.EventID + ".zst"
}

func uintToStr(n uint64) string {
	if n == 0 {
		return "0"
	}
	buf := make([]byte, 0, 20)
	for n > 0 {
		buf = append(buf, byte('0'+n%10))
		n /= 10
	}
	// reverse
	for i, j := 0, len(buf)-1; i < j; i, j = i+1, j-1 {
		buf[i], buf[j] = buf[j], buf[i]
	}
	return string(buf)
}
