-- +migrate Up
ALTER TABLE convoy.configurations ADD COLUMN s3_prefix text;

-- +migrate Down
ALTER TABLE convoy.configurations DROP COLUMN s3_prefix;

