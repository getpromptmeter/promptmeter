// Package handlers provides HTTP handlers for the Dashboard API.
package handlers

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"time"

	"github.com/promptmeter/promptmeter/server/internal/middleware"
	"github.com/promptmeter/promptmeter/server/internal/storage"
)

// validPeriods maps period strings to their durations.
var validPeriods = map[string]time.Duration{
	"1d":  24 * time.Hour,
	"7d":  7 * 24 * time.Hour,
	"30d": 30 * 24 * time.Hour,
	"90d": 90 * 24 * time.Hour,
}

// validGroupBy lists allowed group_by values.
var validGroupBy = map[string]bool{
	"model":   true,
	"feature": true,
	"project": true,
}

// granularityForPeriod returns the auto-selected granularity for a period.
func granularityForPeriod(period string) string {
	switch period {
	case "1d":
		return "hour"
	case "7d", "30d":
		return "day"
	case "90d":
		return "week"
	default:
		return "day"
	}
}

// parseDashboardParams extracts common query parameters from the request.
func parseDashboardParams(r *http.Request) (storage.DashboardQueryParams, string, error) {
	period := r.URL.Query().Get("period")
	if period == "" {
		period = "7d"
	}
	dur, ok := validPeriods[period]
	if !ok {
		return storage.DashboardQueryParams{}, "", fmt.Errorf("invalid period: %s", period)
	}

	tz := r.URL.Query().Get("timezone")
	if tz == "" {
		tz = "UTC"
	}
	loc, err := time.LoadLocation(tz)
	if err != nil {
		return storage.DashboardQueryParams{}, "", fmt.Errorf("invalid timezone: %s", tz)
	}

	now := time.Now().In(loc)
	to := now
	from := now.Add(-dur)

	projectID := r.URL.Query().Get("project")

	orgNumeric := middleware.OrgNumericFromContext(r.Context())

	return storage.DashboardQueryParams{
		OrgID:     orgNumeric,
		ProjectID: projectID,
		From:      from,
		To:        to,
		Timezone:  tz,
	}, period, nil
}

// cacheKey generates a deterministic cache key for dashboard queries.
func cacheKey(orgID string, endpoint string, params ...string) string {
	h := sha256.New()
	for _, p := range params {
		h.Write([]byte(p))
	}
	hash := hex.EncodeToString(h.Sum(nil))[:12]
	return fmt.Sprintf("dashboard:%s:%s:%s", orgID, endpoint, hash)
}
