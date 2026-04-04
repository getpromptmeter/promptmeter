CREATE TABLE IF NOT EXISTS mv_cost_by_tag_daily_target
(
    org_id          UInt64,
    date            Date,
    tag_key         LowCardinality(String),
    tag_value       String,
    total_cost      Float64,
    request_count   UInt64
)
ENGINE = SummingMergeTree((total_cost, request_count))
ORDER BY (org_id, date, tag_key, tag_value)
PARTITION BY toYYYYMM(date);

CREATE MATERIALIZED VIEW IF NOT EXISTS mv_cost_by_tag_daily
TO mv_cost_by_tag_daily_target
AS
SELECT
    org_id,
    toDate(timestamp)            AS date,
    tag_key,
    tag_value,
    sum(cost_usd)                AS total_cost,
    count()                      AS request_count
FROM events
ARRAY JOIN
    mapKeys(tags)   AS tag_key,
    mapValues(tags) AS tag_value
GROUP BY org_id, date, tag_key, tag_value;
