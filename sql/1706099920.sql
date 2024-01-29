-- +migrate Up
-- +migrate StatementBegin
CREATE OR REPLACE FUNCTION convoy.bootstrap_default_user_and_organisation() RETURNS VOID
AS
$$
DECLARE
    oid text;
    uid text;
BEGIN
    select id into uid from convoy.users order by created_at limit 1;
    select id into oid from convoy.organisations order by created_at limit 1;

    CASE WHEN uid IS NULL
             THEN
                 uid := convoy.generate_ulid();
                 INSERT INTO convoy.users (id, first_name, last_name, email, password, email_verified, reset_password_token, email_verification_token, created_at, reset_password_expires_at, email_verification_expires_at, updated_at, deleted_at)
                 values (uid, 'default', 'default', 'superuser@default.com', public.crypt('default', public.gen_salt('bf', 12)), true, '', now(), now(), now(), now(), now(), null);
         ELSE
        END CASE;

    CASE WHEN oid IS NULL
        THEN
            oid := convoy.generate_ulid();
            insert into convoy.organisations (id, name, owner_id, custom_domain, assigned_domain, created_at, updated_at, deleted_at)
            values (oid, 'default', uid, null,  null, now(), now(), null);

            insert into convoy.organisation_members (id, role_type, role_project, role_endpoint, user_id, organisation_id, created_at, updated_at, deleted_at)
            values (convoy.generate_ulid(), 'super_user', null, null, uid, oid, now(), now(), null);
         ELSE
        END CASE;
END
$$ LANGUAGE plpgsql;
-- +migrate StatementEnd

-- +migrate Up
select convoy.bootstrap_default_user_and_organisation();








