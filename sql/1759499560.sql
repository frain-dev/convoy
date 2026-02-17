-- +migrate Up
SET lock_timeout = '2s';
SET statement_timeout = '30s';
CREATE TYPE convoy.endpoint_content_types AS ENUM ('application/json', 'application/x-www-form-urlencoded');
ALTER TABLE convoy.endpoints ADD COLUMN IF NOT EXISTS content_type convoy.endpoint_content_types NOT NULL DEFAULT 'application/json';

-- +migrate Down
ALTER TABLE convoy.endpoints DROP COLUMN IF EXISTS content_type;
DROP TYPE IF EXISTS convoy.endpoint_content_types;
