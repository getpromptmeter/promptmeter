package domain

import (
	"encoding/binary"
	"time"
)

// OrgIDToUint64 converts a UUID string org_id to a uint64 for ClickHouse.
// Uses the first 8 bytes of the UUID parsed as big-endian uint64.
func OrgIDToUint64(orgID string) uint64 {
	// Parse UUID hex digits (strip dashes): xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx
	cleaned := make([]byte, 0, 32)
	for _, c := range orgID {
		if c != '-' {
			cleaned = append(cleaned, byte(c))
		}
	}
	if len(cleaned) < 16 {
		return 0
	}
	// Parse first 8 hex bytes (16 hex chars) as uint64
	var buf [8]byte
	for i := 0; i < 8; i++ {
		buf[i] = hexByte(cleaned[i*2], cleaned[i*2+1])
	}
	return binary.BigEndian.Uint64(buf[:])
}

func hexByte(hi, lo byte) byte {
	return hexVal(hi)<<4 | hexVal(lo)
}

func hexVal(b byte) byte {
	switch {
	case b >= '0' && b <= '9':
		return b - '0'
	case b >= 'a' && b <= 'f':
		return b - 'a' + 10
	case b >= 'A' && b <= 'F':
		return b - 'A' + 10
	default:
		return 0
	}
}

// Organization represents a tenant in the system.
type Organization struct {
	ID                   string    `json:"id"`
	Name                 string    `json:"name"`
	Slug                 string    `json:"slug"`
	Tier                 OrgTier   `json:"tier"`
	Timezone             string    `json:"timezone"`
	PIIEnabled           bool      `json:"pii_enabled"`
	SlackWebhookURL      *string   `json:"slack_webhook_url,omitempty"`
	StripeCustomerID     *string   `json:"stripe_customer_id,omitempty"`
	StripeSubscriptionID *string   `json:"stripe_subscription_id,omitempty"`
	CreatedAt            time.Time `json:"created_at"`
	UpdatedAt            time.Time `json:"updated_at"`
}

// OrgTier represents the subscription tier of an organization.
type OrgTier string

const (
	TierFree       OrgTier = "free"
	TierPro        OrgTier = "pro"
	TierBusiness   OrgTier = "business"
	TierEnterprise OrgTier = "enterprise"
)

// RateLimitForTier returns the per-org rate limit (requests per second)
// for ingestion based on the organization tier.
func RateLimitForTier(tier OrgTier) int {
	switch tier {
	case TierFree:
		return 100
	case TierPro:
		return 500
	case TierBusiness:
		return 2000
	case TierEnterprise:
		return 2000
	default:
		return 100
	}
}

