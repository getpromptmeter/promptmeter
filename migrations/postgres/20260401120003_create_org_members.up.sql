CREATE TABLE org_members (
    org_id                  UUID            NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    user_id                 UUID            NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role                    org_role        NOT NULL DEFAULT 'owner',
    joined_at               TIMESTAMPTZ     NOT NULL DEFAULT now(),

    PRIMARY KEY (org_id, user_id)
);

CREATE INDEX idx_org_members_user ON org_members (user_id);
