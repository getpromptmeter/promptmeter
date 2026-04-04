// Package worker implements the NATS consumer, batch writer, cost calculator,
// and S3 uploader for the Promptmeter Worker service.
package worker

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/promptmeter/promptmeter/server/internal/domain"
	"github.com/promptmeter/promptmeter/server/internal/storage"
)

// priceKey uniquely identifies a model price entry for lookup.
type priceKey struct {
	Provider  string
	ModelName string
}

// PriceCache maintains an in-memory cache of model prices, refreshed periodically.
type PriceCache struct {
	store           storage.PriceStore
	refreshInterval time.Duration
	logger          *slog.Logger

	mu     sync.RWMutex
	prices map[priceKey][]domain.ModelPrice // sorted by effective_from DESC
}

// NewPriceCache creates a new price cache that refreshes from the store.
func NewPriceCache(store storage.PriceStore, refreshInterval time.Duration, logger *slog.Logger) *PriceCache {
	return &PriceCache{
		store:           store,
		refreshInterval: refreshInterval,
		logger:          logger,
		prices:          make(map[priceKey][]domain.ModelPrice),
	}
}

// Start loads prices initially and begins periodic refresh. It blocks until ctx is cancelled.
func (c *PriceCache) Start(ctx context.Context) error {
	if err := c.refresh(ctx); err != nil {
		return fmt.Errorf("price cache initial load: %w", err)
	}

	ticker := time.NewTicker(c.refreshInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if err := c.refresh(ctx); err != nil {
				c.logger.Warn("price cache refresh failed", "error", err)
			}
		}
	}
}

func (c *PriceCache) refresh(ctx context.Context) error {
	allPrices, err := c.store.GetAllPrices(ctx)
	if err != nil {
		return err
	}

	newPrices := make(map[priceKey][]domain.ModelPrice)
	for _, p := range allPrices {
		key := priceKey{Provider: p.Provider, ModelName: p.ModelName}
		newPrices[key] = append(newPrices[key], p)
	}

	c.mu.Lock()
	c.prices = newPrices
	c.mu.Unlock()

	c.logger.Debug("price cache refreshed", "models", len(newPrices))
	return nil
}

// CalculateCost computes the cost in USD for a given event.
// If the model is unknown, returns 0 and adds a _warning tag.
func (c *PriceCache) CalculateCost(event *domain.Event) float64 {
	c.mu.RLock()
	defer c.mu.RUnlock()

	key := priceKey{Provider: event.Provider, ModelName: event.Model}
	prices, ok := c.prices[key]
	if !ok {
		// Unknown model
		if event.Tags == nil {
			event.Tags = make(map[string]string)
		}
		event.Tags["_warning"] = "unknown_model"
		return 0
	}

	// Find the price with max(effective_from) <= event.timestamp
	eventDate := event.Timestamp
	var matched *domain.ModelPrice
	for i := range prices {
		if !prices[i].EffectiveFrom.After(eventDate) {
			matched = &prices[i]
			break // prices are sorted DESC by effective_from
		}
	}

	if matched == nil {
		if event.Tags == nil {
			event.Tags = make(map[string]string)
		}
		event.Tags["_warning"] = "unknown_model"
		return 0
	}

	inputCost := float64(event.PromptTokens) * matched.InputPricePerMillion / 1_000_000
	outputCost := float64(event.CompletionTokens) * matched.OutputPricePerMillion / 1_000_000
	return inputCost + outputCost
}
