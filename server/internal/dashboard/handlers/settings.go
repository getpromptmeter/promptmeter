package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/promptmeter/promptmeter/server/internal/middleware"
	"github.com/promptmeter/promptmeter/server/internal/storage"
)

// SettingsHandler handles org settings endpoints.
type SettingsHandler struct {
	orgs   storage.OrgStore
	logger *slog.Logger
}

// NewSettingsHandler creates a new settings handler.
func NewSettingsHandler(orgs storage.OrgStore, logger *slog.Logger) *SettingsHandler {
	return &SettingsHandler{orgs: orgs, logger: logger}
}

// HandleGet handles GET /api/v1/settings/org.
func (h *SettingsHandler) HandleGet(w http.ResponseWriter, r *http.Request) {
	orgID := middleware.OrgIDFromContext(r.Context())

	org, err := h.orgs.GetOrg(r.Context(), orgID)
	if err != nil || org == nil {
		h.logger.Error("settings: get org", "error", err)
		middleware.WriteError(w, http.StatusNotFound, "NOT_FOUND", "Organization not found", nil)
		return
	}

	middleware.WriteJSON(w, http.StatusOK, map[string]any{
		"name":              org.Name,
		"slug":              org.Slug,
		"timezone":          org.Timezone,
		"pii_enabled":       org.PIIEnabled,
		"slack_webhook_url": org.SlackWebhookURL,
		"tier":              org.Tier,
	})
}

type updateOrgRequest struct {
	Name            *string `json:"name,omitempty"`
	Timezone        *string `json:"timezone,omitempty"`
	PIIEnabled      *bool   `json:"pii_enabled,omitempty"`
	SlackWebhookURL *string `json:"slack_webhook_url,omitempty"`
}

// HandleUpdate handles PUT /api/v1/settings/org.
func (h *SettingsHandler) HandleUpdate(w http.ResponseWriter, r *http.Request) {
	orgID := middleware.OrgIDFromContext(r.Context())

	var req updateOrgRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		middleware.WriteError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid request body", nil)
		return
	}

	org, err := h.orgs.GetOrg(r.Context(), orgID)
	if err != nil || org == nil {
		middleware.WriteError(w, http.StatusNotFound, "NOT_FOUND", "Organization not found", nil)
		return
	}

	// Apply partial updates
	if req.Name != nil {
		name := strings.TrimSpace(*req.Name)
		if name == "" || len(name) > 100 {
			middleware.WriteError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Name must be 1-100 characters", nil)
			return
		}
		org.Name = name
	}

	if req.Timezone != nil {
		if _, err := time.LoadLocation(*req.Timezone); err != nil {
			middleware.WriteError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid IANA timezone", nil)
			return
		}
		org.Timezone = *req.Timezone
	}

	if req.PIIEnabled != nil {
		org.PIIEnabled = *req.PIIEnabled
	}

	if req.SlackWebhookURL != nil {
		url := *req.SlackWebhookURL
		if url != "" && !strings.HasPrefix(url, "https://hooks.slack.com/") {
			middleware.WriteError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Slack webhook URL must start with https://hooks.slack.com/", nil)
			return
		}
		if url == "" {
			org.SlackWebhookURL = nil
		} else {
			org.SlackWebhookURL = &url
		}
	}

	if err := h.orgs.UpdateOrg(r.Context(), org); err != nil {
		h.logger.Error("settings: update org", "error", err)
		middleware.WriteError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to update settings", nil)
		return
	}

	middleware.WriteJSON(w, http.StatusOK, map[string]any{
		"name":              org.Name,
		"slug":              org.Slug,
		"timezone":          org.Timezone,
		"pii_enabled":       org.PIIEnabled,
		"slack_webhook_url": org.SlackWebhookURL,
		"tier":              org.Tier,
	})
}
