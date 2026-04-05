package handlers

import (
	"context"
	"net/http/httptest"
	"testing"

	"github.com/promptmeter/promptmeter/server/internal/middleware"
)

func TestParseDashboardParams_DefaultPeriod(t *testing.T) {
	r := httptest.NewRequest("GET", "/api/v1/dashboard/overview", nil)
	ctx := context.WithValue(r.Context(), middleware.CtxOrgNum, uint64(12345))
	r = r.WithContext(ctx)

	params, period, err := parseDashboardParams(r)
	if err != nil {
		t.Fatalf("parseDashboardParams: %v", err)
	}

	if period != "7d" {
		t.Errorf("period = %q, want %q", period, "7d")
	}

	if params.OrgID != 12345 {
		t.Errorf("OrgID = %d, want %d", params.OrgID, 12345)
	}

	if params.Timezone != "UTC" {
		t.Errorf("Timezone = %q, want %q", params.Timezone, "UTC")
	}
}

func TestParseDashboardParams_WithParams(t *testing.T) {
	r := httptest.NewRequest("GET", "/api/v1/dashboard/overview?period=30d&timezone=America/New_York&project=my-project", nil)
	ctx := context.WithValue(r.Context(), middleware.CtxOrgNum, uint64(12345))
	r = r.WithContext(ctx)

	params, period, err := parseDashboardParams(r)
	if err != nil {
		t.Fatalf("parseDashboardParams: %v", err)
	}

	if period != "30d" {
		t.Errorf("period = %q, want %q", period, "30d")
	}

	if params.ProjectID != "my-project" {
		t.Errorf("ProjectID = %q, want %q", params.ProjectID, "my-project")
	}

	if params.Timezone != "America/New_York" {
		t.Errorf("Timezone = %q, want %q", params.Timezone, "America/New_York")
	}
}

func TestParseDashboardParams_InvalidPeriod(t *testing.T) {
	r := httptest.NewRequest("GET", "/api/v1/dashboard/overview?period=999d", nil)
	ctx := context.WithValue(r.Context(), middleware.CtxOrgNum, uint64(12345))
	r = r.WithContext(ctx)

	_, _, err := parseDashboardParams(r)
	if err == nil {
		t.Error("expected error for invalid period")
	}
}

func TestParseDashboardParams_InvalidTimezone(t *testing.T) {
	r := httptest.NewRequest("GET", "/api/v1/dashboard/overview?timezone=NotATimezone", nil)
	ctx := context.WithValue(r.Context(), middleware.CtxOrgNum, uint64(12345))
	r = r.WithContext(ctx)

	_, _, err := parseDashboardParams(r)
	if err == nil {
		t.Error("expected error for invalid timezone")
	}
}

func TestGranularityForPeriod(t *testing.T) {
	tests := []struct {
		period string
		want   string
	}{
		{"1d", "hour"},
		{"7d", "day"},
		{"30d", "day"},
		{"90d", "week"},
	}

	for _, tt := range tests {
		got := granularityForPeriod(tt.period)
		if got != tt.want {
			t.Errorf("granularityForPeriod(%q) = %q, want %q", tt.period, got, tt.want)
		}
	}
}

func TestPercentChange(t *testing.T) {
	tests := []struct {
		old, new float64
		wantNil  bool
		want     float64
	}{
		{100, 120, false, 20.0},
		{100, 80, false, -20.0},
		{0, 0, false, 0.0},
		{0, 100, true, 0},
	}

	for _, tt := range tests {
		got := percentChange(tt.old, tt.new)
		if tt.wantNil {
			if got != nil {
				t.Errorf("percentChange(%v, %v) = %v, want nil", tt.old, tt.new, *got)
			}
			continue
		}
		if got == nil {
			t.Errorf("percentChange(%v, %v) = nil, want %v", tt.old, tt.new, tt.want)
			continue
		}
		if *got != tt.want {
			t.Errorf("percentChange(%v, %v) = %v, want %v", tt.old, tt.new, *got, tt.want)
		}
	}
}
