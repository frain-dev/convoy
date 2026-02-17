-- +migrate Up
SET lock_timeout = '2s';
SET statement_timeout = '30s';
CREATE TABLE IF NOT EXISTS convoy.early_adopter_features (
    id              VARCHAR NOT NULL PRIMARY KEY,
    organisation_id VARCHAR NOT NULL,
    feature_key     VARCHAR NOT NULL,  -- 'mtls' or 'oauth-token-exchange'
    enabled         BOOLEAN NOT NULL DEFAULT false,
    enabled_by      VARCHAR,            -- User ID who enabled it
    enabled_at      TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    
    UNIQUE(organisation_id, feature_key),
    FOREIGN KEY (organisation_id) REFERENCES convoy.organisations(id) ON DELETE CASCADE
);

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_early_adopter_features_org ON convoy.early_adopter_features(organisation_id);
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_early_adopter_features_key ON convoy.early_adopter_features(feature_key);

INSERT INTO convoy.early_adopter_features (id, organisation_id, feature_key, enabled, enabled_by, enabled_at, created_at, updated_at)
SELECT 
    convoy.generate_ulid(),
    ffo.owner_id,
    ff.feature_key,
    ffo.enabled,
    ffo.enabled_by,
    ffo.enabled_at,
    ffo.created_at,
    ffo.updated_at
FROM convoy.feature_flag_overrides ffo
INNER JOIN convoy.feature_flags ff ON ffo.feature_flag_id = ff.id
INNER JOIN convoy.organisations o ON ffo.owner_id = o.id
WHERE ff.feature_key IN ('mtls', 'oauth-token-exchange')
  AND ffo.owner_type = 'organisation'
ON CONFLICT (organisation_id, feature_key) DO NOTHING;

DELETE FROM convoy.feature_flag_overrides
WHERE feature_flag_id IN (
    SELECT id FROM convoy.feature_flags WHERE feature_key IN ('mtls', 'oauth-token-exchange')
);

DELETE FROM convoy.feature_flag_overrides
WHERE feature_flag_id IN (
    SELECT id FROM convoy.feature_flags WHERE feature_key IN ('full-text-search', 'retention-policy', 'ip-rules')
);

DELETE FROM convoy.feature_flags
WHERE feature_key IN ('full-text-search', 'retention-policy', 'ip-rules', 'mtls', 'oauth-token-exchange');

ALTER TABLE convoy.feature_flags DROP COLUMN IF EXISTS allow_override;

-- +migrate Down
ALTER TABLE convoy.feature_flags ADD COLUMN IF NOT EXISTS allow_override BOOLEAN NOT NULL DEFAULT false;

INSERT INTO convoy.feature_flags (id, feature_key, enabled, allow_override) VALUES
    (convoy.generate_ulid(), 'mtls', false, true),
    (convoy.generate_ulid(), 'oauth-token-exchange', false, true),
    (convoy.generate_ulid(), 'ip-rules', false, true),
    (convoy.generate_ulid(), 'retention-policy', false, true),
    (convoy.generate_ulid(), 'full-text-search', false, true)
ON CONFLICT (feature_key) DO NOTHING;

INSERT INTO convoy.feature_flag_overrides (id, feature_flag_id, owner_type, owner_id, enabled, enabled_at, enabled_by, created_at, updated_at)
SELECT 
    convoy.generate_ulid(),
    ff.id,
    'organisation',
    eaf.organisation_id,
    eaf.enabled,
    eaf.enabled_at,
    eaf.enabled_by,
    eaf.created_at,
    eaf.updated_at
FROM convoy.early_adopter_features eaf
INNER JOIN convoy.feature_flags ff ON ff.feature_key = eaf.feature_key
ON CONFLICT (owner_type, owner_id, feature_flag_id) DO NOTHING;

DROP INDEX CONCURRENTLY IF EXISTS idx_early_adopter_features_key;
DROP INDEX CONCURRENTLY IF EXISTS idx_early_adopter_features_org;
DROP TABLE IF EXISTS convoy.early_adopter_features;
