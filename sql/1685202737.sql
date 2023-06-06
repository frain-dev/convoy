-- +migrate Up
ALTER TABLE convoy.portal_links
    ADD COLUMN IF NOT EXISTS owner_id VARCHAR,
    ADD COLUMN IF NOT EXISTS can_manage_endpoint BOOLEAN,
    ALTER COLUMN can_manage_endpoint SET DEFAULT false,
    ALTER COLUMN endpoints DROP NOT NULL;

-- +migrate Up
CREATE INDEX IF NOT EXISTS idx_portal_links_owner_id_key ON convoy.portal_links (owner_id);

-- +migrate Down
ALTER TABLE convoy.portal_links
    DROP COLUMN IF EXISTS owner_id,
    DROP COLUMN IF EXISTS can_manage_endpoint;

-- +migrate Down
DROP INDEX IF EXISTS convoy.idx_portal_links_owner_id_key;
