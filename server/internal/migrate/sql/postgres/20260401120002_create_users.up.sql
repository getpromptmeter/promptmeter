CREATE TABLE users (
    id                      UUID            PRIMARY KEY DEFAULT gen_random_uuid(),
    email                   TEXT            NOT NULL UNIQUE,
    name                    TEXT,
    avatar_url              TEXT,
    created_at              TIMESTAMPTZ     NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX idx_users_email ON users (email);
