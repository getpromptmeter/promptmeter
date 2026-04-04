package worker

import (
	"context"
	"log/slog"
	"math"
	"os"
	"testing"
	"time"

	"github.com/promptmeter/promptmeter/server/internal/domain"
)

// mockPriceStore returns a fixed set of prices for testing.
type mockPriceStore struct {
	prices []domain.ModelPrice
}

func (m *mockPriceStore) GetAllPrices(_ context.Context) ([]domain.ModelPrice, error) {
	return m.prices, nil
}

func testPrices() []domain.ModelPrice {
	return []domain.ModelPrice{
		{Provider: "openai", ModelName: "gpt-4o", InputPricePerMillion: 2.50, OutputPricePerMillion: 10.00, EffectiveFrom: date("2024-05-13")},
		{Provider: "openai", ModelName: "gpt-4o-mini", InputPricePerMillion: 0.15, OutputPricePerMillion: 0.60, EffectiveFrom: date("2024-07-18")},
		{Provider: "anthropic", ModelName: "claude-3-5-sonnet", InputPricePerMillion: 3.00, OutputPricePerMillion: 15.00, EffectiveFrom: date("2024-10-22")},
		// Historical price: an older gpt-4o price
		{Provider: "openai", ModelName: "gpt-4o", InputPricePerMillion: 5.00, OutputPricePerMillion: 15.00, EffectiveFrom: date("2024-01-01")},
	}
}

func date(s string) time.Time {
	t, _ := time.Parse("2006-01-02", s)
	return t
}

func almostEqual(a, b, tolerance float64) bool {
	return math.Abs(a-b) < tolerance
}

func TestCalculateCost_GPT4o(t *testing.T) {
	cache := newTestCache(t)

	event := &domain.Event{
		Model:            "gpt-4o",
		Provider:         "openai",
		PromptTokens:     1000,
		CompletionTokens: 500,
		Timestamp:        time.Now(),
	}

	cost := cache.CalculateCost(event)
	// 1000 * 2.50/1M + 500 * 10.00/1M = 0.0025 + 0.005 = 0.0075
	expected := 0.0075
	if !almostEqual(cost, expected, 0.0001) {
		t.Errorf("expected cost %f, got %f", expected, cost)
	}
}

func TestCalculateCost_UnknownModel(t *testing.T) {
	cache := newTestCache(t)

	event := &domain.Event{
		Model:            "unknown-model",
		Provider:         "openai",
		PromptTokens:     1000,
		CompletionTokens: 500,
		Timestamp:        time.Now(),
	}

	cost := cache.CalculateCost(event)
	if cost != 0 {
		t.Errorf("expected cost 0 for unknown model, got %f", cost)
	}
	if event.Tags["_warning"] != "unknown_model" {
		t.Error("expected _warning tag for unknown model")
	}
}

func TestCalculateCost_HistoricalPrice(t *testing.T) {
	cache := newTestCache(t)

	// Event from before the new gpt-4o price (2024-05-13), should use old price
	event := &domain.Event{
		Model:            "gpt-4o",
		Provider:         "openai",
		PromptTokens:     1000,
		CompletionTokens: 500,
		Timestamp:        date("2024-03-01"),
	}

	cost := cache.CalculateCost(event)
	// Should use old price: 1000 * 5.00/1M + 500 * 15.00/1M = 0.005 + 0.0075 = 0.0125
	expected := 0.0125
	if !almostEqual(cost, expected, 0.0001) {
		t.Errorf("expected historical cost %f, got %f", expected, cost)
	}
}

func TestCalculateCost_ZeroTokens(t *testing.T) {
	cache := newTestCache(t)

	event := &domain.Event{
		Model:    "gpt-4o",
		Provider: "openai",
		Timestamp: time.Now(),
	}

	cost := cache.CalculateCost(event)
	if cost != 0 {
		t.Errorf("expected cost 0 for zero tokens, got %f", cost)
	}
}

func TestCalculateCost_Anthropic(t *testing.T) {
	cache := newTestCache(t)

	event := &domain.Event{
		Model:            "claude-3-5-sonnet",
		Provider:         "anthropic",
		PromptTokens:     2000,
		CompletionTokens: 1000,
		Timestamp:        time.Now(),
	}

	cost := cache.CalculateCost(event)
	// 2000 * 3.00/1M + 1000 * 15.00/1M = 0.006 + 0.015 = 0.021
	expected := 0.021
	if !almostEqual(cost, expected, 0.0001) {
		t.Errorf("expected cost %f, got %f", expected, cost)
	}
}

func newTestCache(t *testing.T) *PriceCache {
	t.Helper()
	store := &mockPriceStore{prices: testPrices()}
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	cache := NewPriceCache(store, time.Hour, logger)
	if err := cache.refresh(context.Background()); err != nil {
		t.Fatalf("failed to refresh cache: %v", err)
	}
	return cache
}
