-- +migrate Up
SET lock_timeout = '2s';
SET statement_timeout = '30s';

ALTER TABLE convoy.configurations ADD COLUMN IF NOT EXISTS license_key TEXT NOT NULL DEFAULT '';
ALTER TABLE convoy.configurations ADD COLUMN IF NOT EXISTS checkout_attempts JSONB NOT NULL DEFAULT '{}'::jsonb;
ALTER TABLE convoy.configurations ADD COLUMN IF NOT EXISTS active_checkout_attempt_id TEXT NOT NULL DEFAULT '';
ALTER TABLE convoy.configurations ADD COLUMN IF NOT EXISTS checkout_id TEXT NOT NULL DEFAULT '';
ALTER TABLE convoy.configurations ADD COLUMN IF NOT EXISTS external_id TEXT NOT NULL DEFAULT '';
ALTER TABLE convoy.configurations ADD COLUMN IF NOT EXISTS license_synced_at TIMESTAMPTZ;

-- +migrate Down
-- squawk-ignore ban-drop-column
ALTER TABLE convoy.configurations DROP COLUMN IF EXISTS license_key;
-- squawk-ignore ban-drop-column
ALTER TABLE convoy.configurations DROP COLUMN IF EXISTS checkout_attempts;
-- squawk-ignore ban-drop-column
ALTER TABLE convoy.configurations DROP COLUMN IF EXISTS active_checkout_attempt_id;
-- squawk-ignore ban-drop-column
ALTER TABLE convoy.configurations DROP COLUMN IF EXISTS checkout_id;
-- squawk-ignore ban-drop-column
ALTER TABLE convoy.configurations DROP COLUMN IF EXISTS external_id;
-- squawk-ignore ban-drop-column
ALTER TABLE convoy.configurations DROP COLUMN IF EXISTS license_synced_at;
