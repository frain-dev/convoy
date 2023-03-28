-- +migrate Up
CREATE TABLE IF NOT EXISTS convoy.devices_backup AS SELECT * FROM convoy.devices;
ALTER TABLE convoy.devices DROP COLUMN endpoint_id;
DROP TABLE IF EXISTS convoy.devices_backup;

-- +migrate Down
CREATE TABLE IF NOT EXISTS convoy.devices_backup AS SELECT * FROM convoy.devices;
ALTER TABLE convoy.devices ADD COLUMN endpoint_id CHAR(26) REFERENCES convoy.endpoints (id);
DROP TABLE IF EXISTS convoy.devices_backup;

