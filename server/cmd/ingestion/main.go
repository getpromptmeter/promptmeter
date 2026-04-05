// Command ingestion runs the Promptmeter Ingestion API server.
package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/nats-io/nats.go/jetstream"

	"github.com/promptmeter/promptmeter/server/internal/config"
	"github.com/promptmeter/promptmeter/server/internal/ingestion"
	"github.com/promptmeter/promptmeter/server/internal/migrate"
	pmqueue "github.com/promptmeter/promptmeter/server/internal/nats"
	"github.com/promptmeter/promptmeter/server/internal/storage"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	ctx := context.Background()

	cfg := config.LoadIngestion()

	// Run database migrations before anything else
	if err := migrate.RunPostgres(cfg.PGURL, logger); err != nil {
		logger.Error("failed to run postgres migrations", "error", err)
		os.Exit(1)
	}

	// Connect to PostgreSQL
	pgStore, err := storage.NewPostgresStore(ctx, cfg.PGURL)
	if err != nil {
		logger.Error("failed to connect to postgres", "error", err)
		os.Exit(1)
	}
	defer pgStore.Close()

	// Connect to Redis
	redisStore, err := storage.NewRedisStore(ctx, cfg.RedisURL)
	if err != nil {
		logger.Error("failed to connect to redis", "error", err)
		os.Exit(1)
	}
	defer redisStore.Close()

	// Connect to NATS
	nc, err := pmqueue.Connect(cfg.NATSURL)
	if err != nil {
		logger.Error("failed to connect to nats", "error", err)
		os.Exit(1)
	}
	defer nc.Close()

	js, err := jetstream.New(nc)
	if err != nil {
		logger.Error("failed to create jetstream context", "error", err)
		os.Exit(1)
	}

	// Ensure EVENTS stream exists
	if _, err := pmqueue.EnsureStream(js); err != nil {
		logger.Error("failed to ensure nats stream", "error", err)
		os.Exit(1)
	}

	publisher := pmqueue.NewPublisher(js)
	handler := ingestion.NewHandler(publisher, logger)

	srv := ingestion.NewServer(ingestion.ServerConfig{
		Port:    cfg.Port,
		Handler: handler,
		Cache:   redisStore,
		Store:   pgStore,
		Limiter: redisStore,
		Logger:  logger,
	})

	// Graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		if err := srv.Start(); err != nil {
			logger.Error("server error", "error", err)
		}
	}()

	logger.Info("ingestion server started", "port", cfg.Port)

	<-sigCh
	logger.Info("shutdown signal received")

	if err := srv.Shutdown(10 * time.Second); err != nil {
		logger.Error("shutdown error", "error", err)
	}
}
