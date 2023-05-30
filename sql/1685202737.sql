-- +migrate Up
ALTER TABLE convoy.portal_links 
    ADD COLUMN IF NOT EXISTS owner_id VARCHAR,
    ADD COLUMN IF NOT EXISTS endpoint_management BOOLEAN,
    ALTER COLUMN endpoint_management SET DEFAULT false,
    ALTER COLUMN endpoints DROP NOT NULL;

-- +migrate Down
ALTER TABLE convoy.portal_links 
    ALTER COLUMN endpoints SET NOT NULL,
    DROP COLUMN IF EXISTS owner_id,
    DROP COLUMN IF EXISTS endpoint_management;

