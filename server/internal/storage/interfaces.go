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
