package datagen

import (
	"math/rand/v2"
	"time"
)

// ScenarioType identifies the traffic generation scenario.
type ScenarioType string

const (
	ScenarioNormal  ScenarioType = "normal"
	ScenarioSpike   ScenarioType = "spike"
	ScenarioAnomaly ScenarioType = "anomaly"
)

// Scenario controls how events are generated at a given point in time.
type Scenario interface {
	// Params returns the generation parameters for the current elapsed time.
	Params(elapsed time.Duration, rng *rand.Rand) ScenarioParams
}

// ScenarioParams holds the runtime-adjustable generation parameters.
type ScenarioParams struct {
	RPSMultiplier     float64            // Multiplier for the base RPS.
	ErrorRate         float64            // Fraction of events that should be errors.
	ModelOverride     *ModelDistribution // If non-nil, this model dominates traffic.
	ModelOverridePct  float64            // Fraction of traffic for the override model (0-1).
	TokenMultiplier   float64            // Multiplier for completion tokens.
}

// DefaultErrorRate is the baseline error rate for generated events.
const DefaultErrorRate = 0.02

// normalScenario produces stationary traffic.
type normalScenario struct{}

func (normalScenario) Params(_ time.Duration, _ *rand.Rand) ScenarioParams {
	return ScenarioParams{
		RPSMultiplier:   1.0,
		ErrorRate:       DefaultErrorRate,
		TokenMultiplier: 1.0,
	}
}

// spikeScenario cycles: 5 min normal, 5 min spike (5x RPS, 80% gpt-4o), 5 min normal.
type spikeScenario struct{}

func (spikeScenario) Params(elapsed time.Duration, _ *rand.Rand) ScenarioParams {
	cycle := elapsed % (15 * time.Minute)
	if cycle >= 5*time.Minute && cycle < 10*time.Minute {
		return ScenarioParams{
			RPSMultiplier:    5.0,
			ErrorRate:        DefaultErrorRate,
			ModelOverride:    &DefaultModelDistributions[0], // gpt-4o
			ModelOverridePct: 0.80,
			TokenMultiplier:  1.0,
		}
	}
	return ScenarioParams{
		RPSMultiplier:   1.0,
		ErrorRate:       DefaultErrorRate,
		TokenMultiplier: 1.0,
	}
}

// anomalyScenario triggers anomaly patterns every 3-5 minutes.
type anomalyScenario struct {
	nextAnomalyAt time.Duration
	anomalyEnd    time.Duration
	anomalyType   int // 0=error_burst, 1=model_switch, 2=token_explosion
}

func (a *anomalyScenario) Params(elapsed time.Duration, rng *rand.Rand) ScenarioParams {
	// Initialize first anomaly timing.
	if a.nextAnomalyAt == 0 {
		a.nextAnomalyAt = time.Duration(3+rng.IntN(3)) * time.Minute
	}

	// Check if we're inside an anomaly window.
	if elapsed >= a.nextAnomalyAt && a.anomalyEnd == 0 {
		a.anomalyType = rng.IntN(3)
		switch a.anomalyType {
		case 0: // error_burst: 60s
			a.anomalyEnd = a.nextAnomalyAt + 60*time.Second
		case 1: // model_switch: 2 min
			a.anomalyEnd = a.nextAnomalyAt + 2*time.Minute
		case 2: // token_explosion: 90s
			a.anomalyEnd = a.nextAnomalyAt + 90*time.Second
		}
	}

	// During anomaly.
	if a.anomalyEnd > 0 && elapsed < a.anomalyEnd {
		switch a.anomalyType {
		case 0: // error_burst
			return ScenarioParams{
				RPSMultiplier:   1.0,
				ErrorRate:       0.30,
				TokenMultiplier: 1.0,
			}
		case 1: // model_switch to gpt-4-turbo
			return ScenarioParams{
				RPSMultiplier:    1.0,
				ErrorRate:        DefaultErrorRate,
				ModelOverride:    &DefaultModelDistributions[6], // gpt-4-turbo
				ModelOverridePct: 0.90,
				TokenMultiplier:  1.0,
			}
		case 2: // token_explosion
			return ScenarioParams{
				RPSMultiplier:   1.0,
				ErrorRate:       DefaultErrorRate,
				TokenMultiplier: 5.0,
			}
		}
	}

	// After anomaly ends, schedule next one.
	if a.anomalyEnd > 0 && elapsed >= a.anomalyEnd {
		a.nextAnomalyAt = a.anomalyEnd + time.Duration(3+rng.IntN(3))*time.Minute
		a.anomalyEnd = 0
	}

	return ScenarioParams{
		RPSMultiplier:   1.0,
		ErrorRate:       DefaultErrorRate,
		TokenMultiplier: 1.0,
	}
}

// NewScenario creates a Scenario by type name.
func NewScenario(st ScenarioType) Scenario {
	switch st {
	case ScenarioSpike:
		return &spikeScenario{}
	case ScenarioAnomaly:
		return &anomalyScenario{}
	default:
		return normalScenario{}
	}
}
