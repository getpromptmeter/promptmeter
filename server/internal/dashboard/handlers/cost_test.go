package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/promptmeter/promptmeter/server/internal/domain"
	"github.com/promptmeter/promptmeter/server/internal/middleware"
	"github.com/promptmeter/promptmeter/server/internal/storage"
)

// --- Mock implementations ---

type mockDashboardReader struct {
	breakdownFn  func(ctx context.Context, params storage.DashboardQueryParams, groupBy string, limit int) ([]domain.CostBreakdownItem, error)
	timeseriesFn func(ctx context.Context, params storage.DashboardQueryParams, groupBy string, granularity string) ([]domain.TimeseriesSeries, error)
	compareFn    func(ctx context.Context, current, previous storage.DashboardQueryParams, groupBy string, limit int) (*domain.CostCompareResponse, error)
	filtersFn    func(ctx context.Context, params storage.DashboardQueryParams) (*domain.CostFiltersResponse, error)
}

func (m *mockDashboardReader) GetOverviewKPIs(ctx context.Context, params storage.DashboardQueryParams) (*domain.OverviewKPIs, error) {
	return &domain.OverviewKPIs{}, nil
}

func (m *mockDashboardReader) GetCostBreakdown(ctx context.Context, params storage.DashboardQueryParams, groupBy string, limit int) ([]domain.CostBreakdownItem, error) {
	if m.breakdownFn != nil {
		return m.breakdownFn(ctx, params, groupBy, limit)
	}
	return nil, nil
}

func (m *mockDashboardReader) GetCostTimeseries(ctx context.Context, params storage.DashboardQueryParams, groupBy string, granularity string) ([]domain.TimeseriesSeries, error) {
	if m.timeseriesFn != nil {
		return m.timeseriesFn(ctx, params, groupBy, granularity)
	}
	return nil, nil
}

func (m *mockDashboardReader) GetCostCompare(ctx context.Context, current, previous storage.DashboardQueryParams, groupBy string, limit int) (*domain.CostCompareResponse, error) {
	if m.compareFn != nil {
		return m.compareFn(ctx, current, previous, groupBy, limit)
	}
	return &domain.CostCompareResponse{}, nil
}

func (m *mockDashboardReader) GetCostFilters(ctx context.Context, params storage.DashboardQueryParams) (*domain.CostFiltersResponse, error) {
	if m.filtersFn != nil {
		return m.filtersFn(ctx, params)
	}
	return &domain.CostFiltersResponse{Models: []string{}, Providers: []string{}, Features: []string{}}, nil
}

type mockCache struct{}

func (m *mockCache) GetCached(ctx context.Context, key string) ([]byte, error) { return nil, nil }
func (m *mockCache) SetCached(ctx context.Context, key string, data []byte, ttl time.Duration) error {
	return nil
}

func setAuthContext(r *http.Request) *http.Request {
	ctx := context.WithValue(r.Context(), middleware.CtxOrgID, "test-org-id")
	ctx = context.WithValue(ctx, middleware.CtxOrgNum, uint64(123))
	return r.WithContext(ctx)
}

// --- Tests ---

