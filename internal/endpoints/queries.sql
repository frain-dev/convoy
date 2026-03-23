-- Endpoints Queries

-- name: CreateEndpoint :exec
INSERT INTO convoy.endpoints (
    id, name, status, secrets, owner_id, url, description, http_timeout,
    rate_limit, rate_limit_duration, advanced_signatures, slack_webhook_url,
    support_email, app_id, project_id, authentication_type,
    authentication_type_api_key_header_name,
    authentication_type_api_key_header_value,
    is_encrypted, secrets_cipher,
    authentication_type_api_key_header_value_cipher,
    mtls_client_cert, mtls_client_cert_cipher,
    oauth2_config, oauth2_config_cipher,
    basic_auth_config, basic_auth_config_cipher,
    content_type
)
VALUES (
    @id, @name, @status,
    CASE WHEN @is_encrypted::boolean THEN '[]'::jsonb ELSE @secrets::jsonb END,
    @owner_id, @url, @description, @http_timeout,
    @rate_limit, @rate_limit_duration, @advanced_signatures, @slack_webhook_url,
    @support_email, @app_id, @project_id, @authentication_type,
    @authentication_type_api_key_header_name,
    CASE WHEN @is_encrypted::boolean THEN '' ELSE @authentication_type_api_key_header_value END,
    @is_encrypted::boolean,
    CASE WHEN @is_encrypted::boolean THEN pgp_sym_encrypt(@secrets::TEXT, @encryption_key) END,
    CASE WHEN @is_encrypted::boolean THEN pgp_sym_encrypt(@authentication_type_api_key_header_value, @encryption_key) END,
    CASE WHEN @is_encrypted::boolean THEN NULL ELSE @mtls_client_cert::jsonb END,
    CASE WHEN @is_encrypted::boolean THEN pgp_sym_encrypt(@mtls_client_cert::TEXT, @encryption_key) END,
    CASE WHEN @is_encrypted::boolean THEN NULL ELSE @oauth2_config::jsonb END,
    CASE WHEN @is_encrypted::boolean THEN pgp_sym_encrypt(@oauth2_config::TEXT, @encryption_key) END,
    CASE WHEN @is_encrypted::boolean THEN NULL ELSE @basic_auth_config::jsonb END,
    CASE WHEN @is_encrypted::boolean THEN pgp_sym_encrypt(@basic_auth_config::TEXT, @encryption_key) END,
    @content_type
);

-- name: FindEndpointByID :one
SELECT
    e.id, e.name, e.status, e.owner_id, e.url, e.description,
    e.http_timeout, e.rate_limit, e.rate_limit_duration, e.advanced_signatures,
    e.slack_webhook_url, e.support_email, e.app_id, e.project_id,
    CASE
        WHEN e.is_encrypted THEN pgp_sym_decrypt(e.secrets_cipher::bytea, @encryption_key)::jsonb
        ELSE e.secrets
    END AS secrets,
    e.created_at, e.updated_at,
    COALESCE(e.authentication_type, '') AS authentication_type,
    COALESCE(e.authentication_type_api_key_header_name, '') AS authentication_type_api_key_header_name,
    CASE
        WHEN e.is_encrypted THEN pgp_sym_decrypt(e.authentication_type_api_key_header_value_cipher::bytea, @encryption_key)::TEXT
        ELSE e.authentication_type_api_key_header_value
    END AS authentication_type_api_key_header_value,
    CASE
        WHEN e.is_encrypted THEN pgp_sym_decrypt(e.mtls_client_cert_cipher::bytea, @encryption_key)::jsonb
        ELSE e.mtls_client_cert
    END AS mtls_client_cert,
    CASE
        WHEN e.is_encrypted THEN pgp_sym_decrypt(e.oauth2_config_cipher::bytea, @encryption_key)::jsonb
        ELSE e.oauth2_config
    END AS oauth2_config,
    CASE
        WHEN e.is_encrypted THEN pgp_sym_decrypt(e.basic_auth_config_cipher::bytea, @encryption_key)::jsonb
        ELSE e.basic_auth_config
    END AS basic_auth_config,
    e.content_type
