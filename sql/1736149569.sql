-- +migrate Up
UPDATE convoy.api_keys
SET role_type = 'organisation_admin'
WHERE role_type = 'super_user';

UPDATE convoy.organisation_members
SET role_type = 'organisation_admin'
WHERE role_type = 'super_user';

UPDATE convoy.organisation_invites
SET role_type = 'organisation_admin'
WHERE role_type = 'super_user';

-- +migrate Down
UPDATE convoy.api_keys
SET role_type = 'super_user'
WHERE role_type = 'organisation_admin';

UPDATE convoy.organisation_members
SET role_type = 'super_user'
WHERE role_type = 'organisation_admin';

UPDATE convoy.organisation_invites
SET role_type = 'super_user'
WHERE role_type = 'organisation_admin';


