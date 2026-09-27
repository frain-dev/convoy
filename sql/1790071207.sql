-- +migrate Up
SET lock_timeout = '2s';
SET statement_timeout = '30s';

-- An upgrade must not erase an operation, an execution receipt, or a closed
-- fence. Resolve existing maintenance state before removing this feature.
-- Hold these locks through the check and drops so an older process cannot
-- create an operation after the check but before the tables disappear.
LOCK TABLE convoy.queue_operation_scopes, convoy.queue_operations,
    convoy.queue_operation_history, convoy.queue_operation_receipts,
    convoy.queue_operation_workers, convoy.queue_execution_ownership,
    convoy.queue_store_fence IN ACCESS EXCLUSIVE MODE;
-- +migrate StatementBegin
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM convoy.queue_operation_scopes)
        OR EXISTS (SELECT 1 FROM convoy.queue_operations)
        OR EXISTS (SELECT 1 FROM convoy.queue_operation_history)
        OR EXISTS (SELECT 1 FROM convoy.queue_operation_receipts)
        OR EXISTS (SELECT 1 FROM convoy.queue_operation_workers)
        OR EXISTS (SELECT 1 FROM convoy.queue_execution_ownership)
        OR EXISTS (SELECT 1 FROM convoy.queue_store_fence)
    THEN
        RAISE EXCEPTION 'queue maintenance state exists; resolve it before removing the schema';
    END IF;
END;
$$;
-- +migrate StatementEnd

DROP TRIGGER queue_jobs_store_fence ON convoy.queue_jobs;
DROP TRIGGER queue_state_store_fence ON convoy.queue_state;
DROP FUNCTION convoy.enforce_queue_store_fence();
DROP TABLE convoy.queue_store_fence;
DROP TABLE convoy.queue_operation_workers;
DROP TABLE convoy.queue_execution_ownership;
DROP TABLE convoy.queue_operation_receipts;
DROP TABLE convoy.queue_operation_history;
DROP TABLE convoy.queue_operations;
DROP TABLE convoy.queue_operation_scopes;

RESET lock_timeout;
RESET statement_timeout;

-- +migrate Down
SET lock_timeout = '2s';
SET statement_timeout = '30s';

-- Rollback restores the schema, not any rows that were intentionally removed.
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
CREATE TABLE convoy.queue_operation_receipts (
    id TEXT PRIMARY KEY,
    scope TEXT NOT NULL REFERENCES convoy.queue_operation_scopes(scope),
    store_id TEXT NOT NULL CHECK (length(store_id) BETWEEN 1 AND 200),
    epoch BIGINT NOT NULL,
    kind TEXT NOT NULL CHECK (kind IN ('producer', 'claim')),
    admitted_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    settled_at TIMESTAMPTZ
);
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
CREATE TABLE convoy.queue_execution_ownership (
    scope TEXT NOT NULL,
    task_name TEXT NOT NULL,
    task_id TEXT NOT NULL,
    store_id TEXT NOT NULL,
    state TEXT NOT NULL CHECK(state IN ('running','retry','completed')),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    PRIMARY KEY(scope,task_name,task_id)
);
CREATE TABLE convoy.queue_store_fence (
    singleton BOOLEAN PRIMARY KEY DEFAULT TRUE CHECK (singleton),
    scope TEXT NOT NULL,
    store_id TEXT NOT NULL,
    executor_role TEXT NOT NULL,
    operation_id TEXT,
    epoch BIGINT NOT NULL DEFAULT 0,
    closed BOOLEAN NOT NULL DEFAULT FALSE
);
-- +migrate StatementBegin
CREATE FUNCTION convoy.enforce_queue_store_fence() RETURNS trigger
LANGUAGE plpgsql SECURITY DEFINER SET search_path = pg_catalog, convoy AS $$
DECLARE
    gate_closed BOOLEAN;
    allowed_role TEXT;
BEGIN
    SELECT closed, executor_role INTO gate_closed, allowed_role
      FROM convoy.queue_store_fence WHERE singleton FOR SHARE;
    IF gate_closed AND session_user <> allowed_role THEN
        RAISE EXCEPTION 'queue store is fenced' USING ERRCODE = '55000';
    END IF;
    RETURN NULL;
END;
$$;
-- +migrate StatementEnd
CREATE TRIGGER queue_jobs_store_fence
BEFORE INSERT OR UPDATE OR DELETE OR TRUNCATE ON convoy.queue_jobs
FOR EACH STATEMENT EXECUTE FUNCTION convoy.enforce_queue_store_fence();
CREATE TRIGGER queue_state_store_fence
BEFORE INSERT OR UPDATE OR DELETE OR TRUNCATE ON convoy.queue_state
FOR EACH STATEMENT EXECUTE FUNCTION convoy.enforce_queue_store_fence();
ALTER TABLE convoy.queue_jobs ENABLE ALWAYS TRIGGER queue_jobs_store_fence;
ALTER TABLE convoy.queue_state ENABLE ALWAYS TRIGGER queue_state_store_fence;

RESET lock_timeout;
RESET statement_timeout;
