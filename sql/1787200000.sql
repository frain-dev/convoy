-- +migrate Up
SET lock_timeout = '2s';
SET statement_timeout = '30s';
-- Safe jsonb projection of events.data (BYTEA). Invalid UTF-8 or JSON
-- becomes NULL so containment misses the row instead of aborting the query.
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

-- +migrate Down
-- +migrate StatementBegin
DROP FUNCTION IF EXISTS convoy.event_payload_jsonb(bytea);
-- +migrate StatementEnd
