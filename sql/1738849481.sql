-- +migrate Up
CREATE TABLE IF NOT EXISTS convoy.test_rollback(
	id VARCHAR NOT NULL PRIMARY KEY
);

-- +migrate Down
DROP TABLE IF EXISTS convoy.test_rollback;
