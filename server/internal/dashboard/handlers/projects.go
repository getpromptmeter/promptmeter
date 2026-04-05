package handlers

import (
	"log/slog"
	"net/http"

	"github.com/promptmeter/promptmeter/server/internal/middleware"
	"github.com/promptmeter/promptmeter/server/internal/storage"
)

// ProjectsHandler handles project-related endpoints.
type ProjectsHandler struct {
	projects storage.ProjectStore
	logger   *slog.Logger
}

// NewProjectsHandler creates a new projects handler.
func NewProjectsHandler(projects storage.ProjectStore, logger *slog.Logger) *ProjectsHandler {
	return &ProjectsHandler{projects: projects, logger: logger}
}

// HandleList handles GET /api/v1/projects.
func (h *ProjectsHandler) HandleList(w http.ResponseWriter, r *http.Request) {
	orgID := middleware.OrgIDFromContext(r.Context())

	projects, err := h.projects.ListProjects(r.Context(), orgID)
	if err != nil {
		h.logger.Error("projects: list", "error", err)
		middleware.WriteError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to list projects", nil)
		return
	}

	type projectResponse struct {
		ID          string `json:"id"`
		Name        string `json:"name"`
		Slug        string `json:"slug"`
		Description string `json:"description"`
		IsDefault   bool   `json:"is_default"`
		CreatedAt   string `json:"created_at"`
	}

	var result []projectResponse
	for _, p := range projects {
		result = append(result, projectResponse{
			ID:          p.ID,
			Name:        p.Name,
			Slug:        p.Slug,
			Description: p.Description,
			IsDefault:   p.IsDefault,
			CreatedAt:   p.CreatedAt.Format("2006-01-02T15:04:05Z"),
		})
	}

	middleware.WriteJSON(w, http.StatusOK, result)
}
