package ingestion

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/promptmeter/promptmeter/server/internal/auth"
	"github.com/promptmeter/promptmeter/server/internal/domain"
	"github.com/promptmeter/promptmeter/server/internal/storage"
)

type contextKey string

const (
	ctxOrgID   contextKey = "org_id"
	ctxOrgTier contextKey = "org_tier"
	ctxReqID   contextKey = "request_id"
)

// OrgIDFromContext extracts the authenticated org_id from the request context.
func OrgIDFromContext(ctx context.Context) string {
	v, _ := ctx.Value(ctxOrgID).(string)
	return v
}

// OrgTierFromContext extracts the authenticated org tier from the request context.
func OrgTierFromContext(ctx context.Context) domain.OrgTier {
	v, _ := ctx.Value(ctxOrgTier).(domain.OrgTier)
	return v
}

// RequestIDFromContext extracts the request ID from the context.
func RequestIDFromContext(ctx context.Context) string {
	v, _ := ctx.Value(ctxReqID).(string)
	return v
}

// RecoveryMiddleware catches panics and returns 500.
func RecoveryMiddleware(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if rec := recover(); rec != nil {
					logger.Error("panic recovered", "error", rec)
					writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Internal server error", nil)
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}

// RequestIDMiddleware generates a request ID and adds it to context and response headers.
func RequestIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqID := generateRequestID()
		ctx := context.WithValue(r.Context(), ctxReqID, reqID)
		w.Header().Set("X-Request-Id", reqID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// LoggerMiddleware logs each request with method, path, status, and latency.
func LoggerMiddleware(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			rw := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}
			next.ServeHTTP(rw, r)
			logger.Info("request",
				"method", r.Method,
				"path", r.URL.Path,
				"status", rw.statusCode,
				"latency_ms", time.Since(start).Milliseconds(),
				"request_id", RequestIDFromContext(r.Context()),
			)
		})
	}
}

// BodySizeLimitMiddleware rejects requests exceeding the size limit.
func BodySizeLimitMiddleware(maxBytes int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
			next.ServeHTTP(w, r)
		})
	}
}

// IPRateLimitMiddleware enforces per-IP rate limiting (pre-auth).
func IPRateLimitMiddleware(limiter storage.RateLimiter, limit int, window time.Duration) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := clientIP(r)
			allowed, remaining, resetAt, err := limiter.AllowIP(r.Context(), ip, limit, window)
			if err != nil {
				// If Redis is down, allow the request through
				next.ServeHTTP(w, r)
				return
			}

			setRateLimitHeaders(w, limit, remaining, resetAt)
			if !allowed {
				retryAfter := int(time.Until(resetAt).Seconds()) + 1
				w.Header().Set("Retry-After", fmt.Sprintf("%d", retryAfter))
				writeError(w, http.StatusTooManyRequests, "RATE_LIMIT_EXCEEDED", "Rate limit exceeded", map[string]any{
					"limit":          limit,
					"retry_after_ms": retryAfter * 1000,
				})
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// AuthMiddleware authenticates requests using API key (Bearer token).
// It checks Redis cache first, then falls back to PostgreSQL.
func AuthMiddleware(cache storage.APIKeyCache, store storage.APIKeyStore, logger *slog.Logger) func(http.Handler) http.Handler {
	const cacheTTL = 5 * time.Minute

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Extract Bearer token
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Invalid or missing API key", nil)
				return
			}

			token := strings.TrimPrefix(authHeader, "Bearer ")
			if token == authHeader {
				writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Invalid or missing API key", nil)
				return
			}

			// Validate format
			if err := auth.ValidateFormat(token); err != nil {
				writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Invalid or missing API key", nil)
				return
			}

			keyHash := auth.HashKey(token)

			// Check Redis cache
			apiKey, err := cache.GetCachedAPIKey(r.Context(), keyHash)
			if err != nil {
				logger.Warn("auth: redis cache error", "error", err)
			}

			// Cache miss -- fall back to PostgreSQL
			if apiKey == nil {
				apiKey, err = store.GetAPIKeyByHash(r.Context(), keyHash)
				if err != nil {
					logger.Error("auth: postgres error", "error", err)
					writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Internal server error", nil)
					return
				}
				if apiKey == nil {
					writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Invalid or missing API key", nil)
					return
				}

				// Cache the result
				if cacheErr := cache.SetCachedAPIKey(r.Context(), keyHash, apiKey, cacheTTL); cacheErr != nil {
					logger.Warn("auth: redis cache set error", "error", cacheErr)
				}
			}

			// Check revocation
			if apiKey.IsRevoked() {
				writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Invalid or missing API key", nil)
				return
			}

			// Check write scope for ingestion
			if !apiKey.HasScope("write") {
				writeError(w, http.StatusForbidden, "FORBIDDEN", "Insufficient permissions", nil)
				return
			}

			// Inject org_id and tier into context
			ctx := context.WithValue(r.Context(), ctxOrgID, apiKey.OrgID)
			ctx = context.WithValue(ctx, ctxOrgTier, apiKey.Tier)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// OrgRateLimitMiddleware enforces per-org rate limiting (post-auth).
func OrgRateLimitMiddleware(limiter storage.RateLimiter) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			orgID := OrgIDFromContext(r.Context())
			tier := OrgTierFromContext(r.Context())
			limit := domain.RateLimitForTier(tier)
			window := time.Second

			allowed, remaining, resetAt, err := limiter.AllowOrg(r.Context(), orgID, limit, window)
			if err != nil {
				next.ServeHTTP(w, r)
				return
			}

			setRateLimitHeaders(w, limit, remaining, resetAt)
			if !allowed {
				retryAfter := 1
				w.Header().Set("Retry-After", fmt.Sprintf("%d", retryAfter))
				writeError(w, http.StatusTooManyRequests, "RATE_LIMIT_EXCEEDED", "Rate limit exceeded", map[string]any{
					"limit":          limit,
					"retry_after_ms": retryAfter * 1000,
				})
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// --- helpers ---

type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

func generateRequestID() string {
	b := make([]byte, 6)
	_, _ = rand.Read(b)
	return "req_" + hex.EncodeToString(b)[:6]
}

func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		return strings.TrimSpace(parts[0])
	}
	if xri := r.Header.Get("X-Real-Ip"); xri != "" {
		return xri
	}
	// Strip port from RemoteAddr
	addr := r.RemoteAddr
	if idx := strings.LastIndex(addr, ":"); idx != -1 {
		return addr[:idx]
	}
	return addr
}

func setRateLimitHeaders(w http.ResponseWriter, limit, remaining int, resetAt time.Time) {
	w.Header().Set("X-RateLimit-Limit", fmt.Sprintf("%d", limit))
	w.Header().Set("X-RateLimit-Remaining", fmt.Sprintf("%d", remaining))
	w.Header().Set("X-RateLimit-Reset", fmt.Sprintf("%d", resetAt.Unix()))
}

func writeError(w http.ResponseWriter, status int, code, message string, details map[string]any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	resp := map[string]any{
		"error": map[string]any{
			"code":    code,
			"message": message,
		},
		"meta": map[string]any{
			"timestamp": time.Now().UTC().Format(time.RFC3339),
		},
	}
	if details != nil {
		resp["error"].(map[string]any)["details"] = details
	}
	json.NewEncoder(w).Encode(resp)
}
