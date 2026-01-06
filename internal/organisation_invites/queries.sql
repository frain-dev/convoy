-- name: CreateOrganisationInvite :exec
INSERT INTO convoy.organisation_invites (
    id,
    organisation_id,
    invitee_email,
    token,
    role_type,
    role_project,
    role_endpoint,
    status,
    expires_at
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9
);

-- name: UpdateOrganisationInvite :exec
UPDATE convoy.organisation_invites
SET
    role_type = $2,
    role_project = $3,
    role_endpoint = $4,
    status = $5,
    expires_at = $6,
    updated_at = NOW(),
    deleted_at = $7
WHERE id = $1 AND deleted_at IS NULL;

-- name: DeleteOrganisationInvite :exec
UPDATE convoy.organisation_invites
SET deleted_at = NOW()
WHERE id = $1 AND deleted_at IS NULL;

-- name: FetchOrganisationInviteByID :one
SELECT
    id,
    organisation_id,
    invitee_email,
    token,
    status,
    role_type,
    COALESCE(role_project, '') AS role_project,
    COALESCE(role_endpoint, '') AS role_endpoint,
    created_at,
    updated_at,
    expires_at
FROM convoy.organisation_invites
WHERE id = $1 AND deleted_at IS NULL;

-- name: FetchOrganisationInviteByToken :one
SELECT
    id,
    organisation_id,
    invitee_email,
    token,
    status,
    role_type,
    COALESCE(role_project, '') AS role_project,
    COALESCE(role_endpoint, '') AS role_endpoint,
    created_at,
    updated_at,
    expires_at
FROM convoy.organisation_invites
WHERE token = $1 AND deleted_at IS NULL;

-- name: FetchOrganisationInvitesPaginated :many
WITH filtered_invites AS (
    SELECT
        id,
        organisation_id,
        invitee_email,
        status,
        role_type,
        COALESCE(role_project, '') AS role_project,
        COALESCE(role_endpoint, '') AS role_endpoint,
        created_at,
        updated_at,
        expires_at
    FROM convoy.organisation_invites
    WHERE organisation_id = @org_id
        AND status = @status
        AND deleted_at IS NULL
        -- Cursor-based pagination
        AND (
            CASE
                WHEN @direction::text = 'next' THEN id <= @cursor
                WHEN @direction::text = 'prev' THEN id >= @cursor
                ELSE true
            END
        )
    GROUP BY id
    -- Sort order: DESC for forward, ASC for backward
    ORDER BY
        CASE
            WHEN @direction::text = 'next' THEN id
        END DESC,
        CASE
            WHEN @direction::text = 'prev' THEN id
        END ASC
    LIMIT @limit_val
)
-- Final select: reverse order for backward pagination
SELECT * FROM filtered_invites
ORDER BY
    CASE
        WHEN @direction::text = 'prev' THEN id
    END DESC,
    CASE
        WHEN @direction::text = 'next' THEN id
    END DESC;

-- name: CountPrevOrganisationInvites :one
SELECT COALESCE(COUNT(DISTINCT id), 0) AS count
FROM convoy.organisation_invites
WHERE organisation_id = @org_id
    AND deleted_at IS NULL
    AND id > @cursor;
