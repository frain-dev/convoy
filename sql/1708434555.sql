-- +migrate Up
create unlogged table if not exists convoy.token_bucket (
    key text not null primary key,
    rate integer not null,
    tokens integer default 1,
    created_at timestamptz default now(),
    updated_at timestamptz default now(),
    expires_at timestamptz not null
);

-- take_token should be run in a transaction to acquire a row lock, returns true is a token was taken
-- +migrate StatementBegin
create or replace function convoy.take_token(_key text, _rate integer, _bucket_size integer) returns boolean
    language plpgsql
as
$$
declare
    row record;
    next_min timestamptz;
    new_rate int;
begin
    select * from convoy.token_bucket where key = _key for update into row;
    next_min := now() + make_interval(secs := _bucket_size);

    -- the bucket doesn't exist yet
    if row is null then
        insert into convoy.token_bucket (key, rate, expires_at)
        SELECT _key, _rate, next_min
        WHERE NOT EXISTS (
            SELECT 1 FROM convoy.token_bucket WHERE key = _key
        );

        return true;
    end if;

    -- update the rate if it's different from what's in the db
    new_rate = case when row.rate != _rate then _rate else row.rate end;

    -- this bucket has expired, reset it
    if now() > row.expires_at then
        UPDATE convoy.token_bucket
        SET tokens = 1,
            expires_at = next_min,
            updated_at = default,
            rate = new_rate
        WHERE key = _key;
        return true;
    end if;

    -- take a token
    if row.tokens < new_rate then
        update convoy.token_bucket
        set tokens = row.tokens + 1,
            expires_at = next_min,
            updated_at = default,
            rate = new_rate
        where key = _key;
        return true;
    end if;

    -- no tokens for you sorry
    return false;
end;
$$;
-- +migrate StatementEnd

-- +migrate Up
UPDATE convoy.endpoints
SET
    rate_limit = 0,
    rate_limit_duration = 0
WHERE
    rate_limit = 5000 AND
    rate_limit_duration = 60 AND
    deleted_at IS NULL;

-- +migrate Down
drop function if exists convoy.take_token(_key text, _rate integer, _bucket_size integer);

-- +migrate Down
drop table if exists convoy.token_bucket;

-- +migrate Down
UPDATE convoy.endpoints
SET
    rate_limit = 5000,
    rate_limit_duration = 60
WHERE
    rate_limit = 0 AND
    rate_limit_duration = 0 AND
    deleted_at IS NULL;



