package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/promptmeter/promptmeter/server/internal/middleware"
	"github.com/promptmeter/promptmeter/server/internal/storage"
)

// CostHandler handles cost breakdown and timeseries endpoints.
type CostHandler struct {
	reader storage.DashboardReader
	cache  storage.DashboardCache
	logger *slog.Logger
}

// NewCostHandler creates a new cost handler.
func NewCostHandler(reader storage.DashboardReader, cache storage.DashboardCache, logger *slog.Logger) *CostHandler {
	return &CostHandler{reader: reader, cache: cache, logger: logger}
}

// HandleCostBreakdown handles GET /api/v1/dashboard/cost.
func (h *CostHandler) HandleCostBreakdown(w http.ResponseWriter, r *http.Request) {
	params, period, err := parseDashboardParams(r)
	if err != nil {
		middleware.WriteError(w, http.StatusBadRequest, "VALIDATION_ERROR", err.Error(), nil)
		return
	}

	groupBy := r.URL.Query().Get("group_by")
	if groupBy == "" {
		groupBy = "model"
	}
	if !validGroupBy[groupBy] {
		middleware.WriteError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid group_by parameter", nil)
		return
	}

	orgID := middleware.OrgIDFromContext(r.Context())
	key := cacheKey(orgID, "cost", period, params.ProjectID, groupBy, params.Timezone)

	// Check cache
	if cached, _ := h.cache.GetCached(r.Context(), key); cached != nil {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Cache", "HIT")
		w.Write(cached)
		return
	}

	items, err := h.reader.GetCostBreakdown(r.Context(), params, groupBy, 20)
	if err != nil {
		h.logger.Error("cost breakdown: query error", "error", err)
		middleware.WriteError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to load cost data", nil)
		return
	}

	// Calculate percent_of_total
	var totalCost float64
	for _, item := range items {
		totalCost += item.CostUSD
	}
	if totalCost > 0 {
		for i := range items {
			items[i].PercentOfTotal = items[i].CostUSD / totalCost * 100
		}
	}

	data, _ := json.Marshal(map[string]any{
		"data": items,
		"meta": map[string]any{
			"request_id": w.Header().Get("X-Request-Id"),
			"timestamp":  time.Now().UTC().Format(time.RFC3339),
			"group_by":   groupBy,
			"period":     period,
		},
	})

	if cacheErr := h.cache.SetCached(r.Context(), key, data, 60*time.Second); cacheErr != nil {
		h.logger.Warn("cost: cache set error", "error", cacheErr)
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Cache", "MISS")
	w.Write(data)
}

// HandleCostTimeseries handles GET /api/v1/dashboard/cost/timeseries.
func (h *CostHandler) HandleCostTimeseries(w http.ResponseWriter, r *http.Request) {
	params, period, err := parseDashboardParams(r)
	if err != nil {
		middleware.WriteError(w, http.StatusBadRequest, "VALIDATION_ERROR", err.Error(), nil)
		return
	}

	groupBy := r.URL.Query().Get("group_by")
	if groupBy != "" && groupBy != "model" {
		middleware.WriteError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Timeseries group_by must be 'model' or empty", nil)
		return
	}

	granularity := granularityForPeriod(period)

	orgID := middleware.OrgIDFromContext(r.Context())
	key := cacheKey(orgID, "timeseries", period, params.ProjectID, groupBy, params.Timezone)

	// Check cache
	if cached, _ := h.cache.GetCached(r.Context(), key); cached != nil {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Cache", "HIT")
		w.Write(cached)
		return
	}

	series, err := h.reader.GetCostTimeseries(r.Context(), params, groupBy, granularity)
	if err != nil {
		h.logger.Error("cost timeseries: query error", "error", err)
		middleware.WriteError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to load timeseries data", nil)
		return
	}

	data, _ := json.Marshal(map[string]any{
		"data": map[string]any{
			"series":      series,
			"granularity": granularity,
		},
		"meta": map[string]any{
			"request_id": w.Header().Get("X-Request-Id"),
			"timestamp":  time.Now().UTC().Format(time.RFC3339),
			"period":     period,
			"group_by":   groupBy,
		},
	})

	if cacheErr := h.cache.SetCached(r.Context(), key, data, 60*time.Second); cacheErr != nil {
		h.logger.Warn("timeseries: cache set error", "error", cacheErr)
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Cache", "MISS")
	w.Write(data)
}
