package storage

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/promptmeter/promptmeter/server/internal/domain"
)

// PostgresStore implements APIKeyStore and PriceStore using PostgreSQL.
type PostgresStore struct {
	pool *pgxpool.Pool
}

// NewPostgresStore creates a new PostgreSQL store with a connection pool.
func NewPostgresStore(ctx context.Context, connStr string) (*PostgresStore, error) {
	pool, err := pgxpool.New(ctx, connStr)
	if err != nil {
		return nil, fmt.Errorf("postgres: connect: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("postgres: ping: %w", err)
	}
	return &PostgresStore{pool: pool}, nil
}

// Close closes the connection pool.
func (s *PostgresStore) Close() {
	s.pool.Close()
}

// GetAPIKeyByHash looks up a non-revoked API key by its SHA-256 hash.
func (s *PostgresStore) GetAPIKeyByHash(ctx context.Context, keyHash string) (*domain.APIKey, error) {
	query := `
		SELECT ak.id, ak.org_id, ak.key_prefix, ak.key_hash, ak.name, ak.scopes,
		       ak.last_used_at, ak.created_at, ak.revoked_at, o.tier
		FROM api_keys ak
		JOIN organizations o ON o.id = ak.org_id
		WHERE ak.key_hash = $1
	`
	row := s.pool.QueryRow(ctx, query, keyHash)

	var key domain.APIKey
	err := row.Scan(
		&key.ID, &key.OrgID, &key.KeyPrefix, &key.KeyHash, &key.Name,
		&key.Scopes, &key.LastUsedAt, &key.CreatedAt, &key.RevokedAt, &key.Tier,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("postgres: get api key: %w", err)
	}
	return &key, nil
}

// GetAllPrices returns all model prices from the model_prices table.
func (s *PostgresStore) GetAllPrices(ctx context.Context) ([]domain.ModelPrice, error) {
	query := `
		SELECT id, provider, model_name, input_price_per_million,
		       output_price_per_million, effective_from, created_at
		FROM model_prices
		ORDER BY provider, model_name, effective_from DESC
	`
	rows, err := s.pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("postgres: get prices: %w", err)
	}
	defer rows.Close()

	var prices []domain.ModelPrice
	for rows.Next() {
		var p domain.ModelPrice
		if err := rows.Scan(
			&p.ID, &p.Provider, &p.ModelName, &p.InputPricePerMillion,
			&p.OutputPricePerMillion, &p.EffectiveFrom, &p.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("postgres: scan price: %w", err)
		}
		prices = append(prices, p)
	}
	return prices, rows.Err()
}

// Pool returns the underlying connection pool for use in tests or custom queries.
func (s *PostgresStore) Pool() *pgxpool.Pool {
	return s.pool
}
