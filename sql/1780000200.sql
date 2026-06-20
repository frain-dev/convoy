-- +migrate Up
SET lock_timeout = '2s';
SET statement_timeout = '30s';

-- checkout_license_key holds the guest-checkout (purchased) license, owned solely
-- by the checkout flow. license_key now holds the EFFECTIVE license (env/file key
-- wins, else the purchased key), and license_key_source records its provenance so
-- the resolved state is debuggable from the row alone.
ALTER TABLE convoy.configurations ADD COLUMN IF NOT EXISTS checkout_license_key TEXT NOT NULL DEFAULT '';
ALTER TABLE convoy.configurations ADD COLUMN IF NOT EXISTS license_key_source TEXT NOT NULL DEFAULT '';

-- Backfill: existing license_key values were written solely by guest checkout, so
-- seed the checkout-owned column and provenance from them. An env/file license is
-- re-resolved at boot and overwrites license_key/license_key_source.
UPDATE convoy.configurations
SET checkout_license_key = license_key,
    license_key_source = 'guest_checkout'
WHERE deleted_at IS NULL AND license_key <> '';

-- +migrate Down
-- squawk-ignore ban-drop-column
ALTER TABLE convoy.configurations DROP COLUMN IF EXISTS checkout_license_key;
-- squawk-ignore ban-drop-column
ALTER TABLE convoy.configurations DROP COLUMN IF EXISTS license_key_source;
