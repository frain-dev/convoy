-- +migrate Up

ALTER TABLE convoy.project_configurations
    ADD COLUMN cb_sample_rate INTEGER DEFAULT 30,
    ADD COLUMN cb_error_timeout INTEGER DEFAULT 30,
    ADD COLUMN cb_failure_threshold INTEGER DEFAULT 70,
    ADD COLUMN cb_success_threshold INTEGER DEFAULT 5,
    ADD COLUMN cb_observability_window INTEGER DEFAULT 5,
    ADD COLUMN cb_minimum_request_count INTEGER DEFAULT 10,
    ADD COLUMN cb_consecutive_failure_threshold INTEGER DEFAULT 10;

UPDATE convoy.project_configurations
SET
    cb_sample_rate = COALESCE(c.cb_sample_rate, 30),
    cb_error_timeout = COALESCE(c.cb_error_timeout, 30),
    cb_failure_threshold = COALESCE(c.cb_failure_threshold, 70),
    cb_success_threshold = COALESCE(c.cb_success_threshold, 5),
    cb_observability_window = COALESCE(c.cb_observability_window, 5),
    cb_minimum_request_count = COALESCE(c.cb_minimum_request_count, 10),
    cb_consecutive_failure_threshold = COALESCE(c.cb_consecutive_failure_threshold, 10)
FROM convoy.configurations c;

-- +migrate Down

ALTER TABLE convoy.project_configurations
    DROP COLUMN cb_sample_rate,
    DROP COLUMN cb_error_timeout,
    DROP COLUMN cb_failure_threshold,
    DROP COLUMN cb_success_threshold,
    DROP COLUMN cb_observability_window,
    DROP COLUMN cb_minimum_request_count,
    DROP COLUMN cb_consecutive_failure_threshold;
