-- +migrate Up
-- Set all instance_admin roles to organisation_admin
UPDATE convoy.organisation_members
SET role_type = 'organisation_admin'
WHERE role_type = 'instance_admin';

-- +migrate Down
-- Revert the first user's organisation_admin role back to instance_admin
-- Note: This only reverts the first user (by created_at), not all users
UPDATE convoy.organisation_members
SET role_type = 'instance_admin'
WHERE role_type = 'organisation_admin'
AND id = (
    SELECT id FROM convoy.organisation_members
    WHERE role_type = 'organisation_admin'
    ORDER BY created_at ASC
    LIMIT 1
);

