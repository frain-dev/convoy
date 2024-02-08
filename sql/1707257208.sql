-- +migrate Up
ALTER TABLE convoy.project_configurations
    DROP COLUMN IF EXISTS disable_endpoint,
    ADD COLUMN IF NOT EXISTS circuit_breaker_duration int NOT NULL DEFAULT 600,
    ADD COLUMN IF NOT EXISTS circuit_breaker_error_threshold int NOT NULL DEFAULT 20;

ALTER TABLE convoy.endpoints
    ADD COLUMN IF NOT EXISTS disable_endpoint BOOLEAN NOT NULL DEFAULT false,
    ADD COLUMN IF NOT EXISTS circuit_breaker_duration int NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS circuit_breaker_error_threshold int NOT NULL DEFAULT 0;

-- +migrate Down
ALTER TABLE convoy.project_configurations
    ADD COLUMN IF NOT EXISTS disable_endpoint BOOLEAN NOT NULL DEFAULT false,
    DROP COLUMN IF EXISTS circuit_breaker_duration,
    DROP COLUMN IF EXISTS circuit_breaker_error_threshold;

ALTER TABLE convoy.endpoints
    DROP COLUMN IF EXISTS disable_endpoint,
    DROP COLUMN IF EXISTS circuit_breaker_duration,
    DROP COLUMN IF EXISTS circuit_breaker_error_threshold;
