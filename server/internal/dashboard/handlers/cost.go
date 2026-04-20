package handlers

import (
	"encoding/json"
	"log/slog"
	"math"
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

// validTimeseriesGroupBy lists allowed group_by values for timeseries endpoint.
var validTimeseriesGroupBy = map[string]bool{
	"":        true,
	"model":   true,
	"feature": true,
}

// HandleCostBreakdown handles GET /api/v1/dashboard/cost.
func (h *CostHandler) HandleCostBreakdown(w http.ResponseWriter, r *http.Request) {
	params, period, err := parseDashboardParams(r)
	if err != nil {
		middleware.WriteError(w, http.StatusBadRequest, "VALIDATION_ERROR", err.Error(), nil)
		return
	}

	modelF, providerF, featureF := parseFilterParams(r)
	params.ModelFilter = modelF
	params.ProviderFilter = providerF
	params.FeatureFilter = featureF

	groupBy := r.URL.Query().Get("group_by")
	if groupBy == "" {
		groupBy = "model"
	}
	if !validGroupBy[groupBy] {
		middleware.WriteError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid group_by parameter", nil)
		return
	}

	orgID := middleware.OrgIDFromContext(r.Context())
	key := cacheKey(orgID, "cost", period, params.ProjectID, groupBy, modelF, providerF, featureF, params.Timezone)

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

	filters := map[string]string{}
	if modelF != "" {
		filters["model"] = modelF
	}
	if providerF != "" {
		filters["provider"] = providerF
	}
	if featureF != "" {
		filters["feature"] = featureF
	}

	data, _ := json.Marshal(map[string]any{
		"data": items,
		"meta": map[string]any{
			"request_id": w.Header().Get("X-Request-Id"),
			"timestamp":  time.Now().UTC().Format(time.RFC3339),
			"group_by":   groupBy,
			"period":     period,
			"filters":    filters,
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

	modelF, providerF, featureF := parseFilterParams(r)
	params.ModelFilter = modelF
	params.ProviderFilter = providerF
	params.FeatureFilter = featureF

	groupBy := r.URL.Query().Get("group_by")
	if !validTimeseriesGroupBy[groupBy] {
		middleware.WriteError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Timeseries group_by must be 'model', 'feature', or empty", nil)
		return
	}

	granularity := granularityForPeriod(period)

	orgID := middleware.OrgIDFromContext(r.Context())
	key := cacheKey(orgID, "timeseries", period, params.ProjectID, groupBy, modelF, providerF, featureF, params.Timezone)

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

// HandleCostCompare handles GET /api/v1/dashboard/cost/compare.
func (h *CostHandler) HandleCostCompare(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	// Parse required date params
	currentFrom, err := time.Parse(time.RFC3339, q.Get("current_from"))
	if err != nil {
		middleware.WriteError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid current_from: must be ISO 8601", nil)
		return
	}
	currentTo, err := time.Parse(time.RFC3339, q.Get("current_to"))
	if err != nil {
		middleware.WriteError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid current_to: must be ISO 8601", nil)
		return
	}
	previousFrom, err := time.Parse(time.RFC3339, q.Get("previous_from"))
	if err != nil {
		middleware.WriteError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid previous_from: must be ISO 8601", nil)
		return
	}
	previousTo, err := time.Parse(time.RFC3339, q.Get("previous_to"))
	if err != nil {
		middleware.WriteError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid previous_to: must be ISO 8601", nil)
		return
	}

	// Validate periods have same duration (within 1 second tolerance)
	currentDur := currentTo.Sub(currentFrom)
	previousDur := previousTo.Sub(previousFrom)
	if math.Abs(currentDur.Seconds()-previousDur.Seconds()) > 1 {
		middleware.WriteError(w, http.StatusBadRequest, "VALIDATION_ERROR",
			"Current and previous periods must have the same duration",
			map[string]any{
				"current_duration_hours":  currentDur.Hours(),
				"previous_duration_hours": previousDur.Hours(),
			})
		return
	}

	groupBy := q.Get("group_by")
	if groupBy == "" {
		groupBy = "model"
	}
	if !validGroupBy[groupBy] {
		middleware.WriteError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid group_by parameter", nil)
		return
	}

	tz := q.Get("timezone")
	if tz == "" {
		tz = "UTC"
	}

	project := q.Get("project")
	modelF, providerF, featureF := parseFilterParams(r)

	orgNumeric := middleware.OrgNumericFromContext(r.Context())
	orgID := middleware.OrgIDFromContext(r.Context())

	currentParams := storage.DashboardQueryParams{
		OrgID: orgNumeric, ProjectID: project, From: currentFrom, To: currentTo,
		Timezone: tz, ModelFilter: modelF, ProviderFilter: providerF, FeatureFilter: featureF,
	}
	previousParams := storage.DashboardQueryParams{
		OrgID: orgNumeric, ProjectID: project, From: previousFrom, To: previousTo,
		Timezone: tz, ModelFilter: modelF, ProviderFilter: providerF, FeatureFilter: featureF,
	}

	key := cacheKey(orgID, "compare",
		currentFrom.Format(time.RFC3339), currentTo.Format(time.RFC3339),
		previousFrom.Format(time.RFC3339), previousTo.Format(time.RFC3339),
		project, groupBy, modelF, providerF, featureF, tz)

	if cached, _ := h.cache.GetCached(r.Context(), key); cached != nil {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Cache", "HIT")
		w.Write(cached)
		return
	}

	result, err := h.reader.GetCostCompare(r.Context(), currentParams, previousParams, groupBy, 20)
	if err != nil {
		h.logger.Error("cost compare: query error", "error", err)
		middleware.WriteError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to load compare data", nil)
		return
	}

	data, _ := json.Marshal(map[string]any{
		"data": result,
		"meta": map[string]any{
			"request_id": w.Header().Get("X-Request-Id"),
			"timestamp":  time.Now().UTC().Format(time.RFC3339),
			"group_by":   groupBy,
		},
	})

	if cacheErr := h.cache.SetCached(r.Context(), key, data, 60*time.Second); cacheErr != nil {
		h.logger.Warn("compare: cache set error", "error", cacheErr)
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Cache", "MISS")
	w.Write(data)
}

// HandleCostFilters handles GET /api/v1/dashboard/cost/filters.
func (h *CostHandler) HandleCostFilters(w http.ResponseWriter, r *http.Request) {
	params, _, err := parseDashboardParams(r)
	if err != nil {
		middleware.WriteError(w, http.StatusBadRequest, "VALIDATION_ERROR", err.Error(), nil)
		return
	}

	orgID := middleware.OrgIDFromContext(r.Context())
	key := cacheKey(orgID, "cost-filters", params.ProjectID, params.Timezone)

	if cached, _ := h.cache.GetCached(r.Context(), key); cached != nil {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Cache", "HIT")
		w.Write(cached)
		return
	}

	result, err := h.reader.GetCostFilters(r.Context(), params)
	if err != nil {
		h.logger.Error("cost filters: query error", "error", err)
		middleware.WriteError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to load filter values", nil)
		return
	}

	data, _ := json.Marshal(map[string]any{
		"data": result,
		"meta": map[string]any{
			"request_id": w.Header().Get("X-Request-Id"),
			"timestamp":  time.Now().UTC().Format(time.RFC3339),
		},
	})

	if cacheErr := h.cache.SetCached(r.Context(), key, data, 5*time.Minute); cacheErr != nil {
		h.logger.Warn("cost-filters: cache set error", "error", cacheErr)
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Cache", "MISS")
	w.Write(data)
}
