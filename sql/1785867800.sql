-- +migrate Up
SET lock_timeout = '2s';
SET statement_timeout = '30s';

-- Nullable with no default and no index, so Postgres records a catalog change
-- only. convoy.events is the hottest table in the schema and must not be
-- rewritten or held under an exclusive lock while events are being ingested.
ALTER TABLE convoy.events
ADD COLUMN IF NOT EXISTS failure_reason TEXT;

-- events_search mirrors events for the full-text list path. Columns added to one
-- and not the other have broken list queries before, so both move together.
ALTER TABLE convoy.events_search
ADD COLUMN IF NOT EXISTS failure_reason TEXT;

RESET lock_timeout;
RESET statement_timeout;

-- +migrate Up
SET lock_timeout = '2s';
SET statement_timeout = '30s';

-- +migrate StatementBegin
CREATE OR REPLACE FUNCTION convoy.copy_rows(pid VARCHAR, dur INTEGER) RETURNS VOID AS
$$
DECLARE
    cs CURSOR FOR
        SELECT * FROM convoy.events
        WHERE project_id = pid
        AND created_at >= NOW() - MAKE_INTERVAL(hours := dur);
    row_data RECORD;
BEGIN
    OPEN cs;
    LOOP
        FETCH cs INTO row_data;
        EXIT WHEN NOT FOUND;
        INSERT INTO convoy.events_search (id, event_type, endpoints, project_id, source_id, headers, raw, data,
                                          created_at, updated_at, deleted_at, url_query_params, url_path,
                                          idempotency_key, is_duplicate_event, acknowledged_at, status, metadata,
                                          failure_reason)
        VALUES (row_data.id, row_data.event_type, row_data.endpoints, row_data.project_id, row_data.source_id,
                row_data.headers, row_data.raw, row_data.data, row_data.created_at, row_data.updated_at,
                row_data.deleted_at, row_data.url_query_params, row_data.url_path, row_data.idempotency_key,
                row_data.is_duplicate_event, row_data.acknowledged_at, row_data.status, row_data.metadata,
                row_data.failure_reason);
    END LOOP;
    CLOSE cs;
END;
$$ LANGUAGE plpgsql;
-- +migrate StatementEnd

RESET lock_timeout;
RESET statement_timeout;

-- +migrate Down
SET lock_timeout = '2s';
SET statement_timeout = '30s';

-- +migrate StatementBegin
CREATE OR REPLACE FUNCTION convoy.copy_rows(pid VARCHAR, dur INTEGER) RETURNS VOID AS
$$
DECLARE
    cs CURSOR FOR
        SELECT * FROM convoy.events
        WHERE project_id = pid
        AND created_at >= NOW() - MAKE_INTERVAL(hours := dur);
    row_data RECORD;
BEGIN
    OPEN cs;
    LOOP
        FETCH cs INTO row_data;
        EXIT WHEN NOT FOUND;
        INSERT INTO convoy.events_search (id, event_type, endpoints, project_id, source_id, headers, raw, data,
                                          created_at, updated_at, deleted_at, url_query_params, url_path,
                                          idempotency_key, is_duplicate_event, acknowledged_at, status, metadata)
        VALUES (row_data.id, row_data.event_type, row_data.endpoints, row_data.project_id, row_data.source_id,
                row_data.headers, row_data.raw, row_data.data, row_data.created_at, row_data.updated_at,
                row_data.deleted_at, row_data.url_query_params, row_data.url_path, row_data.idempotency_key,
                row_data.is_duplicate_event, row_data.acknowledged_at, row_data.status, row_data.metadata);
    END LOOP;
    CLOSE cs;
END;
$$ LANGUAGE plpgsql;
-- +migrate StatementEnd

-- squawk-ignore ban-drop-column
ALTER TABLE convoy.events DROP COLUMN IF EXISTS failure_reason;

-- squawk-ignore ban-drop-column
ALTER TABLE convoy.events_search DROP COLUMN IF EXISTS failure_reason;

RESET lock_timeout;
RESET statement_timeout;
