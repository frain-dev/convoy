-- +migrate Up
SET lock_timeout = '2min';
SET statement_timeout = '10min';
-- Update copy_rows function to copy new columns (acknowledged_at, status, metadata)
-- that were added to both events and events_search tables in earlier migrations
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
                                          created_at, updated_at, deleted_at, url_query_params, idempotency_key,
                                          is_duplicate_event, acknowledged_at, status, metadata)
        VALUES (row_data.id, row_data.event_type, row_data.endpoints, row_data.project_id, row_data.source_id,
                row_data.headers, row_data.raw, row_data.data, row_data.created_at, row_data.updated_at,
                row_data.deleted_at, row_data.url_query_params, row_data.idempotency_key, row_data.is_duplicate_event,
                row_data.acknowledged_at, row_data.status, row_data.metadata);
    END LOOP;
    CLOSE cs;
END;
$$ LANGUAGE plpgsql;
-- +migrate StatementEnd

RESET lock_timeout;
RESET statement_timeout;

-- +migrate Down
-- Revert copy_rows function to not include new columns
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
                                          created_at, updated_at, deleted_at, url_query_params, idempotency_key,
                                          is_duplicate_event)
        VALUES (row_data.id, row_data.event_type, row_data.endpoints, row_data.project_id, row_data.source_id,
                row_data.headers, row_data.raw, row_data.data, row_data.created_at, row_data.updated_at,
                row_data.deleted_at, row_data.url_query_params, row_data.idempotency_key, row_data.is_duplicate_event);
    END LOOP;
    CLOSE cs;
END;
$$ LANGUAGE plpgsql;
-- +migrate StatementEnd