FROM convoy.endpoints AS e
WHERE e.deleted_at IS NULL AND e.id = @id AND e.project_id = @project_id;

-- name: FindEndpointsByIDs :many
SELECT
    e.id, e.name, e.status, e.owner_id, e.url, e.description,
    e.http_timeout, e.rate_limit, e.rate_limit_duration, e.advanced_signatures,
    e.slack_webhook_url, e.support_email, e.app_id, e.project_id,
    CASE
        WHEN e.is_encrypted THEN pgp_sym_decrypt(e.secrets_cipher::bytea, @encryption_key)::jsonb
        ELSE e.secrets
    END AS secrets,
    e.created_at, e.updated_at,
    COALESCE(e.authentication_type, '') AS authentication_type,
    COALESCE(e.authentication_type_api_key_header_name, '') AS authentication_type_api_key_header_name,
    CASE
        WHEN e.is_encrypted THEN pgp_sym_decrypt(e.authentication_type_api_key_header_value_cipher::bytea, @encryption_key)::TEXT
        ELSE e.authentication_type_api_key_header_value
    END AS authentication_type_api_key_header_value,
    CASE
        WHEN e.is_encrypted THEN pgp_sym_decrypt(e.mtls_client_cert_cipher::bytea, @encryption_key)::jsonb
        ELSE e.mtls_client_cert
    END AS mtls_client_cert,
    CASE
        WHEN e.is_encrypted THEN pgp_sym_decrypt(e.oauth2_config_cipher::bytea, @encryption_key)::jsonb
        ELSE e.oauth2_config
    END AS oauth2_config,
    CASE
        WHEN e.is_encrypted THEN pgp_sym_decrypt(e.basic_auth_config_cipher::bytea, @encryption_key)::jsonb
        ELSE e.basic_auth_config
    END AS basic_auth_config,
    e.content_type
FROM convoy.endpoints AS e
WHERE e.deleted_at IS NULL AND e.id = ANY(@ids::text[]) AND e.project_id = @project_id
ORDER BY e.id;

-- name: FindEndpointsByAppID :many
SELECT
    e.id, e.name, e.status, e.owner_id, e.url, e.description,
    e.http_timeout, e.rate_limit, e.rate_limit_duration, e.advanced_signatures,
    e.slack_webhook_url, e.support_email, e.app_id, e.project_id,
    CASE
        WHEN e.is_encrypted THEN pgp_sym_decrypt(e.secrets_cipher::bytea, @encryption_key)::jsonb
        ELSE e.secrets
    END AS secrets,
    e.created_at, e.updated_at,
    COALESCE(e.authentication_type, '') AS authentication_type,
    COALESCE(e.authentication_type_api_key_header_name, '') AS authentication_type_api_key_header_name,
    CASE
        WHEN e.is_encrypted THEN pgp_sym_decrypt(e.authentication_type_api_key_header_value_cipher::bytea, @encryption_key)::TEXT
        ELSE e.authentication_type_api_key_header_value
    END AS authentication_type_api_key_header_value,
    CASE
        WHEN e.is_encrypted THEN pgp_sym_decrypt(e.mtls_client_cert_cipher::bytea, @encryption_key)::jsonb
        ELSE e.mtls_client_cert
    END AS mtls_client_cert,
    CASE
        WHEN e.is_encrypted THEN pgp_sym_decrypt(e.oauth2_config_cipher::bytea, @encryption_key)::jsonb
        ELSE e.oauth2_config
    END AS oauth2_config,
    CASE
        WHEN e.is_encrypted THEN pgp_sym_decrypt(e.basic_auth_config_cipher::bytea, @encryption_key)::jsonb
        ELSE e.basic_auth_config
    END AS basic_auth_config,
    e.content_type
FROM convoy.endpoints AS e
WHERE e.deleted_at IS NULL AND e.app_id = @app_id AND e.project_id = @project_id
ORDER BY e.id;

