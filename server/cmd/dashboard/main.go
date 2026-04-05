// Command dashboard runs the Promptmeter Dashboard API server.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/promptmeter/promptmeter/server/internal/auth"
	"github.com/promptmeter/promptmeter/server/internal/config"
	"github.com/promptmeter/promptmeter/server/internal/dashboard"
	"github.com/promptmeter/promptmeter/server/internal/domain"
	"github.com/promptmeter/promptmeter/server/internal/migrate"
	"github.com/promptmeter/promptmeter/server/internal/storage"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	ctx := context.Background()

	cfg := config.LoadDashboard()

	// Run database migrations before anything else
	if err := migrate.RunPostgres(cfg.PGURL, logger); err != nil {
		logger.Error("failed to run postgres migrations", "error", err)
		os.Exit(1)
	}
	if err := migrate.RunClickHouse(cfg.CHURL, logger); err != nil {
		logger.Error("failed to run clickhouse migrations", "error", err)
		os.Exit(1)
	}

	// Safety check: autologin + cloud = fatal error
	if cfg.AuthMode == "autologin" && cfg.DeploymentMode == "cloud" {
		logger.Error("AUTH_MODE=autologin is not allowed with DEPLOYMENT_MODE=cloud")
		os.Exit(1)
	}

	// Connect to PostgreSQL
	pgStore, err := storage.NewPostgresStore(ctx, cfg.PGURL)
	if err != nil {
		logger.Error("failed to connect to postgres", "error", err)
		os.Exit(1)
	}
	defer pgStore.Close()

	// Connect to ClickHouse
	chStore, err := storage.NewClickHouseStore(ctx, cfg.CHURL)
	if err != nil {
		logger.Error("failed to connect to clickhouse", "error", err)
		os.Exit(1)
	}
	defer chStore.Close()

	// Connect to Redis
	redisStore, err := storage.NewRedisStore(ctx, cfg.RedisURL)
	if err != nil {
		logger.Error("failed to connect to redis", "error", err)
		os.Exit(1)
	}
	defer redisStore.Close()

	// Bootstrap defaults for autologin mode
	var defaultUserID, defaultOrgID string
	var defaultOrgNumeric uint64

	if cfg.AuthMode == "autologin" {
		defaultUserID, defaultOrgID, defaultOrgNumeric, err = bootstrapSelfHosted(ctx, pgStore, logger)
		if err != nil {
			logger.Error("failed to bootstrap self-hosted defaults", "error", err)
			os.Exit(1)
		}
	}

	srv := dashboard.NewServer(dashboard.ServerConfig{
		Port:              cfg.Port,
		JWTSecret:         cfg.JWTSecret,
		CORSOrigins:       cfg.CORSOrigins,
		AuthMode:          cfg.AuthMode,
		InternalSecret:    cfg.InternalSecret,
		PG:                pgStore,
		CH:                chStore,
		Redis:             redisStore,
		DefaultUserID:     defaultUserID,
		DefaultOrgID:      defaultOrgID,
		DefaultOrgNumeric: defaultOrgNumeric,
		Logger:            logger,
	})

	// Graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		if err := srv.Start(); err != nil {
			logger.Error("server error", "error", err)
		}
	}()

	logger.Info("dashboard server started",
		"port", cfg.Port,
		"auth_mode", cfg.AuthMode,
		"deployment_mode", cfg.DeploymentMode,
	)

	<-sigCh
	logger.Info("shutdown signal received")

	if err := srv.Shutdown(10 * time.Second); err != nil {
		logger.Error("shutdown error", "error", err)
	}
}

// bootstrapSelfHosted ensures a default org, user, project, and API key exist
// for self-hosted autologin mode. Returns the default user/org IDs.
func bootstrapSelfHosted(ctx context.Context, pg *storage.PostgresStore, logger *slog.Logger) (userID, orgID string, orgNumeric uint64, err error) {
	count, err := pg.CountOrgs(ctx)
	if err != nil {
		return "", "", 0, fmt.Errorf("count orgs: %w", err)
	}

	if count > 0 {
		// Get existing default user
		user, _, err := pg.CreateOrGetUser(ctx, "admin@localhost", "Admin", "")
		if err != nil {
			return "", "", 0, fmt.Errorf("get default user: %w", err)
		}

		org, err := pg.GetOrgByUserID(ctx, user.ID)
		if err != nil {
			return "", "", 0, fmt.Errorf("get org for user: %w", err)
		}
		if org == nil {
			return "", "", 0, fmt.Errorf("no org found for default user")
		}

		return user.ID, org.ID, domain.OrgIDToUint64(org.ID), nil
	}

	// Create default org
	org, err := pg.CreateOrg(ctx, "Default Organization", "default")
	if err != nil {
		return "", "", 0, fmt.Errorf("create default org: %w", err)
	}

	// Create default user
	user, _, err := pg.CreateOrGetUser(ctx, "admin@localhost", "Admin", "")
	if err != nil {
		return "", "", 0, fmt.Errorf("create default user: %w", err)
	}

	if err := pg.AddOrgMember(ctx, org.ID, user.ID, "owner"); err != nil {
		return "", "", 0, fmt.Errorf("add org member: %w", err)
	}

	// Create default project
	project := &domain.Project{
		OrgID:      org.ID,
		Name:       "Default",
		Slug:       "default",
		PIIEnabled: true,
		IsDefault:  true,
	}
	if err := pg.CreateProject(ctx, project); err != nil {
		logger.Warn("bootstrap: create default project", "error", err)
	}

	// Create default API key
	plaintext, keyHash, keyPrefix, keyErr := auth.GenerateKey(auth.PrefixLive)
	if keyErr == nil {
		_, err := pg.CreateAPIKey(ctx, org.ID, "Default Key", keyPrefix, keyHash, []string{"write", "read"}, nil)
		if err != nil {
			logger.Warn("bootstrap: create default api key", "error", err)
		} else {
			logger.Info("bootstrap: default API key created",
				"prefix", keyPrefix)
			_ = plaintext // logged only in bootstrap
		}
	}

	logger.Info("bootstrap: self-hosted defaults created",
		"org_id", org.ID,
		"user_id", user.ID,
	)

	return user.ID, org.ID, domain.OrgIDToUint64(org.ID), nil
}
