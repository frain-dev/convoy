-- Reconcile endpoints NOT NULL schema drift from in-place rewrites of
-- sql/1709729972.sql (name, url) and sql/1705575999.sql (http_timeout,
-- rate_limit_duration). Fresh installs got CHECK (... IS NOT NULL) without
-- attnotnull; long-lived DBs got ALTER COLUMN SET NOT NULL from the originals.
--
-- Repair policy: NULL rows fail the migration. Backfill name/url/http_timeout/
-- rate_limit_duration on affected rows before retrying; do not silently default.
--
-- +migrate Up
SET lock_timeout = '2s';
SET statement_timeout = '30s';

-- +migrate StatementBegin
DO $$
DECLARE
    col record;
    null_count bigint;
    endpoints_oid oid := 'convoy.endpoints'::regclass;
BEGIN
    FOR col IN
        SELECT *
        FROM (VALUES
            ('name', 'name_not_null'),
            ('url', 'url_not_null'),
            ('http_timeout', 'new_http_timeout_not_null'),
            ('rate_limit_duration', 'new_rate_limit_duration_not_null')
        ) AS t(column_name, check_constraint_name)
    LOOP
        IF EXISTS (
            SELECT 1
            FROM pg_attribute a
            WHERE a.attrelid = endpoints_oid
              AND a.attname = col.column_name
              AND a.attnum > 0
              AND NOT a.attisdropped
              AND a.attnotnull
        ) THEN
            CONTINUE;
        END IF;

        EXECUTE format(
            'SELECT count(*) FROM convoy.endpoints WHERE %I IS NULL',
            col.column_name
        ) INTO null_count;

        IF null_count > 0 THEN
            RAISE EXCEPTION
                'convoy.endpoints.% has % NULL row(s); backfill before re-running migration 1787188800',
                col.column_name,
                null_count;
        END IF;

        IF EXISTS (
            SELECT 1
            FROM pg_constraint c
            WHERE c.conrelid = endpoints_oid
              AND c.conname = col.check_constraint_name
              AND NOT c.convalidated
        ) THEN
            EXECUTE format(
                'ALTER TABLE convoy.endpoints VALIDATE CONSTRAINT %I',
                col.check_constraint_name
            );
        END IF;

        -- Keep the validated CHECK through SET NOT NULL so PostgreSQL can skip a
        -- heap scan; drop the redundant CHECK only after attnotnull is set.
        EXECUTE format(
            'ALTER TABLE convoy.endpoints ALTER COLUMN %I SET NOT NULL',
            col.column_name
        );

        IF EXISTS (
            SELECT 1
            FROM pg_constraint c
            WHERE c.conrelid = endpoints_oid
              AND c.conname = col.check_constraint_name
        ) THEN
            EXECUTE format(
                'ALTER TABLE convoy.endpoints DROP CONSTRAINT %I',
                col.check_constraint_name
            );
        END IF;
    END LOOP;
END $$;
-- +migrate StatementEnd

RESET lock_timeout;
RESET statement_timeout;

-- +migrate Down
-- Intentionally empty: production DBs already enforced NOT NULL before this migration.
