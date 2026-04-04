-- +migrate Up
-- +migrate StatementBegin
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_publication WHERE pubname = 'convoy_backup') THEN
        EXECUTE 'CREATE PUBLICATION convoy_backup FOR TABLE convoy.events, convoy.event_deliveries, convoy.delivery_attempts';
    END IF;
END;
$$;
-- +migrate StatementEnd

-- +migrate Down
DROP PUBLICATION IF EXISTS convoy_backup;
