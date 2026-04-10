package datagen

import (
	"time"
)

// Event represents a generated LLM event suitable for insertion into ClickHouse.
type Event struct {
	OrgID            uint64
	EventID          string
	ProjectID        string
	Timestamp        time.Time
	Model            string
	Provider         string
	PromptTokens     uint32
	CompletionTokens uint32
	TotalTokens      uint32
	CostUSD          float64
	LatencyMs        uint32
	StatusCode       uint32
	Tags             map[string]string
	S3Status         string
	SchemaVersion    uint8
}

// PriceEntry holds pricing info for a single model, used for cost calculation.
type PriceEntry struct {
	Provider              string
	ModelName             string
	InputPricePerMillion  float64
	OutputPricePerMillion float64
	EffectiveFrom         time.Time
}

// PriceTable maps "provider/model" to a slice of PriceEntry sorted by EffectiveFrom DESC.
type PriceTable map[string][]PriceEntry

// priceKey returns the lookup key for the price table.
func priceKey(provider, model string) string {
	return provider + "/" + model
}

// LookupPrice finds the applicable price for a model at a given timestamp.
// Returns nil if no price is found.
func (pt PriceTable) LookupPrice(provider, model string, ts time.Time) *PriceEntry {
	entries, ok := pt[priceKey(provider, model)]
	if !ok {
		return nil
	}
	for i := range entries {
		if !entries[i].EffectiveFrom.After(ts) {
			return &entries[i]
		}
	}
	return nil
}

// calculateCost computes cost_usd from tokens and price entry.
func calculateCost(promptTokens, completionTokens uint32, price *PriceEntry) float64 {
	if price == nil {
		return 0
	}
	return float64(promptTokens)*price.InputPricePerMillion/1_000_000 +
		float64(completionTokens)*price.OutputPricePerMillion/1_000_000
}

// ProjectDistribution defines the weight for distributing events across projects.
type ProjectDistribution struct {
	ProjectID string
	Weight    float64
}

// DefaultProjectDistributions defines the default project distribution.
// Project IDs are set externally; these use slug-based placeholders.
var DefaultProjectDistributions = []ProjectDistribution{
	{ProjectID: "backend-api", Weight: 0.50},
	{ProjectID: "chat-support", Weight: 0.35},
	{ProjectID: "internal-tools", Weight: 0.15},
}
