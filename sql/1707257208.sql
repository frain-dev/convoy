-- +migrate Up
ALTER TABLE convoy.project_configurations
    ADD COLUMN IF NOT EXISTS circuit_breaker_duration int NOT NULL DEFAULT 600,
    ADD COLUMN IF NOT EXISTS circuit_breaker_error_threshold int NOT NULL DEFAULT 20;

ALTER TABLE convoy.endpoints
    ADD COLUMN IF NOT EXISTS circuit_breaker_duration int NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS circuit_breaker_error_threshold int NOT NULL DEFAULT 0;

-- +migrate Down
ALTER TABLE convoy.project_configurations
    DROP COLUMN IF EXISTS circuit_breaker_duration,
    DROP COLUMN IF EXISTS circuit_breaker_error_threshold;

ALTER TABLE convoy.endpoints
    DROP COLUMN IF EXISTS circuit_breaker_duration,
    DROP COLUMN IF EXISTS circuit_breaker_error_threshold;
