-- Configuration Queries
-- SQLc queries for Configuration repository operations

-- name: CreateConfiguration :exec
INSERT INTO convoy.configurations (
	id,
	is_analytics_enabled,
	is_signup_enabled,
	storage_policy_type,
	on_prem_path,
	s3_prefix,
	s3_bucket,
	s3_access_key,
	s3_secret_key,
	s3_region,
	s3_session_token,
	s3_endpoint,
	retention_policy_policy,
	retention_policy_enabled
) VALUES (
	@id,
	@is_analytics_enabled,
	@is_signup_enabled,
	@storage_policy_type,
	@on_prem_path,
	@s3_prefix,
	@s3_bucket,
	@s3_access_key,
	@s3_secret_key,
	@s3_region,
	@s3_session_token,
	@s3_endpoint,
	@retention_policy_policy,
	@retention_policy_enabled
);

-- name: LoadConfiguration :one
-- Loads the single configuration (should only be one row)
SELECT
	id,
	is_analytics_enabled,
	is_signup_enabled,
	storage_policy_type,
	on_prem_path,
	s3_prefix,
	s3_bucket,
	s3_access_key,
	s3_secret_key,
	s3_region,
	s3_session_token,
	s3_endpoint,
	retention_policy_policy,
	retention_policy_enabled,
	created_at,
	updated_at,
	deleted_at
FROM convoy.configurations
WHERE deleted_at IS NULL
LIMIT 1;

-- name: UpdateConfiguration :execresult
UPDATE convoy.configurations
SET
	is_analytics_enabled = @is_analytics_enabled,
	is_signup_enabled = @is_signup_enabled,
	storage_policy_type = @storage_policy_type,
	on_prem_path = @on_prem_path,
	s3_prefix = @s3_prefix,
	s3_bucket = @s3_bucket,
	s3_access_key = @s3_access_key,
	s3_secret_key = @s3_secret_key,
	s3_region = @s3_region,
	s3_session_token = @s3_session_token,
	s3_endpoint = @s3_endpoint,
	retention_policy_policy = @retention_policy_policy,
	retention_policy_enabled = @retention_policy_enabled,
	updated_at = NOW()
WHERE id = @id AND deleted_at IS NULL;
