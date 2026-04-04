-- Enums for Promptmeter. Forward-only migration.

CREATE TYPE org_role AS ENUM ('owner', 'admin', 'member', 'viewer');
CREATE TYPE alert_type AS ENUM ('budget', 'spike', 'error_rate');
CREATE TYPE alert_state AS ENUM ('pending', 'firing', 'resolved', 'cooldown');
CREATE TYPE org_tier AS ENUM ('free', 'pro', 'business', 'enterprise');
CREATE TYPE poll_status AS ENUM ('idle', 'polling', 'error', 'completed');
