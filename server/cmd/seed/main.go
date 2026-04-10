// Command seed generates historical LLM event data and inserts it directly
// into ClickHouse. It is intended for development and demo environments.
package main

import (
	"context"
	"crypto/sha256"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"sort"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/promptmeter/promptmeter/server/internal/datagen"
	"github.com/promptmeter/promptmeter/server/internal/domain"
)

// Fixed UUIDs for demo data idempotency.
const (
	demoOrgID     = "00000000-0000-0000-0000-000000000001"
	demoUserID    = "00000000-0000-0000-0000-000000000002"
	demoKeyID     = "00000000-0000-0000-0000-000000000020"
	demoAPIKey    = "pm_test_SeedDemoKeyForDev000000000000000"
	demoKeyPrefix = "pm_test_"
)

var demoProjects = []struct {
	ID        string
	Name      string
	Slug      string
	IsDefault bool
}{
	{ID: "00000000-0000-0000-0000-000000000010", Name: "Backend API", Slug: "backend-api", IsDefault: true},
	{ID: "00000000-0000-0000-0000-000000000011", Name: "Chat Support", Slug: "chat-support", IsDefault: false},
	{ID: "00000000-0000-0000-0000-000000000012", Name: "Internal Tools", Slug: "internal-tools", IsDefault: false},
}

func main() {
	var (
		days         = flag.Int("days", 30, "Number of days of historical data to generate")
		orgSlug      = flag.String("org", "demo", "Organization slug")
		eventsPerDay = flag.Int("events-per-day", 5000, "Average number of events per day")
		seed         = flag.Int64("seed", 42, "Random seed for reproducibility")
		chDSN        = flag.String("clickhouse", "", "ClickHouse DSN (default: env CH_DSN)")
		pgDSN        = flag.String("postgres", "", "PostgreSQL DSN (default: env PG_DSN)")
		batchSize    = flag.Int("batch-size", 10000, "Batch size for ClickHouse INSERT")
		drop         = flag.Bool("drop", false, "Delete existing events for the org before seeding")
	)
	flag.Parse()

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))

	if *chDSN == "" {
		*chDSN = os.Getenv("CH_DSN")
	}
	if *chDSN == "" {
		*chDSN = "clickhouse://default:promptmeter@localhost:9000/promptmeter"
	}
	if *pgDSN == "" {
		*pgDSN = os.Getenv("PG_DSN")
	}
	if *pgDSN == "" {
		*pgDSN = "postgres://promptmeter:promptmeter@localhost:5432/promptmeter?sslmode=disable"
	}

	ctx := context.Background()
	startTime := time.Now()

	// Connect to PostgreSQL.
	pgPool, err := pgxpool.New(ctx, *pgDSN)
	if err != nil {
		logger.Error("failed to connect to PostgreSQL", "error", err)
		os.Exit(1)
	}
	defer pgPool.Close()

	if err := pgPool.Ping(ctx); err != nil {
		logger.Error("failed to ping PostgreSQL", "error", err)
		os.Exit(1)
	}
	logger.Info("connected to PostgreSQL")

	// Connect to ClickHouse.
	chOpts, err := clickhouse.ParseDSN(*chDSN)
	if err != nil {
		logger.Error("failed to parse ClickHouse DSN", "error", err)
		os.Exit(1)
	}
	chConn, err := clickhouse.Open(chOpts)
	if err != nil {
		logger.Error("failed to connect to ClickHouse", "error", err)
		os.Exit(1)
	}
	defer chConn.Close()

	if err := chConn.Ping(ctx); err != nil {
		logger.Error("failed to ping ClickHouse", "error", err)
		os.Exit(1)
	}
	logger.Info("connected to ClickHouse")

	// Bootstrap PostgreSQL data.
	orgID, err := bootstrapPostgres(ctx, pgPool, *orgSlug, logger)
	if err != nil {
		logger.Error("failed to bootstrap PostgreSQL", "error", err)
		os.Exit(1)
	}
	logger.Info("PostgreSQL bootstrap complete", "org_id", orgID)

	// Load and verify model prices.
	priceTable, err := loadPrices(ctx, pgPool, logger)
	if err != nil {
		logger.Error("failed to load model prices", "error", err)
		os.Exit(1)
	}

	// Check for missing models.
	missing := datagen.CheckMissingModels(priceTable)
	if len(missing) > 0 {
		logger.Error("missing model prices -- add them to model_prices table", "models", missing)
		os.Exit(1)
	}

	orgIDUint := domain.OrgIDToUint64(orgID)

	// Build project distributions with actual project IDs.
	projects := make([]datagen.ProjectDistribution, len(demoProjects))
	for i, p := range demoProjects {
		projects[i] = datagen.ProjectDistribution{
			ProjectID: p.Slug,
			Weight:    datagen.DefaultProjectDistributions[i].Weight,
		}
	}

	// Drop existing events if requested.
	if *drop {
		logger.Info("dropping existing events", "org_id", orgIDUint)
		if err := chConn.Exec(ctx, "ALTER TABLE events DELETE WHERE org_id = $1", orgIDUint); err != nil {
			logger.Error("failed to drop events", "error", err)
			os.Exit(1)
		}
		logger.Info("existing events dropped")
	}

	// Generate and insert events.
	gen := datagen.NewGenerator(*seed, orgIDUint, priceTable, projects)

	now := time.Now().UTC()
	to := now.Truncate(24*time.Hour).AddDate(0, 0, 1) // End of today (start of tomorrow).
	from := to.AddDate(0, 0, -*days)

	logger.Info("generating events",
		"from", from.Format("2006-01-02"),
		"to", to.Format("2006-01-02"),
		"events_per_day", *eventsPerDay,
		"seed", *seed,
	)

	events := gen.GenerateBatch(from, now, *eventsPerDay)

	// Sort by timestamp for ordered insertion.
	sort.Slice(events, func(i, j int) bool {
		return events[i].Timestamp.Before(events[j].Timestamp)
	})

	logger.Info("generated events", "count", len(events))

	// Batch insert into ClickHouse.
	totalInserted := 0
	stats := make(map[string]modelStats)

	for i := 0; i < len(events); i += *batchSize {
		end := i + *batchSize
		if end > len(events) {
			end = len(events)
		}
		batch := events[i:end]

		if err := insertBatch(ctx, chConn, batch); err != nil {
			logger.Error("failed to insert batch", "error", err, "offset", i)
			os.Exit(1)
		}

		for _, e := range batch {
			s := stats[e.Model]
			s.Count++
			s.Cost += e.CostUSD
			stats[e.Model] = s
		}
		totalInserted += len(batch)

		if totalInserted%((*batchSize)*5) == 0 || end == len(events) {
			logger.Info("insert progress", "inserted", totalInserted, "total", len(events))
		}
	}

	elapsed := time.Since(startTime)

	// Print statistics.
	var totalCost float64
	fmt.Println("\n--- Seed Statistics ---")
	fmt.Printf("Total events:  %d\n", totalInserted)

	// Sort models by count for display.
	type modelRow struct {
		Model string
		Count int
		Cost  float64
	}
	var rows []modelRow
	for model, s := range stats {
		rows = append(rows, modelRow{Model: model, Count: s.Count, Cost: s.Cost})
		totalCost += s.Cost
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].Count > rows[j].Count })

	fmt.Printf("Total cost:    $%.2f\n", totalCost)
	fmt.Printf("Time elapsed:  %s\n", elapsed.Round(time.Millisecond))
	fmt.Printf("\nBy model:\n")
	for _, r := range rows {
		fmt.Printf("  %-20s %6d events  $%.2f\n", r.Model, r.Count, r.Cost)
	}
	fmt.Println()
}

