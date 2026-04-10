package datagen

import (
	"encoding/binary"
	"math/rand/v2"
	"time"

	"github.com/google/uuid"
)

// Generator produces deterministic events for development data generation.
type Generator struct {
	rng        *rand.Rand
	seed       int64
	prices     PriceTable
	orgID      uint64
	projects   []ProjectDistribution
	models     []ModelDistribution
	cumWeights []float64
	projCum    []float64
}

// NewGenerator creates a new deterministic event generator.
func NewGenerator(seed int64, orgID uint64, prices PriceTable, projects []ProjectDistribution) *Generator {
	models := DefaultModelDistributions
	src := rand.NewPCG(uint64(seed), uint64(seed)^0xdeadbeef)
	rng := rand.New(src)

	projCum := make([]float64, len(projects))
	var total float64
	for i, p := range projects {
		total += p.Weight
		projCum[i] = total
	}
	for i := range projCum {
		projCum[i] /= total
	}

	return &Generator{
		rng:        rng,
		seed:       seed,
		prices:     prices,
		orgID:      orgID,
		projects:   projects,
		models:     models,
		cumWeights: buildCumulativeWeights(models),
		projCum:    projCum,
	}
}

// GenerateEvent generates a single event at the given timestamp.
func (g *Generator) GenerateEvent(ts time.Time) Event {
	return g.generateEvent(ts, nil)
}

// GenerateEventWithParams generates an event using scenario parameters.
func (g *Generator) GenerateEventWithParams(ts time.Time, params *ScenarioParams) Event {
	return g.generateEvent(ts, params)
}

func (g *Generator) generateEvent(ts time.Time, params *ScenarioParams) Event {
	errorRate := DefaultErrorRate
	var modelOverride *ModelDistribution
	var modelOverridePct float64
	tokenMultiplier := 1.0

	if params != nil {
		errorRate = params.ErrorRate
		modelOverride = params.ModelOverride
		modelOverridePct = params.ModelOverridePct
		tokenMultiplier = params.TokenMultiplier
	}

	// Pick model.
	var model *ModelDistribution
	if modelOverride != nil && g.rng.Float64() < modelOverridePct {
		model = modelOverride
	} else {
		model = pickModel(g.rng, g.models, g.cumWeights)
	}

	// Determine if this is an error event.
	var statusCode uint32 = 200
	isError := false
	if g.rng.Float64() < errorRate {
		isError = true
		r := g.rng.Float64()
		switch {
		case r < 0.50: // 50% of errors are 429
			statusCode = 429
		case r < 0.75: // 25% of errors are 500
			statusCode = 500
		default: // 25% of errors are 400
			statusCode = 400
		}
	}

	// Generate tokens and latency.
	prompt, completion := generateTokens(g.rng, model, isError)
	if tokenMultiplier != 1.0 && !isError {
		completion = uint32(float64(completion) * tokenMultiplier)
	}
	latency := generateLatency(g.rng, model, statusCode)

	// Pick project.
	projectID := g.pickProject()

	// Calculate cost.
	price := g.prices.LookupPrice(model.Provider, model.Model, ts)
	cost := calculateCost(prompt, completion, price)

	// Generate deterministic event ID.
	eventID := g.deterministicEventID(ts)

	return Event{
		OrgID:            g.orgID,
		EventID:          eventID,
		ProjectID:        projectID,
		Timestamp:        ts,
		Model:            model.Model,
		Provider:         model.Provider,
		PromptTokens:     prompt,
		CompletionTokens: completion,
		TotalTokens:      prompt + completion,
		CostUSD:          cost,
		LatencyMs:        latency,
		StatusCode:       statusCode,
		Tags:             generateTags(g.rng),
		S3Status:         "none",
		SchemaVersion:    1,
	}
}

func (g *Generator) pickProject() string {
	r := g.rng.Float64()
	for i, cw := range g.projCum {
		if r <= cw {
			return g.projects[i].ProjectID
		}
	}
	return g.projects[len(g.projects)-1].ProjectID
}

// deterministicEventID generates a UUID v7 using the timestamp and random bytes from the seeded RNG.
func (g *Generator) deterministicEventID(ts time.Time) string {
	ms := uint64(ts.UnixMilli())
	var buf [16]byte

	// UUID v7: 48-bit timestamp (ms) + 4-bit version + 12-bit random + 2-bit variant + 62-bit random
	binary.BigEndian.PutUint32(buf[0:4], uint32(ms>>16))
	binary.BigEndian.PutUint16(buf[4:6], uint16(ms))

	// Random bits from seeded RNG.
	randBits := g.rng.Uint64()
	randBits2 := g.rng.Uint64()

	buf[6] = 0x70 | byte((randBits>>56)&0x0F) // version 7
	buf[7] = byte(randBits >> 48)
	buf[8] = 0x80 | byte((randBits>>40)&0x3F) // variant 10
	buf[9] = byte(randBits >> 32)
	buf[10] = byte(randBits >> 24)
	buf[11] = byte(randBits >> 16)
	buf[12] = byte(randBits2 >> 24)
	buf[13] = byte(randBits2 >> 16)
	buf[14] = byte(randBits2 >> 8)
	buf[15] = byte(randBits2)

	id, _ := uuid.FromBytes(buf[:])
	return id.String()
}

