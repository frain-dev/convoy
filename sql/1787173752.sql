-- +migrate Up
SET lock_timeout = '2s';
SET statement_timeout = '30s';

-- Appended rather than placed next to slack_webhook_url: the generated endpoint
-- update statement addresses columns positionally, so inserting mid-list would
-- silently reassign every parameter after it.
ALTER TABLE convoy.endpoints
ADD COLUMN IF NOT EXISTS teams_webhook_url TEXT;

RESET lock_timeout;
RESET statement_timeout;

-- +migrate Down
SET lock_timeout = '2s';
SET statement_timeout = '30s';

-- squawk-ignore ban-drop-column
ALTER TABLE convoy.endpoints DROP COLUMN IF EXISTS teams_webhook_url;

RESET lock_timeout;
RESET statement_timeout;
