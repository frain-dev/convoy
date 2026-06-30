-- +migrate Up notransaction
-- Composite index for the period (history) failure rate: counts of Success/Failure
-- deliveries per endpoint over a time range. Matches the WHERE/GROUP BY of
-- CountDeliveriesByEndpointAndStatus. CONCURRENTLY avoids locking writes.
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_event_deliveries_endpoint_status_created
    ON convoy.event_deliveries (project_id, endpoint_id, status, created_at)
    WHERE deleted_at IS NULL;

-- +migrate Down notransaction
DROP INDEX CONCURRENTLY IF EXISTS convoy.idx_event_deliveries_endpoint_status_created;
