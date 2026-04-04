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
	ID                   string   `json:"id"`
	Name                 string   `json:"name"`
	Slug                 string   `json:"slug"`
	Tier                 OrgTier  `json:"tier"`
	StripeCustomerID     *string  `json:"stripe_customer_id,omitempty"`
	StripeSubscriptionID *string  `json:"stripe_subscription_id,omitempty"`
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
	ID         string    `json:"id"`
	OrgID      string    `json:"org_id"`
	KeyPrefix  string    `json:"key_prefix"`
	KeyHash    string    `json:"key_hash"`
	Name       string    `json:"name"`
	Scopes     []string  `json:"scopes"`
	LastUsedAt *time.Time `json:"last_used_at,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
	RevokedAt  *time.Time `json:"revoked_at,omitempty"`
	Tier       OrgTier   `json:"tier"`
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
