-- +migrate Up
ALTER TABLE convoy.portal_links 
    ADD COLUMN IF NOT EXISTS owner_id VARCHAR,
    ADD COLUMN IF NOT EXISTS endpoint_management BOOLEAN,
    ALTER COLUMN endpoint_management SET DEFAULT false;

-- +migrate Down
ALTER TABLE convoy.portal_links 
    DROP COLUMN IF EXISTS owner_id,
    DROP COLUMN IF EXISTS endpoint_management;
