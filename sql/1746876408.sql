-- +migrate Up
SET lock_timeout = '2s';
SET statement_timeout = '30s';
UPDATE convoy.organisation_members
SET role_type = 'instance_admin'
WHERE role_type = 'super_user';

UPDATE convoy.organisation_members
SET role_type = 'project_admin'
WHERE role_type = 'admin';

UPDATE convoy.organisation_invites
SET role_type = 'instance_admin'
WHERE role_type = 'super_user';

UPDATE convoy.organisation_invites
SET role_type = 'project_admin'
WHERE role_type = 'admin';

UPDATE convoy.api_keys
SET role_type = 'instance_admin'
WHERE role_type = 'super_user';

UPDATE convoy.api_keys
SET role_type = 'project_admin'
WHERE role_type = 'admin';

-- +migrate Down
UPDATE convoy.organisation_members
SET role_type = 'super_user'
WHERE role_type = 'instance_admin';

UPDATE convoy.organisation_members
SET role_type = 'admin'
WHERE role_type = 'project_admin';

UPDATE convoy.organisation_invites
SET role_type = 'super_user'
WHERE role_type = 'instance_admin';

UPDATE convoy.organisation_invites
SET role_type = 'admin'
WHERE role_type = 'project_admin';

UPDATE convoy.api_keys
SET role_type = 'super_user'
WHERE role_type = 'instance_admin';

UPDATE convoy.api_keys
SET role_type = 'admin'