-- name: FindEndpointsByOwnerID :many
SELECT
    e.id, e.name, e.status, e.owner_id, e.url, e.description,
    e.http_timeout, e.rate_limit, e.rate_limit_duration, e.advanced_signatures,
    e.slack_webhook_url, e.support_email, e.app_id, e.project_id,
    CASE
        WHEN e.is_encrypted THEN pgp_sym_decrypt(e.secrets_cipher::bytea, @encryption_key)::jsonb
        ELSE e.secrets
    END AS secrets,
    e.created_at, e.updated_at,
    COALESCE(e.authentication_type, '') AS authentication_type,
    COALESCE(e.authentication_type_api_key_header_name, '') AS authentication_type_api_key_header_name,
    CASE
        WHEN e.is_encrypted THEN pgp_sym_decrypt(e.authentication_type_api_key_header_value_cipher::bytea, @encryption_key)::TEXT
        ELSE e.authentication_type_api_key_header_value
    END AS authentication_type_api_key_header_value,
    CASE
        WHEN e.is_encrypted THEN pgp_sym_decrypt(e.mtls_client_cert_cipher::bytea, @encryption_key)::jsonb
        ELSE e.mtls_client_cert
    END AS mtls_client_cert,
    CASE
        WHEN e.is_encrypted THEN pgp_sym_decrypt(e.oauth2_config_cipher::bytea, @encryption_key)::jsonb
        ELSE e.oauth2_config
    END AS oauth2_config,
    CASE
        WHEN e.is_encrypted THEN pgp_sym_decrypt(e.basic_auth_config_cipher::bytea, @encryption_key)::jsonb
        ELSE e.basic_auth_config
    END AS basic_auth_config,
    e.content_type
FROM convoy.endpoints AS e
WHERE e.deleted_at IS NULL AND e.project_id = @project_id AND e.owner_id = @owner_id
ORDER BY e.id;

-- name: FetchEndpointIDsByOwnerID :many
SELECT e.id
FROM convoy.endpoints e
WHERE e.deleted_at IS NULL AND e.project_id = @project_id AND e.owner_id = @owner_id
ORDER BY e.id;

-- name: FindEndpointByTargetURL :one
SELECT
    e.id, e.name, e.status, e.owner_id, e.url, e.description,
    e.http_timeout, e.rate_limit, e.rate_limit_duration, e.advanced_signatures,
    e.slack_webhook_url, e.support_email, e.app_id, e.project_id,
    CASE
        WHEN e.is_encrypted THEN pgp_sym_decrypt(e.secrets_cipher::bytea, @encryption_key)::jsonb
        ELSE e.secrets
    END AS secrets,
    e.created_at, e.updated_at,
    COALESCE(e.authentication_type, '') AS authentication_type,
    COALESCE(e.authentication_type_api_key_header_name, '') AS authentication_type_api_key_header_name,
    CASE
        WHEN e.is_encrypted THEN pgp_sym_decrypt(e.authentication_type_api_key_header_value_cipher::bytea, @encryption_key)::TEXT
        ELSE e.authentication_type_api_key_header_value
    END AS authentication_type_api_key_header_value,
    CASE
        WHEN e.is_encrypted THEN pgp_sym_decrypt(e.mtls_client_cert_cipher::bytea, @encryption_key)::jsonb
        ELSE e.mtls_client_cert
    END AS mtls_client_cert,
    CASE
        WHEN e.is_encrypted THEN pgp_sym_decrypt(e.oauth2_config_cipher::bytea, @encryption_key)::jsonb
        ELSE e.oauth2_config
    END AS oauth2_config,
    CASE
        WHEN e.is_encrypted THEN pgp_sym_decrypt(e.basic_auth_config_cipher::bytea, @encryption_key)::jsonb
        ELSE e.basic_auth_config
    END AS basic_auth_config,
    e.content_type
FROM convoy.endpoints AS e
WHERE e.deleted_at IS NULL AND e.url = @url AND e.project_id = @project_id;

