-- +migrate Up
SET lock_timeout = '2s';
SET statement_timeout = '30s';
ALTER TABLE convoy.sources ADD COLUMN IF NOT EXISTS custom_response_body VARCHAR;
ALTER TABLE convoy.sources ADD COLUMN IF NOT EXISTS custom_response_content_type VARCHAR;
-- +migrate Down
-- squawk-ignore ban-drop-column
ALTER TABLE convoy.sources DROP COLUMN IF EXISTS custom_response_content_type;
-- squawk-ignore ban-drop-column
ALTER TABLE convoy.sources DROP COLUMN IF EXISTS custom_response_body;

