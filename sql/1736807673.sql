-- +migrate Up
alter table convoy.delivery_attempts
    alter column response_data type bytea
        using response_data::bytea;

-- +migrate Down
alter table convoy.delivery_attempts
    alter column response_data type text
        using encode(response_data, 'escape');
