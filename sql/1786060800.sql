-- +migrate Up
SET lock_timeout = '2s';
SET statement_timeout = '30s';

-- Default FALSE preserves the existing strict behaviour: once a project has at
-- least one endpoint URL template, a dynamic URL matching none of them is
-- rejected. Projects that also receive non-templated partner URLs opt in.
ALTER TABLE convoy.project_configurations
ADD COLUMN IF NOT EXISTS allow_unmatched_dynamic_urls BOOLEAN NOT NULL DEFAULT FALSE;

RESET lock_timeout;
RESET statement_timeout;

-- +migrate Down
SET lock_timeout = '2s';
SET statement_timeout = '30s';

-- squawk-ignore ban-drop-column
ALTER TABLE convoy.project_configurations DROP COLUMN IF EXISTS allow_unmatched_dynamic_urls;

RESET lock_timeout;
RESET statement_timeout;
