-- Feature Flags Repository SQL Queries
-- Tables: convoy.feature_flags, convoy.feature_flag_overrides, convoy.early_adopter_features

-- ============================================================================
-- convoy.feature_flags
-- ============================================================================

-- name: FetchFeatureFlagByKey :one
SELECT id, feature_key, enabled, created_at, updated_at
FROM convoy.feature_flags
WHERE feature_key = @feature_key;

-- name: FetchFeatureFlagByID :one
SELECT id, feature_key, enabled, created_at, updated_at
FROM convoy.feature_flags
WHERE id = @id;

-- name: LoadFeatureFlags :many
SELECT id, feature_key, enabled, created_at, updated_at
FROM convoy.feature_flags
ORDER BY feature_key;

-- name: UpdateFeatureFlag :exec
UPDATE convoy.feature_flags
SET enabled = @enabled, updated_at = NOW()
WHERE id = @id;

-- ============================================================================
-- convoy.feature_flag_overrides
-- ============================================================================

-- name: FetchFeatureFlagOverride :one
SELECT id, feature_flag_id, owner_type, owner_id, enabled, enabled_at, enabled_by, created_at, updated_at
FROM convoy.feature_flag_overrides
WHERE owner_type = @owner_type AND owner_id = @owner_id AND feature_flag_id = @feature_flag_id;

-- name: LoadFeatureFlagOverridesByOwner :many
SELECT id, feature_flag_id, owner_type, owner_id, enabled, enabled_at, enabled_by, created_at, updated_at
FROM convoy.feature_flag_overrides
WHERE owner_type = @owner_type AND owner_id = @owner_id;

-- name: LoadFeatureFlagOverridesByFeatureFlag :many
SELECT id, feature_flag_id, owner_type, owner_id, enabled, enabled_at, enabled_by, created_at, updated_at
FROM convoy.feature_flag_overrides
WHERE feature_flag_id = @feature_flag_id;

-- name: UpsertFeatureFlagOverride :exec
INSERT INTO convoy.feature_flag_overrides (id, feature_flag_id, owner_type, owner_id, enabled, enabled_at, enabled_by)
VALUES (@id, @feature_flag_id, @owner_type, @owner_id, @enabled, @enabled_at, @enabled_by)
ON CONFLICT (owner_type, owner_id, feature_flag_id)
DO UPDATE SET enabled = @enabled, enabled_at = @enabled_at, enabled_by = @enabled_by, updated_at = NOW();

-- name: DeleteFeatureFlagOverride :exec
DELETE FROM convoy.feature_flag_overrides
WHERE owner_type = @owner_type AND owner_id = @owner_id AND feature_flag_id = @feature_flag_id;

-- ============================================================================
-- convoy.early_adopter_features
-- ============================================================================

-- name: FetchEarlyAdopterFeature :one
SELECT id, organisation_id, feature_key, enabled, enabled_by, enabled_at, created_at, updated_at
FROM convoy.early_adopter_features
WHERE organisation_id = @organisation_id AND feature_key = @feature_key;

-- name: LoadEarlyAdopterFeaturesByOrg :many
SELECT id, organisation_id, feature_key, enabled, enabled_by, enabled_at, created_at, updated_at
FROM convoy.early_adopter_features
WHERE organisation_id = @organisation_id
ORDER BY feature_key;

-- name: UpsertEarlyAdopterFeature :exec
INSERT INTO convoy.early_adopter_features (id, organisation_id, feature_key, enabled, enabled_by, enabled_at)
VALUES (@id, @organisation_id, @feature_key, @enabled, @enabled_by, @enabled_at)
ON CONFLICT (organisation_id, feature_key)
DO UPDATE SET enabled = @enabled, enabled_by = @enabled_by, enabled_at = @enabled_at, updated_at = NOW();

-- name: DeleteEarlyAdopterFeature :exec
DELETE FROM convoy.early_adopter_features
WHERE organisation_id = @organisation_id AND feature_key = @feature_key;
