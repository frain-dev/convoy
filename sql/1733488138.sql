-- +migrate Up notransaction
SET lock_timeout = '2s';
SET statement_timeout = '30s';
create unique index CONCURRENTLY if not exists idx_event_types_name_project_id
    on convoy.event_types (project_id, name);

-- +migrate Down
drop index concurrently if exists convoy.idx_event_types_name_project_id;
