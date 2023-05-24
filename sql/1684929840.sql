-- +migrate Up
ALTER TABLE convoy.sources ADD COLUMN custom_response VARCHAR;
-- +migrate Down
ALTER TABLE convoy.sources DROP COLUMN custom_response;