func TestHandleCostBreakdown_WithFilters(t *testing.T) {
	var capturedParams storage.DashboardQueryParams
	var capturedGroupBy string

	reader := &mockDashboardReader{
		breakdownFn: func(ctx context.Context, params storage.DashboardQueryParams, groupBy string, limit int) ([]domain.CostBreakdownItem, error) {
			capturedParams = params
			capturedGroupBy = groupBy
			return []domain.CostBreakdownItem{
				{Group: "chat-support", CostUSD: 100.0, Requests: 500},
				{Group: "search", CostUSD: 50.0, Requests: 200},
			}, nil
		},
	}

	handler := NewCostHandler(reader, &mockCache{}, nil)

	req := httptest.NewRequest("GET", "/api/v1/dashboard/cost?group_by=feature&model=gpt-4o&provider=openai&period=7d", nil)
	req = setAuthContext(req)
	w := httptest.NewRecorder()

	handler.HandleCostBreakdown(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	if capturedParams.ModelFilter != "gpt-4o" {
		t.Errorf("expected model filter 'gpt-4o', got %q", capturedParams.ModelFilter)
	}
	if capturedParams.ProviderFilter != "openai" {
		t.Errorf("expected provider filter 'openai', got %q", capturedParams.ProviderFilter)
	}
	if capturedGroupBy != "feature" {
		t.Errorf("expected group_by 'feature', got %q", capturedGroupBy)
	}

	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	meta := resp["meta"].(map[string]any)
	filters := meta["filters"].(map[string]any)
	if filters["model"] != "gpt-4o" {
		t.Errorf("expected meta filters to contain model=gpt-4o")
	}
}

func TestHandleCostBreakdown_CrossDimensional(t *testing.T) {
	var capturedParams storage.DashboardQueryParams

	reader := &mockDashboardReader{
		breakdownFn: func(ctx context.Context, params storage.DashboardQueryParams, groupBy string, limit int) ([]domain.CostBreakdownItem, error) {
			capturedParams = params
			return []domain.CostBreakdownItem{
				{Group: "chat-support", CostUSD: 1200.30, Requests: 52100},
			}, nil
		},
	}

	handler := NewCostHandler(reader, &mockCache{}, nil)

	// group_by=feature with model filter = cross-dimensional
	req := httptest.NewRequest("GET", "/api/v1/dashboard/cost?group_by=feature&model=gpt-4o&period=7d", nil)
	req = setAuthContext(req)
	w := httptest.NewRecorder()

	handler.HandleCostBreakdown(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	if capturedParams.ModelFilter != "gpt-4o" {
		t.Errorf("expected model filter passed to storage")
	}
	if capturedParams.FeatureFilter != "" {
		t.Errorf("expected no feature filter, got %q", capturedParams.FeatureFilter)
	}
}

func TestHandleCostTimeseries_GroupByFeature(t *testing.T) {
	var capturedGroupBy string

	reader := &mockDashboardReader{
		timeseriesFn: func(ctx context.Context, params storage.DashboardQueryParams, groupBy string, granularity string) ([]domain.TimeseriesSeries, error) {
			capturedGroupBy = groupBy
			return []domain.TimeseriesSeries{
				{Group: "chat", Points: []domain.TimeseriesPoint{{Timestamp: time.Now(), CostUSD: 10, Requests: 5}}},
			}, nil
		},
	}

	handler := NewCostHandler(reader, &mockCache{}, nil)

	req := httptest.NewRequest("GET", "/api/v1/dashboard/cost/timeseries?group_by=feature&period=7d", nil)
	req = setAuthContext(req)
	w := httptest.NewRecorder()

	handler.HandleCostTimeseries(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	if capturedGroupBy != "feature" {
		t.Errorf("expected group_by 'feature', got %q", capturedGroupBy)
	}
}

func TestHandleCostTimeseries_InvalidGroupBy(t *testing.T) {
	handler := NewCostHandler(&mockDashboardReader{}, &mockCache{}, nil)

	req := httptest.NewRequest("GET", "/api/v1/dashboard/cost/timeseries?group_by=project&period=7d", nil)
	req = setAuthContext(req)
	w := httptest.NewRecorder()

	handler.HandleCostTimeseries(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestHandleCostCompare_HappyPath(t *testing.T) {
	reader := &mockDashboardReader{
		compareFn: func(ctx context.Context, current, previous storage.DashboardQueryParams, groupBy string, limit int) (*domain.CostCompareResponse, error) {
			return &domain.CostCompareResponse{
				Current:  domain.CostComparePeriod{TotalCost: 2000, Requests: 1000},
				Previous: domain.CostComparePeriod{TotalCost: 1500, Requests: 800},
				Changes: domain.CostCompareChanges{
					CostDelta: 500,
				},
				Breakdown: []domain.CostCompareBreakdownItem{
					{Group: "gpt-4o", CurrentCost: 1500, PreviousCost: 1200, Requests: 700},
				},
			}, nil
		},
	}

	handler := NewCostHandler(reader, &mockCache{}, nil)

	req := httptest.NewRequest("GET",
		"/api/v1/dashboard/cost/compare?current_from=2026-04-01T00:00:00Z&current_to=2026-04-08T00:00:00Z&previous_from=2026-03-25T00:00:00Z&previous_to=2026-04-01T00:00:00Z&group_by=model",
		nil)
	req = setAuthContext(req)
	w := httptest.NewRecorder()

	handler.HandleCostCompare(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	data := resp["data"].(map[string]any)
	current := data["current"].(map[string]any)
	if current["total_cost"].(float64) != 2000 {
		t.Errorf("expected current total_cost 2000")
	}
}

func TestHandleCostCompare_DifferentPeriodLengths(t *testing.T) {
	handler := NewCostHandler(&mockDashboardReader{}, &mockCache{}, nil)

	// 7 days current, 3 days previous
	req := httptest.NewRequest("GET",
		"/api/v1/dashboard/cost/compare?current_from=2026-04-01T00:00:00Z&current_to=2026-04-08T00:00:00Z&previous_from=2026-03-29T00:00:00Z&previous_to=2026-04-01T00:00:00Z",
		nil)
	req = setAuthContext(req)
	w := httptest.NewRecorder()

	handler.HandleCostCompare(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]any
	json.Unmarshal(w.Body.Bytes(), &resp)
	errObj := resp["error"].(map[string]any)
	if errObj["code"] != "VALIDATION_ERROR" {
		t.Errorf("expected VALIDATION_ERROR code")
	}
}

func TestHandleCostCompare_MissingDates(t *testing.T) {
	handler := NewCostHandler(&mockDashboardReader{}, &mockCache{}, nil)

	req := httptest.NewRequest("GET", "/api/v1/dashboard/cost/compare?current_from=2026-04-01T00:00:00Z", nil)
	req = setAuthContext(req)
	w := httptest.NewRecorder()

	handler.HandleCostCompare(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestHandleCostFilters(t *testing.T) {
	reader := &mockDashboardReader{
		filtersFn: func(ctx context.Context, params storage.DashboardQueryParams) (*domain.CostFiltersResponse, error) {
			return &domain.CostFiltersResponse{
				Models:    []string{"gpt-4o", "gpt-4o-mini"},
				Providers: []string{"openai"},
				Features:  []string{"chat-support", "search"},
			}, nil
		},
	}

	handler := NewCostHandler(reader, &mockCache{}, nil)

	req := httptest.NewRequest("GET", "/api/v1/dashboard/cost/filters?period=7d", nil)
	req = setAuthContext(req)
	w := httptest.NewRecorder()

	handler.HandleCostFilters(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]any
	json.Unmarshal(w.Body.Bytes(), &resp)
	data := resp["data"].(map[string]any)

	models := data["models"].([]any)
	if len(models) != 2 {
		t.Errorf("expected 2 models, got %d", len(models))
	}
	features := data["features"].([]any)
	if len(features) != 2 {
		t.Errorf("expected 2 features, got %d", len(features))
	}
}

func TestHandleCostBreakdown_EmptyResult(t *testing.T) {
	reader := &mockDashboardReader{
		breakdownFn: func(ctx context.Context, params storage.DashboardQueryParams, groupBy string, limit int) ([]domain.CostBreakdownItem, error) {
			return nil, nil
		},
	}

	handler := NewCostHandler(reader, &mockCache{}, nil)

	req := httptest.NewRequest("GET", "/api/v1/dashboard/cost?period=7d", nil)
	req = setAuthContext(req)
	w := httptest.NewRecorder()

	handler.HandleCostBreakdown(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestCacheKey_IncludesFilters(t *testing.T) {
	key1 := cacheKey("org1", "cost", "7d", "", "model", "", "", "", "UTC")
	key2 := cacheKey("org1", "cost", "7d", "", "model", "gpt-4o", "", "", "UTC")
	key3 := cacheKey("org1", "cost", "7d", "", "model", "gpt-4o", "openai", "", "UTC")

	if key1 == key2 {
		t.Error("cache keys should differ when model filter changes")
	}
	if key2 == key3 {
		t.Error("cache keys should differ when provider filter changes")
	}
}
