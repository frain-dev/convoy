-- +migrate Up
CREATE UNIQUE INDEX IF NOT EXISTS organisation_invites_invitee_email_1 ON convoy.organisation_invites(organisation_id, invitee_email, deleted_at) NULLS NOT DISTINCT;

DROP INDEX IF EXISTS convoy.organisation_invites_invitee_email;

ALTER INDEX convoy.organisation_invites_invitee_email_1 RENAME TO organisation_invites_invitee_email;

-- +migrate Down
CREATE UNIQUE INDEX IF NOT EXISTS convoy.organisation_invites_invitee_email_1 ON convoy.organisation_invites(organisation_id, invitee_email);

DROP INDEX convoy.organisation_invites_invitee_email;

ALTER INDEX convoy.organisation_invites_invitee_email_1 RENAME TO organisation_invites_invitee_email;

