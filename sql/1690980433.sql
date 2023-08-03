-- +migrate Up
CREATE TABLE IF NOT EXISTS convoy.events_search
(
    id                 VARCHAR NOT NULL
        PRIMARY KEY,
    event_type         TEXT    NOT NULL,
    endpoints          TEXT,
    project_id         VARCHAR NOT NULL
        REFERENCES convoy.projects (id),
    source_id          VARCHAR
        REFERENCES convoy.sources (id),
    headers            JSONB,
    raw                TEXT    NOT NULL,
    data               BYTEA   NOT NULL,
    created_at         TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at         TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    deleted_at         TIMESTAMP WITH TIME ZONE,
    url_query_params   VARCHAR,
    idempotency_key    TEXT,
    is_duplicate_event BOOLEAN DEFAULT FALSE,
    search_token       TSVECTOR GENERATED ALWAYS AS (to_tsvector('english', raw)) STORED
);

-- +migrate Up
-- +migrate StatementBegin
CREATE OR REPLACE FUNCTION copy_rows() RETURNS VOID AS
$$
DECLARE
    cs CURSOR FOR
        SELECT *, to_tsvector('english', raw) AS search_token
        FROM convoy.events;
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

-- +migrate Up
SELECT copy_rows();

-- +migrate Up
CREATE INDEX IF NOT EXISTS idx_events_search_token_key
    ON convoy.events_search USING GIN (search_token);

-- +migrate Up
CREATE INDEX IF NOT EXISTS idx_events_search_created_at_key
    ON convoy.events_search (created_at);

-- +migrate Up
CREATE INDEX IF NOT EXISTS idx_events_search_deleted_at_key
    ON convoy.events_search (deleted_at);

-- +migrate Up
CREATE INDEX IF NOT EXISTS idx_events_search_project_id_key
    ON convoy.events_search (project_id);

-- +migrate Up
CREATE INDEX IF NOT EXISTS idx_events_search_project_id_deleted_at_key
    ON convoy.events_search (project_id, deleted_at);

-- +migrate Up
CREATE INDEX IF NOT EXISTS idx_events_search_source_id_key
    ON convoy.events_search (source_id);

-- +migrate Up
CREATE INDEX IF NOT EXISTS idx_events_search_idempotency_key_key
    ON convoy.events_search (idempotency_key);

-- +migrate Up
ALTER TABLE IF EXISTS convoy.events
    RENAME TO "events_copy";

-- +migrate Up
ALTER TABLE IF EXISTS convoy.events_search
    RENAME TO "events";

-- +migrate Up
DROP TABLE IF EXISTS convoy.events_search;

-- +migrate Up
ALTER TABLE convoy.events_endpoints
    DROP CONSTRAINT events_endpoints_event_id_fkey;

-- +migrate Up
ALTER TABLE convoy.events_endpoints
    ADD FOREIGN KEY (event_id) REFERENCES convoy.events
        ON DELETE CASCADE;

-- +migrate Up
ALTER TABLE convoy.event_deliveries
    DROP CONSTRAINT event_deliveries_event_id_fkey;

-- +migrate Up
ALTER TABLE convoy.event_deliveries
    ADD FOREIGN KEY (event_id) REFERENCES convoy.events;

-- +migrate Down
ALTER TABLE IF EXISTS convoy.events
    DROP COLUMN search_token;

-- +migrate Down
DROP INDEX IF EXISTS idx_events_search_token_key;
