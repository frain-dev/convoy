-- +migrate Up
alter table convoy.configurations add column if not exists cb_sample_rate int not null default 30;
alter table convoy.configurations add column if not exists cb_error_timeout int not null default 30;
alter table convoy.configurations add column if not exists cb_failure_threshold float not null default 0.1;
alter table convoy.configurations add column if not exists cb_failure_count int not null default 1;
alter table convoy.configurations add column if not exists cb_success_threshold int not null default 5;
alter table convoy.configurations add column if not exists cb_observability_window int not null default 5;
alter table convoy.configurations add column if not exists cb_notification_thresholds int[] not null default ARRAY[5, 10];
alter table convoy.configurations add column if not exists cb_consecutive_failure_threshold int not null default 5;

-- +migrate Down
alter table convoy.configurations drop column if exists cb_sample_rate;
alter table convoy.configurations drop column if exists cb_error_timeout;
alter table convoy.configurations drop column if exists cb_failure_threshold;
alter table convoy.configurations drop column if exists cb_failure_count;
alter table convoy.configurations drop column if exists cb_success_threshold;
alter table convoy.configurations drop column if exists cb_observability_window;
alter table convoy.configurations drop column if exists cb_notification_thresholds;
alter table convoy.configurations drop column if exists cb_consecutive_failure_threshold;
