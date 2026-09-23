-- +migrate Up
SET lock_timeout = '2s';
SET statement_timeout = '30s';

CREATE TABLE convoy.queue_operation_scopes (
    scope TEXT PRIMARY KEY CHECK (length(scope) BETWEEN 1 AND 200),
    epoch BIGINT NOT NULL DEFAULT 0 CHECK (epoch >= 0),
    current_operation TEXT
);

CREATE TABLE convoy.queue_operations (
    id TEXT PRIMARY KEY,
    scope TEXT NOT NULL REFERENCES convoy.queue_operation_scopes(scope),
    idempotency_key TEXT NOT NULL CHECK (length(idempotency_key) BETWEEN 1 AND 200),
    document JSONB NOT NULL,
    UNIQUE (scope, idempotency_key)
);

CREATE TABLE convoy.queue_operation_history (
    operation_id TEXT NOT NULL REFERENCES convoy.queue_operations(id),
    revision BIGINT NOT NULL,
    state TEXT NOT NULL,
    actor TEXT NOT NULL DEFAULT 'runtime',
    observed_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    PRIMARY KEY (operation_id, revision)
);

-- Receipts are durable accounting, not expiring leases. Losing contact with a
-- process does not prove its accepted writes finished. Recovery must reconcile
-- the receipt before settling it; time alone must never remove this blocker.
CREATE TABLE convoy.queue_operation_receipts (
    id TEXT PRIMARY KEY,
    scope TEXT NOT NULL REFERENCES convoy.queue_operation_scopes(scope),
    store_id TEXT NOT NULL CHECK (length(store_id) BETWEEN 1 AND 200),
    epoch BIGINT NOT NULL,
    kind TEXT NOT NULL CHECK (kind IN ('producer', 'claim')),
    admitted_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    settled_at TIMESTAMPTZ
);
RESET lock_timeout;
RESET statement_timeout;

-- +migrate Down
SET lock_timeout = '2s';
SET statement_timeout = '30s';

DROP TABLE convoy.queue_operation_receipts;
DROP TABLE convoy.queue_operation_history;
DROP TABLE convoy.queue_operations;
DROP TABLE convoy.queue_operation_scopes;

RESET lock_timeout;
RESET statement_timeout;
