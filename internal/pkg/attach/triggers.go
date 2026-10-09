package attach

import "fmt"

// DropConstraintSQL drops a named constraint from table and from table_default
// if that adopted child exists. After attach conversion the real FK lives on
// the child; DROP on the parent name is a no-op.
func DropConstraintSQL(table, constraint string) []string {
	return []string{
		fmt.Sprintf(`ALTER TABLE convoy.%s DROP CONSTRAINT IF EXISTS %s`,
			quoteIdent(table), quoteIdent(constraint)),
		fmt.Sprintf(`ALTER TABLE IF EXISTS convoy.%s DROP CONSTRAINT IF EXISTS %s`,
			quoteIdent(table+"_default"), quoteIdent(constraint)),
	}
}

// EventFKSQL stands in for event_deliveries.event_id → events while events or
// event_deliveries is partitioned. Postgres 16 rejects NOT VALID foreign keys
// on a partitioned parent, and a validated key scans every partition under
// SHARE ROW EXCLUSIVE.
const EventFKSQL = `
CREATE OR REPLACE FUNCTION convoy.enforce_event_fk()
    RETURNS TRIGGER AS $$
BEGIN
    IF EXISTS (SELECT 1 FROM convoy.events WHERE id = NEW.event_id) THEN
        RETURN NEW;
    END IF;
    -- Detach renames the partitioned parent to events_partitioned and only
    -- copies those rows back after the swap. Look there too, or a delivery
    -- for a post-conversion event is rejected for the whole drain.
    IF to_regclass('convoy.events_partitioned') IS NOT NULL THEN
        IF EXISTS (SELECT 1 FROM convoy.events_partitioned WHERE id = NEW.event_id) THEN
            RETURN NEW;
        END IF;
    END IF;
    RAISE EXCEPTION 'Foreign key violation: event_id % does not exist in events', NEW.event_id;
END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE TRIGGER event_fk_check
    BEFORE INSERT ON convoy.event_deliveries
    FOR EACH ROW EXECUTE FUNCTION convoy.enforce_event_fk();`

// AttemptFKSQL stands in for delivery_attempts.event_delivery_id → event_deliveries
// for the same reason. Deliveries install it in Swap (attempts still has its
// original name). Attempts install it in SwapEnd, after the parent exists.
const AttemptFKSQL = `
CREATE OR REPLACE FUNCTION convoy.enforce_event_delivery_fk()
    RETURNS TRIGGER AS $$
BEGIN
    IF EXISTS (SELECT 1 FROM convoy.event_deliveries WHERE id = NEW.event_delivery_id) THEN
        RETURN NEW;
    END IF;
    IF to_regclass('convoy.event_deliveries_partitioned') IS NOT NULL THEN
        IF EXISTS (SELECT 1 FROM convoy.event_deliveries_partitioned WHERE id = NEW.event_delivery_id) THEN
            RETURN NEW;
        END IF;
    END IF;
    RAISE EXCEPTION 'Foreign key violation: event_delivery_id % does not exist in event deliveries', NEW.event_delivery_id;
END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE TRIGGER event_delivery_fk_check
    BEFORE INSERT ON convoy.delivery_attempts
    FOR EACH ROW EXECUTE FUNCTION convoy.enforce_event_delivery_fk();`

// RestoreEventFKSQL keeps the stand-in trigger while either table is still
// partitioned, and restores the real FK once both are ordinary tables.
const RestoreEventFKSQL = `
DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM pg_catalog.pg_class c
        JOIN pg_catalog.pg_namespace n ON n.oid = c.relnamespace
        WHERE n.nspname = 'convoy' AND c.relkind = 'p'
          AND c.relname IN ('events', 'event_deliveries')
    ) THEN
        CREATE OR REPLACE TRIGGER event_fk_check
            BEFORE INSERT ON convoy.event_deliveries
            FOR EACH ROW EXECUTE FUNCTION convoy.enforce_event_fk();
    ELSE
        DROP TRIGGER IF EXISTS event_fk_check ON convoy.event_deliveries;
        IF NOT EXISTS (
            SELECT 1 FROM pg_catalog.pg_constraint
            WHERE conrelid = 'convoy.event_deliveries'::REGCLASS
              AND conname = 'event_deliveries_event_id_fkey'
        ) THEN
            ALTER TABLE convoy.event_deliveries
                ADD CONSTRAINT event_deliveries_event_id_fkey
                    FOREIGN KEY (event_id) REFERENCES convoy.events (id) NOT VALID;
            ALTER TABLE convoy.event_deliveries
                VALIDATE CONSTRAINT event_deliveries_event_id_fkey;
        END IF;
    END IF;
END $$;`

// RestoreAttemptFKSQL keeps the stand-in trigger while either table is still
// partitioned. A real FK on a partitioned delivery_attempts parent cannot use
// only event_delivery_id: Postgres requires every partition-key column in the
// key, and rejects NOT VALID foreign keys on a partitioned parent.
const RestoreAttemptFKSQL = `
DROP TRIGGER IF EXISTS event_delivery_fk_check ON convoy.delivery_attempts;
ALTER TABLE convoy.delivery_attempts DROP CONSTRAINT IF EXISTS delivery_attempts_event_delivery_id_fkey;
DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM pg_catalog.pg_class c
        JOIN pg_catalog.pg_namespace n ON n.oid = c.relnamespace
        WHERE n.nspname = 'convoy' AND c.relkind = 'p'
          AND c.relname IN ('event_deliveries', 'delivery_attempts')
    ) THEN
        CREATE OR REPLACE TRIGGER event_delivery_fk_check
            BEFORE INSERT ON convoy.delivery_attempts
            FOR EACH ROW EXECUTE FUNCTION convoy.enforce_event_delivery_fk();
    ELSE
        ALTER TABLE convoy.delivery_attempts
            ADD CONSTRAINT delivery_attempts_event_delivery_id_fkey
                FOREIGN KEY (event_delivery_id) REFERENCES convoy.event_deliveries (id) NOT VALID;
        ALTER TABLE convoy.delivery_attempts
            VALIDATE CONSTRAINT delivery_attempts_event_delivery_id_fkey;
    END IF;
END $$;`
