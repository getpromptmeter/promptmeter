CREATE MATERIALIZED VIEW IF NOT EXISTS mv_cost_by_model_daily_v2
TO mv_cost_by_model_daily_v2_target
AS
SELECT
    org_id,
    project_id,
    toDate(timestamp)            AS date,
    model,
    provider,
    sum(cost_usd)                AS total_cost,
    sum(total_tokens)            AS total_tokens,
    count()                      AS request_count
FROM events
GROUP BY org_id, project_id, date, model, provider;
