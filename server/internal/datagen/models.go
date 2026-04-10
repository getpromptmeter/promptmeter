// Package datagen provides realistic data generation for development and testing.
package datagen

import "math/rand/v2"

// ModelDistribution defines the probability of selecting a model and parameters
// for generating tokens and latency.
type ModelDistribution struct {
	Model    string  // e.g. "gpt-4o"
	Provider string  // e.g. "openai"
	Weight   float64 // 0.30 = 30% of traffic

	// Token distribution parameters (log-normal).
	PromptTokensMean   float64
	PromptTokensStdDev float64
	CompletionMean     float64
	CompletionStdDev   float64

	// Latency distribution parameters (normal).
	LatencyMeanMs   float64
	LatencyStdDevMs float64
}

// DefaultModelDistributions contains realistic model distributions for data generation.
// All models listed here must have entries in the model_prices table.
var DefaultModelDistributions = []ModelDistribution{
	{
		Model: "gpt-4o", Provider: "openai", Weight: 0.30,
		PromptTokensMean: 1200, PromptTokensStdDev: 400,
		CompletionMean: 400, CompletionStdDev: 150,
		LatencyMeanMs: 2500, LatencyStdDevMs: 500,
	},
	{
		Model: "gpt-4o-mini", Provider: "openai", Weight: 0.25,
		PromptTokensMean: 800, PromptTokensStdDev: 300,
		CompletionMean: 200, CompletionStdDev: 80,
		LatencyMeanMs: 800, LatencyStdDevMs: 200,
	},
	{
		Model: "claude-3-5-sonnet", Provider: "anthropic", Weight: 0.20,
		PromptTokensMean: 1500, PromptTokensStdDev: 500,
		CompletionMean: 600, CompletionStdDev: 200,
		LatencyMeanMs: 3000, LatencyStdDevMs: 600,
	},
	{
		Model: "claude-3-5-haiku", Provider: "anthropic", Weight: 0.10,
		PromptTokensMean: 600, PromptTokensStdDev: 200,
		CompletionMean: 150, CompletionStdDev: 60,
		LatencyMeanMs: 500, LatencyStdDevMs: 100,
	},
	{
		Model: "gemini-1.5-pro", Provider: "google", Weight: 0.08,
		PromptTokensMean: 1000, PromptTokensStdDev: 350,
		CompletionMean: 350, CompletionStdDev: 120,
		LatencyMeanMs: 2000, LatencyStdDevMs: 400,
	},
	{
		Model: "gemini-1.5-flash", Provider: "google", Weight: 0.05,
		PromptTokensMean: 500, PromptTokensStdDev: 150,
		CompletionMean: 120, CompletionStdDev: 50,
		LatencyMeanMs: 400, LatencyStdDevMs: 100,
	},
	{
		Model: "gpt-4-turbo", Provider: "openai", Weight: 0.02,
		PromptTokensMean: 2000, PromptTokensStdDev: 600,
		CompletionMean: 800, CompletionStdDev: 300,
		LatencyMeanMs: 4000, LatencyStdDevMs: 800,
	},
}

// pickModel selects a model based on weighted random distribution.
// The cumulative weights must be precomputed.
func pickModel(rng *rand.Rand, models []ModelDistribution, cumWeights []float64) *ModelDistribution {
	r := rng.Float64()
	for i, cw := range cumWeights {
		if r <= cw {
			return &models[i]
		}
	}
	return &models[len(models)-1]
}

// buildCumulativeWeights precomputes the cumulative weight array for weighted selection.
func buildCumulativeWeights(models []ModelDistribution) []float64 {
	cumWeights := make([]float64, len(models))
	var total float64
	for i, m := range models {
		total += m.Weight
		cumWeights[i] = total
	}
	// Normalize to handle weights that don't sum to exactly 1.0.
	for i := range cumWeights {
		cumWeights[i] /= total
	}
	return cumWeights
}
