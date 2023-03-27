-- +migrate Up
CREATE TABLE IF NOT EXISTS convoy.subscriptions_backup AS SELECT * FROM convoy.subscriptions;

-- +migrate Up
ALTER TABLE convoy.subscriptions
DROP CONSTRAINT IF EXISTS subscriptions_endpoint_id_fkey;

-- +migrate Up
ALTER TABLE convoy.subscriptions
ALTER COLUMN endpoint_id DROP NOT NULL;

-- +migrate Up
ALTER TABLE convoy.subscriptions
ADD CONSTRAINT subscriptions_endpoint_id_fkey
FOREIGN KEY (endpoint_id) REFERENCES convoy.endpoints(id);

-- +migrate Up
UPDATE convoy.subscriptions SET endpoint_id = NULL WHERE endpoint_id = '';

-- +migrate Up
SELECT * FROM convoy.subscriptions;

-- +migrate Up
DROP TABLE IF EXISTS convoy.subscriptions_backup;


-- +migrate Down
CREATE TABLE IF NOT EXISTS convoy.subscriptions_backup AS SELECT * FROM convoy.subscriptions;

-- +migrate Down
ALTER TABLE convoy.subscriptions
DROP CONSTRAINT IF EXISTS subscriptions_endpoint_id_fkey;

-- +migrate Down
ALTER TABLE convoy.subscriptions
ALTER COLUMN endpoint_id SET NOT NULL;

-- +migrate Down
ALTER TABLE convoy.subscriptions
ADD CONSTRAINT subscriptions_endpoint_id_fkey
FOREIGN KEY (endpoint_id) REFERENCES convoy.endpoints(id);

-- +migrate Down
UPDATE convoy.subscriptions SET endpoint_id = '' WHERE endpoint_id IS NULL;

-- +migrate Down
SELECT * FROM convoy.subscriptions;

-- +migrate Down
DROP TABLE IF EXISTS convoy.subscriptions_backup;
