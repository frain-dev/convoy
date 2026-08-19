-- +migrate Up
SET lock_timeout = '2s';
SET statement_timeout = '30s';

-- Rebuilding a dropped index is the same kind of work a partition conversion is:
-- one long operation on one table, holding locks, needing progress to reach a
-- session other than the one running it. It gets the same run row rather than a
-- table of its own, so there is one runner concept, one polling pattern, and one
-- instance-wide single-active guard.
--
-- That guard is the reason to share rather than mirror. It is a unique index on
-- status = 'running' over this whole table, so a rebuild and a conversion now
-- exclude each other. That is what we want: the index being rebuilt lives on a
-- table a conversion may be rewriting, and the two would contend for locks on
-- the same relation.
ALTER TABLE convoy.partition_runs
    DROP CONSTRAINT IF EXISTS partition_runs_operation_check;

ALTER TABLE convoy.partition_runs
    ADD CONSTRAINT partition_runs_operation_check
    CHECK (operation IN ('partition', 'unpartition', 'rebuild_index'));

-- table_name says which table the work is on, which a rebuild has too. The index
-- is the part a rebuild needs and the conversions have no equivalent for, so it
-- is its own nullable column rather than a reuse of an existing one.
ALTER TABLE convoy.partition_runs
    ADD COLUMN IF NOT EXISTS index_name TEXT;

-- The column is only meaningful for one operation, so the pairing is enforced
-- here instead of being left to the writer. A rebuild row without an index names
-- no work, and a conversion row with one claims work it did not do.
ALTER TABLE convoy.partition_runs
    DROP CONSTRAINT IF EXISTS partition_runs_index_name_check;

ALTER TABLE convoy.partition_runs
    ADD CONSTRAINT partition_runs_index_name_check
    CHECK ((operation = 'rebuild_index') = (index_name IS NOT NULL));

RESET lock_timeout;
RESET statement_timeout;

-- +migrate Down
SET lock_timeout = '2s';
SET statement_timeout = '30s';

-- Rebuild rows cannot survive the narrowed CHECK, and they describe work that
-- already happened, so they go with the column rather than blocking the down
-- migration.
DELETE FROM convoy.partition_runs WHERE operation = 'rebuild_index';

ALTER TABLE convoy.partition_runs
    DROP CONSTRAINT IF EXISTS partition_runs_index_name_check;

ALTER TABLE convoy.partition_runs
    DROP COLUMN IF EXISTS index_name;

ALTER TABLE convoy.partition_runs
    DROP CONSTRAINT IF EXISTS partition_runs_operation_check;

ALTER TABLE convoy.partition_runs
    ADD CONSTRAINT partition_runs_operation_check
    CHECK (operation IN ('partition', 'unpartition'));

RESET lock_timeout;
RESET statement_timeout;
