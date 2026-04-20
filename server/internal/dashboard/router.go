package dashboard

import (
	"net/http"
	"time"

	"github.com/promptmeter/promptmeter/server/internal/dashboard/handlers"
	"github.com/promptmeter/promptmeter/server/internal/middleware"
)

// NewRouter creates the HTTP router with all dashboard routes.
func NewRouter(cfg ServerConfig) http.Handler {
	mux := http.NewServeMux()

	// Health endpoint -- no auth required
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	})

	// Auth handlers -- no JWT required
	authHandler := handlers.NewAuthHandler(
		cfg.PG, cfg.PG, cfg.PG, cfg.PG, cfg.PG,
		cfg.JWTSecret, cfg.InternalSecret,
		cfg.AuthMode, cfg.Logger,
	)

	authMux := http.NewServeMux()
	authMux.HandleFunc("POST /api/v1/auth/oauth-callback", authHandler.HandleOAuthCallback)
	authMux.HandleFunc("POST /api/v1/auth/refresh", authHandler.HandleRefresh)
	authMux.HandleFunc("DELETE /api/v1/auth/logout", authHandler.HandleLogout)

	// IP rate limit on auth endpoints (10 req/min)
	var authRoutes http.Handler = authMux
	authRoutes = middleware.IPRateLimitMiddleware(cfg.Redis, 10, time.Minute)(authRoutes)
	mux.Handle("/api/v1/auth/", authRoutes)

	// Authenticated endpoints -- require JWT
	apiMux := http.NewServeMux()

	// Auth: GET /auth/me requires JWT
	apiMux.HandleFunc("GET /api/v1/auth/me", authHandler.HandleMe)

	// Overview + cost handlers
	overviewHandler := handlers.NewOverviewHandler(cfg.CH, cfg.Redis, cfg.Logger)
	apiMux.HandleFunc("GET /api/v1/dashboard/overview", overviewHandler.HandleOverview)

	costHandler := handlers.NewCostHandler(cfg.CH, cfg.Redis, cfg.Logger)
	apiMux.HandleFunc("GET /api/v1/dashboard/cost", costHandler.HandleCostBreakdown)
	apiMux.HandleFunc("GET /api/v1/dashboard/cost/timeseries", costHandler.HandleCostTimeseries)
	apiMux.HandleFunc("GET /api/v1/dashboard/cost/compare", costHandler.HandleCostCompare)
	apiMux.HandleFunc("GET /api/v1/dashboard/cost/filters", costHandler.HandleCostFilters)

	// CRUD handlers
	apiKeysHandler := handlers.NewAPIKeysHandler(cfg.PG, cfg.Redis, cfg.Logger)
	apiMux.HandleFunc("GET /api/v1/api-keys", apiKeysHandler.HandleList)
	apiMux.HandleFunc("POST /api/v1/api-keys", apiKeysHandler.HandleCreate)
	apiMux.HandleFunc("DELETE /api/v1/api-keys/{id}", apiKeysHandler.HandleRevoke)

	settingsHandler := handlers.NewSettingsHandler(cfg.PG, cfg.Logger)
	apiMux.HandleFunc("GET /api/v1/settings/org", settingsHandler.HandleGet)
	apiMux.HandleFunc("PUT /api/v1/settings/org", settingsHandler.HandleUpdate)

	projectsHandler := handlers.NewProjectsHandler(cfg.PG, cfg.Logger)
	apiMux.HandleFunc("GET /api/v1/projects", projectsHandler.HandleList)

	// Apply auth middleware (outermost applied last, executed first)
	var apiRoutes http.Handler = apiMux
	apiRoutes = middleware.JWTAuthMiddleware(cfg.JWTSecret, cfg.Logger)(apiRoutes)
	if cfg.AuthMode == "autologin" {
		apiRoutes = middleware.AutoLoginMiddleware(
			cfg.JWTSecret,
			cfg.DefaultUserID, cfg.DefaultOrgID, cfg.DefaultOrgNumeric,
			cfg.Logger,
		)(apiRoutes)
	}
	apiRoutes = middleware.IPRateLimitMiddleware(cfg.Redis, 200, time.Minute)(apiRoutes)

	mux.Handle("/api/v1/", apiRoutes)

	return mux
}
