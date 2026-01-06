-- Portal Links Queries

-- name: CreatePortalLink :exec
INSERT INTO convoy.portal_links (id, project_id, name, token, endpoints, owner_id, can_manage_endpoint, auth_type)
VALUES (@id, @project_id, @name, @token, @endpoints, @owner_id, @can_manage_endpoint, @auth_type);

-- name: CreatePortalLinkAuthToken :exec
INSERT INTO convoy.portal_tokens (id, portal_link_id, token_mask_id, token_hash, token_salt, token_expires_at)
VALUES (@id, @portal_link_id, @token_mask_id, @token_hash, @token_salt, @token_expires_at);

-- name: BulkWritePortalAuthTokens :exec
INSERT INTO convoy.portal_tokens (id, portal_link_id, token_mask_id, token_hash, token_salt, token_expires_at)
VALUES (@id, @portal_link_id, @token_mask_id, @token_hash, @token_salt, @token_expires_at);

-- name: CreatePortalLinkEndpoint :exec
INSERT INTO convoy.portal_links_endpoints (portal_link_id, endpoint_id)
VALUES (@portal_link_id, @endpoint_id);

-- name: UpdatePortalLink :exec
UPDATE convoy.portal_links
SET
    endpoints = @endpoints,
    owner_id = @owner_id,
    can_manage_endpoint = @can_manage_endpoint,
    name = @name,
    auth_type = @auth_type,
    updated_at = NOW()
WHERE id = @id AND project_id = @project_id AND deleted_at IS NULL;

-- name: DeletePortalLinkEndpoints :exec
DELETE FROM convoy.portal_links_endpoints
WHERE portal_link_id = @portal_link_id OR endpoint_id = @endpoint_id;

-- name: UpdateEndpointOwnerID :exec
UPDATE convoy.endpoints
SET owner_id = @owner_id
WHERE id = @id AND project_id = @project_id AND deleted_at IS NULL;

-- name: FetchPortalLinkById :one
SELECT
    p.id,
    p.project_id,
    p.name,
    p.token,
    p.endpoints,
    p.auth_type,
    COALESCE(p.can_manage_endpoint, FALSE) AS can_manage_endpoint,
    COALESCE(p.owner_id, '') AS owner_id,
    CASE
        WHEN p.owner_id != '' THEN (SELECT count(id) FROM convoy.endpoints WHERE owner_id = p.owner_id)
        ELSE (SELECT count(portal_link_id) FROM convoy.portal_links_endpoints WHERE portal_link_id = p.id)
    END AS endpoint_count,
    p.created_at,
    p.updated_at,
    ARRAY_TO_JSON(ARRAY_AGG(DISTINCT
        CASE WHEN e.id IS NOT NULL THEN
            cast(JSON_BUILD_OBJECT('uid', e.id, 'name', e.name, 'project_id', e.project_id, 'url', e.url, 'secrets', e.secrets) as jsonb)
        END
    )) AS endpoints_metadata
FROM convoy.portal_links p
LEFT JOIN convoy.portal_links_endpoints pe
    ON p.id = pe.portal_link_id
LEFT JOIN convoy.endpoints e
    ON e.id = pe.endpoint_id
WHERE p.id = @id AND p.project_id = @project_id AND p.deleted_at IS NULL
GROUP BY p.id;

-- name: FetchPortalLinkByOwnerID :one
SELECT
    p.id,
    p.project_id,
    p.name,
    p.token,
    p.endpoints,
    p.auth_type,
    COALESCE(p.can_manage_endpoint, FALSE) AS can_manage_endpoint,
    COALESCE(p.owner_id, '') AS owner_id,
    CASE
        WHEN p.owner_id != '' THEN (SELECT count(id) FROM convoy.endpoints WHERE owner_id = p.owner_id)
        ELSE (SELECT count(portal_link_id) FROM convoy.portal_links_endpoints WHERE portal_link_id = p.id)
    END AS endpoint_count,
    p.created_at,
    p.updated_at,
    ARRAY_TO_JSON(ARRAY_AGG(DISTINCT
        CASE WHEN e.id IS NOT NULL THEN
            cast(JSON_BUILD_OBJECT('uid', e.id, 'name', e.name, 'project_id', e.project_id, 'url', e.url, 'secrets', e.secrets) as jsonb)
        END
    )) AS endpoints_metadata