-- name: UpdateEndpoint :execresult
UPDATE convoy.endpoints SET
    name = @name, status = @status, owner_id = @owner_id,
    url = @url, description = @description, http_timeout = @http_timeout,
    rate_limit = @rate_limit, rate_limit_duration = @rate_limit_duration,
    advanced_signatures = @advanced_signatures,
    slack_webhook_url = @slack_webhook_url, support_email = @support_email,
    authentication_type = @authentication_type,
    authentication_type_api_key_header_name = @authentication_type_api_key_header_name,
    authentication_type_api_key_header_value_cipher = CASE
        WHEN is_encrypted THEN pgp_sym_encrypt(@authentication_type_api_key_header_value, @encryption_key)
    END,
    authentication_type_api_key_header_value = CASE
        WHEN is_encrypted THEN ''
        ELSE @authentication_type_api_key_header_value
    END,
    secrets_cipher = CASE
        WHEN is_encrypted THEN pgp_sym_encrypt(@secrets_text::TEXT, @encryption_key)
    END,
    secrets = CASE
        WHEN is_encrypted THEN '[]'
        ELSE @secrets_text::jsonb
    END,
    mtls_client_cert = CASE
        WHEN is_encrypted THEN NULL
        ELSE @mtls_client_cert_text::jsonb
    END,
    mtls_client_cert_cipher = CASE
        WHEN is_encrypted THEN pgp_sym_encrypt(@mtls_client_cert_text::TEXT, @encryption_key)
    END,
    oauth2_config = CASE
        WHEN is_encrypted THEN NULL
        ELSE @oauth2_config_text::jsonb
    END,
    oauth2_config_cipher = CASE
        WHEN is_encrypted THEN pgp_sym_encrypt(@oauth2_config_text::TEXT, @encryption_key)
    END,
    basic_auth_config = CASE
        WHEN is_encrypted THEN NULL
        ELSE @basic_auth_config_text::jsonb
    END,
    basic_auth_config_cipher = CASE
        WHEN is_encrypted THEN pgp_sym_encrypt(@basic_auth_config_text::TEXT, @encryption_key)
    END,
    updated_at = NOW(), content_type = @content_type
WHERE id = @id AND project_id = @project_id AND deleted_at IS NULL;

-- name: UpdateEndpointStatus :one
UPDATE convoy.endpoints SET status = @status
WHERE id = @id AND project_id = @project_id AND deleted_at IS NULL
RETURNING
    id, name, status, owner_id, url, description,
    http_timeout, rate_limit, rate_limit_duration, advanced_signatures,
    slack_webhook_url, support_email, app_id, project_id,
    CASE
        WHEN is_encrypted THEN pgp_sym_decrypt(secrets_cipher::bytea, @encryption_key)::jsonb
        ELSE secrets
    END AS secrets,
    created_at, updated_at,
    COALESCE(authentication_type, '') AS authentication_type,
    COALESCE(authentication_type_api_key_header_name, '') AS authentication_type_api_key_header_name,
    CASE
        WHEN is_encrypted THEN pgp_sym_decrypt(authentication_type_api_key_header_value_cipher::bytea, @encryption_key)::TEXT
        ELSE authentication_type_api_key_header_value
    END AS authentication_type_api_key_header_value,
    CASE
        WHEN is_encrypted THEN pgp_sym_decrypt(mtls_client_cert_cipher::bytea, @encryption_key)::jsonb
        ELSE mtls_client_cert
    END AS mtls_client_cert,
    CASE
        WHEN is_encrypted THEN pgp_sym_decrypt(oauth2_config_cipher::bytea, @encryption_key)::jsonb
        ELSE oauth2_config
    END AS oauth2_config,
    CASE
        WHEN is_encrypted THEN pgp_sym_decrypt(basic_auth_config_cipher::bytea, @encryption_key)::jsonb
        ELSE basic_auth_config
    END AS basic_auth_config,
    content_type;

-- name: UpdateEndpointSecrets :one
UPDATE convoy.endpoints SET
    secrets_cipher = CASE
        WHEN is_encrypted THEN pgp_sym_encrypt(@secrets_text::TEXT, @encryption_key)
    END,
    secrets = CASE
        WHEN is_encrypted THEN '[]'
        ELSE @secrets_text::jsonb
    END,
    updated_at = NOW()
