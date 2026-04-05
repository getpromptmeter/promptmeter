-- Add API key -> project association.
-- NULL = org-wide key (works with all projects).
-- On project deletion, key becomes org-wide (ON DELETE SET NULL).
ALTER TABLE api_keys ADD COLUMN project_id UUID REFERENCES projects(id) ON DELETE SET NULL;
