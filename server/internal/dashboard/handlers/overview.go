package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/promptmeter/promptmeter/server/internal/middleware"
	"github.com/promptmeter/promptmeter/server/internal/storage"
)

// OverviewHandler handles GET /api/v1/dashboard/overview.
type OverviewHandler struct {
	reader storage.DashboardReader
	cache  storage.DashboardCache
	logger *slog.Logger
}

// NewOverviewHandler creates a new overview handler.
func NewOverviewHandler(reader storage.DashboardReader, cache storage.DashboardCache, logger *slog.Logger) *OverviewHandler {
	return &OverviewHandler{reader: reader, cache: cache, logger: logger}
}

type overviewResponse struct {
	TotalCost       float64  `json:"total_cost"`
	TotalRequests   uint64   `json:"total_requests"`
	ErrorRate       float64  `json:"error_rate"`
	AvgCostPerReq   float64  `json:"avg_cost_per_request"`
	AvgLatencyMs    float64  `json:"avg_latency_ms"`
	CostChange      *float64 `json:"cost_change_percent"`
	RequestsChange  *float64 `json:"requests_change_percent"`
	ErrorRateChange *float64 `json:"error_rate_change_percent"`
	CostPerReqChange *float64 `json:"cost_per_req_change_percent"`
	TopModel        string   `json:"top_model,omitempty"`
	TopFeature      string   `json:"top_feature,omitempty"`
}

// HandleOverview handles GET /api/v1/dashboard/overview.
func (h *OverviewHandler) HandleOverview(w http.ResponseWriter, r *http.Request) {
	params, period, err := parseDashboardParams(r)
	if err != nil {
		middleware.WriteError(w, http.StatusBadRequest, "VALIDATION_ERROR", err.Error(), nil)
		return
	}

	orgID := middleware.OrgIDFromContext(r.Context())
	key := cacheKey(orgID, "overview", period, params.ProjectID, params.Timezone)

	// Check cache
	if cached, _ := h.cache.GetCached(r.Context(), key); cached != nil {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Cache", "HIT")
		w.Write(cached)
		return
	}

	// Get current period KPIs
	current, err := h.reader.GetOverviewKPIs(r.Context(), params)
	if err != nil {
		h.logger.Error("overview: get current kpis", "error", err)
		middleware.WriteError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to load overview data", nil)
		return
	}

	// Get previous period KPIs for comparison
	dur := params.To.Sub(params.From)
	prevParams := params
	prevParams.To = params.From
	prevParams.From = params.From.Add(-dur)

	previous, err := h.reader.GetOverviewKPIs(r.Context(), prevParams)
	if err != nil {
		h.logger.Warn("overview: get previous kpis", "error", err)
		// Continue without comparison data
		previous = nil
	}

	// Get top model
	models, err := h.reader.GetCostBreakdown(r.Context(), params, "model", 1)
	if err != nil {
		h.logger.Warn("overview: get top model", "error", err)
	}

	// Get top feature
	features, err := h.reader.GetCostBreakdown(r.Context(), params, "feature", 1)
	if err != nil {
		h.logger.Warn("overview: get top feature", "error", err)
	}

	// Build response
	resp := overviewResponse{
		TotalCost:     current.TotalCost,
		TotalRequests: current.TotalRequests,
		AvgLatencyMs:  current.AvgLatencyMs,
	}

	if current.TotalRequests > 0 {
		resp.ErrorRate = float64(current.TotalErrors) / float64(current.TotalRequests) * 100
		resp.AvgCostPerReq = current.TotalCost / float64(current.TotalRequests)
	}

	if len(models) > 0 {
		resp.TopModel = models[0].Group
	}
	if len(features) > 0 {
		resp.TopFeature = features[0].Group
	}

	// Calculate percent changes
	if previous != nil {
		resp.CostChange = percentChange(previous.TotalCost, current.TotalCost)
		resp.RequestsChange = percentChange(float64(previous.TotalRequests), float64(current.TotalRequests))

		var prevErrorRate, prevCostPerReq float64
		if previous.TotalRequests > 0 {
			prevErrorRate = float64(previous.TotalErrors) / float64(previous.TotalRequests) * 100
			prevCostPerReq = previous.TotalCost / float64(previous.TotalRequests)
		}
		resp.ErrorRateChange = percentChange(prevErrorRate, resp.ErrorRate)
		resp.CostPerReqChange = percentChange(prevCostPerReq, resp.AvgCostPerReq)
	}

	// Cache the response
	data, _ := json.Marshal(map[string]any{
		"data": resp,
		"meta": map[string]any{
			"request_id": w.Header().Get("X-Request-Id"),
			"timestamp":  time.Now().UTC().Format(time.RFC3339),
		},
	})
	if cacheErr := h.cache.SetCached(r.Context(), key, data, 60*time.Second); cacheErr != nil {
		h.logger.Warn("overview: cache set error", "error", cacheErr)
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Cache", "MISS")
	w.Write(data)
}

// percentChange calculates the percent change from old to new.
// Returns nil if old is zero (no comparison possible).
func percentChange(old, new float64) *float64 {
	if old == 0 {
		if new == 0 {
			zero := 0.0
			return &zero
		}
		return nil
	}
	change := (new - old) / old * 100
	return &change
}
