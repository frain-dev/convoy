-- +migrate Up
SET lock_timeout = '2s';
SET statement_timeout = '30s';

-- Instance configuration is a singleton. Older installs can accumulate
-- multiple live rows; LoadConfiguration used unordered LIMIT 1, so Admin
-- toggles and /cmd backup could read different rows than Update wrote.
-- Keep the newest live row (by updated_at, then id, then ctid), soft-delete
-- the rest, then enforce at most one live row going forward.
WITH ranked AS (
  SELECT ctid,
         ROW_NUMBER() OVER (
           ORDER BY updated_at DESC NULLS LAST, id DESC, ctid
         ) AS rn
  FROM convoy.configurations
  WHERE deleted_at IS NULL
)
UPDATE convoy.configurations c
SET deleted_at = NOW(),
    updated_at = NOW()
FROM ranked r
WHERE c.ctid = r.ctid
  AND r.rn > 1;

CREATE UNIQUE INDEX IF NOT EXISTS configurations_one_live_row
  ON convoy.configurations ((true))
  WHERE deleted_at IS NULL;

RESET lock_timeout;
RESET statement_timeout;

-- +migrate Down
SET lock_timeout = '2s';
SET statement_timeout = '30s';

DROP INDEX IF EXISTS convoy.configurations_one_live_row;

RESET lock_timeout;
RESET statement_timeout;
