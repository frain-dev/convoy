-- +migrate Up
SET lock_timeout = '2s';
SET statement_timeout = '30s';
alter table convoy.configurations add column if not exists cb_sample_rate int not null default 30; -- seconds
alter table convoy.configurations add column if not exists cb_error_timeout int not null default 30; -- seconds
alter table convoy.configurations add column if not exists cb_failure_threshold int not null default 70; -- percentage
alter table convoy.configurations add column if not exists cb_success_threshold int not null default 1; -- percentage
alter table convoy.configurations add column if not exists cb_observability_window int not null default 30; -- minutes
alter table convoy.configurations add column if not exists cb_minimum_request_count int not null default 10;
alter table convoy.configurations add column if not exists cb_consecutive_failure_threshold int not null default 10;

-- +migrate Up notransaction
SET lock_timeout = '2s';
SET statement_timeout = '30s';
create index CONCURRENTLY if not exists idx_delivery_attempts_created_at on convoy.delivery_attempts (created_at);
create index CONCURRENTLY if not exists idx_delivery_attempts_event_delivery_id_created_at on convoy.delivery_attempts (event_delivery_id, created_at);
create index CONCURRENTLY if not exists idx_delivery_attempts_event_delivery_id on convoy.delivery_attempts (event_delivery_id);

-- +migrate Down
-- squawk-ignore ban-drop-column
alter table convoy.configurations drop column if exists cb_sample_rate;
-- squawk-ignore ban-drop-column
alter table convoy.configurations drop column if exists cb_error_timeout;
-- squawk-ignore ban-drop-column
alter table convoy.configurations drop column if exists cb_failure_threshold;
-- squawk-ignore ban-drop-column
alter table convoy.configurations drop column if exists cb_success_threshold;
-- squawk-ignore ban-drop-column
alter table convoy.configurations drop column if exists cb_observability_window;
-- squawk-ignore ban-drop-column
alter table convoy.configurations drop column if exists cb_consecutive_failure_threshold;

-- +migrate Down notransaction
drop index concurrently if exists convoy.idx_delivery_attempts_created_at;
drop index concurrently if exists convoy.idx_delivery_attempts_event_delivery_id_created_at;
