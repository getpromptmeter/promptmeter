package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/promptmeter/promptmeter/server/internal/auth"
	"github.com/promptmeter/promptmeter/server/internal/middleware"
	"github.com/promptmeter/promptmeter/server/internal/storage"
)

// APIKeysHandler handles API key CRUD endpoints.
type APIKeysHandler struct {
	keys   storage.APIKeyManager
	cache  storage.APIKeyCache
	logger *slog.Logger
}

// NewAPIKeysHandler creates a new API keys handler.
func NewAPIKeysHandler(keys storage.APIKeyManager, cache storage.APIKeyCache, logger *slog.Logger) *APIKeysHandler {
	return &APIKeysHandler{keys: keys, cache: cache, logger: logger}
}

// HandleList handles GET /api/v1/api-keys.
func (h *APIKeysHandler) HandleList(w http.ResponseWriter, r *http.Request) {
	orgID := middleware.OrgIDFromContext(r.Context())

	keys, err := h.keys.ListAPIKeys(r.Context(), orgID)
	if err != nil {
		h.logger.Error("api-keys: list", "error", err)
		middleware.WriteError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to list API keys", nil)
		return
	}

	// Format response -- no full key, only prefix
	type keyResponse struct {
		ID         string  `json:"id"`
		Name       string  `json:"name"`
		KeyPrefix  string  `json:"key_prefix"`
		ProjectID  *string `json:"project_id,omitempty"`
		Scopes     []string `json:"scopes"`
		LastUsedAt *string `json:"last_used_at,omitempty"`
		CreatedAt  string  `json:"created_at"`
		RevokedAt  *string `json:"revoked_at,omitempty"`
	}

	var result []keyResponse
	for _, k := range keys {
		kr := keyResponse{
			ID:        k.ID,
			Name:      k.Name,
			KeyPrefix: k.KeyPrefix,
			ProjectID: k.ProjectID,
			Scopes:    k.Scopes,
			CreatedAt: k.CreatedAt.Format("2006-01-02T15:04:05Z"),
		}
		if k.LastUsedAt != nil {
			s := k.LastUsedAt.Format("2006-01-02T15:04:05Z")
			kr.LastUsedAt = &s
		}
		if k.RevokedAt != nil {
			s := k.RevokedAt.Format("2006-01-02T15:04:05Z")
			kr.RevokedAt = &s
		}
		result = append(result, kr)
	}

	middleware.WriteJSON(w, http.StatusOK, result)
}

type createAPIKeyRequest struct {
	Name      string  `json:"name"`
	Type      string  `json:"type"` // "live" or "test"
	ProjectID *string `json:"project_id,omitempty"`
}

// HandleCreate handles POST /api/v1/api-keys.
func (h *APIKeysHandler) HandleCreate(w http.ResponseWriter, r *http.Request) {
	orgID := middleware.OrgIDFromContext(r.Context())

	var req createAPIKeyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		middleware.WriteError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid request body", nil)
		return
	}

	if req.Name == "" {
		middleware.WriteError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Name is required", nil)
		return
	}
	if len(req.Name) > 100 {
		middleware.WriteError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Name must be 100 characters or fewer", nil)
		return
	}

	prefix := auth.PrefixLive
	if req.Type == "test" {
		prefix = auth.PrefixTest
	}

	plaintext, keyHash, displayPrefix, err := auth.GenerateKey(prefix)
	if err != nil {
		h.logger.Error("api-keys: generate", "error", err)
		middleware.WriteError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to generate API key", nil)
		return
	}

	key, err := h.keys.CreateAPIKey(r.Context(), orgID, req.Name, displayPrefix, keyHash, []string{"write", "read"}, req.ProjectID)
	if err != nil {
		h.logger.Error("api-keys: create", "error", err)
		middleware.WriteError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to create API key", nil)
		return
	}

	middleware.WriteJSON(w, http.StatusCreated, map[string]any{
		"id":         key.ID,
		"name":       key.Name,
		"key":        plaintext, // shown once only
		"key_prefix": displayPrefix,
		"project_id": key.ProjectID,
		"scopes":     key.Scopes,
		"created_at": key.CreatedAt.Format("2006-01-02T15:04:05Z"),
	})
}

// HandleRevoke handles DELETE /api/v1/api-keys/{id}.
func (h *APIKeysHandler) HandleRevoke(w http.ResponseWriter, r *http.Request) {
	orgID := middleware.OrgIDFromContext(r.Context())
	keyID := r.PathValue("id")
	if keyID == "" {
		middleware.WriteError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Key ID is required", nil)
		return
	}

	// Get key to find hash for cache invalidation
	key, err := h.keys.GetAPIKeyByHash(r.Context(), keyID)
	if err != nil {
		h.logger.Warn("api-keys: get key for cache invalidation", "error", err)
	}

	if err := h.keys.RevokeAPIKey(r.Context(), orgID, keyID); err != nil {
		h.logger.Error("api-keys: revoke", "error", err)
		middleware.WriteError(w, http.StatusNotFound, "NOT_FOUND", "API key not found or already revoked", nil)
		return
	}

	// Invalidate Redis cache for this key
	if key != nil {
		if cacheErr := h.cache.DeleteCachedAPIKey(r.Context(), key.KeyHash); cacheErr != nil {
			h.logger.Warn("api-keys: cache invalidation", "error", cacheErr)
		}
	}

	middleware.WriteJSON(w, http.StatusOK, map[string]any{"revoked": true})
}
