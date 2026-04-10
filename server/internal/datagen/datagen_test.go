package datagen

import (
	"math"
	"testing"
	"time"
)

func testPriceTable() PriceTable {
	return BuildPriceTable(SeedModelPrices)
}

func testProjects() []ProjectDistribution {
	return []ProjectDistribution{
		{ProjectID: "proj-1", Weight: 0.50},
		{ProjectID: "proj-2", Weight: 0.35},
		{ProjectID: "proj-3", Weight: 0.15},
	}
}

func TestDeterminism(t *testing.T) {
	ts := time.Date(2026, 3, 15, 10, 30, 0, 0, time.UTC)
	pt := testPriceTable()

	g1 := NewGenerator(42, 1, pt, testProjects())
	g2 := NewGenerator(42, 1, pt, testProjects())

	for i := 0; i < 100; i++ {
		e1 := g1.GenerateEvent(ts.Add(time.Duration(i) * time.Second))
		e2 := g2.GenerateEvent(ts.Add(time.Duration(i) * time.Second))

		if e1.EventID != e2.EventID {
			t.Fatalf("event %d: EventID mismatch: %s != %s", i, e1.EventID, e2.EventID)
		}
		if e1.Model != e2.Model {
			t.Fatalf("event %d: Model mismatch: %s != %s", i, e1.Model, e2.Model)
		}
		if e1.PromptTokens != e2.PromptTokens {
			t.Fatalf("event %d: PromptTokens mismatch: %d != %d", i, e1.PromptTokens, e2.PromptTokens)
		}
		if e1.CostUSD != e2.CostUSD {
			t.Fatalf("event %d: CostUSD mismatch: %f != %f", i, e1.CostUSD, e2.CostUSD)
		}
	}
}

func TestModelDistribution(t *testing.T) {
	pt := testPriceTable()
	g := NewGenerator(42, 1, pt, testProjects())
	ts := time.Date(2026, 3, 15, 10, 0, 0, 0, time.UTC)

	counts := make(map[string]int)
	total := 10000
	for i := 0; i < total; i++ {
		e := g.GenerateEvent(ts.Add(time.Duration(i) * time.Millisecond))
		counts[e.Model]++
	}

	for _, m := range DefaultModelDistributions {
		actual := float64(counts[m.Model]) / float64(total)
		diff := math.Abs(actual - m.Weight)
		if diff > 0.05 {
			t.Errorf("model %s: expected weight %.2f, got %.4f (diff %.4f > 0.05)",
				m.Model, m.Weight, actual, diff)
		}
	}
}

func TestHourlyWeights(t *testing.T) {
	nightWeight := HourlyWeight(3)   // 0.2
	peakWeight := HourlyWeight(11)   // 1.0

	if nightWeight >= peakWeight {
		t.Errorf("night weight %.2f should be less than peak weight %.2f", nightWeight, peakWeight)
	}
}

func TestDayWeight(t *testing.T) {
	weekday := DayWeight(time.Monday)
	weekend := DayWeight(time.Saturday)

	if weekend >= weekday {
		t.Errorf("weekend weight %.2f should be less than weekday weight %.2f", weekend, weekday)
	}
}

func TestErrorRate(t *testing.T) {
	pt := testPriceTable()
	g := NewGenerator(42, 1, pt, testProjects())
	ts := time.Date(2026, 3, 15, 10, 0, 0, 0, time.UTC)

	total := 10000
	errors := 0
	for i := 0; i < total; i++ {
		e := g.GenerateEvent(ts.Add(time.Duration(i) * time.Millisecond))
		if e.StatusCode != 200 {
			errors++
		}
	}

	errorRate := float64(errors) / float64(total)
	if math.Abs(errorRate-DefaultErrorRate) > 0.02 {
		t.Errorf("error rate %.4f is too far from expected %.2f", errorRate, DefaultErrorRate)
	}
}

func TestCostCalculation(t *testing.T) {
	pt := testPriceTable()
	g := NewGenerator(42, 1, pt, testProjects())
	ts := time.Date(2026, 3, 15, 10, 0, 0, 0, time.UTC)

	// Generate events until we find a successful one.
	for i := 0; i < 100; i++ {
		e := g.GenerateEvent(ts.Add(time.Duration(i) * time.Millisecond))
		if e.StatusCode != 200 {
			continue
		}

		price := pt.LookupPrice(e.Provider, e.Model, e.Timestamp)
		if price == nil {
			t.Fatalf("no price for %s/%s", e.Provider, e.Model)
		}

		expected := float64(e.PromptTokens)*price.InputPricePerMillion/1_000_000 +
			float64(e.CompletionTokens)*price.OutputPricePerMillion/1_000_000

		if math.Abs(e.CostUSD-expected) > 1e-10 {
			t.Errorf("cost mismatch: got %.10f, want %.10f", e.CostUSD, expected)
		}
		return // One successful check is enough.
	}
	t.Fatal("no successful events generated in 100 attempts")
}

