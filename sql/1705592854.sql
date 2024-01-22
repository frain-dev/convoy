-- +migrate Up
CREATE TABLE IF NOT EXISTS convoy.endpoints_portal_owner_ids (
    endpoint_id VARCHAR NOT NULL REFERENCES convoy.endpoints (id),
    owner_id VARCHAR NOT NULL,
    deleted_at TIMESTAMPTZ,
    FOREIGN KEY (owner_id, deleted_at) REFERENCES convoy.portal_links (owner_id, deleted_at)
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_unique_owner_id_deleted_at ON convoy.portal_links (owner_id, deleted_at) WHERE deleted_at IS NOT NULL;

-- +migrate Up
CREATE INDEX IF NOT EXISTS idx_endpoints_portal_owner_ids_endpoint_id ON convoy.endpoints_portal_owner_ids (endpoint_id, owner_id, deleted_at);

-- +migrate Up
INSERT INTO convoy.endpoints_portal_owner_ids (endpoint_id, owner_id, deleted_at)
SELECT e.id AS endpoint_id, e.owner_id, e.deleted_at
FROM convoy.endpoints e INNER JOIN convoy.portal_links l ON e.owner_id = l.owner_id
WHERE e.owner_id IS NOT NULL AND COALESCE(TRIM(e.owner_id), '') <> '';

-- +migrate Down
DROP table IF EXISTS convoy.endpoints_portal_owner_ids;

-- +migrate Down
DROP INDEX IF EXISTS convoy.idx_unique_owner_id_deleted_at;

-- +migrate Down
DROP INDEX IF EXISTS convoy.idx_endpoints_portal_owner_ids_endpoint_id;
