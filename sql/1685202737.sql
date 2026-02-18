-- squawk-ignore-file ban-drop-column, ban-drop-not-null
-- +migrate Up
SET lock_timeout = '2s';
SET statement_timeout = '30s';
-- squawk-ignore ban-drop-not-null
ALTER TABLE convoy.portal_links
    ADD COLUMN IF NOT EXISTS owner_id VARCHAR,
    ADD COLUMN IF NOT EXISTS can_manage_endpoint BOOLEAN,
    ALTER COLUMN can_manage_endpoint SET DEFAULT false,
    ALTER COLUMN endpoints DROP NOT NULL;

-- +migrate Up
SET lock_timeout = '2s';
SET statement_timeout = '30s';
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_portal_links_owner_id_key ON convoy.portal_links (owner_id);

-- +migrate Down
-- squawk-ignore ban-drop-column
ALTER TABLE convoy.portal_links
    DROP COLUMN IF EXISTS owner_id,
    DROP COLUMN IF EXISTS can_manage_endpoint;

-- +migrate Down
DROP INDEX CONCURRENTLY IF EXISTS convoy.idx_portal_links_owner_id_key;
