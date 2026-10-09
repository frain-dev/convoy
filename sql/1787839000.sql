-- +migrate Up
SET lock_timeout = '2s';
SET statement_timeout = '30s';

ALTER TABLE convoy.configurations
	ADD COLUMN IF NOT EXISTS admin_managed boolean;

-- Existing rows stay NULL until first boot marks ownership known as env
-- (admin_managed=false). Admin Managed is opt-in via the Admin UI.
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
