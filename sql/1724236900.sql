-- +migrate Up
alter table convoy.configurations add column if not exists cb_sample_rate int not null default 30; -- seconds
alter table convoy.configurations add column if not exists cb_error_timeout int not null default 30; -- seconds
alter table convoy.configurations add column if not exists cb_failure_threshold int not null default 70; -- percentage
alter table convoy.configurations add column if not exists cb_success_threshold int not null default 1; -- percentage
alter table convoy.configurations add column if not exists cb_observability_window int not null default 30; -- minutes
alter table convoy.configurations add column if not exists cb_minimum_request_count int not null default 10;
alter table convoy.configurations add column if not exists cb_notification_thresholds int[] not null default ARRAY[10, 30, 50];
alter table convoy.configurations add column if not exists cb_consecutive_failure_threshold int not null default 10;
create index if not exists idx_delivery_attempts_created_at on convoy.delivery_attempts (created_at);
create index if not exists idx_delivery_attempts_event_delivery_id_created_at on convoy.delivery_attempts (event_delivery_id, created_at);
create index if not exists idx_delivery_attempts_event_delivery_id on convoy.delivery_attempts (event_delivery_id);

-- +migrate Down
alter table convoy.configurations drop column if exists cb_sample_rate;
alter table convoy.configurations drop column if exists cb_error_timeout;
alter table convoy.configurations drop column if exists cb_failure_threshold;
alter table convoy.configurations drop column if exists cb_success_threshold;
alter table convoy.configurations drop column if exists cb_observability_window;
alter table convoy.configurations drop column if exists cb_notification_thresholds;
alter table convoy.configurations drop column if exists cb_consecutive_failure_threshold;
drop index if exists convoy.idx_delivery_attempts_created_at;
drop index if exists convoy.idx_delivery_attempts_event_delivery_id_created_at;
drop index if exists convoy.idx_delivery_attempts_event_delivery_id;

