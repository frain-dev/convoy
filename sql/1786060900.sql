-- +migrate Up
SET lock_timeout = '2s';
SET statement_timeout = '30s';

-- Renames sync_dynamic_event_ack, shipped in v26.7.0, to match the dashboard
-- label the setting has always carried. The old column is left in place rather
-- than renamed so pods still running v26.7.0 keep reading a column that exists
-- during a rolling deploy; a follow-up release drops it once none remain.
--
-- New code dual-writes both columns, but a v26.7.0 pod handling an update mid-roll
-- writes only the old one, so the two can diverge for that project. Accepted:
-- toggling this setting during a rolling deploy is a narrow window and re-saving
-- heals it. The migration that drops the column must re-copy first, so a project
-- last updated by an old pod does not lose the value:
--   UPDATE convoy.project_configurations SET verify_dynamic_events = TRUE
--   WHERE sync_dynamic_event_ack IS TRUE AND verify_dynamic_events IS FALSE;
ALTER TABLE convoy.project_configurations
ADD COLUMN IF NOT EXISTS verify_dynamic_events BOOLEAN NOT NULL DEFAULT FALSE;

UPDATE convoy.project_configurations
SET verify_dynamic_events = TRUE
WHERE sync_dynamic_event_ack IS TRUE;

RESET lock_timeout;
RESET statement_timeout;

-- +migrate Down
SET lock_timeout = '2s';
SET statement_timeout = '30s';

UPDATE convoy.project_configurations
SET sync_dynamic_event_ack = TRUE
WHERE verify_dynamic_events IS TRUE;

-- squawk-ignore ban-drop-column
ALTER TABLE convoy.project_configurations DROP COLUMN IF EXISTS verify_dynamic_events;

RESET lock_timeout;
RESET statement_timeout;
