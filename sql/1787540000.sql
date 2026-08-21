-- +migrate Up
SET lock_timeout = '2s';
SET statement_timeout = '30s';

-- A unique rebuild that hits duplicate rows is not a transient failure. Retrying
-- it every boot would hold the slot and fail the same way until an operator
-- fixes the data, so the debt records why it is blocked.
--
-- The function that clears these columns lives in the next migration: the
-- structure linter reads line by line, so a DROP INDEX inside a function body
-- counts as an index operation and cannot share a block with column DDL.
ALTER TABLE convoy.dropped_indexes
    ADD COLUMN IF NOT EXISTS blocked_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS blocked_reason TEXT;

RESET lock_timeout;
RESET statement_timeout;

-- +migrate Down
SET lock_timeout = '2s';
SET statement_timeout = '30s';

ALTER TABLE convoy.dropped_indexes
    DROP COLUMN IF EXISTS blocked_reason,
    DROP COLUMN IF EXISTS blocked_at;

RESET lock_timeout;
RESET statement_timeout;
