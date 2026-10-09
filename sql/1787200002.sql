-- +migrate Up
SET lock_timeout = '2s';
SET statement_timeout = '30s';
-- CREATE OR REPLACE on already-migrated instances. plpgsql EXCEPTION
-- handlers are subtransactions; PARALLEL SAFE made parallel seq scans
-- fail with SQLSTATE 25000. Fail closed to serial execution so invalid
-- rows still become NULL instead of aborting the query.
-- +migrate StatementBegin
CREATE OR REPLACE FUNCTION convoy.event_payload_jsonb(data bytea)
RETURNS jsonb
LANGUAGE plpgsql
IMMUTABLE
PARALLEL UNSAFE
AS $$
BEGIN
    IF data IS NULL THEN
        RETURN NULL;
    END IF;
    RETURN convert_from(data, 'UTF8')::jsonb;
EXCEPTION
    WHEN OTHERS THEN
        RETURN NULL;
END;
$$;
-- +migrate StatementEnd

RESET lock_timeout;
RESET statement_timeout;

-- +migrate Down
SET lock_timeout = '2s';
SET statement_timeout = '30s';
-- Restore the previous marker. Do not drop the function; 1787200000 owns it.
-- +migrate StatementBegin
CREATE OR REPLACE FUNCTION convoy.event_payload_jsonb(data bytea)
RETURNS jsonb
LANGUAGE plpgsql
IMMUTABLE
PARALLEL SAFE
AS $$
BEGIN
    IF data IS NULL THEN
        RETURN NULL;
    END IF;
    RETURN convert_from(data, 'UTF8')::jsonb;
EXCEPTION
    WHEN OTHERS THEN
        RETURN NULL;
END;
$$;
-- +migrate StatementEnd

RESET lock_timeout;
RESET statement_timeout;
