package middleware

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/promptmeter/promptmeter/server/internal/auth"
	"github.com/promptmeter/promptmeter/server/internal/domain"
)

// JWTAuthMiddleware parses the pm_session cookie, verifies the JWT,
// and injects user/org context. Returns 401 if cookie is missing or invalid.
func JWTAuthMiddleware(secret string, logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			cookie, err := r.Cookie("pm_session")
			if err != nil || cookie.Value == "" {
				WriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required", nil)
				return
			}

			claims, err := auth.VerifyJWT(cookie.Value, secret)
			if err != nil {
				if err == auth.ErrTokenExpired {
					WriteError(w, http.StatusUnauthorized, "TOKEN_EXPIRED", "Token expired, refresh required", nil)
					return
				}
				WriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Invalid token", nil)
				return
			}

			// Inject claims into context -- org_id always from JWT, never from client
			ctx := r.Context()
			ctx = context.WithValue(ctx, CtxUserID, claims.Sub)
			ctx = context.WithValue(ctx, CtxOrgID, claims.Org)
			ctx = context.WithValue(ctx, CtxOrgNum, claims.OrgNumeric)
			ctx = context.WithValue(ctx, CtxOrgTier, domain.OrgTier(claims.Tier))
			ctx = context.WithValue(ctx, CtxRole, claims.Role)

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// AutoLoginMiddleware automatically issues a JWT for the default user
// in self-hosted mode when AUTH_MODE=autologin.
func AutoLoginMiddleware(secret string, defaultUserID, defaultOrgID string, defaultOrgNumeric uint64, logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Check if already authenticated
			cookie, err := r.Cookie("pm_session")
			if err == nil && cookie.Value != "" {
				if _, verifyErr := auth.VerifyJWT(cookie.Value, secret); verifyErr == nil {
					// Valid JWT exists, proceed normally through JWT middleware
					next.ServeHTTP(w, r)
					return
				}
			}

			// Auto-issue JWT for default user
			claims := auth.JWTClaims{
				Sub:        defaultUserID,
				Org:        defaultOrgID,
				OrgNumeric: defaultOrgNumeric,
				Role:       "owner",
				Tier:       string(domain.TierFree),
			}

			token, signErr := auth.SignJWT(claims, secret)
			if signErr != nil {
				logger.Error("autologin: failed to sign JWT", "error", signErr)
				WriteError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Internal server error", nil)
				return
			}

			http.SetCookie(w, &http.Cookie{
				Name:     "pm_session",
				Value:    token,
				Path:     "/",
				HttpOnly: true,
				Secure:   false, // self-hosted is typically localhost
				SameSite: http.SameSiteLaxMode,
				MaxAge:   900, // 15 min
			})

			// Inject claims into context
			ctx := r.Context()
			ctx = context.WithValue(ctx, CtxUserID, defaultUserID)
			ctx = context.WithValue(ctx, CtxOrgID, defaultOrgID)
			ctx = context.WithValue(ctx, CtxOrgNum, defaultOrgNumeric)
			ctx = context.WithValue(ctx, CtxOrgTier, domain.TierFree)
			ctx = context.WithValue(ctx, CtxRole, "owner")

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
