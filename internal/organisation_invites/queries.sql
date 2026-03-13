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
    @id, @organisation_id, @invitee_email, @token, @role_type, @role_project, @role_endpoint, @status, @expires_at
);

-- name: UpdateOrganisationInvite :exec
UPDATE convoy.organisation_invites
SET
    role_type = @role_type,
    role_project = @role_project,
    role_endpoint = @role_endpoint,
    status = @status,
    expires_at = @expires_at,
    updated_at = NOW(),
    deleted_at = @deleted_at
WHERE id = @id AND deleted_at IS NULL;

-- name: DeleteOrganisationInvite :exec
UPDATE convoy.organisation_invites
SET deleted_at = NOW()
WHERE id = @id AND deleted_at IS NULL;

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
WHERE id = @id AND deleted_at IS NULL;

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
WHERE token = @token AND deleted_at IS NULL;

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
    WHERE organisation_id = @organisation_id
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
SELECT
    id, organisation_id, invitee_email, status, role_type, role_project,
    role_endpoint, created_at, updated_at, expires_at
FROM filtered_invites
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
