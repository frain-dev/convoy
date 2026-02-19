-- +migrate Up
SET lock_timeout = '2s';
SET statement_timeout = '30s';
ALTER TABLE convoy.endpoints
    ADD COLUMN IF NOT EXISTS new_http_timeout int,
    ADD COLUMN IF NOT EXISTS new_rate_limit_duration int;

-- +migrate Up
SET lock_timeout = '2s';
SET statement_timeout = '30s';
UPDATE convoy.endpoints
    SET new_http_timeout = convoy.duration_to_seconds(http_timeout::interval),
        new_rate_limit_duration = convoy.duration_to_seconds(rate_limit_duration::interval);

-- +migrate Up
SET lock_timeout = '2s';
SET statement_timeout = '30s';
ALTER TABLE convoy.endpoints
    ADD CONSTRAINT new_http_timeout_not_null CHECK (new_http_timeout IS NOT NULL) NOT VALID;

-- +migrate Up
ALTER TABLE convoy.endpoints VALIDATE CONSTRAINT new_http_timeout_not_null;

-- +migrate Up
SET lock_timeout = '2s';
SET statement_timeout = '30s';
-- squawk-ignore ban-drop-column
ALTER TABLE convoy.endpoints DROP COLUMN IF EXISTS http_timeout;

-- +migrate Up
SET lock_timeout = '2s';
SET statement_timeout = '30s';
ALTER TABLE convoy.endpoints
    ADD CONSTRAINT new_rate_limit_duration_not_null CHECK (new_rate_limit_duration IS NOT NULL) NOT VALID;

-- +migrate Up
ALTER TABLE convoy.endpoints VALIDATE CONSTRAINT new_rate_limit_duration_not_null;

-- +migrate Up
SET lock_timeout = '2s';
SET statement_timeout = '30s';
-- squawk-ignore ban-drop-column
ALTER TABLE convoy.endpoints DROP COLUMN IF EXISTS rate_limit_duration;

-- +migrate Up
SET lock_timeout = '2s';
SET statement_timeout = '30s';
-- squawk-ignore renaming-column
ALTER TABLE convoy.endpoints RENAME COLUMN new_http_timeout TO http_timeout;

-- +migrate Up
SET lock_timeout = '2s';
SET statement_timeout = '30s';
-- squawk-ignore renaming-column
ALTER TABLE convoy.endpoints RENAME COLUMN new_rate_limit_duration TO rate_limit_duration;

-- +migrate Down
ALTER TABLE convoy.endpoints
    ADD COLUMN IF NOT EXISTS old_http_timeout text,
    ADD COLUMN IF NOT EXISTS old_rate_limit_duration text;

-- +migrate Down
UPDATE convoy.endpoints
    SET old_http_timeout = convoy.seconds_to_interval(http_timeout),
        old_rate_limit_duration = convoy.seconds_to_interval(rate_limit_duration);

-- +migrate Down
ALTER TABLE convoy.endpoints
    ADD CONSTRAINT old_http_timeout_not_null CHECK (old_http_timeout IS NOT NULL) NOT VALID,
    ADD CONSTRAINT old_rate_limit_duration_not_null CHECK (old_rate_limit_duration IS NOT NULL) NOT VALID;

-- +migrate Down
ALTER TABLE convoy.endpoints
    VALIDATE CONSTRAINT old_http_timeout_not_null,
    VALIDATE CONSTRAINT old_rate_limit_duration_not_null;

-- +migrate Down
-- squawk-ignore ban-drop-column
ALTER TABLE convoy.endpoints DROP COLUMN IF EXISTS http_timeout, DROP COLUMN IF EXISTS rate_limit_duration;


-- +migrate Down
-- squawk-ignore renaming-column
ALTER TABLE convoy.endpoints RENAME COLUMN old_http_timeout TO http_timeout;

-- +migrate Down
-- squawk-ignore renaming-column
ALTER TABLE convoy.endpoints RENAME COLUMN old_rate_limit_duration TO rate_limit_duration;
