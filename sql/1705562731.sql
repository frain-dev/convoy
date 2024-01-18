-- +migrate Up
-- +migrate StatementBegin
CREATE OR REPLACE FUNCTION convoy.duration_to_seconds(duration_text interval) RETURNS INTEGER AS $$
DECLARE
    duration_interval INTERVAL;
BEGIN
    duration_interval := EXTRACT(EPOCH FROM duration_text);

    RETURN EXTRACT(EPOCH FROM duration_interval)::INTEGER;
END;
$$ LANGUAGE plpgsql;
-- +migrate StatementEnd

-- +migrate Up
-- +migrate StatementBegin
CREATE OR REPLACE FUNCTION convoy.seconds_to_interval(sec INT) RETURNS TEXT AS $$
DECLARE
    hours INT;
    minutes INT;
    seconds INT;
    result TEXT := '';
BEGIN
    -- Calculate hours, minutes, and remaining seconds
    hours := sec / 3600;
    minutes := (sec / 60) % 60;
    seconds := sec % 60;

    -- Build the result string
    IF hours > 0 THEN
            result := result || hours || 'h';
    END IF;
    IF minutes > 0 OR (hours > 0 AND seconds > 0) THEN
            result := result || minutes || 'm';
    END IF;
    IF seconds > 0 THEN
            result := result || seconds || 's';
    END IF;

    RETURN result;
END;
$$ LANGUAGE plpgsql;
-- +migrate StatementEnd
