-- +migrate Up
SET lock_timeout = '2s';
SET statement_timeout = '30s';

-- Honest column names for the four product knobs on configurations.
-- retention_policy_enabled stored webhook archiving only (not partition drop);
-- retention_policy_policy stored the keep window. Partition drop was gated by
-- env CONVOY_RETENTION_ENABLED (default true), not a configurations column.
-- retention_enabled is nullable on ADD so existing rows stay NULL (= unset).
-- First boot copies env into the column once; after that DB/UI owns it.
-- Do not DEFAULT true (would ignore env=false upgrades) or backfill from
-- webhook_archiving_enabled (archiving-off must not lose partition drop).
ALTER TABLE convoy.configurations
  RENAME COLUMN retention_policy_enabled TO webhook_archiving_enabled;
ALTER TABLE convoy.configurations
  RENAME COLUMN retention_policy_policy TO retention_period;
ALTER TABLE convoy.configurations
  ADD COLUMN retention_enabled BOOLEAN;

RESET lock_timeout;
RESET statement_timeout;

-- +migrate Down
SET lock_timeout = '2s';
SET statement_timeout = '30s';

ALTER TABLE convoy.configurations
  DROP COLUMN IF EXISTS retention_enabled;
ALTER TABLE convoy.configurations
  RENAME COLUMN retention_period TO retention_policy_policy;
ALTER TABLE convoy.configurations
  RENAME COLUMN webhook_archiving_enabled TO retention_policy_enabled;

RESET lock_timeout;
RESET statement_timeout;
