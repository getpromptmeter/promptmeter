package datagen

import (
	"math"
	"math/rand/v2"
)

// generateTokens generates prompt and completion token counts using a log-normal
// distribution parameterized by the model. Returns 0 for both if the event is an error.
func generateTokens(rng *rand.Rand, model *ModelDistribution, isError bool) (prompt, completion uint32) {
	if isError {
		return 0, 0
	}
	prompt = logNormalSample(rng, model.PromptTokensMean, model.PromptTokensStdDev)
	completion = logNormalSample(rng, model.CompletionMean, model.CompletionStdDev)
	return prompt, completion
}

// generateLatency generates latency in milliseconds using a normal distribution.
// Error events get fixed latency values depending on the status code.
func generateLatency(rng *rand.Rand, model *ModelDistribution, statusCode uint32) uint32 {
	switch statusCode {
	case 429:
		return 100
	case 500:
		return 30000
	case 400:
		return 50
	}
	lat := model.LatencyMeanMs + rng.NormFloat64()*model.LatencyStdDevMs
	if lat < 50 {
		lat = 50
	}
	return uint32(lat)
}

// logNormalSample generates a sample from a log-normal distribution given the
// desired mean and stddev in the original (non-log) space.
func logNormalSample(rng *rand.Rand, mean, stddev float64) uint32 {
	if mean <= 0 {
		return 0
	}
	// Convert mean/stddev to log-normal parameters.
	variance := stddev * stddev
	mu := math.Log(mean*mean/math.Sqrt(variance+mean*mean)) //nolint:mnd
	sigma := math.Sqrt(math.Log(1 + variance/(mean*mean)))

	sample := math.Exp(mu + sigma*rng.NormFloat64())
	if sample < 1 {
		sample = 1
	}
	return uint32(sample)
}
