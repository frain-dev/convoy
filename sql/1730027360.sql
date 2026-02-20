-- +migrate Up
SET lock_timeout = '2s';
SET statement_timeout = '30s';
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

-- +migrate Up notransaction
SET lock_timeout = '2s';
SET statement_timeout = '30s';
create index CONCURRENTLY if not exists idx_event_types_name
    on convoy.event_types(name);

create index CONCURRENTLY if not exists idx_event_types_category
    on convoy.event_types(category);

create index CONCURRENTLY if not exists idx_event_types_name_not_deprecated
    on convoy.event_types(name) where deprecated_at is null;

create index CONCURRENTLY if not exists idx_event_types_name_deprecated
    on convoy.event_types(name) where deprecated_at is not null;

create index CONCURRENTLY if not exists idx_event_types_category_not_deprecated
    on convoy.event_types(category) where deprecated_at is null;

create index CONCURRENTLY if not exists idx_event_types_category_deprecated
    on convoy.event_types(category) where deprecated_at is not null;

-- +migrate Down notransaction
drop index concurrently if exists convoy.idx_event_types_category;
drop index concurrently if exists convoy.idx_event_types_name;
drop index concurrently if exists convoy.idx_event_types_category_deprecated;
drop index concurrently if exists convoy.idx_event_types_category_not_deprecated;
drop index concurrently if exists convoy.idx_event_types_name_deprecated;
drop index concurrently if exists convoy.idx_event_types_name_not_deprecated;

-- +migrate Down
drop table if exists convoy.event_types;


