-- +migrate Up notransaction
SET lock_timeout = '2s';
SET statement_timeout = '30s';

-- Keep the schema intact when it contains maintenance state.
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

DROP INDEX CONCURRENTLY IF EXISTS convoy.queue_operation_unsettled;
DROP INDEX CONCURRENTLY IF EXISTS convoy.queue_execution_unsettled;
RESET lock_timeout;
RESET statement_timeout;

-- +migrate Down notransaction
SET lock_timeout = '2s';
SET statement_timeout = '30s';
CREATE INDEX CONCURRENTLY IF NOT EXISTS queue_operation_unsettled ON convoy.queue_operation_receipts (scope, store_id)
    WHERE settled_at IS NULL;
CREATE INDEX CONCURRENTLY IF NOT EXISTS queue_execution_unsettled ON convoy.queue_execution_ownership (scope, store_id)
    WHERE state = 'running';
RESET lock_timeout;
RESET statement_timeout;