WHERE id = @id AND project_id = @project_id AND deleted_at IS NULL
RETURNING
    id, name, status, owner_id, url, description,
    http_timeout, rate_limit, rate_limit_duration, advanced_signatures,
    slack_webhook_url, support_email, app_id, project_id,
    CASE
        WHEN is_encrypted THEN pgp_sym_decrypt(secrets_cipher::bytea, @encryption_key)::jsonb
        ELSE secrets
    END AS secrets,
    created_at, updated_at,
    COALESCE(authentication_type, '') AS authentication_type,
    COALESCE(authentication_type_api_key_header_name, '') AS authentication_type_api_key_header_name,
    CASE
        WHEN is_encrypted THEN pgp_sym_decrypt(authentication_type_api_key_header_value_cipher::bytea, @encryption_key)::TEXT
        ELSE authentication_type_api_key_header_value
    END AS authentication_type_api_key_header_value,
    CASE
        WHEN is_encrypted THEN pgp_sym_decrypt(mtls_client_cert_cipher::bytea, @encryption_key)::jsonb
        ELSE mtls_client_cert
    END AS mtls_client_cert,
    CASE
        WHEN is_encrypted THEN pgp_sym_decrypt(oauth2_config_cipher::bytea, @encryption_key)::jsonb
        ELSE oauth2_config
    END AS oauth2_config,
    CASE
        WHEN is_encrypted THEN pgp_sym_decrypt(basic_auth_config_cipher::bytea, @encryption_key)::jsonb
        ELSE basic_auth_config
    END AS basic_auth_config,
    content_type;

-- name: DeleteEndpoint :exec
UPDATE convoy.endpoints SET deleted_at = NOW()
WHERE id = @id AND project_id = @project_id AND deleted_at IS NULL;

-- name: DeleteEndpointSubscriptions :exec
UPDATE convoy.subscriptions SET deleted_at = NOW()
WHERE endpoint_id = @endpoint_id AND project_id = @project_id AND deleted_at IS NULL;

-- name: DeletePortalLinkEndpoints :exec
DELETE FROM convoy.portal_links_endpoints WHERE endpoint_id = @endpoint_id;

-- name: CountProjectEndpoints :one
SELECT COUNT(*) AS count FROM convoy.endpoints
WHERE project_id = @project_id AND deleted_at IS NULL;

-- name: CheckEncryptionStatus :one
SELECT EXISTS(SELECT 1 FROM convoy.endpoints WHERE is_encrypted = TRUE LIMIT 1) AS is_encrypted;

-- name: FetchEndpointsPagedForward :many
SELECT
    e.id, e.name, e.status, e.owner_id, e.url, e.description,
    e.http_timeout, e.rate_limit, e.rate_limit_duration, e.advanced_signatures,
    e.slack_webhook_url, e.support_email, e.app_id, e.project_id,
    CASE
        WHEN e.is_encrypted THEN pgp_sym_decrypt(e.secrets_cipher::bytea, @encryption_key)::jsonb
        ELSE e.secrets
    END AS secrets,
    e.created_at, e.updated_at,
    COALESCE(e.authentication_type, '') AS authentication_type,
    COALESCE(e.authentication_type_api_key_header_name, '') AS authentication_type_api_key_header_name,
    CASE
        WHEN e.is_encrypted THEN pgp_sym_decrypt(e.authentication_type_api_key_header_value_cipher::bytea, @encryption_key)::TEXT
        ELSE e.authentication_type_api_key_header_value
    END AS authentication_type_api_key_header_value,
    CASE
        WHEN e.is_encrypted THEN pgp_sym_decrypt(e.mtls_client_cert_cipher::bytea, @encryption_key)::jsonb
        ELSE e.mtls_client_cert
    END AS mtls_client_cert,
    CASE
        WHEN e.is_encrypted THEN pgp_sym_decrypt(e.oauth2_config_cipher::bytea, @encryption_key)::jsonb
        ELSE e.oauth2_config
    END AS oauth2_config,
    CASE
        WHEN e.is_encrypted THEN pgp_sym_decrypt(e.basic_auth_config_cipher::bytea, @encryption_key)::jsonb
        ELSE e.basic_auth_config
    END AS basic_auth_config,
    e.content_type
