-- +migrate Up
SET lock_timeout = '2s';
SET statement_timeout = '30s';

-- Capture and drop one invalid index, reporting whether it dropped anything. The
-- definition goes into convoy.dropped_indexes first, because after the drop there
-- is nothing left to read it from.
--
-- Future migrations that build an index concurrently call this first, so a retry
-- after a killed build drops the invalid leftover and rebuilds it, rather than
-- being skipped by IF NOT EXISTS and recorded as done.
--
-- The drop is not CONCURRENTLY: that cannot run inside a function, and it does not
-- need to. The index is invalid, so no plan depends on it, and the brief ACCESS
-- EXCLUSIVE it takes is bounded by the caller's lock_timeout.
--
-- p_record is FALSE for a drop whose debt is already recorded elsewhere: the
-- rebuild of a partitioned index clears each partition's copy on the way, and
-- those names mean nothing on their own. Rebuilding one alone would leave the
-- parent no closer to valid, so queueing it would be queueing the wrong work.
-- +migrate StatementBegin
CREATE OR REPLACE FUNCTION convoy.drop_invalid_index(p_index TEXT, p_record BOOLEAN DEFAULT TRUE) RETURNS BOOLEAN
    LANGUAGE plpgsql AS $$
DECLARE
    v_oid   OID;
    v_table TEXT;
    v_def   TEXT;
BEGIN
    -- relkind 'i' excludes a partitioned index, which is invalid for a reason
    -- that is not a failure: it stays that way until every partition has an
    -- index attached, which is a normal intermediate state of a conversion. The
    -- pg_inherits check excludes a child of one, which Postgres refuses to drop
    -- on its own anyway.
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

    -- An index still being built is also marked invalid, and the catalog alone
    -- cannot tell that apart from one abandoned by a dead build. Dropping it
    -- would destroy work someone is waiting on, so leave it: it either finishes
    -- and turns valid, or it fails and the next caller finds it idle.
    --
    -- The progress view is checked first but cannot be trusted on its own: a role
    -- without pg_read_all_stats sees only its own sessions there, so a build
    -- started by the application would look absent. A lock nobody else should be
    -- holding is the reliable half. The planner ignores invalid indexes, so no
    -- ordinary query locks this one, which leaves a live build or another repair
    -- as the explanations. Both checks fail towards leaving the index alone.
    IF EXISTS (SELECT 1 FROM pg_stat_progress_create_index WHERE index_relid = v_oid)
        OR EXISTS (SELECT 1 FROM pg_locks
                    WHERE relation = v_oid AND granted AND pid <> pg_backend_pid()) THEN
        RAISE NOTICE 'convoy.% is invalid but a build is in progress, leaving it alone', p_index;
        RETURN FALSE;
    END IF;

    -- A constraint owns its index and the drop has to go through the constraint,
    -- which changes what the table enforces. That is not this function's call.
    IF EXISTS (SELECT 1 FROM pg_constraint WHERE conindid = v_oid) THEN
        RAISE NOTICE 'convoy.% is invalid but a constraint depends on it, leaving it alone', p_index;
        RETURN FALSE;
    END IF;

    -- REINDEX CONCURRENTLY leftovers are transient copies of an index that still
    -- exists under its own name. Recording one would queue a rebuild that
    -- recreates the copy rather than the index, so drop it without recording.
    IF p_record AND p_index NOT LIKE '%\_ccnew%' AND p_index NOT LIKE '%\_ccold%' THEN
        INSERT INTO convoy.dropped_indexes (index_name, table_name, definition)
        VALUES (p_index, v_table, v_def)
        ON CONFLICT (index_name) DO UPDATE
            SET table_name = EXCLUDED.table_name,
                definition = EXCLUDED.definition,
                dropped_at = NOW(),
                rebuilt_at = NULL;
    END IF;

    EXECUTE FORMAT('DROP INDEX convoy.%I', p_index);

    -- Only a recorded drop leaves work behind to advise about. The unrecorded
    -- one is a rebuild clearing a leftover on its way to building the index
    -- back, so pointing the caller at --rebuild would name what they are
    -- already running.
    IF NOT p_record THEN
        RAISE NOTICE 'cleared the invalid index convoy.% left by an earlier attempt', p_index;
    ELSIF v_def LIKE 'CREATE UNIQUE %' THEN
        -- An invalid index is ignored by the planner but is still maintained by
        -- writes once it reached the validation scan, so a unique one was
        -- enforcing its key right up to this drop. Losing an index costs speed;
        -- losing this one also costs the uniqueness.
        RAISE WARNING 'dropped invalid unique index convoy.%: its key is no longer unique until rebuilt with convoy utils indexes --rebuild', p_index;
    ELSE
        RAISE NOTICE 'dropped invalid index convoy.%, rebuild it with: convoy utils indexes --rebuild', p_index;
    END IF;
    RETURN TRUE;
END $$;
-- +migrate StatementEnd

-- Repair what earlier upgrades left behind. This runs inside the migration's
-- transaction, so each capture and its drop commit together or not at all, and a
-- lock_timeout means a table busy with long queries fails the migration rather
-- than blocking the boot. A failed migration is not recorded, so the next boot
-- retries this cleanly.
-- +migrate StatementBegin
DO $$
DECLARE
    r RECORD;
BEGIN
    FOR r IN
        SELECT c.relname
          FROM pg_index i
          JOIN pg_class c ON c.oid = i.indexrelid
          JOIN pg_namespace n ON n.oid = c.relnamespace
         WHERE n.nspname = 'convoy'
           AND c.relkind = 'i'
           AND NOT i.indisvalid
         ORDER BY c.relname
        LOOP
            PERFORM convoy.drop_invalid_index(r.relname);
        END LOOP;
END $$;
-- +migrate StatementEnd

RESET lock_timeout;
RESET statement_timeout;

-- +migrate Down
SET lock_timeout = '2s';
SET statement_timeout = '30s';

DROP FUNCTION IF EXISTS convoy.drop_invalid_index(TEXT, BOOLEAN);

RESET lock_timeout;
RESET statement_timeout;
