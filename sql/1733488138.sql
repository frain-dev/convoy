-- +migrate Up
create unique index if not exists idx_event_types_name_project_id
    on convoy.event_types (project_id, name);

-- +migrate Down
drop index if exists convoy.idx_event_types_name_project_id;
