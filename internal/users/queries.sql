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
);

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

-- Rotate verification token only while the account is still unverified.
-- Prevents a stale resend UpdateUser from undoing VerifyEmailService.
-- name: RotateEmailVerificationToken :execresult
UPDATE convoy.users SET
    email_verification_token = @email_verification_token,
    email_verification_expires_at = @email_verification_expires_at,
    updated_at = CURRENT_TIMESTAMP
WHERE id = @id AND deleted_at IS NULL AND email_verified = false;


-- ============================================================================
-- FETCH Operations
-- ============================================================================

-- name: FindUserByID :one
SELECT
    id, first_name, last_name, email, password, email_verified,
    reset_password_token, email_verification_token,
    reset_password_expires_at, email_verification_expires_at,
    auth_type, created_at, updated_at, deleted_at
FROM convoy.users
WHERE id = @id AND deleted_at IS NULL;

-- name: FindUserByEmail :one
SELECT
    id, first_name, last_name, email, password, email_verified,
    reset_password_token, email_verification_token,
    reset_password_expires_at, email_verification_expires_at,
    auth_type, created_at, updated_at, deleted_at
FROM convoy.users
WHERE email = @email AND deleted_at IS NULL;

-- name: FindUserByToken :one
SELECT
    id, first_name, last_name, email, password, email_verified,
    reset_password_token, email_verification_token,
    reset_password_expires_at, email_verification_expires_at,
    auth_type, created_at, updated_at, deleted_at
FROM convoy.users
WHERE reset_password_token = @reset_password_token AND deleted_at IS NULL;

-- name: FindUserByEmailVerificationToken :one
SELECT
    id, first_name, last_name, email, password, email_verified,
    reset_password_token, email_verification_token,
    reset_password_expires_at, email_verification_expires_at,
    auth_type, created_at, updated_at, deleted_at
FROM convoy.users
WHERE email_verification_token = @email_verification_token AND deleted_at IS NULL;

-- ============================================================================
-- COUNT Operations
-- ============================================================================

-- name: CountUsers :one
SELECT COUNT(*) AS count FROM convoy.users WHERE deleted_at IS NULL;
