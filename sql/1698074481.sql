-- +migrate Up
SET lock_timeout = '2s';
SET statement_timeout = '30s';
ALTER TABLE convoy.configurations ADD COLUMN s3_prefix text;

-- +migrate Down
-- squawk-ignore ban-drop-column
ALTER TABLE convoy.configurations DROP COLUMN s3_prefix;

