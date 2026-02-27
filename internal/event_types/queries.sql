-- Event Types Queries
-- Schema: convoy.event_types
-- Columns: id, name, description, category, project_id, json_schema, created_at, updated_at, deprecated_at

-- ============================================================================
-- CREATE Operations
-- ============================================================================

-- name: CreateEventType :exec
INSERT INTO convoy.event_types (
    id, name, description, category, project_id, json_schema, created_at, updated_at
) VALUES (
    @id, @name, @description, @category, @project_id, @json_schema, NOW(), NOW()
);

-- name: CreateDefaultEventType :exec
INSERT INTO convoy.event_types (
    id, name, description, category, project_id, json_schema, created_at, updated_at
) VALUES (
    @id, @name, @description, @category, @project_id, @json_schema, NOW(), NOW()
);

-- ============================================================================
-- UPDATE Operations
-- ============================================================================

-- name: UpdateEventType :execresult
UPDATE convoy.event_types
SET
    description = @description,
    category = @category,
    json_schema = @json_schema,
    updated_at = NOW()
WHERE id = @id AND project_id = @project_id;

-- name: DeprecateEventType :one
UPDATE convoy.event_types
SET
    deprecated_at = NOW()
WHERE id = @id AND project_id = @project_id
RETURNING
    id,
    name,
    description,
    category,
    project_id,
    json_schema,
    created_at,
    updated_at,
    deprecated_at;

-- ============================================================================
-- READ Operations - Single Record
-- ============================================================================

-- name: FetchEventTypeByID :one
SELECT
    id,
    name,
    description,
    category,
    project_id,
    json_schema,
    created_at,
    updated_at,
    deprecated_at
FROM convoy.event_types
WHERE id = @id AND project_id = @project_id;

-- name: FetchEventTypeByName :one
SELECT
    id,
    name,
    description,
    category,
    project_id,
    json_schema,
    created_at,
    updated_at,
    deprecated_at
FROM convoy.event_types
WHERE name = @name AND project_id = @project_id;

-- ============================================================================
-- READ Operations - Multiple Records
-- ============================================================================

-- name: FetchAllEventTypes :many
SELECT
    id,
    name,
    description,
    category,
    project_id,
    json_schema,
    created_at,
    updated_at,
    deprecated_at
FROM convoy.event_types
WHERE project_id = @project_id
ORDER BY created_at DESC;

-- ============================================================================
-- CHECK Operations
-- ============================================================================

-- name: CheckEventTypeExists :one
SELECT EXISTS(
    SELECT 1
    FROM convoy.event_types
    WHERE name = @name AND project_id = @project_id
) AS exists;
