-- +migrate Up
SET lock_timeout = '2s';
SET statement_timeout = '30s';
ALTER TABLE convoy.endpoints
    ADD COLUMN IF NOT EXISTS name text,
    ADD COLUMN IF NOT EXISTS url text;

-- +migrate Up
SET lock_timeout = '2s';
SET statement_timeout = '30s';
UPDATE convoy.endpoints
SET name = title, url = target_url;

-- +migrate Up
SET lock_timeout = '2s';
SET statement_timeout = '30s';
ALTER TABLE convoy.endpoints
    ADD CONSTRAINT url_not_null CHECK (url IS NOT NULL) NOT VALID,
    ADD CONSTRAINT name_not_null CHECK (name IS NOT NULL) NOT VALID;

-- +migrate Up
ALTER TABLE convoy.endpoints
    VALIDATE CONSTRAINT url_not_null,
    VALIDATE CONSTRAINT name_not_null;

-- +migrate Up
SET lock_timeout = '2s';
SET statement_timeout = '30s';
ALTER TABLE convoy.endpoints
    DROP COLUMN IF EXISTS title,
    DROP COLUMN IF EXISTS target_url;

-- +migrate Down
ALTER TABLE convoy.endpoints
    ADD COLUMN IF NOT EXISTS title text,
    ADD COLUMN IF NOT EXISTS target_url text;

-- +migrate Down
UPDATE convoy.endpoints
SET title = name, target_url = url;

-- +migrate Down
ALTER TABLE convoy.endpoints
    ADD CONSTRAINT title_not_null CHECK (title IS NOT NULL) NOT VALID,
    ADD CONSTRAINT target_url_not_null CHECK (target_url IS NOT NULL) NOT VALID;

-- +migrate Down
ALTER TABLE convoy.endpoints
    VALIDATE CONSTRAINT title_not_null,
    VALIDATE CONSTRAINT target_url_not_null;

-- +migrate Down
ALTER TABLE convoy.endpoints
    DROP COLUMN IF EXISTS url,
    DROP COLUMN IF EXISTS name;
