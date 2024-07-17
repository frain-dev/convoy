-- +migrate Up
create table if not exists convoy.delivery_attempts (
    id                   varchar primary key,
    url                  text not null,
    method               varchar not null,
    api_version          varchar not null,
    endpoint_id          varchar not null references convoy.endpoints(id),
    event_delivery_id    varchar not null references convoy.event_deliveries(id),

    ip_address           varchar,
    request_http_header  jsonb,
    response_http_header jsonb,
    http_status          varchar,
    response_data        text,
    error                text,
    status               bool,

    created_at           timestamptz not null default now()
);

create index if not exists idx_delivery_attempts_event_delivery_id
    on convoy.delivery_attempts using brin (created_at, id, event_delivery_id);

-- +migrate Down
drop table if exists convoy.delivery_attempts;
drop index if exists convoy.idx_delivery_attempts_event_delivery_id;