// GenerateBatch generates events for a time range with business-hours distribution.
// Events with timestamps after `to` are not generated (safe for partial days).
// Returns events sorted by timestamp.
func (g *Generator) GenerateBatch(from, to time.Time, eventsPerDay int) []Event {
	hourlySum := totalHourlyWeightSum()
	var events []Event

	endOfRange := to.Truncate(24 * time.Hour).AddDate(0, 0, 1)
	for day := from; day.Before(endOfRange); day = day.AddDate(0, 0, 1) {
		dayMultiplier := DayWeight(day.Weekday())
		totalForDay := int(float64(eventsPerDay) * dayMultiplier)

		for hour := 0; hour < 24; hour++ {
			hourWeight := HourlyWeight(hour)
			eventsInHour := int(float64(totalForDay) * hourWeight / hourlySum)

			for i := 0; i < eventsInHour; i++ {
				// Distribute events randomly within the hour.
				minuteOffset := g.rng.IntN(60)
				secondOffset := g.rng.IntN(60)
				milliOffset := g.rng.IntN(1000)

				ts := time.Date(day.Year(), day.Month(), day.Day(), hour, minuteOffset, secondOffset, milliOffset*1_000_000, time.UTC)
				if ts.After(to) {
					continue
				}
				events = append(events, g.GenerateEvent(ts))
			}
		}
	}
	return events
}

// SeedModelPrices returns the model prices to insert if the model_prices table is empty.
// These match the models in DefaultModelDistributions.
var SeedModelPrices = []PriceEntry{
	{Provider: "openai", ModelName: "gpt-4o", InputPricePerMillion: 2.50, OutputPricePerMillion: 10.00, EffectiveFrom: time.Date(2024, 5, 13, 0, 0, 0, 0, time.UTC)},
	{Provider: "openai", ModelName: "gpt-4o-mini", InputPricePerMillion: 0.15, OutputPricePerMillion: 0.60, EffectiveFrom: time.Date(2024, 7, 18, 0, 0, 0, 0, time.UTC)},
	{Provider: "anthropic", ModelName: "claude-3-5-sonnet", InputPricePerMillion: 3.00, OutputPricePerMillion: 15.00, EffectiveFrom: time.Date(2024, 10, 22, 0, 0, 0, 0, time.UTC)},
	{Provider: "anthropic", ModelName: "claude-3-5-haiku", InputPricePerMillion: 0.80, OutputPricePerMillion: 4.00, EffectiveFrom: time.Date(2024, 11, 4, 0, 0, 0, 0, time.UTC)},
	{Provider: "google", ModelName: "gemini-1.5-pro", InputPricePerMillion: 3.50, OutputPricePerMillion: 10.50, EffectiveFrom: time.Date(2024, 5, 14, 0, 0, 0, 0, time.UTC)},
	{Provider: "google", ModelName: "gemini-1.5-flash", InputPricePerMillion: 0.075, OutputPricePerMillion: 0.30, EffectiveFrom: time.Date(2024, 5, 14, 0, 0, 0, 0, time.UTC)},
	{Provider: "openai", ModelName: "gpt-4-turbo", InputPricePerMillion: 10.00, OutputPricePerMillion: 30.00, EffectiveFrom: time.Date(2024, 4, 9, 0, 0, 0, 0, time.UTC)},
}

// BuildPriceTable creates a PriceTable from a slice of PriceEntry.
func BuildPriceTable(entries []PriceEntry) PriceTable {
	pt := make(PriceTable)
	for _, e := range entries {
		key := priceKey(e.Provider, e.ModelName)
		pt[key] = append(pt[key], e)
	}
	return pt
}

// CheckMissingModels returns model names from DefaultModelDistributions that
// have no entry in the given price table.
func CheckMissingModels(pt PriceTable) []string {
	var missing []string
	for _, m := range DefaultModelDistributions {
		key := priceKey(m.Provider, m.Model)
		if _, ok := pt[key]; !ok {
			missing = append(missing, m.Provider+"/"+m.Model)
		}
	}
	return missing
}