func TestTagsPresent(t *testing.T) {
	pt := testPriceTable()
	g := NewGenerator(42, 1, pt, testProjects())
	ts := time.Date(2026, 3, 15, 10, 0, 0, 0, time.UTC)

	for i := 0; i < 100; i++ {
		e := g.GenerateEvent(ts.Add(time.Duration(i) * time.Millisecond))
		if _, ok := e.Tags["feature"]; !ok {
			t.Fatal("event missing 'feature' tag")
		}
		if _, ok := e.Tags["team"]; !ok {
			t.Fatal("event missing 'team' tag")
		}
	}
}

func TestGenerateBatch(t *testing.T) {
	pt := testPriceTable()
	g := NewGenerator(42, 1, pt, testProjects())

	from := time.Date(2026, 3, 10, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 3, 13, 0, 0, 0, 0, time.UTC)

	events := g.GenerateBatch(from, to, 1000)
	if len(events) == 0 {
		t.Fatal("expected events from GenerateBatch")
	}

	// Should be roughly 3 days * 1000 events/day, adjusted for weekends.
	if len(events) < 500 || len(events) > 5000 {
		t.Errorf("unexpected event count: %d", len(events))
	}

	// All events should have valid fields.
	for i, e := range events {
		if e.EventID == "" {
			t.Fatalf("event %d has empty EventID", i)
		}
		if e.S3Status != "none" {
			t.Fatalf("event %d has S3Status %q, want 'none'", i, e.S3Status)
		}
	}
}

func TestCheckMissingModels(t *testing.T) {
	// Empty price table should report all models as missing.
	missing := CheckMissingModels(PriceTable{})
	if len(missing) != len(DefaultModelDistributions) {
		t.Errorf("expected %d missing models, got %d", len(DefaultModelDistributions), len(missing))
	}

	// Full price table should report no missing models.
	pt := testPriceTable()
	missing = CheckMissingModels(pt)
	if len(missing) != 0 {
		t.Errorf("expected 0 missing models, got %d: %v", len(missing), missing)
	}
}

func TestErrorEventsHaveZeroTokens(t *testing.T) {
	pt := testPriceTable()
	g := NewGenerator(99, 1, pt, testProjects())
	ts := time.Date(2026, 3, 15, 10, 0, 0, 0, time.UTC)

	for i := 0; i < 10000; i++ {
		e := g.GenerateEvent(ts.Add(time.Duration(i) * time.Millisecond))
		if e.StatusCode != 200 && e.TotalTokens != 0 {
			t.Errorf("error event (status %d) has non-zero tokens: %d", e.StatusCode, e.TotalTokens)
		}
	}
}

func TestProjectDistribution(t *testing.T) {
	pt := testPriceTable()
	projects := testProjects()
	g := NewGenerator(42, 1, pt, projects)
	ts := time.Date(2026, 3, 15, 10, 0, 0, 0, time.UTC)

	counts := make(map[string]int)
	total := 10000
	for i := 0; i < total; i++ {
		e := g.GenerateEvent(ts.Add(time.Duration(i) * time.Millisecond))
		counts[e.ProjectID]++
	}

	for _, p := range projects {
		actual := float64(counts[p.ProjectID]) / float64(total)
		diff := math.Abs(actual - p.Weight)
		if diff > 0.05 {
			t.Errorf("project %s: expected weight %.2f, got %.4f (diff %.4f > 0.05)",
				p.ProjectID, p.Weight, actual, diff)
		}
	}
}

func TestScenarioSpike(t *testing.T) {
	s := NewScenario(ScenarioSpike)

	// At 0 minutes (normal phase).
	params := s.Params(0, nil)
	if params.RPSMultiplier != 1.0 {
		t.Errorf("at 0m: expected RPS multiplier 1.0, got %.1f", params.RPSMultiplier)
	}

	// At 7 minutes (spike phase).
	params = s.Params(7*time.Minute, nil)
	if params.RPSMultiplier != 5.0 {
		t.Errorf("at 7m: expected RPS multiplier 5.0, got %.1f", params.RPSMultiplier)
	}
	if params.ModelOverride == nil {
		t.Error("at 7m: expected model override")
	}

	// At 12 minutes (normal again).
	params = s.Params(12*time.Minute, nil)
	if params.RPSMultiplier != 1.0 {
		t.Errorf("at 12m: expected RPS multiplier 1.0, got %.1f", params.RPSMultiplier)
	}
}
