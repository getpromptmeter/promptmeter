// Package middleware provides shared HTTP middleware for Promptmeter services.
package middleware

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

	"github.com/promptmeter/promptmeter/server/internal/storage"
)

type contextKey string

const (
	CtxRequestID contextKey = "request_id"
	CtxOrgID     contextKey = "org_id"
	CtxOrgTier   contextKey = "org_tier"
	CtxUserID    contextKey = "user_id"
	CtxRole      contextKey = "role"
	CtxOrgNum    contextKey = "org_numeric"
)

// RequestIDFromContext extracts the request ID from the context.
func RequestIDFromContext(ctx context.Context) string {
	v, _ := ctx.Value(CtxRequestID).(string)
	return v
}

// OrgIDFromContext extracts the authenticated org_id from the context.
func OrgIDFromContext(ctx context.Context) string {
	v, _ := ctx.Value(CtxOrgID).(string)
	return v
}

// UserIDFromContext extracts the authenticated user_id from the context.
func UserIDFromContext(ctx context.Context) string {
	v, _ := ctx.Value(CtxUserID).(string)
	return v
}

// OrgNumericFromContext extracts the org_id as uint64 for ClickHouse queries.
func OrgNumericFromContext(ctx context.Context) uint64 {
	v, _ := ctx.Value(CtxOrgNum).(uint64)
	return v
}

// RecoveryMiddleware catches panics and returns 500.
func RecoveryMiddleware(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if rec := recover(); rec != nil {
					logger.Error("panic recovered", "error", rec)
					WriteError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Internal server error", nil)
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
		ctx := context.WithValue(r.Context(), CtxRequestID, reqID)
		w.Header().Set("X-Request-Id", reqID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// LoggerMiddleware logs each request with method, path, status, and latency.
func LoggerMiddleware(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			rw := &ResponseWriter{ResponseWriter: w, StatusCode: http.StatusOK}
			next.ServeHTTP(rw, r)
			logger.Info("request",
				"method", r.Method,
				"path", r.URL.Path,
				"status", rw.StatusCode,
				"latency_ms", time.Since(start).Milliseconds(),
				"request_id", RequestIDFromContext(r.Context()),
			)
		})
	}
}

// IPRateLimitMiddleware enforces per-IP rate limiting (pre-auth).
func IPRateLimitMiddleware(limiter storage.RateLimiter, limit int, window time.Duration) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := ClientIP(r)
			allowed, remaining, resetAt, err := limiter.AllowIP(r.Context(), ip, limit, window)
			if err != nil {
				next.ServeHTTP(w, r)
				return
			}

			SetRateLimitHeaders(w, limit, remaining, resetAt)
			if !allowed {
				retryAfter := int(time.Until(resetAt).Seconds()) + 1
				w.Header().Set("Retry-After", fmt.Sprintf("%d", retryAfter))
				WriteError(w, http.StatusTooManyRequests, "RATE_LIMIT_EXCEEDED", "Rate limit exceeded", map[string]any{
					"limit":          limit,
					"retry_after_ms": retryAfter * 1000,
				})
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// ResponseWriter wraps http.ResponseWriter to capture the status code.
type ResponseWriter struct {
	http.ResponseWriter
	StatusCode int
}

// WriteHeader captures the status code before delegating.
func (rw *ResponseWriter) WriteHeader(code int) {
	rw.StatusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

func generateRequestID() string {
	b := make([]byte, 6)
	_, _ = rand.Read(b)
	return "req_" + hex.EncodeToString(b)[:6]
}

// ClientIP extracts the client IP from the request, considering proxy headers.
func ClientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		return strings.TrimSpace(parts[0])
	}
	if xri := r.Header.Get("X-Real-Ip"); xri != "" {
		return xri
	}
	addr := r.RemoteAddr
	if idx := strings.LastIndex(addr, ":"); idx != -1 {
		return addr[:idx]
	}
	return addr
}

// SetRateLimitHeaders sets X-RateLimit-* headers on the response.
func SetRateLimitHeaders(w http.ResponseWriter, limit, remaining int, resetAt time.Time) {
	w.Header().Set("X-RateLimit-Limit", fmt.Sprintf("%d", limit))
	w.Header().Set("X-RateLimit-Remaining", fmt.Sprintf("%d", remaining))
	w.Header().Set("X-RateLimit-Reset", fmt.Sprintf("%d", resetAt.Unix()))
}

// WriteError writes a JSON error response in the standard envelope format.
func WriteError(w http.ResponseWriter, status int, code, message string, details map[string]any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	resp := map[string]any{
		"error": map[string]any{
			"code":    code,
			"message": message,
		},
		"meta": map[string]any{
			"request_id": w.Header().Get("X-Request-Id"),
			"timestamp":  time.Now().UTC().Format(time.RFC3339),
		},
	}
	if details != nil {
		resp["error"].(map[string]any)["details"] = details
	}
	json.NewEncoder(w).Encode(resp)
}

// WriteJSON writes a JSON success response in the standard envelope format.
func WriteJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	resp := map[string]any{
		"data": data,
		"meta": map[string]any{
			"request_id": w.Header().Get("X-Request-Id"),
			"timestamp":  time.Now().UTC().Format(time.RFC3339),
		},
	}
	json.NewEncoder(w).Encode(resp)
}
