-- +migrate Up
ALTER TABLE convoy.portal_links ADD COLUMN IF NOT EXISTS owner_id VARCHAR;
ALTER TABLE convoy.portal_links ADD COLUMN IF NOT EXISTS endpoint_management VARCHAR;

-- +migrate Down
ALTER TABLE convoy.portal_links DROP COLUMN IF EXISTS owner_id VARCHAR;
ALTER TABLE convoy.portal_links DROP COLUMN IF EXISTS endpoint_management VARCHAR;
