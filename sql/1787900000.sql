-- +migrate Up
SET lock_timeout = '2s';
SET statement_timeout = '30s';

-- One row per data plane replica, holding the snapshot it last published.
-- The dashboard is served by a different process than the one running a data
-- plane, so a plane's depth and backlog have to cross the database to be
-- readable, the same way scheduler entries and queue metrics already do.
--
-- The sections are stored as JSON because they are named by the publishing
-- plane. A column per stage or writer would make renaming one a migration, and
-- the reader only ever loads whole rows. There is no index: the table holds one
-- row per replica, so every read and the expiry sweep are a handful of rows.
--
-- mode and running are duplicated out of the JSON deliberately. The reader takes
-- them from the snapshot, so nothing in the application selects these columns;
-- they exist so an operator with psql can see which replicas are up and what
-- they are running without unpacking JSON. sampled_at is not duplication: the
-- expiry sweep filters on it.
CREATE TABLE IF NOT EXISTS convoy.data_plane_snapshots (
    replica     TEXT PRIMARY KEY,
    mode        TEXT NOT NULL,
    running     BOOLEAN NOT NULL,
    sampled_at  TIMESTAMPTZ NOT NULL,
    snapshot    JSONB NOT NULL,
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

RESET lock_timeout;
RESET statement_timeout;

-- +migrate Down
SET lock_timeout = '2s';
SET statement_timeout = '30s';

DROP TABLE IF EXISTS convoy.data_plane_snapshots;

RESET lock_timeout;
RESET statement_timeout;
