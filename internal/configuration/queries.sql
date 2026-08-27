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
	azure_account_name,
	azure_account_key,
	azure_container_name,
	azure_endpoint,
	azure_prefix,
	retention_period,
	retention_enabled,
	webhook_archiving_enabled,
	admin_managed
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
	@azure_account_name,
	@azure_account_key,
	@azure_container_name,
	@azure_endpoint,
	@azure_prefix,
	@retention_period,
	@retention_enabled,
	@webhook_archiving_enabled,
	@admin_managed
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
	azure_account_name,
	azure_account_key,
	azure_container_name,
	azure_endpoint,
	azure_prefix,
	retention_period,
	retention_enabled,
	webhook_archiving_enabled,
	admin_managed,
	created_at,
	updated_at,
	deleted_at
FROM convoy.configurations
WHERE deleted_at IS NULL
ORDER BY updated_at DESC NULLS LAST, id DESC
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
	azure_account_name = @azure_account_name,
	azure_account_key = @azure_account_key,
	azure_container_name = @azure_container_name,
	azure_endpoint = @azure_endpoint,
	azure_prefix = @azure_prefix,
	retention_period = @retention_period,
	retention_enabled = @retention_enabled,
	webhook_archiving_enabled = @webhook_archiving_enabled,
	admin_managed = @admin_managed,
	updated_at = NOW()
WHERE id = @id AND deleted_at IS NULL;

-- name: CompleteAdminManagedMigration :execresult
UPDATE convoy.configurations
SET
	admin_managed = true,
	retention_enabled = COALESCE(retention_enabled, @retention_enabled),
	updated_at = NOW()
WHERE id = @id
	AND admin_managed IS NULL
	AND deleted_at IS NULL;
