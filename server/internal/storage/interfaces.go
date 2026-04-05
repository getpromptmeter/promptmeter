// Package storage defines interfaces for all data stores used by Promptmeter.
package storage

import (
	"context"
	"time"

	"github.com/promptmeter/promptmeter/server/internal/domain"
)

// APIKeyStore provides lookup of API keys for auth middleware.
type APIKeyStore interface {
	GetAPIKeyByHash(ctx context.Context, keyHash string) (*domain.APIKey, error)
}

// APIKeyCache provides Redis-backed caching for API key lookups.
type APIKeyCache interface {
	GetCachedAPIKey(ctx context.Context, keyHash string) (*domain.APIKey, error)
	SetCachedAPIKey(ctx context.Context, keyHash string, key *domain.APIKey, ttl time.Duration) error
	DeleteCachedAPIKey(ctx context.Context, keyHash string) error
}

// RateLimiter provides sliding window rate limiting via Redis.
type RateLimiter interface {
	// AllowIP checks the per-IP rate limit. Returns whether the request is allowed,
	// the remaining count, and the reset time.
	AllowIP(ctx context.Context, ip string, limit int, window time.Duration) (allowed bool, remaining int, resetAt time.Time, err error)
	// AllowOrg checks the per-org rate limit.
	AllowOrg(ctx context.Context, orgID string, limit int, window time.Duration) (allowed bool, remaining int, resetAt time.Time, err error)
}

// EventWriter provides batch insertion of events into ClickHouse.
type EventWriter interface {
	InsertEvents(ctx context.Context, events []domain.Event) error
}

// PriceStore provides access to model prices from PostgreSQL.
type PriceStore interface {
	GetAllPrices(ctx context.Context) ([]domain.ModelPrice, error)
}

// ObjectStore provides S3-compatible object storage for prompt/response text.
type ObjectStore interface {
	Upload(ctx context.Context, key string, data []byte) error
}

// PendingEventsStore provides access to events with pending S3 uploads
// for the reconciler.
type PendingEventsStore interface {
	GetPendingS3Events(ctx context.Context, limit int) ([]domain.Event, error)
	UpdateS3Status(ctx context.Context, eventID string, status string, s3Key string) error
}

// UserStore provides user CRUD operations for auth.
type UserStore interface {
	// CreateOrGetUser finds a user by email or creates a new one.
	// Returns the user and whether it was newly created.
	CreateOrGetUser(ctx context.Context, email, name, avatarURL string) (*domain.User, bool, error)
	// GetUserByID returns a user by ID.
	GetUserByID(ctx context.Context, userID string) (*domain.User, error)
}

// RefreshTokenStore provides refresh token operations.
type RefreshTokenStore interface {
	// CreateRefreshToken stores a new refresh token hash.
	CreateRefreshToken(ctx context.Context, userID, tokenHash string, expiresAt time.Time) (*domain.RefreshToken, error)
	// GetRefreshTokenByHash looks up a refresh token by its SHA-256 hash.
	GetRefreshTokenByHash(ctx context.Context, tokenHash string) (*domain.RefreshToken, error)
	// RevokeRefreshToken marks a token as revoked.
	RevokeRefreshToken(ctx context.Context, tokenID string) error
	// RevokeAllUserTokens revokes all refresh tokens for a user (reuse detection).
	RevokeAllUserTokens(ctx context.Context, userID string) error
}

// OrgStore provides organization CRUD operations.
type OrgStore interface {
	// GetOrg returns the organization for the given ID.
	GetOrg(ctx context.Context, orgID string) (*domain.Organization, error)
	// UpdateOrg updates mutable organization fields (name, timezone, pii, slack).
	UpdateOrg(ctx context.Context, org *domain.Organization) error
	// GetOrgByUserID returns the organization for a user (via org_members).
	GetOrgByUserID(ctx context.Context, userID string) (*domain.Organization, error)
	// CreateOrg creates a new organization.
	CreateOrg(ctx context.Context, name, slug string) (*domain.Organization, error)
	// AddOrgMember adds a user to an organization with the given role.
	AddOrgMember(ctx context.Context, orgID, userID, role string) error
	// CountOrgs returns the total number of organizations.
	CountOrgs(ctx context.Context) (int, error)
}

// ProjectStore provides project read operations (CRUD is Wave 2).
type ProjectStore interface {
	// ListProjects returns all active projects for an organization.
	ListProjects(ctx context.Context, orgID string) ([]domain.Project, error)
	// CreateProject creates a new project.
	CreateProject(ctx context.Context, project *domain.Project) error
}

// APIKeyManager provides full API key CRUD (extends APIKeyStore for dashboard).
type APIKeyManager interface {
	APIKeyStore
	// CreateAPIKey creates a new API key in PostgreSQL.
	CreateAPIKey(ctx context.Context, orgID, name, keyPrefix, keyHash string, scopes []string, projectID *string) (*domain.APIKey, error)
	// ListAPIKeys returns all API keys for an organization (no full key, only prefix).
	ListAPIKeys(ctx context.Context, orgID string) ([]domain.APIKey, error)
	// RevokeAPIKey sets revoked_at on an API key.
	RevokeAPIKey(ctx context.Context, orgID, keyID string) error
}

// DashboardQueryParams holds common parameters for dashboard queries.
type DashboardQueryParams struct {
	OrgID     uint64
	ProjectID string
	From      time.Time
	To        time.Time
	Timezone  string
}

// DashboardReader provides read-only access to ClickHouse for dashboard queries.
type DashboardReader interface {
	// GetOverviewKPIs returns aggregated KPIs for a time period.
	GetOverviewKPIs(ctx context.Context, params DashboardQueryParams) (*domain.OverviewKPIs, error)
	// GetCostBreakdown returns cost breakdown by model or feature.
	GetCostBreakdown(ctx context.Context, params DashboardQueryParams, groupBy string, limit int) ([]domain.CostBreakdownItem, error)
	// GetCostTimeseries returns cost timeseries data, optionally split by model.
	GetCostTimeseries(ctx context.Context, params DashboardQueryParams, groupBy string, granularity string) ([]domain.TimeseriesSeries, error)
}

// DashboardCache provides caching for dashboard query results.
type DashboardCache interface {
	// GetCached retrieves a cached value by key. Returns nil if not found.
	GetCached(ctx context.Context, key string) ([]byte, error)
	// SetCached stores a value with the given TTL.
	SetCached(ctx context.Context, key string, data []byte, ttl time.Duration) error
}
