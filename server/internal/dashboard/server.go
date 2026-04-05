// Package dashboard provides the Dashboard API HTTP server.
package dashboard

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/promptmeter/promptmeter/server/internal/middleware"
	"github.com/promptmeter/promptmeter/server/internal/storage"
)

// Server is the Dashboard API HTTP server.
type Server struct {
	httpServer *http.Server
	logger     *slog.Logger
}

// ServerConfig holds all dependencies for creating the dashboard server.
type ServerConfig struct {
	Port           string
	JWTSecret      string
	CORSOrigins    string
	AuthMode       string // "oauth" or "autologin"
	InternalSecret string

	// Storage dependencies
	PG    *storage.PostgresStore
	CH    *storage.ClickHouseStore
	Redis *storage.RedisStore

	// Autologin defaults (populated during bootstrap)
	DefaultUserID    string
	DefaultOrgID     string
	DefaultOrgNumeric uint64

	Logger *slog.Logger
}

// NewServer creates a new Dashboard API server with the full middleware chain.
func NewServer(cfg ServerConfig) *Server {
	router := NewRouter(cfg)

	// Global middleware chain (outermost first when applied bottom-up)
	var handler http.Handler = router
	handler = middleware.LoggerMiddleware(cfg.Logger)(handler)
	handler = middleware.RequestIDMiddleware(handler)
	handler = middleware.CORSMiddleware(cfg.CORSOrigins)(handler)
	handler = middleware.RecoveryMiddleware(cfg.Logger)(handler)

	return &Server{
		httpServer: &http.Server{
			Addr:         ":" + cfg.Port,
			Handler:      handler,
			ReadTimeout:  30 * time.Second,
			WriteTimeout: 30 * time.Second,
			IdleTimeout:  120 * time.Second,
		},
		logger: cfg.Logger,
	}
}

// Start begins listening for HTTP requests.
func (s *Server) Start() error {
	s.logger.Info("dashboard server starting", "addr", s.httpServer.Addr)
	return s.httpServer.ListenAndServe()
}

// Shutdown gracefully stops the server with the given timeout.
func (s *Server) Shutdown(timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	s.logger.Info("dashboard server shutting down")
	if err := s.httpServer.Shutdown(ctx); err != nil {
		return fmt.Errorf("dashboard server shutdown: %w", err)
	}
	return nil
}
