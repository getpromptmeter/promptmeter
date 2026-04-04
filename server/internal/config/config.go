// Package config provides environment-based configuration for Promptmeter services.
package config

import (
	"os"
	"strconv"
	"time"
)

// Ingestion holds configuration for the Ingestion API server.
type Ingestion struct {
	Port           string
	PGURL          string
	RedisURL       string
	NATSURL        string
	DeploymentMode string
}

// LoadIngestion loads ingestion config from environment variables.
func LoadIngestion() Ingestion {
	return Ingestion{
		Port:           envOr("INGESTION_PORT", "8443"),
		PGURL:          envOr("PG_URL", "postgres://promptmeter:promptmeter@localhost:5432/promptmeter?sslmode=disable"),
		RedisURL:       envOr("REDIS_URL", "redis://localhost:6379"),
		NATSURL:        envOr("NATS_URL", "nats://localhost:4222"),
		DeploymentMode: envOr("DEPLOYMENT_MODE", "self-hosted"),
	}
}

// Worker holds configuration for the Worker service.
type Worker struct {
	PGURL                string
	CHURL                string
	NATSURL              string
	S3Endpoint           string
	S3Bucket             string
	S3AccessKey          string
	S3SecretKey          string
	S3UseSSL             bool
	BatchSize            int
	FlushInterval        time.Duration
	WorkerCount          int
	PriceRefreshInterval time.Duration
}

// LoadWorker loads worker config from environment variables.
func LoadWorker() Worker {
	return Worker{
		PGURL:                envOr("PG_URL", "postgres://promptmeter:promptmeter@localhost:5432/promptmeter?sslmode=disable"),
		CHURL:                envOr("CH_URL", "clickhouse://localhost:9000/promptmeter"),
		NATSURL:              envOr("NATS_URL", "nats://localhost:4222"),
		S3Endpoint:           envOr("S3_ENDPOINT", "localhost:9001"),
		S3Bucket:             envOr("S3_BUCKET", "promptmeter-prompts"),
		S3AccessKey:          envOr("S3_ACCESS_KEY", "promptmeter"),
		S3SecretKey:          envOr("S3_SECRET_KEY", "promptmeter"),
		S3UseSSL:             envOr("S3_USE_SSL", "false") == "true",
		BatchSize:            envInt("BATCH_SIZE", 10000),
		FlushInterval:        envDuration("FLUSH_INTERVAL", 5*time.Second),
		WorkerCount:          envInt("WORKER_COUNT", 3),
		PriceRefreshInterval: envDuration("PRICE_REFRESH_INTERVAL", 5*time.Minute),
	}
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envInt(key string, fallback int) int {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return n
}

func envDuration(key string, fallback time.Duration) time.Duration {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return fallback
	}
	return d
}