type modelStats struct {
	Count int
	Cost  float64
}

func bootstrapPostgres(ctx context.Context, pool *pgxpool.Pool, orgSlug string, logger *slog.Logger) (string, error) {
	// Create organization.
	_, err := pool.Exec(ctx, `
		INSERT INTO organizations (id, name, slug, tier)
		VALUES ($1, 'Demo Org', $2, 'pro')
		ON CONFLICT (slug) DO NOTHING
	`, demoOrgID, orgSlug)
	if err != nil {
		return "", fmt.Errorf("create org: %w", err)
	}
	logger.Info("organization ready", "slug", orgSlug)

	// Create demo user. Use admin@localhost to match dashboard autologin bootstrap.
	// avatar_url must be non-NULL — dashboard's CreateOrGetUser scans into *string.
	_, err = pool.Exec(ctx, `
		INSERT INTO users (id, email, name, avatar_url)
		VALUES ($1, 'admin@localhost', 'Admin', '')
		ON CONFLICT (email) DO NOTHING
	`, demoUserID)
	if err != nil {
		return "", fmt.Errorf("create user: %w", err)
	}

	// Create org membership.
	_, err = pool.Exec(ctx, `
		INSERT INTO org_members (org_id, user_id, role)
		VALUES ($1, $2, 'owner')
		ON CONFLICT (org_id, user_id) DO NOTHING
	`, demoOrgID, demoUserID)
	if err != nil {
		return "", fmt.Errorf("create org member: %w", err)
	}

	// Create projects.
	for _, p := range demoProjects {
		_, err = pool.Exec(ctx, `
			INSERT INTO projects (id, org_id, name, slug, is_default)
			VALUES ($1, $2, $3, $4, $5)
			ON CONFLICT (org_id, slug) DO NOTHING
		`, p.ID, demoOrgID, p.Name, p.Slug, p.IsDefault)
		if err != nil {
			return "", fmt.Errorf("create project %s: %w", p.Slug, err)
		}
	}
	logger.Info("projects ready", "count", len(demoProjects))

	// Create API key.
	keyHash := fmt.Sprintf("%x", sha256.Sum256([]byte(demoAPIKey)))
	_, err = pool.Exec(ctx, `
		INSERT INTO api_keys (id, org_id, key_prefix, key_hash, name, scopes)
		VALUES ($1, $2, $3, $4, 'Seed Key', ARRAY['read','write'])
		ON CONFLICT (key_hash) DO NOTHING
	`, demoKeyID, demoOrgID, demoKeyPrefix, keyHash)
	if err != nil {
		return "", fmt.Errorf("create api key: %w", err)
	}
	logger.Info("API key ready", "prefix", demoKeyPrefix)

	return demoOrgID, nil
}

