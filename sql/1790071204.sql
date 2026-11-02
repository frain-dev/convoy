-- +migrate Up notransaction
CREATE INDEX CONCURRENTLY queue_operation_unsettled ON convoy.queue_operation_receipts (scope, store_id)
    WHERE settled_at IS NULL;

-- +migrate Down notransaction
DROP INDEX CONCURRENTLY convoy.queue_operation_unsettled;