FROM convoy.endpoints AS e
WHERE e.deleted_at IS NULL
    AND e.project_id = @project_id
    AND (CASE WHEN @has_owner_filter::boolean THEN e.owner_id = @owner_id ELSE true END)
    AND (CASE WHEN @has_name_filter::boolean THEN e.name ILIKE @name_query ELSE true END)
    AND (CASE WHEN @has_endpoint_filter::boolean THEN e.id = ANY(@endpoint_ids::text[]) ELSE true END)
    AND e.id <= @cursor
ORDER BY e.id DESC
LIMIT @limit_val;

-- name: FetchEndpointsPagedBackward :many
-- Note: Returns results in ASC order. Caller must reverse to get DESC order.
SELECT
    e.id, e.name, e.status, e.owner_id, e.url, e.description,
    e.http_timeout, e.rate_limit, e.rate_limit_duration, e.advanced_signatures,
    e.slack_webhook_url, e.support_email, e.app_id, e.project_id,
    CASE
        WHEN e.is_encrypted THEN pgp_sym_decrypt(e.secrets_cipher::bytea, @encryption_key)::jsonb
        ELSE e.secrets
    END AS secrets,
    e.created_at, e.updated_at,
    COALESCE(e.authentication_type, '') AS authentication_type,
    COALESCE(e.authentication_type_api_key_header_name, '') AS authentication_type_api_key_header_name,
    CASE
        WHEN e.is_encrypted THEN pgp_sym_decrypt(e.authentication_type_api_key_header_value_cipher::bytea, @encryption_key)::TEXT
        ELSE e.authentication_type_api_key_header_value
    END AS authentication_type_api_key_header_value,
    CASE
        WHEN e.is_encrypted THEN pgp_sym_decrypt(e.mtls_client_cert_cipher::bytea, @encryption_key)::jsonb
        ELSE e.mtls_client_cert
    END AS mtls_client_cert,
    CASE
        WHEN e.is_encrypted THEN pgp_sym_decrypt(e.oauth2_config_cipher::bytea, @encryption_key)::jsonb
        ELSE e.oauth2_config
    END AS oauth2_config,
    CASE
        WHEN e.is_encrypted THEN pgp_sym_decrypt(e.basic_auth_config_cipher::bytea, @encryption_key)::jsonb
        ELSE e.basic_auth_config
    END AS basic_auth_config,
    e.content_type
FROM convoy.endpoints AS e
WHERE e.deleted_at IS NULL
    AND e.project_id = @project_id
    AND (CASE WHEN @has_owner_filter::boolean THEN e.owner_id = @owner_id ELSE true END)
    AND (CASE WHEN @has_name_filter::boolean THEN e.name ILIKE @name_query ELSE true END)
    AND (CASE WHEN @has_endpoint_filter::boolean THEN e.id = ANY(@endpoint_ids::text[]) ELSE true END)
    AND e.id >= @cursor
ORDER BY e.id ASC
LIMIT @limit_val;

-- name: CountPrevEndpoints :one
SELECT COALESCE(COUNT(DISTINCT e.id), 0) AS count
FROM convoy.endpoints e
WHERE e.deleted_at IS NULL
    AND e.project_id = @project_id
    AND e.id > @cursor
    -- Optional owner_id filter
    AND (
        CASE
            WHEN @has_owner_filter::boolean THEN e.owner_id = @owner_id
            ELSE true
        END
    )
    -- Optional name ILIKE filter
    AND (
        CASE
            WHEN @has_name_filter::boolean THEN e.name ILIKE @name_query
            ELSE true
        END
    )
    -- Optional endpoint_ids filter
    AND (
        CASE
            WHEN @has_endpoint_filter::boolean THEN e.id = ANY(@endpoint_ids::text[])
            ELSE true
        END
    );
