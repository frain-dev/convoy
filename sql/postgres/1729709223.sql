-- +migrate Up
ALTER TABLE convoy.users ADD COLUMN IF NOT EXISTS auth_type TEXT NOT NULL DEFAULT 'local';

-- +migrate Down
ALTER TABLE convoy.users DROP COLUMN IF EXISTS auth_type;
