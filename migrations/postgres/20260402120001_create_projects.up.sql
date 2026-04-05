-- Projects: logical grouping of API keys, events, and alerts within an organization.
-- Analogous to Project in Sentry. org_id remains the primary isolation boundary,
-- project_id is a filter within org.
-- On org bootstrap, a Default project is automatically created (is_default=true).
-- Keys and alerts without explicit project binding are org-wide (project_id=NULL).
CREATE TABLE projects (
    id              UUID            PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id          UUID            NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    name            TEXT            NOT NULL,
    slug            TEXT            NOT NULL,
    description     TEXT            DEFAULT '',
    pii_enabled     BOOLEAN         NOT NULL DEFAULT true,
    is_default      BOOLEAN         NOT NULL DEFAULT false,
    created_at      TIMESTAMPTZ     NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ     NOT NULL DEFAULT now(),
    archived_at     TIMESTAMPTZ,

    UNIQUE (org_id, slug)
);

-- Only active projects (non-archived) -- primary lookup for API requests.
CREATE INDEX idx_projects_org ON projects (org_id) WHERE archived_at IS NULL;
