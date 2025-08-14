-- +migrate Up
ALTER TABLE convoy.project_configurations ADD COLUMN IF NOT EXISTS cb_sample_rate INTEGER NOT NULL DEFAULT 30;
ALTER TABLE convoy.project_configurations ADD COLUMN IF NOT EXISTS cb_error_timeout INTEGER NOT NULL DEFAULT 30;
ALTER TABLE convoy.project_configurations ADD COLUMN IF NOT EXISTS cb_failure_threshold INTEGER NOT NULL DEFAULT 70;
ALTER TABLE convoy.project_configurations ADD COLUMN IF NOT EXISTS cb_success_threshold INTEGER NOT NULL DEFAULT 5;
ALTER TABLE convoy.project_configurations ADD COLUMN IF NOT EXISTS cb_observability_window INTEGER NOT NULL DEFAULT 5;
ALTER TABLE convoy.project_configurations ADD COLUMN IF NOT EXISTS cb_minimum_request_count INTEGER NOT NULL DEFAULT 10;
ALTER TABLE convoy.project_configurations ADD COLUMN IF NOT EXISTS cb_consecutive_failure_threshold INTEGER NOT NULL DEFAULT 10;

UPDATE convoy.project_configurations
SET
    cb_sample_rate = (SELECT cb_sample_rate FROM convoy.configurations LIMIT 1),
    cb_error_timeout = (SELECT cb_error_timeout FROM convoy.configurations LIMIT 1),
    cb_failure_threshold = (SELECT cb_failure_threshold FROM convoy.configurations LIMIT 1),
    cb_success_threshold = (SELECT cb_success_threshold FROM convoy.configurations LIMIT 1),
    cb_observability_window = (SELECT cb_observability_window FROM convoy.configurations LIMIT 1),
    cb_minimum_request_count = (SELECT cb_minimum_request_count FROM convoy.configurations LIMIT 1),
    cb_consecutive_failure_threshold = (SELECT cb_consecutive_failure_threshold FROM convoy.configurations LIMIT 1)
WHERE EXISTS (SELECT 1 FROM convoy.configurations);

ALTER TABLE convoy.configurations DROP COLUMN IF EXISTS cb_sample_rate;
ALTER TABLE convoy.configurations DROP COLUMN IF EXISTS cb_error_timeout;
ALTER TABLE convoy.configurations DROP COLUMN IF EXISTS cb_failure_threshold;
ALTER TABLE convoy.configurations DROP COLUMN IF EXISTS cb_success_threshold;
ALTER TABLE convoy.configurations DROP COLUMN IF EXISTS cb_observability_window;
ALTER TABLE convoy.configurations DROP COLUMN IF EXISTS cb_minimum_request_count;
ALTER TABLE convoy.configurations DROP COLUMN IF EXISTS cb_consecutive_failure_threshold;

-- +migrate Down
ALTER TABLE convoy.configurations ADD COLUMN IF NOT EXISTS cb_sample_rate INTEGER NOT NULL DEFAULT 30;
ALTER TABLE convoy.configurations ADD COLUMN IF NOT EXISTS cb_error_timeout INTEGER NOT NULL DEFAULT 30;
ALTER TABLE convoy.configurations ADD COLUMN IF NOT EXISTS cb_failure_threshold INTEGER NOT NULL DEFAULT 70;
ALTER TABLE convoy.configurations ADD COLUMN IF NOT EXISTS cb_success_threshold INTEGER NOT NULL DEFAULT 5;
ALTER TABLE convoy.configurations ADD COLUMN IF NOT EXISTS cb_observability_window INTEGER NOT NULL DEFAULT 5;
ALTER TABLE convoy.configurations ADD COLUMN IF NOT EXISTS cb_minimum_request_count INTEGER NOT NULL DEFAULT 10;
ALTER TABLE convoy.configurations ADD COLUMN IF NOT EXISTS cb_consecutive_failure_threshold INTEGER NOT NULL DEFAULT 10;

UPDATE convoy.configurations
SET
    cb_sample_rate = (SELECT cb_sample_rate FROM convoy.project_configurations LIMIT 1),
    cb_error_timeout = (SELECT cb_error_timeout FROM convoy.project_configurations LIMIT 1),
    cb_failure_threshold = (SELECT cb_failure_threshold FROM convoy.project_configurations LIMIT 1),
    cb_success_threshold = (SELECT cb_success_threshold FROM convoy.project_configurations LIMIT 1),
    cb_observability_window = (SELECT cb_observability_window FROM convoy.project_configurations LIMIT 1),
    cb_minimum_request_count = (SELECT cb_minimum_request_count FROM convoy.project_configurations LIMIT 1),
    cb_consecutive_failure_threshold = (SELECT cb_consecutive_failure_threshold FROM convoy.project_configurations LIMIT 1)
WHERE EXISTS (SELECT 1 FROM convoy.project_configurations);

ALTER TABLE convoy.project_configurations DROP COLUMN IF EXISTS cb_sample_rate;
ALTER TABLE convoy.project_configurations DROP COLUMN IF EXISTS cb_error_timeout;
ALTER TABLE convoy.project_configurations DROP COLUMN IF EXISTS cb_failure_threshold;
ALTER TABLE convoy.project_configurations DROP COLUMN IF EXISTS cb_success_threshold;
ALTER TABLE convoy.project_configurations DROP COLUMN IF EXISTS cb_observability_window;
ALTER TABLE convoy.project_configurations DROP COLUMN IF EXISTS cb_minimum_request_count;
ALTER TABLE convoy.project_configurations DROP COLUMN IF EXISTS cb_consecutive_failure_threshold;
