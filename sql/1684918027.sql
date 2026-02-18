-- +migrate Up notransaction
SET lock_timeout = '2s';
SET statement_timeout = '30s';
CREATE UNIQUE INDEX CONCURRENTLY IF NOT EXISTS organisation_invites_invitee_email_1 ON convoy.organisation_invites(organisation_id, invitee_email, deleted_at) NULLS NOT DISTINCT;

DROP INDEX CONCURRENTLY IF EXISTS convoy.organisation_invites_invitee_email;

-- squawk-ignore renaming-table
ALTER INDEX convoy.organisation_invites_invitee_email_1 RENAME TO organisation_invites_invitee_email;

-- +migrate Down notransaction
CREATE UNIQUE INDEX CONCURRENTLY IF NOT EXISTS organisation_invites_invitee_email_1 ON convoy.organisation_invites(organisation_id, invitee_email);

DROP INDEX CONCURRENTLY IF EXISTS convoy.organisation_invites_invitee_email;

-- squawk-ignore renaming-table
ALTER INDEX convoy.organisation_invites_invitee_email_1 RENAME TO organisation_invites_invitee_email;

