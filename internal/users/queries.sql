-- Users Queries

-- ============================================================================
-- CREATE Operations
-- ============================================================================

-- name: CreateUser :exec
INSERT INTO convoy.users (
    id, first_name, last_name, email, password,
    email_verified, reset_password_token, email_verification_token,
    reset_password_expires_at, email_verification_expires_at, auth_type,
    created_at, updated_at
) VALUES (
    @id, @first_name, @last_name, @email, @password,
    @email_verified, @reset_password_token, @email_verification_token,
    @reset_password_expires_at, @email_verification_expires_at, @auth_type,
    CURRENT_TIMESTAMP, CURRENT_TIMESTAMP
) ON CONFLICT (email, deleted_at) DO NOTHING;

-- ============================================================================
-- UPDATE Operations
-- ============================================================================

-- name: UpdateUser :execresult
UPDATE convoy.users SET
    first_name = @first_name,
    last_name = @last_name,
    email = @email,
    password = @password,
    email_verified = @email_verified,
    reset_password_token = @reset_password_token,
    email_verification_token = @email_verification_token,
    reset_password_expires_at = @reset_password_expires_at,
    email_verification_expires_at = @email_verification_expires_at,
    updated_at = CURRENT_TIMESTAMP
WHERE id = @id AND deleted_at IS NULL;

-- ============================================================================
-- FETCH Operations
-- ============================================================================

-- name: FindUserByID :one
SELECT * FROM convoy.users WHERE id = @id AND deleted_at IS NULL;

-- name: FindUserByEmail :one
SELECT * FROM convoy.users WHERE email = @email AND deleted_at IS NULL;

-- name: FindUserByToken :one
SELECT * FROM convoy.users WHERE reset_password_token = sqlc.arg(token)::text AND deleted_at IS NULL;

-- name: FindUserByEmailVerificationToken :one
SELECT * FROM convoy.users WHERE email_verification_token = sqlc.arg(token)::text AND deleted_at IS NULL;

-- ============================================================================
-- COUNT Operations
-- ============================================================================

-- name: CountUsers :one
SELECT COUNT(*) AS count FROM convoy.users WHERE deleted_at IS NULL;
