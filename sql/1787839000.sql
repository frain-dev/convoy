-- +migrate Up
SET lock_timeout = '2s';
SET statement_timeout = '30s';

ALTER TABLE convoy.configurations
	ADD COLUMN IF NOT EXISTS admin_managed boolean;

-- Existing rows stay NULL until first boot preserves their DB settings and
-- completes the one-time ownership migration. New rows default to env.
ALTER TABLE convoy.configurations
	ALTER COLUMN admin_managed SET DEFAULT false;

RESET lock_timeout;
RESET statement_timeout;

-- +migrate Down
SET lock_timeout = '2s';
SET statement_timeout = '30s';

ALTER TABLE convoy.configurations
	DROP COLUMN IF EXISTS admin_managed;

RESET lock_timeout;
RESET statement_timeout;
