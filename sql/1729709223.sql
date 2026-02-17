-- +migrate Up
SET lock_timeout = '2s';
SET statement_timeout = '30s';
ALTER TABLE convoy.users ADD COLUMN IF NOT EXISTS auth_type TEXT NOT NULL DEFAULT 'local';

-- +migrate Down
ALTER TABLE convoy.users DROP COLUMN IF EXISTS auth_type;
