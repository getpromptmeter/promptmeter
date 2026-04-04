CREATE TABLE api_keys (
    id                      UUID            PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id                  UUID            NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    key_prefix              TEXT            NOT NULL,
    key_hash                TEXT            NOT NULL UNIQUE,
    name                    TEXT            NOT NULL DEFAULT 'Default',
    scopes                  TEXT[]          NOT NULL DEFAULT '{read,write}',
    last_used_at            TIMESTAMPTZ,
    created_at              TIMESTAMPTZ     NOT NULL DEFAULT now(),
    revoked_at              TIMESTAMPTZ
);

CREATE INDEX idx_api_keys_org ON api_keys (org_id);
CREATE INDEX idx_api_keys_hash ON api_keys (key_hash) WHERE revoked_at IS NULL;
