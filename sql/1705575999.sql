-- +migrate Up
ALTER TABLE convoy.endpoints
    ADD COLUMN IF NOT EXISTS new_http_timeout int,
    ADD COLUMN IF NOT EXISTS new_rate_limit_duration int;

-- +migrate Up
UPDATE convoy.endpoints
    SET new_http_timeout = duration_to_seconds(http_timeout::interval),
        new_rate_limit_duration = duration_to_seconds(rate_limit_duration::interval);

-- +migrate Up
ALTER TABLE convoy.endpoints
    ALTER COLUMN new_http_timeout SET NOT NULL,
    DROP COLUMN IF EXISTS http_timeout;

-- +migrate Up
ALTER TABLE convoy.endpoints
    ALTER COLUMN new_rate_limit_duration SET NOT NULL,
    DROP COLUMN IF EXISTS rate_limit_duration;

-- +migrate Up
ALTER TABLE convoy.endpoints
    RENAME COLUMN new_http_timeout TO http_timeout;

-- +migrate Up
ALTER TABLE convoy.endpoints
    RENAME COLUMN new_rate_limit_duration TO rate_limit_duration;

-- +migrate Down
ALTER TABLE convoy.endpoints
    ADD COLUMN IF NOT EXISTS old_http_timeout text,
    ADD COLUMN IF NOT EXISTS old_rate_limit_duration text;

-- +migrate Down
UPDATE convoy.endpoints
    SET old_http_timeout = convoy.seconds_to_interval(http_timeout),
    SET old_rate_limit_duration = convoy.seconds_to_interval(rate_limit_duration);

-- +migrate Down
ALTER TABLE convoy.endpoints
    ALTER COLUMN old_http_timeout SET NOT NULL,
    ALTER COLUMN old_rate_limit_duration SET NOT NULL,
    DROP COLUMN IF EXISTS http_timeout,
    DROP COLUMN IF EXISTS rate_limit_duration;


-- +migrate Down
ALTER TABLE convoy.endpoints
    RENAME COLUMN old_http_timeout TO http_timeout;

-- +migrate Down
ALTER TABLE convoy.endpoints
    RENAME COLUMN old_rate_limit_duration TO rate_limit_duration;
