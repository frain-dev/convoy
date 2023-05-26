-- +migrate Up
ALTER TABLE convoy.sources ADD COLUMN custom_response_body VARCHAR;
ALTER TABLE convoy.sources ADD COLUMN custom_response_content_type VARCHAR;
-- +migrate Down
ALTER TABLE convoy.sources DROP COLUMN custom_response_content_type;
ALTER TABLE convoy.sources DROP COLUMN custom_response_body;

