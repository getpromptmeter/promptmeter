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

// --- DashboardReader implementation ---

// selectMV returns the appropriate MV target table based on whether a project filter is set.
// v2 tables include project_id in ORDER BY, v1 are org-wide only.
func selectMV(base, projectID string) string {
	if projectID != "" {
		return base + "_v2_target"
	}
	return base + "_target"
}

// namedArgs is a helper to build clickhouse named parameter slices.
func namedArgs(pairs ...any) []any {
	return pairs
}

// timeArgs returns standard named args for org_id + from/to as UInt32 unix timestamps.
func timeArgs(params DashboardQueryParams) []any {
	return namedArgs(
		clickhouse.Named("org_id", params.OrgID),
		clickhouse.Named("from", uint32(params.From.Unix())),
		clickhouse.Named("to", uint32(params.To.Unix())),
	)
}

// GetOverviewKPIs returns aggregated KPIs for a time period from mv_cost_hourly.
func (s *ClickHouseStore) GetOverviewKPIs(ctx context.Context, params DashboardQueryParams) (*domain.OverviewKPIs, error) {
	table := selectMV("mv_cost_hourly", params.ProjectID)

	args := timeArgs(params)

	query := fmt.Sprintf(`
		SELECT
			coalesce(sum(total_cost), 0) AS total_cost,
			coalesce(sum(request_count), 0) AS total_requests,
			coalesce(sum(error_count), 0) AS total_errors,
			if(count() > 0, avgMerge(avg_latency_ms), 0) AS avg_latency
		FROM %s
		WHERE org_id = {org_id:UInt64}
		  AND hour >= toDateTime({from:UInt32})
		  AND hour < toDateTime({to:UInt32})
	`, table)

	if params.ProjectID != "" {
		query = fmt.Sprintf(`
			SELECT
				coalesce(sum(total_cost), 0) AS total_cost,
				coalesce(sum(request_count), 0) AS total_requests,
				coalesce(sum(error_count), 0) AS total_errors,
				if(count() > 0, avgMerge(avg_latency_ms), 0) AS avg_latency
			FROM %s
			WHERE org_id = {org_id:UInt64}
			  AND project_id = {project_id:String}
			  AND hour >= toDateTime({from:UInt32})
			  AND hour < toDateTime({to:UInt32})
		`, table)
		args = append(args, clickhouse.Named("project_id", params.ProjectID))
	}

	row := s.conn.QueryRow(ctx, query, args...)

	var kpis domain.OverviewKPIs
	if err := row.Scan(&kpis.TotalCost, &kpis.TotalRequests, &kpis.TotalErrors, &kpis.AvgLatencyMs); err != nil {
		return nil, fmt.Errorf("clickhouse: get overview kpis: %w", err)
	}
	return &kpis, nil
}

// GetCostBreakdown returns cost breakdown by model or feature tag.
func (s *ClickHouseStore) GetCostBreakdown(ctx context.Context, params DashboardQueryParams, groupBy string, limit int) ([]domain.CostBreakdownItem, error) {
	switch groupBy {
	case "model":
		return s.getCostByModel(ctx, params, limit)
	case "feature":
		return s.getCostByTag(ctx, params, "feature", limit)
	case "project":
		return s.getCostByProject(ctx, params, limit)
	default:
		return s.getCostByModel(ctx, params, limit)
	}
}

