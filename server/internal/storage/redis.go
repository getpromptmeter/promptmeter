package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/promptmeter/promptmeter/server/internal/domain"
)

// RedisStore implements APIKeyCache and RateLimiter using Redis.
type RedisStore struct {
	client *redis.Client
}

// NewRedisStore creates a new Redis store.
func NewRedisStore(ctx context.Context, url string) (*RedisStore, error) {
	opts, err := redis.ParseURL(url)
	if err != nil {
		return nil, fmt.Errorf("redis: parse url: %w", err)
	}

	client := redis.NewClient(opts)
	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("redis: ping: %w", err)
	}

	return &RedisStore{client: client}, nil
}

// Close closes the Redis connection.
func (s *RedisStore) Close() error {
	return s.client.Close()
}

// --- APIKeyCache implementation ---

func apiKeyCacheKey(keyHash string) string {
	return "apikey:" + keyHash
}

// cachedAPIKey is the JSON-serializable form stored in Redis.
type cachedAPIKey struct {
	ID        string        `json:"id"`
	OrgID     string        `json:"org_id"`
	KeyPrefix string        `json:"key_prefix"`
	Name      string        `json:"name"`
	Scopes    []string      `json:"scopes"`
	Tier      domain.OrgTier `json:"tier"`
	RevokedAt *time.Time    `json:"revoked_at,omitempty"`
}

// GetCachedAPIKey retrieves an API key from Redis cache. Returns nil if not found.
func (s *RedisStore) GetCachedAPIKey(ctx context.Context, keyHash string) (*domain.APIKey, error) {
	val, err := s.client.Get(ctx, apiKeyCacheKey(keyHash)).Result()
	if err == redis.Nil {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("redis: get cached api key: %w", err)
	}

	var cached cachedAPIKey
	if err := json.Unmarshal([]byte(val), &cached); err != nil {
		return nil, fmt.Errorf("redis: unmarshal cached api key: %w", err)
	}

	return &domain.APIKey{
		ID:        cached.ID,
		OrgID:     cached.OrgID,
		KeyPrefix: cached.KeyPrefix,
		KeyHash:   keyHash,
		Name:      cached.Name,
		Scopes:    cached.Scopes,
		Tier:      cached.Tier,
		RevokedAt: cached.RevokedAt,
	}, nil
}

// SetCachedAPIKey stores an API key in Redis cache with the given TTL.
func (s *RedisStore) SetCachedAPIKey(ctx context.Context, keyHash string, key *domain.APIKey, ttl time.Duration) error {
	cached := cachedAPIKey{
		ID:        key.ID,
		OrgID:     key.OrgID,
		KeyPrefix: key.KeyPrefix,
		Name:      key.Name,
		Scopes:    key.Scopes,
		Tier:      key.Tier,
		RevokedAt: key.RevokedAt,
	}
	data, err := json.Marshal(cached)
	if err != nil {
		return fmt.Errorf("redis: marshal api key: %w", err)
	}
	return s.client.Set(ctx, apiKeyCacheKey(keyHash), data, ttl).Err()
}

// DeleteCachedAPIKey removes an API key from Redis cache.
func (s *RedisStore) DeleteCachedAPIKey(ctx context.Context, keyHash string) error {
	return s.client.Del(ctx, apiKeyCacheKey(keyHash)).Err()
}

// --- RateLimiter implementation ---

// AllowIP checks the per-IP rate limit using a sliding window counter.
func (s *RedisStore) AllowIP(ctx context.Context, ip string, limit int, window time.Duration) (bool, int, time.Time, error) {
	key := "rl:ip:" + ip
	return s.checkRateLimit(ctx, key, limit, window)
}

// AllowOrg checks the per-org rate limit using a sliding window counter.
func (s *RedisStore) AllowOrg(ctx context.Context, orgID string, limit int, window time.Duration) (bool, int, time.Time, error) {
	key := "rl:org:" + orgID + ":ingestion"
	return s.checkRateLimit(ctx, key, limit, window)
}

func (s *RedisStore) checkRateLimit(ctx context.Context, key string, limit int, window time.Duration) (bool, int, time.Time, error) {
	pipe := s.client.Pipeline()
	incrCmd := pipe.Incr(ctx, key)
	pipe.Expire(ctx, key, window)

	_, err := pipe.Exec(ctx)
	if err != nil {
		return false, 0, time.Time{}, fmt.Errorf("redis: rate limit: %w", err)
	}

	count := int(incrCmd.Val())
	remaining := limit - count
	if remaining < 0 {
		remaining = 0
	}
	resetAt := time.Now().Add(window)

	return count <= limit, remaining, resetAt, nil
}

// Client returns the underlying Redis client for tests.
func (s *RedisStore) Client() *redis.Client {
	return s.client
}

// --- DashboardCache implementation ---

// GetCached retrieves a cached value by key. Returns nil if not found.
func (s *RedisStore) GetCached(ctx context.Context, key string) ([]byte, error) {
	val, err := s.client.Get(ctx, key).Bytes()
	if err == redis.Nil {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("redis: get cached: %w", err)
	}
	return val, nil
}

// SetCached stores a value with the given TTL.
func (s *RedisStore) SetCached(ctx context.Context, key string, data []byte, ttl time.Duration) error {
	return s.client.Set(ctx, key, data, ttl).Err()
}