FROM convoy.portal_links p
LEFT JOIN convoy.portal_links_endpoints pe
    ON p.id = pe.portal_link_id
LEFT JOIN convoy.endpoints e
    ON e.id = pe.endpoint_id
WHERE p.owner_id = @owner_id AND p.project_id = @project_id AND p.deleted_at IS NULL
GROUP BY p.id;

-- name: FetchPortalLinkByToken :one
SELECT
    p.id,
    p.project_id,
    p.name,
    p.token,
    p.endpoints,
    p.auth_type,
    COALESCE(p.can_manage_endpoint, FALSE) AS can_manage_endpoint,
    COALESCE(p.owner_id, '') AS owner_id,
    CASE
        WHEN p.owner_id != '' THEN (SELECT count(id) FROM convoy.endpoints WHERE owner_id = p.owner_id)
        ELSE (SELECT count(portal_link_id) FROM convoy.portal_links_endpoints WHERE portal_link_id = p.id)
    END AS endpoint_count,
    p.created_at,
    p.updated_at,
    ARRAY_TO_JSON(ARRAY_AGG(DISTINCT
        CASE WHEN e.id IS NOT NULL THEN
            cast(JSON_BUILD_OBJECT('uid', e.id, 'name', e.name, 'project_id', e.project_id, 'url', e.url, 'secrets', e.secrets) as jsonb)
        END
    )) AS endpoints_metadata
FROM convoy.portal_links p
LEFT JOIN convoy.portal_links_endpoints pe
    ON p.id = pe.portal_link_id
LEFT JOIN convoy.endpoints e
    ON e.id = pe.endpoint_id
WHERE p.token = @token AND p.deleted_at IS NULL
GROUP BY p.id;

-- name: FetchPortalLinkByMaskId :one
SELECT
    pl.id,
    pl.project_id,
    pt.token_salt,
    pt.token_mask_id,
    pt.token_expires_at,
    pt.token_hash,
    pl.name,
    pl.token,
    pl.endpoints,
    pl.auth_type,
    COALESCE(pl.can_manage_endpoint, FALSE) AS can_manage_endpoint,
    COALESCE(pl.owner_id, '') AS owner_id,
    CASE
        WHEN pl.owner_id != '' THEN (SELECT count(id) FROM convoy.endpoints WHERE owner_id = pl.owner_id)
        ELSE (SELECT count(portal_link_id) FROM convoy.portal_links_endpoints WHERE portal_link_id = pl.id)
    END AS endpoint_count
FROM convoy.portal_tokens pt
JOIN convoy.portal_links pl ON pl.id = pt.portal_link_id
WHERE pt.token_mask_id = @token_mask_id;

-- name: FetchPortalLinksByOwnerID :many
SELECT
    p.id,
    p.project_id,
    p.name,
    p.token,
    p.endpoints,
    p.auth_type,
    COALESCE(p.can_manage_endpoint, FALSE) AS can_manage_endpoint,
    COALESCE(p.owner_id, '') AS owner_id,
    CASE
        WHEN p.owner_id != '' THEN (SELECT count(id) FROM convoy.endpoints WHERE owner_id = p.owner_id)
        ELSE (SELECT count(portal_link_id) FROM convoy.portal_links_endpoints WHERE portal_link_id = p.id)
    END AS endpoint_count,
    p.created_at,
    p.updated_at,
    ARRAY_TO_JSON(ARRAY_AGG(DISTINCT
        CASE WHEN e.id IS NOT NULL THEN
            cast(JSON_BUILD_OBJECT('uid', e.id, 'name', e.name, 'project_id', e.project_id, 'url', e.url, 'secrets', e.secrets) as jsonb)
        END
    )) AS endpoints_metadata
FROM convoy.portal_links p
LEFT JOIN convoy.portal_links_endpoints pe
    ON p.id = pe.portal_link_id
