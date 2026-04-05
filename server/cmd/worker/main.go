// Command worker runs the Promptmeter Worker service.
package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/nats-io/nats.go/jetstream"

	"github.com/promptmeter/promptmeter/server/internal/config"
	"github.com/promptmeter/promptmeter/server/internal/migrate"
	pmqueue "github.com/promptmeter/promptmeter/server/internal/nats"
	"github.com/promptmeter/promptmeter/server/internal/storage"
	"github.com/promptmeter/promptmeter/server/internal/worker"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	ctx, cancel := context.WithCancel(context.Background())

	cfg := config.LoadWorker()

	// Run database migrations before anything else.
	// ClickHouse migrations are handled exclusively by dashboard-api to avoid
	// race conditions on the schema_migrations table.
	if err := migrate.RunPostgres(cfg.PGURL, logger); err != nil {
		logger.Error("failed to run postgres migrations", "error", err)
		os.Exit(1)
	}

	// Connect to PostgreSQL (for model prices)
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

	// Connect to S3
	s3Store, err := storage.NewS3Store(cfg.S3Endpoint, cfg.S3AccessKey, cfg.S3SecretKey, cfg.S3Bucket, cfg.S3UseSSL)
	if err != nil {
		logger.Error("failed to create s3 client", "error", err)
		os.Exit(1)
	}

	// Ensure S3 bucket exists
	if err := s3Store.EnsureBucket(ctx); err != nil {
		logger.Warn("failed to ensure s3 bucket", "error", err)
	}

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

	// Ensure NATS stream and consumer
	if _, err := pmqueue.EnsureStream(js); err != nil {
		logger.Error("failed to ensure nats stream", "error", err)
		os.Exit(1)
	}
	if _, err := pmqueue.EnsureDLQStream(js); err != nil {
		logger.Warn("failed to ensure dlq stream", "error", err)
	}

	natsConsumer, err := pmqueue.EnsureConsumer(js)
	if err != nil {
		logger.Error("failed to ensure nats consumer", "error", err)
		os.Exit(1)
	}

	// Build components
	eventConsumer := pmqueue.NewEventConsumer(js, natsConsumer, logger)
	priceCache := worker.NewPriceCache(pgStore, cfg.PriceRefreshInterval, logger)
	batchWriter := worker.NewBatchWriter(chStore, cfg.BatchSize, cfg.FlushInterval, logger)
	s3Uploader := worker.NewS3Uploader(s3Store, chStore, logger)

	w := worker.NewWorker(eventConsumer, batchWriter, priceCache, s3Uploader, cfg.WorkerCount, logger)

	// Graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigCh
		logger.Info("shutdown signal received")
		cancel()
	}()

	logger.Info("worker starting", "workers", cfg.WorkerCount, "batch_size", cfg.BatchSize)
	if err := w.Start(ctx); err != nil {
		logger.Error("worker error", "error", err)
		os.Exit(1)
	}
}
