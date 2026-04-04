CREATE TABLE IF NOT EXISTS mv_cost_by_model_daily_target
(
    org_id          UInt64,
    date            Date,
    model           LowCardinality(String),
    provider        LowCardinality(String),
    total_cost      Float64,
    total_tokens    UInt64,
    request_count   UInt64
)
ENGINE = SummingMergeTree((total_cost, total_tokens, request_count))
ORDER BY (org_id, date, model, provider)
PARTITION BY toYYYYMM(date);

CREATE MATERIALIZED VIEW IF NOT EXISTS mv_cost_by_model_daily
TO mv_cost_by_model_daily_target
AS
SELECT
    org_id,
    toDate(timestamp)            AS date,
    model,
    provider,
    sum(cost_usd)                AS total_cost,
    sum(total_tokens)            AS total_tokens,
    count()                      AS request_count
FROM events
GROUP BY org_id, date, model, provider;
