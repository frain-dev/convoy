-- +migrate Up
ALTER table if exists convoy.configurations add column license text default null;

-- +migrate Down
alter table if exists convoy.configurations drop column license;
