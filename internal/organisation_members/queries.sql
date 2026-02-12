-- Organisation Members SQLc Queries
-- Migration from database/postgres/organisation_member.go to SQLc

-- ===========================================================================
-- Core CRUD Operations
-- ===========================================================================

-- name: CreateOrganisationMember :exec
INSERT INTO convoy.organisation_members (
    id,
    organisation_id,
    user_id,
    role_type,
    role_project,
    role_endpoint
) VALUES (
    $1, $2, $3, $4, $5, $6
);

-- name: UpdateOrganisationMember :exec
UPDATE convoy.organisation_members
SET
    role_type = $2,
    role_project = $3,
    role_endpoint = $4,
    updated_at = NOW()
WHERE id = $1 AND deleted_at IS NULL;

-- name: DeleteOrganisationMember :exec
UPDATE convoy.organisation_members
SET deleted_at = NOW()
WHERE id = $1 AND organisation_id = $2 AND deleted_at IS NULL;

-- ===========================================================================
-- Fetch Single Member Queries (with User Metadata)
-- ===========================================================================

-- name: FetchOrganisationMemberByID :one
SELECT
    o.id,
    o.organisation_id,
    o.user_id,
    o.role_type,
    COALESCE(o.role_project, '') AS role_project,
    COALESCE(o.role_endpoint, '') AS role_endpoint,
    u.id AS user_metadata_user_id,
    u.first_name AS user_metadata_first_name,
    u.last_name AS user_metadata_last_name,
    u.email AS user_metadata_email,
    o.created_at,
    o.updated_at
FROM convoy.organisation_members o
LEFT JOIN convoy.users u ON o.user_id = u.id
WHERE o.id = $1 AND o.organisation_id = $2 AND o.deleted_at IS NULL;

-- name: FetchOrganisationMemberByUserID :one
SELECT
    o.id,
    o.organisation_id,
    o.user_id,
    o.role_type,
    COALESCE(o.role_project, '') AS role_project,
    COALESCE(o.role_endpoint, '') AS role_endpoint,
    u.id AS user_metadata_user_id,
    u.first_name AS user_metadata_first_name,
    u.last_name AS user_metadata_last_name,
    u.email AS user_metadata_email,
    o.created_at,
    o.updated_at
FROM convoy.organisation_members o
LEFT JOIN convoy.users u ON o.user_id = u.id
WHERE o.user_id = $1 AND o.organisation_id = $2 AND o.deleted_at IS NULL;

-- ===========================================================================
-- Admin-Specific Queries
-- ===========================================================================

-- name: FetchInstanceAdminByUserID :one
SELECT
    o.id,
    o.organisation_id,
    o.user_id,
    o.role_type,
    COALESCE(o.role_project, '') AS role_project,
    COALESCE(o.role_endpoint, '') AS role_endpoint,
    u.id AS user_metadata_user_id,
    u.first_name AS user_metadata_first_name,
    u.last_name AS user_metadata_last_name,
    u.email AS user_metadata_email,
    o.created_at,
    o.updated_at
FROM convoy.organisation_members o
LEFT JOIN convoy.users u ON o.user_id = u.id
WHERE o.user_id = $1
    AND o.role_type = 'instance_admin'
    AND o.deleted_at IS NULL
LIMIT 1;

-- name: FetchAnyOrganisationAdminByUserID :one
SELECT
    o.id,
    o.organisation_id,
    o.user_id,
    o.role_type,
    COALESCE(o.role_project, '') AS role_project,
    COALESCE(o.role_endpoint, '') AS role_endpoint,
    u.id AS user_metadata_user_id,
    u.first_name AS user_metadata_first_name,
    u.last_name AS user_metadata_last_name,
    u.email AS user_metadata_email,
    o.created_at,
    o.updated_at
FROM convoy.organisation_members o
LEFT JOIN convoy.users u ON o.user_id = u.id
WHERE o.user_id = $1
    AND o.role_type = 'organisation_admin'
    AND o.deleted_at IS NULL
LIMIT 1;

-- name: CountInstanceAdminUsers :one
SELECT COUNT(*)
FROM convoy.organisation_members o
WHERE o.role_type = 'instance_admin'
    AND o.deleted_at IS NULL;

-- name: CountOrganisationAdminUsers :one
SELECT COUNT(*)
FROM convoy.organisation_members o
WHERE o.role_type = 'organisation_admin'
    AND o.deleted_at IS NULL;

-- name: HasInstanceAdminAccess :one
SELECT EXISTS (
    SELECT 1 FROM convoy.organisation_members o
    WHERE o.user_id = $1
        AND o.role_type = 'instance_admin'
        AND o.deleted_at IS NULL
) OR NOT EXISTS (
    SELECT 1 FROM convoy.organisation_members o
    WHERE o.role_type = 'instance_admin'
        AND o.deleted_at IS NULL
        AND o.user_id != $1
);

