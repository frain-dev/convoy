-- +migrate Up
SET lock_timeout = '2s';
SET statement_timeout = '30s';

ALTER TABLE convoy.delivery_attempts
    ADD COLUMN IF NOT EXISTS requested_at TIMESTAMP WITH TIME ZONE,
    ADD COLUMN IF NOT EXISTS responded_at TIMESTAMP WITH TIME ZONE;

RESET lock_timeout;
RESET statement_timeout;

-- +migrate Down
-- squawk-ignore ban-drop-column
ALTER TABLE convoy.delivery_attempts
    DROP COLUMN IF EXISTS requested_at,
    DROP COLUMN IF EXISTS responded_at;
