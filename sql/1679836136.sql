-- +migrate Up
SET lock_timeout = '2s';
SET statement_timeout = '30s';
CREATE TABLE IF NOT EXISTS convoy.devices_backup AS SELECT * FROM convoy.devices;
-- squawk-ignore ban-drop-column
ALTER TABLE convoy.devices DROP COLUMN endpoint_id;
DROP TABLE IF EXISTS convoy.devices_backup;

-- +migrate Down
CREATE TABLE IF NOT EXISTS convoy.devices_backup AS SELECT * FROM convoy.devices;
ALTER TABLE convoy.devices ADD COLUMN endpoint_id VARCHAR(26);
ALTER TABLE convoy.devices ADD CONSTRAINT devices_endpoint_id_fkey
    FOREIGN KEY (endpoint_id)
        REFERENCES convoy.endpoints (id)
        NOT VALID;
ALTER TABLE convoy.devices VALIDATE CONSTRAINT devices_endpoint_id_fkey;
DROP TABLE IF EXISTS convoy.devices_backup;

