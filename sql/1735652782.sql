-- +migrate Up
alter table convoy.subscriptions add column filter_config_filter_raw_headers jsonb not null default '{}';
alter table convoy.subscriptions add column filter_config_filter_raw_body jsonb not null default '{}';

update convoy.subscriptions
set
    filter_config_filter_raw_headers = filter_config_filter_headers,
    filter_config_filter_raw_body = filter_config_filter_body
where
    id > '';

-- +migrate Down
alter table convoy.subscriptions drop column filter_config_filter_raw_headers;
alter table convoy.subscriptions drop column filter_config_filter_raw_body;

