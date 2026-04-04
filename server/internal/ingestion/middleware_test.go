package ingestion

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/promptmeter/promptmeter/server/internal/domain"
)

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stdout, nil))
}

// mockAPIKeyCache is a test double for the Redis API key cache.
type mockAPIKeyCache struct {
	key *domain.APIKey
	err error
}

func (m *mockAPIKeyCache) GetCachedAPIKey(_ context.Context, _ string) (*domain.APIKey, error) {
	return m.key, m.err
}

func (m *mockAPIKeyCache) SetCachedAPIKey(_ context.Context, _ string, _ *domain.APIKey, _ time.Duration) error {
	return nil
}

func (m *mockAPIKeyCache) DeleteCachedAPIKey(_ context.Context, _ string) error {
	return nil
}

// mockAPIKeyStore is a test double for the PostgreSQL API key store.
type mockAPIKeyStore struct {
	key *domain.APIKey
	err error
}

func (m *mockAPIKeyStore) GetAPIKeyByHash(_ context.Context, _ string) (*domain.APIKey, error) {
	return m.key, m.err
}

// mockRateLimiter is a test double for Redis rate limiting.
type mockRateLimiter struct {
	allowed   bool
	remaining int
	resetAt   time.Time
}

func (m *mockRateLimiter) AllowIP(_ context.Context, _ string, _ int, _ time.Duration) (bool, int, time.Time, error) {
	return m.allowed, m.remaining, m.resetAt, nil
}

func (m *mockRateLimiter) AllowOrg(_ context.Context, _ string, _ int, _ time.Duration) (bool, int, time.Time, error) {
	return m.allowed, m.remaining, m.resetAt, nil
}

func TestRequestIDMiddleware(t *testing.T) {
	handler := RequestIDMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqID := RequestIDFromContext(r.Context())
		if reqID == "" {
			t.Error("expected request ID in context")
		}
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Header().Get("X-Request-Id") == "" {
		t.Error("expected X-Request-Id header")
	}
}

func TestIPRateLimitMiddleware_Allowed(t *testing.T) {
	limiter := &mockRateLimiter{allowed: true, remaining: 199, resetAt: time.Now().Add(time.Minute)}
	middleware := IPRateLimitMiddleware(limiter, 200, time.Minute)

	called := false
	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if !called {
		t.Error("handler should have been called")
	}
	if w.Header().Get("X-RateLimit-Limit") != "200" {
		t.Error("expected rate limit header")
	}
}

func TestIPRateLimitMiddleware_Rejected(t *testing.T) {
	limiter := &mockRateLimiter{allowed: false, remaining: 0, resetAt: time.Now().Add(time.Minute)}
	middleware := IPRateLimitMiddleware(limiter, 200, time.Minute)

	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not have been called")
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Result().StatusCode != http.StatusTooManyRequests {
		t.Errorf("expected 429, got %d", w.Result().StatusCode)
	}
}

func TestAuthMiddleware_MissingHeader(t *testing.T) {
	cache := &mockAPIKeyCache{}
	store := &mockAPIKeyStore{}
	logger := testLogger()
	middleware := AuthMiddleware(cache, store, logger)

	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not have been called")
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Result().StatusCode != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Result().StatusCode)
	}
}

func TestAuthMiddleware_InvalidFormat(t *testing.T) {
	cache := &mockAPIKeyCache{}
	store := &mockAPIKeyStore{}
	logger := testLogger()
	middleware := AuthMiddleware(cache, store, logger)

	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not have been called")
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer sk_invalid_key")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Result().StatusCode != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Result().StatusCode)
	}
}

func TestAuthMiddleware_RevokedKey(t *testing.T) {
	revokedAt := time.Now()
	cache := &mockAPIKeyCache{}
	store := &mockAPIKeyStore{
		key: &domain.APIKey{
			ID:        "key-1",
			OrgID:     "550e8400-e29b-41d4-a716-446655440000",
			KeyHash:   "hash",
			Scopes:    []string{"read", "write"},
			Tier:      domain.TierFree,
			RevokedAt: &revokedAt,
		},
	}
	logger := testLogger()
	middleware := AuthMiddleware(cache, store, logger)

	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not have been called for revoked key")
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer pm_test_ABCDEFGHIJKLMNOPQRSTUVWXYZabcdef")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Result().StatusCode != http.StatusUnauthorized {
		t.Errorf("expected 401 for revoked key, got %d", w.Result().StatusCode)
	}
}

func TestAuthMiddleware_MissingWriteScope(t *testing.T) {
	cache := &mockAPIKeyCache{}
	store := &mockAPIKeyStore{
		key: &domain.APIKey{
			ID:     "key-1",
			OrgID:  "550e8400-e29b-41d4-a716-446655440000",
			Scopes: []string{"read"}, // no write scope
			Tier:   domain.TierFree,
		},
	}
	logger := testLogger()
	middleware := AuthMiddleware(cache, store, logger)

	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not have been called without write scope")
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer pm_test_ABCDEFGHIJKLMNOPQRSTUVWXYZabcdef")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Result().StatusCode != http.StatusForbidden {
		t.Errorf("expected 403, got %d", w.Result().StatusCode)
	}
}

func TestAuthMiddleware_ValidKey(t *testing.T) {
	cache := &mockAPIKeyCache{}
	store := &mockAPIKeyStore{
		key: &domain.APIKey{
			ID:     "key-1",
			OrgID:  "550e8400-e29b-41d4-a716-446655440000",
			Scopes: []string{"read", "write"},
			Tier:   domain.TierPro,
		},
	}
	logger := testLogger()
	middleware := AuthMiddleware(cache, store, logger)

	var capturedOrgID string
	var capturedTier domain.OrgTier
	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedOrgID = OrgIDFromContext(r.Context())
		capturedTier = OrgTierFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer pm_test_ABCDEFGHIJKLMNOPQRSTUVWXYZabcdef")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Result().StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Result().StatusCode)
	}
	if capturedOrgID != "550e8400-e29b-41d4-a716-446655440000" {
		t.Errorf("expected org ID in context, got %q", capturedOrgID)
	}
	if capturedTier != domain.TierPro {
		t.Errorf("expected tier pro, got %v", capturedTier)
	}
}

func TestOrgRateLimitMiddleware_Rejected(t *testing.T) {
	limiter := &mockRateLimiter{allowed: false, remaining: 0, resetAt: time.Now().Add(time.Second)}
	middleware := OrgRateLimitMiddleware(limiter)

	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not have been called")
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	ctx := context.WithValue(req.Context(), ctxOrgID, "org-1")
	ctx = context.WithValue(ctx, ctxOrgTier, domain.TierFree)
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Result().StatusCode != http.StatusTooManyRequests {
		t.Errorf("expected 429, got %d", w.Result().StatusCode)
	}
}

func TestClientIP_XForwardedFor(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Forwarded-For", "1.2.3.4, 5.6.7.8")
	ip := clientIP(req)
	if ip != "1.2.3.4" {
		t.Errorf("expected 1.2.3.4, got %s", ip)
	}
}

func TestClientIP_XRealIP(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Real-Ip", "10.0.0.1")
	ip := clientIP(req)
	if ip != "10.0.0.1" {
		t.Errorf("expected 10.0.0.1, got %s", ip)
	}
}