LEFT JOIN convoy.endpoints e
    ON e.id = pe.endpoint_id
WHERE p.owner_id = @owner_id AND p.deleted_at IS NULL
GROUP BY p.id;

-- name: DeletePortalLink :execresult
UPDATE convoy.portal_links
SET deleted_at = NOW()
WHERE id = @id AND project_id = @project_id AND deleted_at IS NULL;

-- Unified Paginated queries using CASE and COALESCE for dynamic filtering
-- These queries handle both forward/backward pagination and optional endpoint filtering

-- name: FetchPortalLinksPaginated :many
-- @direction: 'next' for forward pagination, 'prev' for backward pagination
-- @has_endpoint_filter: true to filter by endpoint_ids, false to skip filtering
WITH filtered_portal_links AS (
    SELECT
        p.id,
        p.project_id,
        p.name,
        p.token,
        p.endpoints,
        p.auth_type,
        COALESCE(p.can_manage_endpoint, FALSE) AS can_manage_endpoint,
        COALESCE(p.owner_id, '') AS owner_id,
        CASE
            WHEN p.owner_id != '' THEN (SELECT count(id) FROM convoy.endpoints WHERE owner_id = p.owner_id)
            ELSE (SELECT count(portal_link_id) FROM convoy.portal_links_endpoints WHERE portal_link_id = p.id)
        END AS endpoint_count,
        p.created_at,
        p.updated_at,
        ARRAY_TO_JSON(ARRAY_AGG(DISTINCT
            CASE WHEN e.id IS NOT NULL THEN
                cast(JSON_BUILD_OBJECT('uid', e.id, 'name', e.name, 'project_id', e.project_id, 'url', e.url, 'secrets', e.secrets) as jsonb)
            END
        )) AS endpoints_metadata
    FROM convoy.portal_links p
    LEFT JOIN convoy.portal_links_endpoints pe
        ON p.id = pe.portal_link_id
    LEFT JOIN convoy.endpoints e
        ON e.id = pe.endpoint_id
    WHERE p.deleted_at IS NULL
        AND (p.project_id = @project_id OR @project_id = '')
        -- Cursor comparison: <= for forward (next), >= for backward (prev)
        AND (
            CASE
                WHEN @direction::text = 'next' THEN p.id <= @cursor
                WHEN @direction::text = 'prev' THEN p.id >= @cursor
                ELSE true
            END
        )
        -- Optional endpoint filter: apply only if has_endpoint_filter is true
        AND (
            CASE
                WHEN @has_endpoint_filter::boolean THEN pe.endpoint_id = ANY(@endpoint_ids::text[])
                ELSE true
            END
        )
    GROUP BY p.id
    -- Sort order: DESC for forward, ASC for backward (will be reversed in outer query for backward)
    ORDER BY
        CASE
            WHEN @direction::text = 'next' THEN p.id
        END DESC,
        CASE
            WHEN @direction::text = 'prev' THEN p.id
        END ASC
    LIMIT @limit_val
)
-- Final select: reverse order for backward pagination to get DESC order
SELECT * FROM filtered_portal_links
ORDER BY
    CASE
        WHEN @direction::text = 'prev' THEN id
    END DESC,
    CASE
        WHEN @direction::text = 'next' THEN id
    END DESC;

-- name: CountPrevPortalLinks :one
-- Unified count query for pagination prev row count
SELECT COALESCE(COUNT(DISTINCT p.id), 0) AS count
FROM convoy.portal_links p
LEFT JOIN convoy.portal_links_endpoints pe
    ON p.id = pe.portal_link_id
LEFT JOIN convoy.endpoints e
    ON e.id = pe.endpoint_id
WHERE p.deleted_at IS NULL
    AND (p.project_id = @project_id OR @project_id = '')
    AND p.id > @cursor
    -- Optional endpoint filter: apply only if has_endpoint_filter is true
    AND (
        CASE
            WHEN @has_endpoint_filter::boolean THEN pe.endpoint_id = ANY(@endpoint_ids::text[])
            ELSE true
        END
    );