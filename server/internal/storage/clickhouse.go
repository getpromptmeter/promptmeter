package storage

import (
	"context"
	"fmt"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/google/uuid"
	"github.com/promptmeter/promptmeter/server/internal/domain"
)

// ClickHouseStore implements EventWriter and PendingEventsStore.
type ClickHouseStore struct {
	conn driver.Conn
}

// NewClickHouseStore creates a new ClickHouse store.
func NewClickHouseStore(ctx context.Context, dsn string) (*ClickHouseStore, error) {
	opts, err := clickhouse.ParseDSN(dsn)
	if err != nil {
		return nil, fmt.Errorf("clickhouse: parse dsn: %w", err)
	}

	conn, err := clickhouse.Open(opts)
	if err != nil {
		return nil, fmt.Errorf("clickhouse: open: %w", err)
	}

	if err := conn.Ping(ctx); err != nil {
		return nil, fmt.Errorf("clickhouse: ping: %w", err)
	}

	return &ClickHouseStore{conn: conn}, nil
}

// Close closes the ClickHouse connection.
func (s *ClickHouseStore) Close() error {
	return s.conn.Close()
}

// InsertEvents performs a batch insert of events into the events table.
func (s *ClickHouseStore) InsertEvents(ctx context.Context, events []domain.Event) error {
	if len(events) == 0 {
		return nil
	}

	batch, err := s.conn.PrepareBatch(ctx, `
		INSERT INTO events (
			org_id, event_id, project_id, timestamp, inserted_at,
			model, provider, prompt_tokens, completion_tokens, total_tokens,
			cost_usd, latency_ms, status_code, tags,
			prompt_hash, s3_key, s3_status, schema_version
		)
	`)
	if err != nil {
		return fmt.Errorf("clickhouse: prepare batch: %w", err)
	}

	now := time.Now()
	for _, e := range events {
		eventUUID, err := uuid.Parse(e.EventID)
		if err != nil {
			return fmt.Errorf("clickhouse: parse event_id %q: %w", e.EventID, err)
		}

		s3Status := e.S3Status
		if s3Status == "" {
			s3Status = domain.S3StatusNone
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
			e.PromptHash,
			e.S3Key,
			s3Status,
			uint8(e.SchemaVersion),
		); err != nil {
			return fmt.Errorf("clickhouse: append row: %w", err)
		}
	}

	if err := batch.Send(); err != nil {
		return fmt.Errorf("clickhouse: send batch: %w", err)
	}
	return nil
}

// GetPendingS3Events returns events with s3_status='pending' for the reconciler.
func (s *ClickHouseStore) GetPendingS3Events(ctx context.Context, limit int) ([]domain.Event, error) {
	query := `
		SELECT org_id, event_id, project_id, timestamp, model, provider,
		       prompt_tokens, completion_tokens, total_tokens, cost_usd,
		       latency_ms, status_code, tags, prompt_hash, s3_key, s3_status
		FROM events
		WHERE s3_status = 'pending'
		LIMIT $1
	`
	rows, err := s.conn.Query(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("clickhouse: get pending s3: %w", err)
	}
	defer rows.Close()

	var events []domain.Event
	for rows.Next() {
		var e domain.Event
		var eventUUID uuid.UUID
		var statusCode uint16
		if err := rows.Scan(
			&e.OrgID, &eventUUID, &e.ProjectID, &e.Timestamp,
			&e.Model, &e.Provider, &e.PromptTokens, &e.CompletionTokens,
			&e.TotalTokens, &e.CostUSD, &e.LatencyMs, &statusCode,
			&e.Tags, &e.PromptHash, &e.S3Key, &e.S3Status,
		); err != nil {
			return nil, fmt.Errorf("clickhouse: scan pending event: %w", err)
		}
		e.EventID = eventUUID.String()
		e.StatusCode = uint32(statusCode)
		events = append(events, e)
	}
	return events, rows.Err()
}

// UpdateS3Status updates the S3 status and key for an event.
// Uses INSERT with newer inserted_at to leverage ReplacingMergeTree.
func (s *ClickHouseStore) UpdateS3Status(ctx context.Context, eventID string, status string, s3Key string) error {
	query := `
		INSERT INTO events (org_id, event_id, project_id, timestamp, inserted_at,
			model, provider, prompt_tokens, completion_tokens, total_tokens,
			cost_usd, latency_ms, status_code, tags, prompt_hash, s3_key, s3_status, schema_version)
		SELECT org_id, event_id, project_id, timestamp, now64(3),
			model, provider, prompt_tokens, completion_tokens, total_tokens,
			cost_usd, latency_ms, status_code, tags, prompt_hash,
			$1, $2, schema_version
		FROM events
		WHERE event_id = $3
		LIMIT 1
	`
	return s.conn.Exec(ctx, query, s3Key, status, eventID)
}

// Conn returns the underlying ClickHouse connection for tests.
func (s *ClickHouseStore) Conn() driver.Conn {
	return s.conn
}
