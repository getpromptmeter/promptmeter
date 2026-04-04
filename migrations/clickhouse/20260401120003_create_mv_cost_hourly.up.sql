CREATE TABLE IF NOT EXISTS mv_cost_hourly_target
(
    org_id          UInt64,
    hour            DateTime,
    total_cost      Float64,
    total_tokens    UInt64,
    request_count   UInt64,
    error_count     UInt64,
    avg_latency_ms  AggregateFunction(avg, UInt32),
    p95_latency_ms  AggregateFunction(quantile(0.95), UInt32)
)
ENGINE = AggregatingMergeTree()
ORDER BY (org_id, hour)
PARTITION BY toYYYYMM(hour);

CREATE MATERIALIZED VIEW IF NOT EXISTS mv_cost_hourly
TO mv_cost_hourly_target
AS
SELECT
    org_id,
    toStartOfHour(timestamp)                        AS hour,
    sum(cost_usd)                                   AS total_cost,
    sum(total_tokens)                                AS total_tokens,
    count()                                          AS request_count,
    countIf(status_code >= 400)                      AS error_count,
    avgState(latency_ms)                             AS avg_latency_ms,
    quantileState(0.95)(latency_ms)                  AS p95_latency_ms
FROM events
GROUP BY org_id, hour;
