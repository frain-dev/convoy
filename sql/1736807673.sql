-- +migrate Up
SET lock_timeout = '2s';
SET statement_timeout = '30s';
update convoy.delivery_attempts set response_data = '' where id > '';
alter table convoy.delivery_attempts
    alter column response_data type bytea
        using response_data::bytea;

-- +migrate Down
alter table convoy.delivery_attempts
    alter column response_data type text
        using encode(response_data, 'escape');
