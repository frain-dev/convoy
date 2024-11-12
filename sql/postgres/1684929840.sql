-- +migrate Up
ALTER TABLE convoy.sources ADD COLUMN IF NOT EXISTS custom_response_body VARCHAR;
ALTER TABLE convoy.sources ADD COLUMN IF NOT EXISTS custom_response_content_type VARCHAR;
-- +migrate Down
ALTER TABLE convoy.sources DROP COLUMN IF EXISTS custom_response_content_type;
ALTER TABLE convoy.sources DROP COLUMN IF EXISTS custom_response_body;