func loadPrices(ctx context.Context, pool *pgxpool.Pool, logger *slog.Logger) (datagen.PriceTable, error) {
	rows, err := pool.Query(ctx, `
		SELECT provider, model_name, input_price_per_million, output_price_per_million, effective_from
		FROM model_prices
		ORDER BY provider, model_name, effective_from DESC
	`)
	if err != nil {
		return nil, fmt.Errorf("query prices: %w", err)
	}
	defer rows.Close()

	var entries []datagen.PriceEntry
	for rows.Next() {
		var e datagen.PriceEntry
		if err := rows.Scan(&e.Provider, &e.ModelName, &e.InputPricePerMillion, &e.OutputPricePerMillion, &e.EffectiveFrom); err != nil {
			return nil, fmt.Errorf("scan price: %w", err)
		}
		entries = append(entries, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate prices: %w", err)
	}

	// If no prices exist, auto-seed them.
	if len(entries) == 0 {
		logger.Info("model_prices table is empty, inserting seed prices")
		if err := seedModelPrices(ctx, pool); err != nil {
			return nil, fmt.Errorf("seed prices: %w", err)
		}
		// Reload after seeding.
		return loadPrices(ctx, pool, logger)
	}

	pt := datagen.BuildPriceTable(entries)
	logger.Info("loaded model prices", "entries", len(entries))

	// Check if any datagen models are missing from the loaded prices.
	missing := datagen.CheckMissingModels(pt)
	if len(missing) > 0 {
		logger.Info("some datagen models missing from model_prices, inserting seed prices for them", "missing", missing)
		if err := seedModelPrices(ctx, pool); err != nil {
			return nil, fmt.Errorf("seed missing prices: %w", err)
		}
		return loadPrices(ctx, pool, logger)
	}

	return pt, nil
}

func seedModelPrices(ctx context.Context, pool *pgxpool.Pool) error {
	for _, p := range datagen.SeedModelPrices {
		_, err := pool.Exec(ctx, `
			INSERT INTO model_prices (provider, model_name, input_price_per_million, output_price_per_million, effective_from)
			VALUES ($1, $2, $3, $4, $5)
			ON CONFLICT DO NOTHING
		`, p.Provider, p.ModelName, p.InputPricePerMillion, p.OutputPricePerMillion, p.EffectiveFrom)
		if err != nil {
			return fmt.Errorf("insert price %s/%s: %w", p.Provider, p.ModelName, err)
		}
	}
	return nil
}

func insertBatch(ctx context.Context, conn driver.Conn, events []datagen.Event) error {
	if len(events) == 0 {
		return nil
	}

	batch, err := conn.PrepareBatch(ctx, `
		INSERT INTO events (
			org_id, event_id, project_id, timestamp, inserted_at,
			model, provider, prompt_tokens, completion_tokens, total_tokens,
			cost_usd, latency_ms, status_code, tags,
			prompt_hash, s3_key, s3_status, schema_version
		)
	`)
	if err != nil {
		return fmt.Errorf("prepare batch: %w", err)
	}

	now := time.Now()
	for _, e := range events {
		eventUUID, err := uuid.Parse(e.EventID)
		if err != nil {
			return fmt.Errorf("parse event_id %q: %w", e.EventID, err)
		}

		if err := batch.Append(
			e.OrgID,
			eventUUID,
			e.ProjectID,
			e.Timestamp,
			now,
			e.Model,
			e.Provider,
			e.PromptTokens,
			e.CompletionTokens,
			e.TotalTokens,
			e.CostUSD,
			e.LatencyMs,
			uint16(e.StatusCode),
			e.Tags,
			"",     // prompt_hash
			"",     // s3_key
			e.S3Status,
			e.SchemaVersion,
		); err != nil {
			return fmt.Errorf("append row: %w", err)
		}
	}

	return batch.Send()
}
