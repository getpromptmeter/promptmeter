package handlers

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/promptmeter/promptmeter/server/internal/auth"
	"github.com/promptmeter/promptmeter/server/internal/domain"
	"github.com/promptmeter/promptmeter/server/internal/middleware"
	"github.com/promptmeter/promptmeter/server/internal/storage"
)

// AuthHandler handles auth-related endpoints.
type AuthHandler struct {
	users    storage.UserStore
	tokens   storage.RefreshTokenStore
	orgs     storage.OrgStore
	projects storage.ProjectStore
	apiKeys  storage.APIKeyManager

	jwtSecret      string
	internalSecret string
	authMode       string
	logger         *slog.Logger
}

// NewAuthHandler creates a new auth handler.
func NewAuthHandler(
	users storage.UserStore,
	tokens storage.RefreshTokenStore,
	orgs storage.OrgStore,
	projects storage.ProjectStore,
	apiKeys storage.APIKeyManager,
	jwtSecret, internalSecret, authMode string,
	logger *slog.Logger,
) *AuthHandler {
	return &AuthHandler{
		users:          users,
		tokens:         tokens,
		orgs:           orgs,
		projects:       projects,
		apiKeys:        apiKeys,
		jwtSecret:      jwtSecret,
		internalSecret: internalSecret,
		authMode:       authMode,
		logger:         logger,
	}
}

type oauthCallbackRequest struct {
	Provider  string `json:"provider"`
	Email     string `json:"email"`
	Name      string `json:"name"`
	AvatarURL string `json:"avatar_url"`
}

// HandleOAuthCallback handles POST /api/v1/auth/oauth-callback.
// Called by NextAuth.js server-side after successful OAuth.
func (h *AuthHandler) HandleOAuthCallback(w http.ResponseWriter, r *http.Request) {
	// Verify internal secret
	if r.Header.Get("X-Internal-Secret") != h.internalSecret {
		middleware.WriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Invalid internal secret", nil)
		return
	}

	var req oauthCallbackRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		middleware.WriteError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid request body", nil)
		return
	}

	if req.Email == "" {
		middleware.WriteError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Email is required", nil)
		return
	}

	// Create or get user
	user, isNew, err := h.users.CreateOrGetUser(r.Context(), req.Email, req.Name, req.AvatarURL)
	if err != nil {
		h.logger.Error("oauth: create/get user", "error", err)
		middleware.WriteError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to process user", nil)
		return
	}

	// Get or create org for user
	org, err := h.orgs.GetOrgByUserID(r.Context(), user.ID)
	if err != nil {
		h.logger.Error("oauth: get org", "error", err)
		middleware.WriteError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Internal server error", nil)
		return
	}

	if org == nil {
		// Create new org for the user
		orgName := req.Name + "'s Organization"
		slug := strings.ToLower(strings.ReplaceAll(req.Name, " ", "-")) + "-org"
		org, err = h.orgs.CreateOrg(r.Context(), orgName, slug)
		if err != nil {
			h.logger.Error("oauth: create org", "error", err)
			middleware.WriteError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to create organization", nil)
			return
		}
		if err := h.orgs.AddOrgMember(r.Context(), org.ID, user.ID, "owner"); err != nil {
			h.logger.Error("oauth: add org member", "error", err)
		}

		// Create default project
		project := &domain.Project{
			OrgID:      org.ID,
			Name:       "Default",
			Slug:       "default",
			PIIEnabled: true,
			IsDefault:  true,
		}
		if err := h.projects.CreateProject(r.Context(), project); err != nil {
			h.logger.Error("oauth: create default project", "error", err)
		}

		// Create default API key
		plaintext, keyHash, keyPrefix, keyErr := auth.GenerateKey(auth.PrefixLive)
		if keyErr == nil {
			_, createErr := h.apiKeys.CreateAPIKey(r.Context(), org.ID, "Default Key", keyPrefix, keyHash, []string{"write", "read"}, nil)
			if createErr != nil {
				h.logger.Error("oauth: create default api key", "error", createErr)
			}
			_ = plaintext // shown only once -- not stored
		}
	}

	// Issue JWT + refresh token
	jwtToken, refreshPlaintext, err := h.issueTokenPair(r.Context(), user, org)
	if err != nil {
		h.logger.Error("oauth: issue tokens", "error", err)
		middleware.WriteError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to issue tokens", nil)
		return
	}

	// Set cookies
	h.setAuthCookies(w, jwtToken, refreshPlaintext)

	middleware.WriteJSON(w, http.StatusOK, map[string]any{
		"user": map[string]any{
			"id":    user.ID,
			"email": user.Email,
			"name":  user.Name,
		},
		"organization": map[string]any{
			"id":   org.ID,
			"name": org.Name,
			"tier": org.Tier,
		},
		"is_new_user": isNew,
	})
}

