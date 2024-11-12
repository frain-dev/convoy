-- +migrate Up
create table if not exists convoy.event_types (
  id            varchar primary key,
  name          varchar not null,
  description   text,
  project_id    varchar not null references convoy.projects(id),
  category      varchar,
  created_at    timestamptz not null default now(),
  updated_at    timestamptz not null default now(),
  deprecated_at timestamptz
);

create index if not exists idx_event_types_name
    on convoy.event_types(name);

create index if not exists idx_event_types_category
    on convoy.event_types(category);

create index if not exists idx_event_types_name_not_deprecated
    on convoy.event_types(name) where deprecated_at is null;

create index if not exists idx_event_types_name_deprecated
    on convoy.event_types(name) where deprecated_at is not null;

create index if not exists idx_event_types_category_not_deprecated
    on convoy.event_types(category) where deprecated_at is null;

create index if not exists idx_event_types_category_deprecated
    on convoy.event_types(category) where deprecated_at is not null;
  
-- +migrate Down
drop index if exists convoy.idx_event_types_category;
drop index if exists convoy.idx_event_types_name;
drop index if exists convoy.idx_event_types_category_deprecated;
drop index if exists convoy.idx_event_types_category_not_deprecated;
drop index if exists convoy.idx_event_types_name_deprecated;
drop index if exists convoy.idx_event_types_name_not_deprecated;
drop table if exists convoy.event_types;