func (s *ClickHouseStore) getCostByModel(ctx context.Context, params DashboardQueryParams, limit int) ([]domain.CostBreakdownItem, error) {
	table := selectMV("mv_cost_by_model_daily", params.ProjectID)

	query := fmt.Sprintf(`
		SELECT model, provider,
		       sum(total_cost) AS cost_usd,
		       sum(total_tokens) AS tokens,
		       sum(request_count) AS requests
		FROM %s
		WHERE org_id = {org_id:UInt64}
		  AND date >= toDate(toDateTime({from:UInt32}))
		  AND date < toDate(toDateTime({to:UInt32}))
	`, table)

	args := namedArgs(
		clickhouse.Named("org_id", params.OrgID),
		clickhouse.Named("from", uint32(params.From.Unix())),
		clickhouse.Named("to", uint32(params.To.Unix())),
	)

	if params.ProjectID != "" {
		query += ` AND project_id = {project_id:String}`
		args = append(args, clickhouse.Named("project_id", params.ProjectID))
	}

	query += fmt.Sprintf(`
		GROUP BY model, provider
		ORDER BY cost_usd DESC
		LIMIT %d
	`, limit)

	rows, err := s.conn.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("clickhouse: get cost by model: %w", err)
	}
	defer rows.Close()

	var items []domain.CostBreakdownItem
	for rows.Next() {
		var item domain.CostBreakdownItem
		if err := rows.Scan(&item.Group, &item.Provider, &item.CostUSD, &item.Tokens, &item.Requests); err != nil {
			return nil, fmt.Errorf("clickhouse: scan cost by model: %w", err)
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *ClickHouseStore) getCostByTag(ctx context.Context, params DashboardQueryParams, tagKey string, limit int) ([]domain.CostBreakdownItem, error) {
	table := selectMV("mv_cost_by_tag_daily", params.ProjectID)

	query := fmt.Sprintf(`
		SELECT tag_value,
		       sum(total_cost) AS cost_usd,
		       sum(request_count) AS requests
		FROM %s
		WHERE org_id = {org_id:UInt64}
		  AND date >= toDate(toDateTime({from:UInt32}))
		  AND date < toDate(toDateTime({to:UInt32}))
		  AND tag_key = {tag_key:String}
	`, table)

	args := namedArgs(
		clickhouse.Named("org_id", params.OrgID),
		clickhouse.Named("from", uint32(params.From.Unix())),
		clickhouse.Named("to", uint32(params.To.Unix())),
		clickhouse.Named("tag_key", tagKey),
	)

	if params.ProjectID != "" {
		query += ` AND project_id = {project_id:String}`
		args = append(args, clickhouse.Named("project_id", params.ProjectID))
	}

	query += fmt.Sprintf(`
		GROUP BY tag_value
		ORDER BY cost_usd DESC
		LIMIT %d
	`, limit)

	rows, err := s.conn.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("clickhouse: get cost by tag: %w", err)
	}
	defer rows.Close()

	var items []domain.CostBreakdownItem
	for rows.Next() {
		var item domain.CostBreakdownItem
		if err := rows.Scan(&item.Group, &item.CostUSD, &item.Requests); err != nil {
			return nil, fmt.Errorf("clickhouse: scan cost by tag: %w", err)
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *ClickHouseStore) getCostByProject(ctx context.Context, params DashboardQueryParams, limit int) ([]domain.CostBreakdownItem, error) {
	query := fmt.Sprintf(`
		SELECT project_id,
		       sum(total_cost) AS cost_usd,
		       sum(total_tokens) AS tokens,
		       sum(request_count) AS requests
		FROM mv_cost_by_model_daily_v2_target
		WHERE org_id = {org_id:UInt64}
		  AND date >= toDate(toDateTime({from:UInt32}))
		  AND date < toDate(toDateTime({to:UInt32}))
		GROUP BY project_id
		ORDER BY cost_usd DESC
		LIMIT %d
	`, limit)

	args := namedArgs(
		clickhouse.Named("org_id", params.OrgID),
		clickhouse.Named("from", uint32(params.From.Unix())),
		clickhouse.Named("to", uint32(params.To.Unix())),
	)

	rows, err := s.conn.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("clickhouse: get cost by project: %w", err)
	}
	defer rows.Close()

	var items []domain.CostBreakdownItem
	for rows.Next() {
		var item domain.CostBreakdownItem
		if err := rows.Scan(&item.Group, &item.CostUSD, &item.Tokens, &item.Requests); err != nil {
			return nil, fmt.Errorf("clickhouse: scan cost by project: %w", err)
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

// GetCostTimeseries returns cost timeseries data, optionally split by model.
func (s *ClickHouseStore) GetCostTimeseries(ctx context.Context, params DashboardQueryParams, groupBy string, granularity string) ([]domain.TimeseriesSeries, error) {
	if groupBy == "model" {
		return s.getTimeseriesByModel(ctx, params, granularity)
	}
	return s.getTimeseriesTotal(ctx, params, granularity)
}

func (s *ClickHouseStore) getTimeseriesTotal(ctx context.Context, params DashboardQueryParams, granularity string) ([]domain.TimeseriesSeries, error) {
	var query string
	args := namedArgs(
		clickhouse.Named("org_id", params.OrgID),
		clickhouse.Named("from", uint32(params.From.Unix())),
		clickhouse.Named("to", uint32(params.To.Unix())),
	)

	projectFilter := ""
	if params.ProjectID != "" {
		projectFilter = ` AND project_id = {project_id:String}`
		args = append(args, clickhouse.Named("project_id", params.ProjectID))
	}

	switch granularity {
	case "hour":
		table := selectMV("mv_cost_hourly", params.ProjectID)
		query = fmt.Sprintf(`
			SELECT hour AS ts,
			       sum(total_cost) AS cost_usd,
			       sum(request_count) AS requests
			FROM %s
			WHERE org_id = {org_id:UInt64} AND hour >= toDateTime({from:UInt32}) AND hour < toDateTime({to:UInt32})%s
			GROUP BY hour ORDER BY hour
		`, table, projectFilter)
	case "week":
		table := selectMV("mv_cost_by_model_daily", params.ProjectID)
		query = fmt.Sprintf(`
			SELECT toStartOfWeek(date) AS ts,
			       sum(total_cost) AS cost_usd,
			       sum(request_count) AS requests
			FROM %s
			WHERE org_id = {org_id:UInt64} AND date >= toDate(toDateTime({from:UInt32})) AND date < toDate(toDateTime({to:UInt32}))%s
			GROUP BY ts ORDER BY ts
		`, table, projectFilter)
	default: // "day"
		table := selectMV("mv_cost_by_model_daily", params.ProjectID)
		query = fmt.Sprintf(`
			SELECT date AS ts,
			       sum(total_cost) AS cost_usd,
			       sum(request_count) AS requests
			FROM %s
			WHERE org_id = {org_id:UInt64} AND date >= toDate(toDateTime({from:UInt32})) AND date < toDate(toDateTime({to:UInt32}))%s
			GROUP BY date ORDER BY date
		`, table, projectFilter)
	}

	rows, err := s.conn.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("clickhouse: get timeseries total: %w", err)
	}
	defer rows.Close()

	series := domain.TimeseriesSeries{Group: "_total"}
	for rows.Next() {
		var pt domain.TimeseriesPoint
		if err := rows.Scan(&pt.Timestamp, &pt.CostUSD, &pt.Requests); err != nil {
			return nil, fmt.Errorf("clickhouse: scan timeseries: %w", err)
		}
		series.Points = append(series.Points, pt)
	}
	return []domain.TimeseriesSeries{series}, rows.Err()
}

func (s *ClickHouseStore) getTimeseriesByModel(ctx context.Context, params DashboardQueryParams, granularity string) ([]domain.TimeseriesSeries, error) {
	table := selectMV("mv_cost_by_model_daily", params.ProjectID)

	args := namedArgs(
		clickhouse.Named("org_id", params.OrgID),
		clickhouse.Named("from", uint32(params.From.Unix())),
		clickhouse.Named("to", uint32(params.To.Unix())),
	)

	projectFilter := ""
	if params.ProjectID != "" {
		projectFilter = ` AND project_id = {project_id:String}`
		args = append(args, clickhouse.Named("project_id", params.ProjectID))
	}

	// Get top 5 models by cost
	topQuery := fmt.Sprintf(`
		SELECT model, sum(total_cost) AS cost
		FROM %s
		WHERE org_id = {org_id:UInt64} AND date >= toDate(toDateTime({from:UInt32})) AND date < toDate(toDateTime({to:UInt32}))%s
		GROUP BY model ORDER BY cost DESC LIMIT 5
	`, table, projectFilter)

	topRows, err := s.conn.Query(ctx, topQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("clickhouse: get top models: %w", err)
	}
	defer topRows.Close()

	var topModels []string
	topModelSet := make(map[string]bool)
	for topRows.Next() {
		var model string
		var cost float64
		if err := topRows.Scan(&model, &cost); err != nil {
			return nil, fmt.Errorf("clickhouse: scan top model: %w", err)
		}
		topModels = append(topModels, model)
		topModelSet[model] = true
	}

	// Get timeseries for all models, then bucket
	var tsExpr string
	switch granularity {
	case "week":
		tsExpr = "toStartOfWeek(date)"
	default:
		tsExpr = "date"
	}

	detailQuery := fmt.Sprintf(`
		SELECT %s AS ts, model,
		       sum(total_cost) AS cost_usd,
		       sum(request_count) AS requests
		FROM %s
		WHERE org_id = {org_id:UInt64} AND date >= toDate(toDateTime({from:UInt32})) AND date < toDate(toDateTime({to:UInt32}))%s
		GROUP BY ts, model ORDER BY ts
	`, tsExpr, table, projectFilter)

	detailRows, err := s.conn.Query(ctx, detailQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("clickhouse: get timeseries by model: %w", err)
	}
	defer detailRows.Close()

	// Organize into series
	seriesMap := make(map[string]*domain.TimeseriesSeries)
	for _, m := range topModels {
		seriesMap[m] = &domain.TimeseriesSeries{Group: m}
	}
	seriesMap["_other"] = &domain.TimeseriesSeries{Group: "_other"}

	for detailRows.Next() {
		var ts time.Time
		var model string
		var cost float64
		var requests uint64
		if err := detailRows.Scan(&ts, &model, &cost, &requests); err != nil {
			return nil, fmt.Errorf("clickhouse: scan timeseries model: %w", err)
		}

		targetSeries := "_other"
		if topModelSet[model] {
			targetSeries = model
		}

		sr := seriesMap[targetSeries]
		if len(sr.Points) > 0 && sr.Points[len(sr.Points)-1].Timestamp.Equal(ts) {
			sr.Points[len(sr.Points)-1].CostUSD += cost
			sr.Points[len(sr.Points)-1].Requests += requests
		} else {
			sr.Points = append(sr.Points, domain.TimeseriesPoint{
				Timestamp: ts,
				CostUSD:   cost,
				Requests:  requests,
			})
		}
	}

	var result []domain.TimeseriesSeries
	for _, m := range topModels {
		if sr := seriesMap[m]; len(sr.Points) > 0 {
			result = append(result, *sr)
		}
	}
	if other := seriesMap["_other"]; len(other.Points) > 0 {
		result = append(result, *other)
	}

	return result, detailRows.Err()
}
