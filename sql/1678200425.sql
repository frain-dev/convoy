-- +migrate Up
-- +migrate StatementBegin
CREATE MATERIALIZED VIEW IF NOT EXISTS convoy.event_metadata AS
SELECT
    ev.id,
    ev.project_id,
    ev.event_type,
    COALESCE(ev.source_id, '') AS source_id,
    ev.headers,
    ev.raw,
    ev.data,
    ev.created_at,
    ev.updated_at,
    ev.deleted_at,
    ARRAY_AGG(e.id) AS endpoints,
    array_to_json(ARRAY_AGG(json_build_object(
        'uid', e.id,
        'title', e.title,
        'project_id', e.project_id,
        'target_url', e.target_url
    ))) AS endpoint_metadata,
    COALESCE(s.id, '') AS "source_metadata.id",
    COALESCE(s.name, '') AS "source_metadata.name"
FROM
    convoy.events AS ev
    LEFT JOIN convoy.events_endpoints ee ON ee.event_id = ev.id
    LEFT JOIN convoy.endpoints e ON e.id = ee.endpoint_id
    LEFT JOIN convoy.sources s ON s.id = ev.source_id
WHERE
    ev.deleted_at IS NULL
GROUP BY ev.id, s.id;
-- +migrate StatementEnd

-- +migrate Up
CREATE INDEX IF NOT EXISTS idx_event_metadata_project_id ON convoy.event_metadata (project_id);
CREATE UNIQUE INDEX IF NOT EXISTS event_metadata_id_unique_index ON convoy.event_metadata (id);
CREATE INDEX IF NOT EXISTS idx_event_metadata_endpoints ON convoy.event_metadata USING GIN ("endpoints");
CREATE INDEX IF NOT EXISTS idx_event_metadata_created_at ON convoy.event_metadata (created_at);

-- +migrate Down
DROP MATERIALIZED VIEW IF EXISTS convoy.event_metadata;
DROP INDEX IF EXISTS convoy.idx_event_metadata_project_id, convoy.event_metadata_id_unique_index, convoy.idx_event_metadata_endpoints, convoy.idx_event_metadata_created_at;

