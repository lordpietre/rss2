-- Add feed health monitoring columns to fuentes_url table
-- Migration: 54-add-feed-health-monitoring.sql
-- Purpose: Track feed health status for automatic deactivation and reactivation

-- System state table for tracking worker state
CREATE TABLE IF NOT EXISTS system_state (
    id INTEGER PRIMARY KEY DEFAULT 1,
    last_reactivation_check TIMESTAMP WITHOUT TIME ZONE,
    created_at TIMESTAMP WITHOUT TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITHOUT TIME ZONE DEFAULT NOW()
);

-- Add failure tracking columns to fuentes_url
ALTER TABLE fuentes_url
    ADD COLUMN IF NOT EXISTS failures_count INTEGER DEFAULT 0,
    ADD COLUMN IF NOT EXISTS last_success TIMESTAMP WITHOUT TIME ZONE,
    ADD COLUMN IF NOT EXISTS is_healthy BOOLEAN DEFAULT TRUE;

-- Add index for querying active/unhealthy feeds (for discovery worker processing)
CREATE INDEX IF NOT EXISTS idx_fuentes_url_active_healthy
    ON fuentes_url(active, is_healthy)
    WHERE active = TRUE AND is_healthy = FALSE;

-- Add index for reactivation checks (inactive feeds that might be healthy again)
CREATE INDEX IF NOT EXISTS idx_fuentes_url_inactive_for_reactivation
    ON fuentes_url(last_check ASC NULLS FIRST)
    WHERE active = FALSE;

-- Comments for documentation
COMMENT ON TABLE system_state IS 'Stores worker state like last reactivation check timestamps';
COMMENT ON COLUMN system_state.last_reactivation_check IS 'Timestamp of the last reactivation check for inactive feeds';
COMMENT ON COLUMN fuentes_url.failures_count IS 'Consecutive failed checks for this feed source';
COMMENT ON COLUMN fuentes_url.last_success IS 'Timestamp of the last successful check (returned feeds)';
COMMENT ON COLUMN fuentes_url.is_healthy IS 'True if feed is considered healthy (hasnt exceeded failure threshold)';
