-- +migrate Up
alter table convoy.event_types add column json_schema jsonb default '{}';

-- +migrate Down
alter table convoy.event_types drop column json_schema;

-- /event-type/import/openapi