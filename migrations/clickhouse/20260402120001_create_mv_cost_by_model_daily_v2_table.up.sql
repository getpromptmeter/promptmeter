-- v2 MV with project_id for project filtering and grouping.
-- project_id is included in ORDER BY of target tables (after org_id).
-- Existing v1 MV remains for backward compatibility.
-- Dashboard API uses v2 when request contains ?project= or group_by=project.
CREATE TABLE IF NOT EXISTS mv_cost_by_model_daily_v2_target
(
    org_id          UInt64,
    project_id      LowCardinality(String),
    date            Date,
    model           LowCardinality(String),
    provider        LowCardinality(String),
    total_cost      Float64,
    total_tokens    UInt64,
    request_count   UInt64
)
ENGINE = SummingMergeTree((total_cost, total_tokens, request_count))
ORDER BY (org_id, project_id, date, model, provider)
PARTITION BY toYYYYMM(date);
