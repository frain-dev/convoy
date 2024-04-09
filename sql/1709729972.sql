-- +migrate Up
ALTER TABLE convoy.endpoints
    ADD COLUMN IF NOT EXISTS name text,
    ADD COLUMN IF NOT EXISTS url text;

-- +migrate Up
UPDATE convoy.endpoints
SET name = title, url = target_url;

-- +migrate Up
ALTER TABLE convoy.endpoints
    ALTER COLUMN url SET NOT NULL,
    ALTER COLUMN name SET NOT NULL,
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
    ALTER COLUMN title SET NOT NULL,
    ALTER COLUMN target_url SET NOT NULL,
    DROP COLUMN IF EXISTS url,
    DROP COLUMN IF EXISTS name;
