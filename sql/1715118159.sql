-- +migrate Up
-- +migrate StatementBegin
create or replace function convoy.take_token(_key text, _rate integer, _bucket_size integer) returns boolean
    language plpgsql
as
$$
DECLARE
    next_min timestamptz;
    _can_take BOOLEAN;
    row RECORD;
BEGIN
    SELECT * FROM convoy.token_bucket WHERE key = _key FOR UPDATE SKIP LOCKED LIMIT 1 INTO row;
    next_min := current_timestamp + make_interval(secs := _bucket_size);

    IF current_timestamp < row.expires_at AND row.tokens = _rate THEN
        RETURN FALSE;
    END IF;

    -- Update existing record
    UPDATE convoy.token_bucket
    SET tokens =
            CASE WHEN current_timestamp > expires_at
                     THEN 1
                 ELSE CASE WHEN tokens < _rate
                               THEN tokens + 1
                           ELSE tokens END
                END,
        expires_at =
            CASE WHEN current_timestamp > expires_at
                     THEN next_min
                 ELSE CASE WHEN tokens < _rate
                               THEN next_min
                           ELSE expires_at
                     END
                END,
        rate = COALESCE(_rate, rate),
        updated_at = DEFAULT
    WHERE key = _key
    RETURNING TRUE INTO _can_take;

    -- Insert if no record found
    IF NOT FOUND THEN
        INSERT INTO convoy.token_bucket (key, rate, expires_at)
        VALUES (_key, _rate, next_min);
        _can_take = true;
    END IF;

    RETURN _can_take;
END;
$$;
-- +migrate StatementEnd

-- +migrate Down
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

