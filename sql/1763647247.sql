-- +migrate Up
-- Create feature_flags table for org-level feature flag definitions
CREATE TABLE IF NOT EXISTS convoy.feature_flags (
    id              VARCHAR NOT NULL PRIMARY KEY,
    feature_key     VARCHAR NOT NULL UNIQUE,
    enabled         BOOLEAN NOT NULL DEFAULT false,
    allow_override  BOOLEAN NOT NULL DEFAULT false,
    created_at      TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMP NOT NULL DEFAULT NOW()
);

-- Create feature_flag_overrides table for per-org overrides
CREATE TABLE IF NOT EXISTS convoy.feature_flag_overrides (
    id              VARCHAR NOT NULL PRIMARY KEY,
    feature_flag_id VARCHAR NOT NULL,
    owner_type      VARCHAR NOT NULL,
    owner_id        VARCHAR NOT NULL,
    enabled         BOOLEAN NOT NULL DEFAULT false,
    enabled_at      TIMESTAMP,
    enabled_by      VARCHAR,
    created_at      TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMP NOT NULL DEFAULT NOW(),
    
    UNIQUE(owner_type, owner_id, feature_flag_id),
    FOREIGN KEY (feature_flag_id) REFERENCES convoy.feature_flags(id) ON DELETE CASCADE
);

-- Create indexes
CREATE INDEX IF NOT EXISTS idx_feature_flag_overrides_owner ON convoy.feature_flag_overrides(owner_type, owner_id);
CREATE INDEX IF NOT EXISTS idx_feature_flag_overrides_feature_flag ON convoy.feature_flag_overrides(feature_flag_id);

-- Insert initial org-level feature flags
INSERT INTO convoy.feature_flags (id, feature_key, enabled, allow_override) VALUES
    (convoy.generate_ulid(), 'circuit-breaker', false, false),     -- System-controlled, binding (no overrides)
    (convoy.generate_ulid(), 'mtls', false, true),                -- User-controlled, can toggle per org
    (convoy.generate_ulid(), 'oauth-token-exchange', false, true), -- User-controlled, can toggle per org
    (convoy.generate_ulid(), 'ip-rules', false, true),            -- System-controlled, can exclude orgs
    (convoy.generate_ulid(), 'retention-policy', false, true),     -- System-controlled, can exclude orgs
    (convoy.generate_ulid(), 'full-text-search', false, true)       -- System-controlled, can exclude orgs
ON CONFLICT (feature_key) DO NOTHING;

-- +migrate Down
DROP INDEX IF EXISTS idx_feature_flag_overrides_feature_flag;
DROP INDEX IF EXISTS idx_feature_flag_overrides_owner;

DROP TABLE IF EXISTS convoy.feature_flag_overrides;
DROP TABLE IF EXISTS convoy.feature_flags;

