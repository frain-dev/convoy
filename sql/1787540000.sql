-- +migrate Up
SET lock_timeout = '2s';
SET statement_timeout = '30s';

-- A unique rebuild that hits duplicate rows is not a transient failure. Retrying
-- it every boot would hold the slot and fail the same way until an operator
-- fixes the data, so the debt records why it is blocked.
ALTER TABLE convoy.dropped_indexes
    ADD COLUMN IF NOT EXISTS blocked_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS blocked_reason TEXT;

-- Re-recording an invalid index starts the debt over, so a blocked marker from
-- an earlier attempt must not survive the drop.
-- +migrate StatementBegin
CREATE OR REPLACE FUNCTION convoy.drop_invalid_index(p_index TEXT, p_record BOOLEAN DEFAULT TRUE) RETURNS BOOLEAN
    LANGUAGE plpgsql AS $$
DECLARE
    v_oid   OID;
    v_table TEXT;
    v_def   TEXT;
BEGIN
    SELECT i.indexrelid, t.relname, pg_get_indexdef(i.indexrelid)
      INTO v_oid, v_table, v_def
      FROM pg_index i
      JOIN pg_class c ON c.oid = i.indexrelid
      JOIN pg_class t ON t.oid = i.indrelid
      JOIN pg_namespace n ON n.oid = c.relnamespace
     WHERE n.nspname = 'convoy'
       AND c.relname = p_index
       AND c.relkind = 'i'
       AND NOT i.indisvalid
       AND NOT EXISTS (SELECT 1 FROM pg_inherits WHERE inhrelid = i.indexrelid);

    IF v_oid IS NULL THEN
        RETURN FALSE;
    END IF;

    IF EXISTS (SELECT 1 FROM pg_stat_progress_create_index WHERE index_relid = v_oid)
        OR EXISTS (SELECT 1 FROM pg_locks
                    WHERE relation = v_oid AND granted AND pid <> pg_backend_pid()) THEN
        RAISE NOTICE 'convoy.% is invalid but a build is in progress, leaving it alone', p_index;
        RETURN FALSE;
    END IF;

    IF EXISTS (SELECT 1 FROM pg_constraint WHERE conindid = v_oid) THEN
        RAISE NOTICE 'convoy.% is invalid but a constraint depends on it, leaving it alone', p_index;
        RETURN FALSE;
    END IF;

    IF p_record AND p_index NOT LIKE '%\_ccnew%' AND p_index NOT LIKE '%\_ccold%' THEN
        INSERT INTO convoy.dropped_indexes (index_name, table_name, definition)
        VALUES (p_index, v_table, v_def)
        ON CONFLICT (index_name) DO UPDATE
            SET table_name = EXCLUDED.table_name,
                definition = EXCLUDED.definition,
                dropped_at = NOW(),
                rebuilt_at = NULL,
                blocked_at = NULL,
                blocked_reason = NULL;
    END IF;

    EXECUTE FORMAT('DROP INDEX convoy.%I', p_index);

    IF NOT p_record THEN
        RAISE NOTICE 'cleared the invalid index convoy.% left by an earlier attempt', p_index;
    ELSIF v_def LIKE 'CREATE UNIQUE %' THEN
        RAISE WARNING 'dropped invalid unique index convoy.%: its key is no longer unique until rebuilt with convoy utils indexes --rebuild', p_index;
    ELSE
        RAISE NOTICE 'dropped invalid index convoy.%, rebuild it with: convoy utils indexes --rebuild', p_index;
    END IF;
    RETURN TRUE;
END $$;
-- +migrate StatementEnd

RESET lock_timeout;
RESET statement_timeout;

-- +migrate Down
SET lock_timeout = '2s';
SET statement_timeout = '30s';

ALTER TABLE convoy.dropped_indexes
    DROP COLUMN IF EXISTS blocked_reason,
    DROP COLUMN IF EXISTS blocked_at;

RESET lock_timeout;
RESET statement_timeout;
