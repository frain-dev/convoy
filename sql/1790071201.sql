-- +migrate Up
SET lock_timeout = '2s';
SET statement_timeout = '30s';

-- There is one queue namespace in this database. The normal writer binds it
-- to a deployment before it can participate in administrative operations.
CREATE TABLE convoy.queue_store_fence (
    singleton BOOLEAN PRIMARY KEY DEFAULT TRUE CHECK (singleton),
    scope TEXT NOT NULL,
    store_id TEXT NOT NULL,
    executor_role TEXT NOT NULL,
    operation_id TEXT,
    epoch BIGINT NOT NULL DEFAULT 0,
    closed BOOLEAN NOT NULL DEFAULT FALSE
);

-- The row lock is retained until the queue mutation commits. Closing the
-- fence takes the conflicting lock, so a legacy transaction is either wholly
-- before closure or rejected. A cached application flag cannot provide this.
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

-- +migrate Down
SET lock_timeout = '2s';
SET statement_timeout = '30s';

DROP TRIGGER queue_jobs_store_fence ON convoy.queue_jobs;
DROP TRIGGER queue_state_store_fence ON convoy.queue_state;
DROP FUNCTION convoy.enforce_queue_store_fence();
DROP TABLE convoy.queue_store_fence;

RESET lock_timeout;
RESET statement_timeout;
