CREATE TABLE model_prices (
    id                          UUID            PRIMARY KEY DEFAULT gen_random_uuid(),
    provider                    TEXT            NOT NULL,
    model_name                  TEXT            NOT NULL,
    input_price_per_million     FLOAT8          NOT NULL,
    output_price_per_million    FLOAT8          NOT NULL,
    effective_from              DATE            NOT NULL,
    created_at                  TIMESTAMPTZ     NOT NULL DEFAULT now(),

    UNIQUE (provider, model_name, effective_from)
);

CREATE INDEX idx_model_prices_lookup ON model_prices (provider, model_name, effective_from DESC);
