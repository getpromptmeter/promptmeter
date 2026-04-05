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
