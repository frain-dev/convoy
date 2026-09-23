-- +migrate Up notransaction
CREATE INDEX CONCURRENTLY queue_execution_unsettled ON convoy.queue_execution_ownership (scope, store_id)
    WHERE state = 'running';

-- +migrate Down notransaction
DROP INDEX CONCURRENTLY convoy.queue_execution_unsettled;