-- name: IsFirstInstanceAdmin :one
SELECT EXISTS (
    SELECT 1 FROM convoy.organisation_members o1
    WHERE o1.user_id = $1
        AND o1.deleted_at IS NULL
        AND (
            -- Case 1: User is first local instance admin
            (o1.role_type = 'instance_admin'
             AND EXISTS (
                 SELECT 1 FROM convoy.users u
                 WHERE u.id = o1.user_id
                     AND (u.auth_type = 'local' OR u.auth_type IS NULL OR u.auth_type = '')
             )
             AND o1.created_at = (
                 SELECT MIN(o2.created_at)
                 FROM convoy.organisation_members o2
                 JOIN convoy.users u2 ON o2.user_id = u2.id
                 WHERE o2.role_type = 'instance_admin'
                     AND o2.deleted_at IS NULL
                     AND (u2.auth_type = 'local' OR u2.auth_type IS NULL OR u2.auth_type = '')
             ))
            OR
            -- Case 2: User is first local organisation_admin when no local instance admins exist
            (o1.role_type = 'organisation_admin'
             AND EXISTS (
                 SELECT 1 FROM convoy.users u
                 WHERE u.id = o1.user_id
                     AND (u.auth_type = 'local' OR u.auth_type IS NULL OR u.auth_type = '')
             )
             AND NOT EXISTS (
                 SELECT 1 FROM convoy.organisation_members o3
                 JOIN convoy.users u3 ON o3.user_id = u3.id
                 WHERE o3.role_type = 'instance_admin'
                     AND o3.deleted_at IS NULL
                     AND (u3.auth_type = 'local' OR u3.auth_type IS NULL OR u3.auth_type = '')
             )
             AND o1.created_at = (
                 SELECT MIN(o4.created_at)
                 FROM convoy.organisation_members o4
                 JOIN convoy.users u2 ON o4.user_id = u2.id
                 WHERE o4.role_type = 'organisation_admin'
                     AND o4.deleted_at IS NULL
                     AND (u2.auth_type = 'local' OR u2.auth_type IS NULL OR u2.auth_type = '')
             )
            )
        )
);

-- ===========================================================================
-- Pagination Queries - Organisation Members
-- ===========================================================================

-- name: FetchOrganisationMembersPaginated :many
WITH filtered_members AS (
    SELECT
        o.id,
        o.organisation_id,
        o.user_id,
        o.role_type,
        COALESCE(o.role_project, '') AS role_project,
        COALESCE(o.role_endpoint, '') AS role_endpoint,
        u.id AS user_metadata_user_id,
        u.first_name AS user_metadata_first_name,
        u.last_name AS user_metadata_last_name,
        u.email AS user_metadata_email,
        o.created_at,
        o.updated_at
    FROM convoy.organisation_members o
    LEFT JOIN convoy.users u ON o.user_id = u.id
    WHERE o.organisation_id = @organisation_id
        AND o.deleted_at IS NULL
        AND (o.user_id = @user_id OR @user_id = '')
        AND (
            CASE
                WHEN @direction::text = 'next' THEN o.id <= @cursor
                WHEN @direction::text = 'prev' THEN o.id >= @cursor
                ELSE true
            END
        )
    ORDER BY
        CASE WHEN @direction::text = 'next' THEN o.id END DESC,
        CASE WHEN @direction::text = 'prev' THEN o.id END ASC
    LIMIT @limit_val
)
SELECT * FROM filtered_members
ORDER BY
    CASE WHEN @direction::text = 'prev' THEN id END DESC,
    CASE WHEN @direction::text = 'next' THEN id END DESC;

-- name: CountPrevOrganisationMembers :one
SELECT COUNT(DISTINCT o.id) AS count
FROM convoy.organisation_members o
WHERE o.organisation_id = @organisation_id
    AND o.deleted_at IS NULL
    AND o.id > @cursor;

-- ===========================================================================
-- Pagination Queries - User Organisations
-- ===========================================================================

-- name: FetchUserOrganisationsPaginated :many
WITH user_organisations AS (
    SELECT
        o.id,
        o.name,
        o.owner_id,
        o.custom_domain,
        o.assigned_domain,
        o.license_data,
        o.created_at,
        o.updated_at,
        o.deleted_at
    FROM convoy.organisation_members m
    JOIN convoy.organisations o ON m.organisation_id = o.id
    WHERE m.user_id = @user_id
        AND o.deleted_at IS NULL
        AND m.deleted_at IS NULL
        AND (
            CASE
                WHEN @direction::text = 'next' THEN o.id <= @cursor
                WHEN @direction::text = 'prev' THEN o.id >= @cursor
                ELSE true
            END
        )
    ORDER BY
        CASE WHEN @direction::text = 'next' THEN o.id END DESC,
        CASE WHEN @direction::text = 'prev' THEN o.id END ASC
    LIMIT @limit_val
)
SELECT * FROM user_organisations
ORDER BY
    CASE WHEN @direction::text = 'prev' THEN id END DESC,
    CASE WHEN @direction::text = 'next' THEN id END DESC;

-- name: CountPrevUserOrganisations :one
SELECT COUNT(DISTINCT o.id) AS count
FROM convoy.organisation_members m
JOIN convoy.organisations o ON m.organisation_id = o.id
WHERE m.user_id = @user_id
    AND o.deleted_at IS NULL
    AND m.deleted_at IS NULL
    AND o.id > @cursor;

-- ===========================================================================
-- Project Queries
-- ===========================================================================

-- name: FindUserProjects :many
SELECT
    p.id,
    p.name,
    p.type,
    p.retained_events,
    p.logo_url,
    p.organisation_id,
    p.project_configuration_id,
    p.created_at,
    p.updated_at
FROM convoy.organisation_members m
RIGHT JOIN convoy.projects p ON p.organisation_id = m.organisation_id
WHERE m.user_id = $1
    AND m.deleted_at IS NULL
    AND p.deleted_at IS NULL;
