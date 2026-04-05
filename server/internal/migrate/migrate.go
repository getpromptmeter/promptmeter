// Package migrate embeds SQL migration files and applies them at startup.
package migrate

import (
	"embed"
	"errors"
	"fmt"
	"log/slog"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/clickhouse"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"
)

//go:embed sql/postgres/*.sql
var pgFS embed.FS

//go:embed sql/clickhouse/*.sql
var chFS embed.FS

// RunPostgres applies all pending PostgreSQL migrations. It is idempotent:
// if the database is already at the latest version, it does nothing.
func RunPostgres(pgURL string, logger *slog.Logger) error {
	return run("postgres", pgFS, "sql/postgres", pgURL, logger)
}

// RunClickHouse applies all pending ClickHouse migrations. It is idempotent:
// if the database is already at the latest version, it does nothing.
func RunClickHouse(chURL string, logger *slog.Logger) error {
	return run("clickhouse", chFS, "sql/clickhouse", chURL, logger)
}

func run(name string, fs embed.FS, dir string, dbURL string, logger *slog.Logger) error {
	src, err := iofs.New(fs, dir)
	if err != nil {
		return fmt.Errorf("migrations: %s: create source: %w", name, err)
	}

	m, err := migrate.NewWithSourceInstance("iofs", src, dbURL)
	if err != nil {
		return fmt.Errorf("migrations: %s: init: %w", name, err)
	}
	defer m.Close()

	err = m.Up()
	if errors.Is(err, migrate.ErrNoChange) {
		logger.Info("migrations: up to date", "database", name)
		return nil
	}
	if err != nil {
		return fmt.Errorf("migrations: %s: up: %w", name, err)
	}

	version, dirty, _ := m.Version()
	logger.Info("migrations: applied", "database", name, "version", version, "dirty", dirty)
	return nil
}
