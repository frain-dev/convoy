-- +migrate Up
SET lock_timeout = '2s';
SET statement_timeout = '30s';

ALTER TABLE convoy.filters
    ADD COLUMN IF NOT EXISTS enabled_at TIMESTAMP WITH TIME ZONE DEFAULT NOW();

RESET lock_timeout;
RESET statement_timeout;

-- +migrate Down
ALTER TABLE convoy.filters
    DROP COLUMN IF EXISTS enabled_at;
