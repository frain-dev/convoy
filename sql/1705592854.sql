-- +migrate Up
ALTER TABLE convoy.portal_links ADD CONSTRAINT portal_links_owner_id unique (owner_id, deleted_at);
CREATE TABLE IF NOT EXISTS convoy.endpoints_portal_links (
    endpoint_id VARCHAR NOT NULL REFERENCES convoy.endpoints (id),
    owner_id VARCHAR NOT NULL,
    deleted_at TIMESTAMPTZ,
    FOREIGN KEY (owner_id, deleted_at) REFERENCES convoy.portal_links (owner_id, deleted_at)
);

-- +migrate Up
CREATE INDEX IF NOT EXISTS idx_endpoints_portal_links_endpoint_id ON convoy.endpoints_portal_links (endpoint_id, owner_id, deleted_at);

-- +migrate Up
INSERT INTO convoy.endpoints_portal_links (endpoint_id, owner_id, deleted_at)
SELECT e.id AS endpoint_id, e.owner_id, e.deleted_at
FROM convoy.endpoints e INNER JOIN convoy.portal_links l ON e.owner_id = l.owner_id
WHERE e.owner_id IS NOT NULL AND COALESCE(TRIM(e.owner_id), '') <> '';

-- +migrate Down
DROP table IF EXISTS convoy.endpoints_portal_links;

-- +migrate Down
ALTER TABLE convoy.portal_links DROP CONSTRAINT IF EXISTS portal_links_owner_id;

-- +migrate Down
DROP INDEX IF EXISTS convoy.idx_endpoints_portal_links_endpoint_id;
