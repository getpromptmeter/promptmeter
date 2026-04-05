CREATE TABLE IF NOT EXISTS events
(
    org_id              UInt64,
    event_id            UUID,
    project_id          String          DEFAULT '',

    timestamp           DateTime64(3),
    inserted_at         DateTime64(3)   DEFAULT now64(3),

    model               LowCardinality(String),
    provider            LowCardinality(String),

    prompt_tokens       UInt32,
    completion_tokens   UInt32,
    total_tokens        UInt32,
    cost_usd            Float64,

    latency_ms          UInt32,
    status_code         UInt16,

    tags                Map(String, String),

    prompt_hash         String          DEFAULT '',
    s3_key              String          DEFAULT '',
    s3_status           Enum8('none' = 0, 'pending' = 1, 'uploaded' = 2),

    schema_version      UInt8           DEFAULT 1
)
ENGINE = ReplacingMergeTree(inserted_at)
ORDER BY (org_id, toDate(timestamp), event_id)
PARTITION BY toYYYYMM(timestamp)
TTL timestamp + INTERVAL 90 DAY
SETTINGS index_granularity = 8192;