// HandleRefresh handles POST /api/v1/auth/refresh.
func (h *AuthHandler) HandleRefresh(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("pm_refresh")
	if err != nil || cookie.Value == "" {
		middleware.WriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Missing refresh token", nil)
		return
	}

	tokenHash := auth.HashRefreshToken(cookie.Value)

	// Lookup token
	storedToken, err := h.tokens.GetRefreshTokenByHash(r.Context(), tokenHash)
	if err != nil {
		h.logger.Error("refresh: lookup token", "error", err)
		middleware.WriteError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Internal server error", nil)
		return
	}

	if storedToken == nil {
		// Token not found -- could be reuse attempt. Check if hash was ever used.
		middleware.WriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Invalid refresh token", nil)
		return
	}

	// Check if already revoked (reuse detection)
	if storedToken.RevokedAt != nil {
		h.logger.Warn("refresh: token reuse detected, revoking all user tokens",
			"user_id", storedToken.UserID)
		if revokeErr := h.tokens.RevokeAllUserTokens(r.Context(), storedToken.UserID); revokeErr != nil {
			h.logger.Error("refresh: revoke all tokens", "error", revokeErr)
		}
		h.clearAuthCookies(w)
		middleware.WriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Token reuse detected", nil)
		return
	}

	// Check expiration
	if time.Now().After(storedToken.ExpiresAt) {
		middleware.WriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Refresh token expired", nil)
		return
	}

	// Revoke current token
	if err := h.tokens.RevokeRefreshToken(r.Context(), storedToken.ID); err != nil {
		h.logger.Error("refresh: revoke current token", "error", err)
	}

	// Get user and org
	user, err := h.users.GetUserByID(r.Context(), storedToken.UserID)
	if err != nil || user == nil {
		middleware.WriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "User not found", nil)
		return
	}

	org, err := h.orgs.GetOrgByUserID(r.Context(), user.ID)
	if err != nil || org == nil {
		middleware.WriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Organization not found", nil)
		return
	}

	// Issue new token pair
	jwtToken, refreshPlaintext, err := h.issueTokenPair(r.Context(), user, org)
	if err != nil {
		h.logger.Error("refresh: issue new tokens", "error", err)
		middleware.WriteError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to refresh tokens", nil)
		return
	}

	h.setAuthCookies(w, jwtToken, refreshPlaintext)
	middleware.WriteJSON(w, http.StatusOK, map[string]any{"refreshed": true})
}

// HandleLogout handles DELETE /api/v1/auth/logout.
func (h *AuthHandler) HandleLogout(w http.ResponseWriter, r *http.Request) {
	// Revoke refresh token if present
	cookie, err := r.Cookie("pm_refresh")
	if err == nil && cookie.Value != "" {
		tokenHash := auth.HashRefreshToken(cookie.Value)
		storedToken, _ := h.tokens.GetRefreshTokenByHash(r.Context(), tokenHash)
		if storedToken != nil {
			if revokeErr := h.tokens.RevokeRefreshToken(r.Context(), storedToken.ID); revokeErr != nil {
				h.logger.Warn("logout: revoke token", "error", revokeErr)
			}
		}
	}

	h.clearAuthCookies(w)
	middleware.WriteJSON(w, http.StatusOK, map[string]any{"logged_out": true})
}

// HandleMe handles GET /api/v1/auth/me.
func (h *AuthHandler) HandleMe(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	orgID := middleware.OrgIDFromContext(r.Context())

	user, err := h.users.GetUserByID(r.Context(), userID)
	if err != nil || user == nil {
		middleware.WriteError(w, http.StatusNotFound, "NOT_FOUND", "User not found", nil)
		return
	}

	org, err := h.orgs.GetOrg(r.Context(), orgID)
	if err != nil || org == nil {
		middleware.WriteError(w, http.StatusNotFound, "NOT_FOUND", "Organization not found", nil)
		return
	}

	middleware.WriteJSON(w, http.StatusOK, map[string]any{
		"user": map[string]any{
			"id":         user.ID,
			"email":      user.Email,
			"name":       user.Name,
			"avatar_url": user.AvatarURL,
		},
		"organization": map[string]any{
			"id":          org.ID,
			"name":        org.Name,
			"slug":        org.Slug,
			"tier":        org.Tier,
			"timezone":    org.Timezone,
			"pii_enabled": org.PIIEnabled,
		},
	})
}

// issueTokenPair creates a new JWT and refresh token pair.
func (h *AuthHandler) issueTokenPair(ctx context.Context, user *domain.User, org *domain.Organization) (jwt, refreshPlaintext string, err error) {
	orgNumeric := domain.OrgIDToUint64(org.ID)

	claims := auth.JWTClaims{
		Sub:        user.ID,
		Org:        org.ID,
		OrgNumeric: orgNumeric,
		Role:       "owner", // MVP: single user = owner
		Tier:       string(org.Tier),
	}

	jwt, err = auth.SignJWT(claims, h.jwtSecret)
	if err != nil {
		return "", "", err
	}

	refreshPlaintext, refreshHash, err := auth.GenerateRefreshToken()
	if err != nil {
		return "", "", err
	}

	expiresAt := time.Now().Add(7 * 24 * time.Hour)
	if _, err := h.tokens.CreateRefreshToken(ctx, user.ID, refreshHash, expiresAt); err != nil {
		return "", "", err
	}

	return jwt, refreshPlaintext, nil
}

func (h *AuthHandler) setAuthCookies(w http.ResponseWriter, jwt, refresh string) {
	http.SetCookie(w, &http.Cookie{
		Name:     "pm_session",
		Value:    jwt,
		Path:     "/",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   900, // 15 min
	})
	http.SetCookie(w, &http.Cookie{
		Name:     "pm_refresh",
		Value:    refresh,
		Path:     "/api/v1/auth",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   604800, // 7 days
	})
}

func (h *AuthHandler) clearAuthCookies(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     "pm_session",
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})
	http.SetCookie(w, &http.Cookie{
		Name:     "pm_refresh",
		Value:    "",
		Path:     "/api/v1/auth",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})
}
