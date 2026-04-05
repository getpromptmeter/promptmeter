-- Add organization settings for Dashboard.
-- timezone: IANA timezone for grouping data by day.
-- pii_enabled: global toggle for storing prompt/response text.
-- slack_webhook_url: for sending notifications (alerts).
ALTER TABLE organizations ADD COLUMN timezone TEXT NOT NULL DEFAULT 'UTC';
ALTER TABLE organizations ADD COLUMN pii_enabled BOOLEAN NOT NULL DEFAULT true;
ALTER TABLE organizations ADD COLUMN slack_webhook_url TEXT;
