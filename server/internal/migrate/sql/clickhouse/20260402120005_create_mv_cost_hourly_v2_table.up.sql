CREATE TABLE IF NOT EXISTS mv_cost_hourly_v2_target
(
    org_id          UInt64,
    project_id      LowCardinality(String),
    hour            DateTime,
    total_cost      Float64,
    total_tokens    UInt64,
    request_count   UInt64,
    error_count     UInt64,
    avg_latency_ms  AggregateFunction(avg, UInt32),
    p95_latency_ms  AggregateFunction(quantile(0.95), UInt32)
)
ENGINE = AggregatingMergeTree()
ORDER BY (org_id, project_id, hour)
PARTITION BY toYYYYMM(hour);
