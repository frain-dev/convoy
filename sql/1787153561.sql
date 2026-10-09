-- +migrate Up
SET lock_timeout = '2s';
SET statement_timeout = '30s';

-- An index build that dies partway leaves the index behind, marked invalid. The
-- planner ignores it from then on, so the table reads as if the index was never
-- created, and nothing in Convoy noticed: the only reader of pg_index.indisvalid
-- was the partition preflight, which is why the first report of this came from an
-- operator trying to partition a table.
--
-- Nothing repaired it either. Index migrations build CONCURRENTLY, which cannot
-- run inside a transaction, so they are marked notransaction: when the build
-- fails, sql-migrate returns the error without recording the migration and
-- without a transaction to roll back. The retry on the next boot runs
-- CREATE INDEX CONCURRENTLY IF NOT EXISTS, which sees the invalid index already
-- holding the name, skips the build, and succeeds. The migration is then recorded
-- as applied and the index stays invalid for good.
--
-- This table is what makes dropping such an index recoverable. An invalid index
-- cannot be rebuilt cheaply on a large table, and doing it at boot would stall an
-- upgrade for hours, so the repair is split: drop now, which is instant and
-- unblocks partitioning, and rebuild later on an operator's schedule from the
-- definition recorded here. The drop itself is the next migration, which cannot
-- share this one: index operations and table DDL belong in separate blocks.
CREATE TABLE IF NOT EXISTS convoy.dropped_indexes (
    index_name TEXT PRIMARY KEY,
    table_name TEXT NOT NULL,
    definition TEXT NOT NULL,
    dropped_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    rebuilt_at TIMESTAMPTZ
);

RESET lock_timeout;
RESET statement_timeout;

-- +migrate Down
SET lock_timeout = '2s';
SET statement_timeout = '30s';

DROP TABLE IF EXISTS convoy.dropped_indexes;

RESET lock_timeout;
RESET statement_timeout;
