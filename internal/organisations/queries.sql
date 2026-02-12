-- Organisation Repository SQLc Queries
-- This file contains all SQL queries for organisation operations

-- name: CreateOrganisation :exec
INSERT INTO convoy.organisations (
    id,
    name,
    owner_id,
    custom_domain,
    assigned_domain,
    license_data
) VALUES (
    $1,
    $2,
    $3,
    $4,
    $5,
    $6
);

-- name: UpdateOrganisation :execresult
UPDATE convoy.organisations
SET
    name = $2,
    custom_domain = $3,
    assigned_domain = $4,
    disabled_at = $5,
    license_data = $6,
    updated_at = NOW()
WHERE id = $1
  AND deleted_at IS NULL;

-- name: UpdateOrganisationLicenseData :execresult
UPDATE convoy.organisations
SET license_data = $2,
    updated_at = NOW()
WHERE id = $1
  AND deleted_at IS NULL;

-- name: DeleteOrganisation :execresult
UPDATE convoy.organisations
SET deleted_at = NOW()
WHERE id = $1
  AND deleted_at IS NULL;

-- name: FetchOrganisationByID :one
SELECT
    id,
    owner_id,
    name,
    custom_domain,
    assigned_domain,
    license_data,
    created_at,
    updated_at,
    deleted_at,
    disabled_at
FROM convoy.organisations
WHERE id = $1
  AND deleted_at IS NULL;

-- name: FetchOrganisationByCustomDomain :one
SELECT
    id,
    owner_id,
    name,
    custom_domain,
    assigned_domain,
    license_data,
    created_at,
    updated_at,
    deleted_at,
    disabled_at
FROM convoy.organisations
WHERE custom_domain = $1
  AND deleted_at IS NULL;

-- name: FetchOrganisationByAssignedDomain :one
SELECT
    id,
    owner_id,
    name,
    custom_domain,
    assigned_domain,
    license_data,
    created_at,
    updated_at,
    deleted_at,
    disabled_at
FROM convoy.organisations
WHERE assigned_domain = $1
  AND deleted_at IS NULL;

-- name: CountOrganisations :one
SELECT COUNT(*) AS count
FROM convoy.organisations
WHERE deleted_at IS NULL;

-- name: FetchOrganisationsPaginated :many
WITH filtered_organisations AS (
    SELECT
        id,
        owner_id,
        name,
        custom_domain,
        assigned_domain,
        license_data,
        created_at,
        updated_at,
        deleted_at,
        disabled_at
    FROM convoy.organisations
    WHERE deleted_at IS NULL
        -- Optional search filter (searches both name and id)
        AND (
            CASE
                WHEN @has_search::boolean THEN
                    (LOWER(name) LIKE LOWER(@search) OR LOWER(id) LIKE LOWER(@search))
                ELSE true
            END
        )
        -- Cursor-based pagination
        AND (
            CASE
                WHEN @direction::text = 'next' THEN id <= @cursor
                WHEN @direction::text = 'prev' THEN id >= @cursor
                ELSE true
            END
        )
    -- Sort order: DESC for forward, ASC for backward
    ORDER BY
        CASE WHEN @direction::text = 'next' THEN id END DESC,
        CASE WHEN @direction::text = 'prev' THEN id END ASC
    LIMIT @limit_val
)
-- Final select: reverse order for backward pagination
SELECT * FROM filtered_organisations
ORDER BY
    CASE WHEN @direction::text = 'prev' THEN id END DESC,
    CASE WHEN @direction::text = 'next' THEN id END DESC;

-- name: CountPrevOrganisations :one
SELECT COALESCE(COUNT(DISTINCT id), 0) AS count
FROM convoy.organisations
WHERE deleted_at IS NULL
    -- Same search filter as main query
    AND (
        CASE
            WHEN @has_search::boolean THEN
                (LOWER(name) LIKE LOWER(@search) OR LOWER(id) LIKE LOWER(@search))
            ELSE true
        END
    )
    -- Count items with ID greater than cursor (items before current page)
    AND id > @cursor;

-- Usage Calculation Queries

-- name: CalculateIngressBytes :one
SELECT
    COALESCE(SUM(LENGTH(e.raw)), 0) AS raw_bytes,
    COALESCE(SUM(OCTET_LENGTH(e.data::text)), 0) AS data_bytes
FROM convoy.events e
JOIN convoy.projects p ON p.id = e.project_id
WHERE p.organisation_id = $1
  AND e.created_at >= $2
  AND e.created_at <= $3
  AND e.deleted_at IS NULL
  AND p.deleted_at IS NULL;

-- name: CalculateEgressBytes :one
SELECT COALESCE(SUM(LENGTH(e.raw)), 0) + COALESCE(SUM(OCTET_LENGTH(e.data::text)), 0) AS bytes
FROM convoy.event_deliveries d
JOIN convoy.events e ON e.id = d.event_id
JOIN convoy.projects p ON p.id = e.project_id
WHERE p.organisation_id = $1
  AND d.status = 'Success'
  AND d.created_at >= $2
  AND d.created_at <= $3
  AND p.deleted_at IS NULL;

-- name: CountOrgEvents :one
SELECT COUNT(*) AS count
FROM convoy.events e
JOIN convoy.projects p ON p.id = e.project_id
WHERE p.organisation_id = $1
  AND e.created_at >= $2
  AND e.created_at <= $3
  AND e.deleted_at IS NULL
  AND p.deleted_at IS NULL;

-- name: CountOrgDeliveries :one
SELECT COUNT(*) AS count
FROM convoy.event_deliveries d
JOIN convoy.events e ON e.id = d.event_id
JOIN convoy.projects p ON p.id = e.project_id
WHERE p.organisation_id = $1
  AND d.status = 'Success'
  AND d.created_at >= $2
  AND d.created_at <= $3
  AND p.deleted_at IS NULL;
