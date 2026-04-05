CREATE TABLE IF NOT EXISTS mv_cost_by_tag_daily_v2_target
(
    org_id          UInt64,
    project_id      LowCardinality(String),
    date            Date,
    tag_key         LowCardinality(String),
    tag_value       String,
    total_cost      Float64,
    request_count   UInt64
)
ENGINE = SummingMergeTree((total_cost, request_count))
ORDER BY (org_id, project_id, date, tag_key, tag_value)
PARTITION BY toYYYYMM(date);
