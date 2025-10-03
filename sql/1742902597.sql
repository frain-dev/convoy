-- +migrate Up
alter table convoy.event_types add column if not exists json_schema jsonb default '{}';

-- +migrate Down
alter table convoy.event_types drop column if exists json_schema;
