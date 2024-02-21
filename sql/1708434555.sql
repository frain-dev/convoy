-- +migrate Up
create unlogged table if not exists convoy.token_bucket (
   key text not null primary key,
   rate integer not null, -- 10 per min, 20 per min
   tokens integer default 1,
   created_at timestamptz default now(),
   updated_at timestamptz default now(),
   expires_at timestamptz not null
);

-- take_token should be run in a transaction to acquire a row lock, returns true is a token was taken
-- +migrate StatementBegin
create function take_token(url text, per_min integer) returns boolean
    language plpgsql
as
$$
declare
    row record;
    next_min timestamptz;
    tempRow record;
begin
    select * from convoy.token_bucket where key = url FOR UPDATE into row;

    -- the bucket doesn't exist yet
    if row is null then
        next_min := now() + make_interval(mins := 1);
        insert into convoy.token_bucket (key, rate, expires_at)
        SELECT url, per_min, next_min
        WHERE NOT EXISTS (
            SELECT 1 FROM convoy.token_bucket WHERE key = url
        );

        return true;
    end if;

    -- this bucket has expired, reset it
    if now() > row.expires_at then
        next_min := now() + make_interval(mins := 1);

        SELECT 1 FROM convoy.token_bucket WHERE key = url FOR UPDATE into tempRow;
        UPDATE convoy.token_bucket
        SET tokens = 1, expires_at = next_min, updated_at = now() WHERE key = url;
        return true;
    end if;

    -- take a token
    if row.tokens < row.rate then
        next_min := now() + make_interval(mins := 1);
        update convoy.token_bucket set tokens = row.tokens + 1, expires_at = next_min, updated_at = now() where key = url;
        return true;
    end if;

    -- no tokens for you sorry
    return false;
end;
$$;
-- +migrate StatementEnd

-- +migrate Down
drop function if exists convoy.take_token(url TEXT, per_min INTEGER);

-- +migrate Down
drop table if exists convoy.token_bucket;


