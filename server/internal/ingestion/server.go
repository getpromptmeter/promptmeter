package ingestion

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/promptmeter/promptmeter/server/internal/storage"
)

// Server is the Ingestion API HTTP server.
type Server struct {
	httpServer *http.Server
	logger     *slog.Logger
}

// ServerConfig holds all dependencies for creating the ingestion server.
type ServerConfig struct {
	Port      string
	Handler   *Handler
	Cache     storage.APIKeyCache
	Store     storage.APIKeyStore
	Limiter   storage.RateLimiter
	Logger    *slog.Logger
}

// NewServer creates a new Ingestion API server with the full middleware chain.
func NewServer(cfg ServerConfig) *Server {
	mux := http.NewServeMux()

	// Health endpoint -- no auth required
	mux.HandleFunc("GET /health", cfg.Handler.HandleHealth)

	// Authenticated endpoints
	authedMux := http.NewServeMux()
	authedMux.HandleFunc("POST /v1/events", cfg.Handler.HandleEvent)
	authedMux.HandleFunc("POST /v1/events/batch", cfg.Handler.HandleBatch)

	// Build middleware chain for authenticated endpoints (innermost first)
	var authedHandler http.Handler = authedMux
	authedHandler = OrgRateLimitMiddleware(cfg.Limiter)(authedHandler)
	authedHandler = AuthMiddleware(cfg.Cache, cfg.Store, cfg.Logger)(authedHandler)
	authedHandler = IPRateLimitMiddleware(cfg.Limiter, 200, time.Minute)(authedHandler)
	authedHandler = BodySizeLimitMiddleware(maxBodyBatch)(authedHandler)

	mux.Handle("/v1/", authedHandler)

	// Global middleware chain
	var handler http.Handler = mux
	handler = LoggerMiddleware(cfg.Logger)(handler)
	handler = RequestIDMiddleware(handler)
	handler = RecoveryMiddleware(cfg.Logger)(handler)

	return &Server{
		httpServer: &http.Server{
			Addr:         ":" + cfg.Port,
			Handler:      handler,
			ReadTimeout:  10 * time.Second,
			WriteTimeout: 10 * time.Second,
			IdleTimeout:  60 * time.Second,
		},
		logger: cfg.Logger,
	}
}

// Start begins listening for HTTP requests. It blocks until the server is stopped.
func (s *Server) Start() error {
	s.logger.Info("ingestion server starting", "addr", s.httpServer.Addr)
	return s.httpServer.ListenAndServe()
}

// Shutdown gracefully stops the server with the given timeout.
func (s *Server) Shutdown(timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	s.logger.Info("ingestion server shutting down")
	if err := s.httpServer.Shutdown(ctx); err != nil {
		return fmt.Errorf("ingestion server shutdown: %w", err)
	}
	return nil
}
