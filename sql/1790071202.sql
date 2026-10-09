-- +migrate Up
CREATE TABLE convoy.queue_operation_workers (
    scope TEXT NOT NULL REFERENCES convoy.queue_operation_scopes(scope),
    id TEXT NOT NULL,
    store_id TEXT NOT NULL,
    configuration_revision TEXT NOT NULL,
    backend_pid INTEGER NOT NULL,
    lease_key BIGINT NOT NULL,
    queues TEXT[] NOT NULL DEFAULT '{}',
    tasks TEXT[] NOT NULL DEFAULT '{}',
    operation_id TEXT NOT NULL DEFAULT '',
    revision BIGINT NOT NULL DEFAULT 0,
    state TEXT NOT NULL CHECK (state IN ('starting','running','stopped','closed')),
    observed_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    PRIMARY KEY(scope,id)
);
-- Records deliberately do not expire: loss of contact is not a stopped claim loop.
-- +migrate Down
DROP TABLE convoy.queue_operation_workers;