// APIKey represents an SDK API key stored in PostgreSQL.
type APIKey struct {
	ID         string     `json:"id"`
	OrgID      string     `json:"org_id"`
	ProjectID  *string    `json:"project_id,omitempty"`
	KeyPrefix  string     `json:"key_prefix"`
	KeyHash    string     `json:"-"`
	Name       string     `json:"name"`
	Scopes     []string   `json:"scopes"`
	LastUsedAt *time.Time `json:"last_used_at,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
	RevokedAt  *time.Time `json:"revoked_at,omitempty"`
	Tier       OrgTier    `json:"tier"`
}

// HasScope returns true if the API key has the given scope.
func (k *APIKey) HasScope(scope string) bool {
	for _, s := range k.Scopes {
		if s == scope {
			return true
		}
	}
	return false
}

// IsRevoked returns true if the API key has been revoked.
func (k *APIKey) IsRevoked() bool {
	return k.RevokedAt != nil
}

// User represents a dashboard user stored in PostgreSQL.
type User struct {
	ID        string    `json:"id"`
	Email     string    `json:"email"`
	Name      string    `json:"name"`
	AvatarURL string    `json:"avatar_url,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

// RefreshToken represents a refresh token stored in PostgreSQL.
type RefreshToken struct {
	ID        string     `json:"id"`
	UserID    string     `json:"user_id"`
	TokenHash string     `json:"-"`
	ExpiresAt time.Time  `json:"expires_at"`
	RevokedAt *time.Time `json:"revoked_at,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
}

// Project represents a logical grouping within an organization.
type Project struct {
	ID          string     `json:"id"`
	OrgID       string     `json:"org_id"`
	Name        string     `json:"name"`
	Slug        string     `json:"slug"`
	Description string     `json:"description"`
	PIIEnabled  bool       `json:"pii_enabled"`
	IsDefault   bool       `json:"is_default"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
	ArchivedAt  *time.Time `json:"archived_at,omitempty"`
}

// DashboardRateLimitForTier returns per-org rate limit (requests per minute)
// for the Dashboard API based on the organization tier.
func DashboardRateLimitForTier(tier OrgTier) int {
	switch tier {
	case TierFree:
		return 60
	case TierPro:
		return 300
	case TierBusiness:
		return 1000
	case TierEnterprise:
		return 1000
	default:
		return 60
	}
}

// OverviewKPIs holds aggregated KPI data for the dashboard overview.
type OverviewKPIs struct {
	TotalCost     float64 `json:"total_cost"`
	TotalRequests uint64  `json:"total_requests"`
	TotalErrors   uint64  `json:"total_errors"`
	AvgLatencyMs  float64 `json:"avg_latency_ms"`
}

// CostBreakdownItem represents a single row in a cost breakdown.
type CostBreakdownItem struct {
	Group          string  `json:"group"`
	Provider       string  `json:"provider,omitempty"`
	CostUSD        float64 `json:"cost_usd"`
	Requests       uint64  `json:"requests"`
	Tokens         uint64  `json:"tokens,omitempty"`
	PercentOfTotal float64 `json:"percent_of_total"`
}

// TimeseriesPoint represents a single data point on a timeseries chart.
type TimeseriesPoint struct {
	Timestamp time.Time `json:"timestamp"`
	CostUSD   float64   `json:"cost_usd"`
	Requests  uint64    `json:"requests"`
}

// TimeseriesSeries represents a named series of timeseries data points.
type TimeseriesSeries struct {
	Group  string            `json:"group"`
	Points []TimeseriesPoint `json:"points"`
}

// CostCompareResponse holds the result of comparing two time periods.
type CostCompareResponse struct {
	Current   CostComparePeriod          `json:"current"`
	Previous  CostComparePeriod          `json:"previous"`
	Changes   CostCompareChanges         `json:"changes"`
	Breakdown []CostCompareBreakdownItem `json:"breakdown"`
}

// CostComparePeriod holds aggregate metrics for a single period.
type CostComparePeriod struct {
	TotalCost float64 `json:"total_cost"`
	Requests  uint64  `json:"requests"`
}

// CostCompareChanges holds computed deltas between two periods.
type CostCompareChanges struct {
	CostDelta      float64  `json:"cost_delta"`
	CostPercent    *float64 `json:"cost_percent"`
	RequestPercent *float64 `json:"request_percent"`
}

// CostCompareBreakdownItem represents a single row in a compare breakdown.
type CostCompareBreakdownItem struct {
	Group        string   `json:"group"`
	CurrentCost  float64  `json:"current_cost"`
	PreviousCost float64  `json:"previous_cost"`
	CostChange   *float64 `json:"cost_change"`
	Requests     uint64   `json:"requests"`
}

// CostFiltersResponse holds available filter values for dropdowns.
type CostFiltersResponse struct {
	Models    []string `json:"models"`
	Providers []string `json:"providers"`
	Features  []string `json:"features"`
}

// ModelPrice represents a model pricing entry from PostgreSQL.
type ModelPrice struct {
	ID                    string    `json:"id"`
	Provider              string    `json:"provider"`
	ModelName             string    `json:"model_name"`
	InputPricePerMillion  float64   `json:"input_price_per_million"`
	OutputPricePerMillion float64   `json:"output_price_per_million"`
	EffectiveFrom         time.Time `json:"effective_from"`
	CreatedAt             time.Time `json:"created_at"`
}
