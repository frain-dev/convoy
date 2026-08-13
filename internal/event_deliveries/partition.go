package event_deliveries

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

// Converting event_deliveries by copying it, which is what this package used to
// do, reads a snapshot and drops the source when it finishes. Every row written
// between those two moments is discarded, and Convoy has already answered 201 to
// the requests that produced them. Measured on a 165 GB table under 3 req/s, one
// conversion destroyed 7,099 acknowledged webhooks and held ACCESS EXCLUSIVE for
// 92.7 seconds across three tables.
//
// This converts by adopting the existing heap as a partition instead. No row is
// copied, so no row can be lost, and the exclusive lock covers a catalog swap
// rather than a rewrite.
//
// The heap becomes the parent's DEFAULT partition rather than a bounded one.
// That is what makes the conversion work on an instance with more than one
// project: the partition key leads with project_id, so a bounded partition
// covers exactly one project, and the heap holds all of them. A default
// partition claims whatever no bounded partition does, so it is indifferent to
// how many projects are in it.
//
// Two constraints on the shape are not obvious and are load-bearing:
//
//   - The name must end in _default. gopartman's importer adopts a default
//     partition only under that name; anything else is skipped, and then its
//     provisioner tries to create its own <parent>_default and collides with the
//     partition already attached here.
//   - The bounds CHECK is permanent, not scaffolding for the attach. It is what
//     lets Postgres refute each forward partition's range and skip scanning the
//     heap, and it is what stops a live row that matches no partition from being
//     absorbed into history instead of failing loudly.
//
// Retention never drops the adopted heap: gopartman filters is_default = false
// when selecting expired partitions. History is reclaimed by dropping that one
// partition once the cutoff is older than the retention period, not day by day.
const (
	// attachPremakeDays is how far ahead forward partitions are created, matching
	// the premake retention already maintains nightly. Reaching further would put
	// the conversion's window at odds with the steady-state one, and each extra
	// day is another partition per project for the planner to prune.
	attachPremakeDays = 10

	// attachLockTimeout bounds the swap. A blocked ALTER holding a partial lock
	// queues every reader behind it, which turns a millisecond swap into an
	// outage, so it fails and is retried instead of waiting.
	attachLockTimeout = "3s"
)

// partitionedPKIndex is built ahead of the swap and promoted to the adopted
// partition's primary key inside it.
const partitionedPKIndex = "event_deliveries_pk_part"

// adoptedIndex is one of the heap's indexes, and the definition the parent will
// carry so that new partitions inherit it.
type adoptedIndex struct {
	name string
	def  string
}

// childIndexName is what the heap's copy is renamed to, freeing the canonical
// name for the parent.
//
// An ordinal rather than a derived name: identifiers are capped at 63 bytes and
// several of these names are already near that, so anything built by adding to
// the original truncates, and two truncated names can collide with each other or
// with the canonical name being created. Nothing reads these names back by
// convention. Unpartitioning recovers the pairing from pg_inherits, which is the
// authority on which child index belongs to which parent index.
func childIndexName(ordinal int) string {
	return fmt.Sprintf("event_deliveries_default_idx%d", ordinal)
}

// attachCutoff is the instant that divides adopted history from partitioned
// future. It is in the future, not now, because a NOT VALID CHECK still applies
// to rows written after it is added: only existing rows go unchecked. A cutoff
// of now would therefore reject every insert from the moment bounds are declared
// until the swap, which is the ingestion pause this design exists to remove.
//
// The start of the day after tomorrow, so the gap is never less than a full day
// however late in the UTC day the conversion starts. The scan it has to outlast
// runs for as long as the table is large, and a cutoff of "tomorrow" chosen at
// 23:50 UTC would be crossed while that scan is still running, at which point
// every insert starts failing on a constraint nothing has swapped in yet.
//
// The cost of reaching further ahead is that rows written before the cutoff land
// in the adopted heap, since no forward partition claims them yet. That is a day
// or two of live rows joining months of history in the same partition, and the
// heap is closed for good once the cutoff passes.
func attachCutoff(now time.Time) time.Time {
	return now.UTC().Truncate(24*time.Hour).AddDate(0, 0, 2)
}

func (s *Service) PartitionEventDeliveriesTable(ctx context.Context) error {
	return s.attachEventDeliveries(ctx, attachCutoff(time.Now()))
}

func (s *Service) attachEventDeliveries(ctx context.Context, cutoff time.Time) error {
	bound := cutoff.Format(time.RFC3339)

	if err := s.attachPreflight(ctx); err != nil {
		return err
	}

	s.notice(ctx, "Building the partitioned primary key...")
	// CONCURRENTLY: two passes over the heap under SHARE UPDATE EXCLUSIVE, which
	// readers and writers pass through. It cannot run inside a transaction, which
	// is why the phases below are separate statements rather than one block.
	if _, err := s.db.Exec(ctx, `
        CREATE UNIQUE INDEX CONCURRENTLY IF NOT EXISTS event_deliveries_pk_part
            ON convoy.event_deliveries (id, created_at, project_id)`); err != nil {
		return fmt.Errorf("building partitioned primary key: %w", err)
	}

	s.notice(ctx, "Declaring the range the existing table covers...")
	if err := s.declareBounds(ctx, bound); err != nil {
		return err
	}

	s.notice(ctx, "Proving it (this is the long phase, writes continue)...")
	// The scan that would otherwise happen inside the exclusive lock during
	// ATTACH. Held under SHARE UPDATE EXCLUSIVE, so ingestion is unaffected.
	if _, err := s.db.Exec(ctx, `
        ALTER TABLE convoy.event_deliveries
            VALIDATE CONSTRAINT event_deliveries_default_bounds`); err != nil {
		// The constraint also asserts both columns are non-null, because a
		// partition may not be laxer than its parent, and the shipped schema
		// leaves them nullable. A row holding a null is the one failure here that
		// is about the data rather than the conversion, and it is fixable.
		return fmt.Errorf("validating history bounds: %w. If this failed on a null, backfill with "+
			"UPDATE convoy.event_deliveries SET created_at = COALESCE(created_at, NOW()), "+
			"delivery_mode = COALESCE(delivery_mode, 'at_least_once') "+
			"WHERE created_at IS NULL OR delivery_mode IS NULL, then run the conversion again", err)
	}

	s.notice(ctx, "Swapping in the partitioned table...")
	if err := s.swap(ctx); err != nil {
		return err
	}

	// Past this point the swap is committed and the table is partitioned. A
	// failure below leaves a working table with something missing, so each step
	// says what is missing rather than reporting the conversion as not having
	// happened.
	s.notice(ctx, "Creating partitions...")
	if err := s.createForwardPartitions(ctx, bound); err != nil {
		return err
	}

	s.notice(ctx, "Restoring event id enforcement...")
	if err := s.restoreEventEnforcement(ctx); err != nil {
		return err
	}

	s.notice(ctx, "Migration complete!")
	return nil
}

// attachPreflight refuses a conversion that cannot finish cleanly, before
// anything has been changed.
func (s *Service) attachPreflight(ctx context.Context) error {
	s.notice(ctx, "Checking the table can be converted...")

	_, err := s.db.Exec(ctx, `
        DO $$
        DECLARE
            kind CHAR;
        BEGIN
            SELECT c.relkind INTO kind
            FROM pg_class c JOIN pg_namespace n ON n.oid = c.relnamespace
            WHERE n.nspname = 'convoy' AND c.relname = 'event_deliveries';

            IF kind IS NULL THEN
                RAISE EXCEPTION 'convoy.event_deliveries does not exist';
            END IF;

            IF kind = 'p' THEN
                RAISE EXCEPTION 'convoy.event_deliveries is already partitioned';
            END IF;

            IF EXISTS (SELECT 1 FROM pg_class c JOIN pg_namespace n ON n.oid = c.relnamespace
                       WHERE n.nspname = 'convoy' AND c.relname = 'event_deliveries_new') THEN
                RAISE EXCEPTION
                    'convoy.event_deliveries_new exists: a copy-based conversion died partway. Drop it before converting';
            END IF;

            IF EXISTS (SELECT 1 FROM pg_class c JOIN pg_namespace n ON n.oid = c.relnamespace
                       WHERE n.nspname = 'convoy' AND c.relname = 'event_deliveries_default') THEN
                RAISE EXCEPTION
                    'convoy.event_deliveries_default exists: an attach-based conversion died partway. Rename it back before converting';
            END IF;

            -- A CREATE INDEX CONCURRENTLY that failed on an earlier attempt leaves
            -- an invalid index behind, and IF NOT EXISTS would then skip rebuilding
            -- it. Promoting an invalid index to the primary key is not possible, so
            -- clear it here rather than failing mid-swap.
            IF EXISTS (
                SELECT 1 FROM pg_class c
                JOIN pg_namespace n ON n.oid = c.relnamespace
                JOIN pg_index i ON i.indexrelid = c.oid
                WHERE n.nspname = 'convoy' AND c.relname = 'event_deliveries_pk_part' AND NOT i.indisvalid
            ) THEN
                DROP INDEX convoy.event_deliveries_pk_part;
            END IF;
        END $$;`)
	if err != nil {
		return fmt.Errorf("preflight: %w", err)
	}

	// The parent's index set is read off the heap rather than declared, and an
	// invalid index cannot be attached to a parent index, so one here would be
	// dropped from the set silently. The parent would then be missing it, and
	// every partition created from that point on inherits the parent's set, so
	// the loss lands on live traffic rather than on history. Refuse instead: an
	// invalid index is a failed CREATE INDEX CONCURRENTLY, and rebuilding it is
	// something an operator can do without stopping anything.
	var invalid []string
	rows, err := s.db.Query(ctx, `
        SELECT c.relname
        FROM pg_index i
        JOIN pg_class c ON c.oid = i.indexrelid
        JOIN pg_class t ON t.oid = i.indrelid
        JOIN pg_namespace n ON n.oid = t.relnamespace
        WHERE n.nspname = 'convoy' AND t.relname = 'event_deliveries' AND NOT i.indisvalid
        ORDER BY c.relname`)
	if err != nil {
		return fmt.Errorf("preflight: reading indexes: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var name string
		if err = rows.Scan(&name); err != nil {
			return fmt.Errorf("preflight: reading indexes: %w", err)
		}
		invalid = append(invalid, name)
	}
	if err = rows.Err(); err != nil {
		return fmt.Errorf("preflight: reading indexes: %w", err)
	}

	if len(invalid) > 0 {
		return fmt.Errorf("convoy.event_deliveries has invalid indexes: %s. Rebuild them with REINDEX INDEX CONCURRENTLY, or drop them, before converting",
			strings.Join(invalid, ", "))
	}

	return nil
}

// rowQuerier is what both the pool and a transaction provide. The index set is
// read inside the swap's transaction so it cannot change between being read and
// being attached.
type rowQuerier interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}

// adoptedIndexes reads the indexes the parent has to carry, off the heap itself.
//
// Declaring this set in Go is what an earlier version did, copying the list out
// of the copy path's DDL. That list was a snapshot of the schema on the day it
// was written: it had already drifted by one name, and it was missing ten
// indexes added by later migrations, including partial and expression ones. The
// cost of that is not a slow conversion, it is that every partition created
// afterwards inherits the parent's set, so the missing indexes are absent from
// exactly the partitions serving live queries. Reading the catalog cannot drift.
//
// Two exclusions. The primary key is handled by the swap, which promotes a
// purpose-built index into it. A unique index that does not contain both
// partition key columns cannot exist on a partitioned parent at all, so it stays
// on the adopted partition, still enforced there, and is not carried forward.
func adoptedIndexes(ctx context.Context, q rowQuerier) ([]adoptedIndex, error) {
	rows, err := q.Query(ctx, `
        SELECT c.relname, pg_get_indexdef(i.indexrelid)
        FROM pg_index i
        JOIN pg_class c ON c.oid = i.indexrelid
        JOIN pg_class t ON t.oid = i.indrelid
        JOIN pg_namespace n ON n.oid = t.relnamespace
        WHERE n.nspname = 'convoy'
          AND t.relname = 'event_deliveries'
          AND i.indisvalid
          AND NOT i.indisprimary
          AND c.relname <> $1
          AND (NOT i.indisunique OR (
                SELECT ARRAY['project_id', 'created_at']::NAME[] <@ COALESCE(array_agg(a.attname), '{}')
                FROM pg_attribute a
                WHERE a.attrelid = t.oid AND a.attnum = ANY (i.indkey)
          ))
        ORDER BY c.relname`, partitionedPKIndex)
	if err != nil {
		return nil, fmt.Errorf("reading the indexes to carry forward: %w", err)
	}
	defer rows.Close()

	var indexes []adoptedIndex
	for rows.Next() {
		var index adoptedIndex
		if err = rows.Scan(&index.name, &index.def); err != nil {
			return nil, fmt.Errorf("reading the indexes to carry forward: %w", err)
		}
		indexes = append(indexes, index)
	}
	return indexes, rows.Err()
}

// declareBounds adds the constraint the swap depends on. It is added NOT VALID
// so it returns immediately, then proven separately.
//
// created_at and delivery_mode are proven NOT NULL here for different reasons.
// created_at is a partition key column and ATTACH requires those NOT NULL;
// delivery_mode is not a key column, but the parent declares it NOT NULL and a
// child may not be laxer than its parent. Both are nullable on the shipped
// schema. Postgres can satisfy SET NOT NULL from a validated CHECK instead of
// rescanning the heap, which is what keeps the swap short.
func (s *Service) declareBounds(ctx context.Context, bound string) error {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err = tx.Exec(ctx, `SET LOCAL lock_timeout = '`+attachLockTimeout+`'`); err != nil {
		return err
	}

	if _, err = tx.Exec(ctx, `
        ALTER TABLE convoy.event_deliveries
            DROP CONSTRAINT IF EXISTS event_deliveries_default_bounds`); err != nil {
		return fmt.Errorf("clearing history bounds: %w", err)
	}

	// The bound is a timestamp this package computed, not caller input, and DDL
	// cannot take bind parameters.
	if _, err = tx.Exec(ctx, fmt.Sprintf(`
        ALTER TABLE convoy.event_deliveries
            ADD CONSTRAINT event_deliveries_default_bounds
            CHECK (created_at IS NOT NULL
                   AND delivery_mode IS NOT NULL
                   AND created_at < '%s'::TIMESTAMPTZ) NOT VALID`, bound)); err != nil {
		return fmt.Errorf("declaring history bounds: %w", err)
	}

	return tx.Commit(ctx)
}

// swap is the only phase that takes ACCESS EXCLUSIVE. Everything in it is
// catalog-only: the constraint validated in the previous phase is what lets
// Postgres prove the bound and the NOT NULLs without reading the heap.
func (s *Service) swap(ctx context.Context) error {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// Read before the rename, so the definitions still name the table the parent
	// is about to become, and inside the transaction, so nothing can add an index
	// between reading the set and attaching it.
	indexes, err := adoptedIndexes(ctx, tx)
	if err != nil {
		return err
	}

	attachIndexes, err := attachIndexStatements(indexes)
	if err != nil {
		return err
	}

	statements := append([]string{
		`SET LOCAL lock_timeout = '` + attachLockTimeout + `'`,

		`ALTER TABLE convoy.event_deliveries RENAME TO event_deliveries_default`,

		`ALTER TABLE convoy.event_deliveries_default
            ALTER COLUMN created_at SET NOT NULL,
            ALTER COLUMN delivery_mode SET NOT NULL`,

		// delivery_attempts' foreign key depends on the primary key being dropped
		// next, so it goes first, and its replacement follows in the same
		// transaction. Installing the trigger after the swap commits would leave a
		// window, however short, where delivery_attempts accepts a row referencing
		// a delivery that does not exist, and a crash in that window leaves the
		// table unenforced until someone notices.
		`ALTER TABLE convoy.delivery_attempts
            DROP CONSTRAINT IF EXISTS delivery_attempts_event_delivery_id_fkey`,

		attemptEnforcementSQL,

		// A partition may not carry its own primary key, and the parent's must
		// contain every partition key column, so uniqueness on id alone cannot
		// survive. The copy path lands on the same wider key.
		`ALTER TABLE convoy.event_deliveries_default
            DROP CONSTRAINT IF EXISTS event_deliveries_pkey`,

		// ATTACH adopts a child's index only when it backs a constraint. Given a
		// bare unique index it ignores it and builds its own while holding ACCESS
		// EXCLUSIVE, which on a large table is the outage this design exists to
		// avoid. USING INDEX is catalog-only.
		`ALTER TABLE convoy.event_deliveries_default
            ADD CONSTRAINT event_deliveries_default_pkey
            PRIMARY KEY USING INDEX ` + partitionedPKIndex,

		createPartitionedTableSQL,

		// Attached while the parent carries only its primary key. Attaching to a
		// parent that already has secondary indexes makes Postgres find or build a
		// matching child index for each one, under the exclusive lock. They are
		// attached explicitly below instead, where a mismatch is an error rather
		// than a silent rebuild.
		`ALTER TABLE convoy.event_deliveries
            ATTACH PARTITION convoy.event_deliveries_default DEFAULT`,
	}, attachIndexes...)

	for _, statement := range statements {
		if _, err = tx.Exec(ctx, statement); err != nil {
			return fmt.Errorf("swapping in the partitioned table: %w", err)
		}
	}

	return tx.Commit(ctx)
}

// attachIndexStatements moves each of the heap's index names out of the way,
// recreates the canonical name on the parent alone, and adopts the heap's
// existing index under it.
//
// The canonical name has to end up on the parent, not the partition. Migrations
// add indexes with CREATE INDEX IF NOT EXISTS against convoy.event_deliveries,
// and index names are unique per schema, so a canonical name left on the
// partition makes every future migration adding that index a silent no-op
// against the parent.
//
// ON ONLY creates the parent's index without recursing into partitions, which is
// what makes this catalog-only; ATTACH then adopts the existing index and marks
// the parent's valid. The definition is rebuilt from the part after USING rather
// than by rewriting the table name in it, because pg_get_indexdef schema
// qualifies according to search_path and both forms have to work.
func attachIndexStatements(indexes []adoptedIndex) ([]string, error) {
	statements := make([]string, 0, len(indexes)*3)
	for i, index := range indexes {
		shape := strings.Index(index.def, " USING ")
		if shape < 0 {
			return nil, fmt.Errorf("cannot read the definition of index %s: %q", index.name, index.def)
		}

		unique := ""
		if strings.HasPrefix(index.def, "CREATE UNIQUE ") {
			unique = "UNIQUE "
		}

		child := childIndexName(i)
		statements = append(statements,
			fmt.Sprintf(`ALTER INDEX convoy.%s RENAME TO %s`, index.name, child),
			fmt.Sprintf(`CREATE %sINDEX %s ON ONLY convoy.event_deliveries%s`, unique, index.name, index.def[shape:]),
			fmt.Sprintf(`ALTER INDEX convoy.%s ATTACH PARTITION convoy.%s`, index.name, child),
		)
	}
	return statements, nil
}

// attemptEnforcementSQL replaces, inside the swap, the foreign key the swap
// drops. delivery_attempts.event_delivery_id cannot be a plain foreign key to a
// partitioned parent under this scheme, so the same trigger the copy path
// installs takes its place.
//
// The function is created rather than assumed: it ships only inside the copy
// path's SQL, so on an instance that never ran that path it does not exist.
// CREATE TRIGGER takes the lock the swap is already holding on
// delivery_attempts, so running it here costs nothing beyond the swap.
const attemptEnforcementSQL = `
CREATE OR REPLACE FUNCTION enforce_event_delivery_fk()
    RETURNS TRIGGER AS $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM convoy.event_deliveries
        WHERE id = NEW.event_delivery_id
    ) THEN
        RAISE EXCEPTION 'Foreign key violation: event_delivery_id % does not exist in event deliveries', NEW.event_delivery_id;
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE TRIGGER event_delivery_fk_check
    BEFORE INSERT ON convoy.delivery_attempts
    FOR EACH ROW EXECUTE FUNCTION enforce_event_delivery_fk()`

// restoreEventEnforcement keeps event_id honest on rows inserted from here on.
//
// The copy path branches here: a trigger when convoy.events is partitioned, a
// real foreign key when it is not. This does not branch, and always installs the
// trigger, because the foreign key is not available at this point without giving
// back what the conversion just bought. Postgres 16 refuses NOT VALID foreign
// keys on a partitioned table ("This feature is not yet supported"), so adding
// one validates against every partition, and validation holds SHARE ROW
// EXCLUSIVE, which blocks writes. On the adopted heap that is a multi-minute
// ingestion outage in the phase after a swap that took milliseconds.
//
// What that gives up, on an instance whose events table is still an ordinary
// table, is enforcement on UPDATE and protection against deleting a referenced
// event; the trigger only fires BEFORE INSERT. That is the same enforcement any
// instance with a partitioned events table already runs with, which is the state
// every instance converges to, and history keeps the stronger guarantee: the
// adopted heap carries its own validated foreign key, which attaching does not
// remove.
func (s *Service) restoreEventEnforcement(ctx context.Context) error {
	// Created rather than assumed: the function ships inside the events
	// conversion, so on an instance that never partitioned events it does not
	// exist, and the trigger below would fail to install.
	if _, err := s.db.Exec(ctx, `
        CREATE OR REPLACE FUNCTION convoy.enforce_event_fk()
            RETURNS TRIGGER AS $$
        BEGIN
            IF NOT EXISTS (
                SELECT 1
                FROM convoy.events
                WHERE id = NEW.event_id
            ) THEN
                RAISE EXCEPTION 'Foreign key violation: event_id % does not exist in events', NEW.event_id;
            END IF;
            RETURN NEW;
        END;
        $$ LANGUAGE plpgsql;

        CREATE OR REPLACE TRIGGER event_fk_check
            BEFORE INSERT ON convoy.event_deliveries
            FOR EACH ROW EXECUTE FUNCTION convoy.enforce_event_fk();`); err != nil {
		return fmt.Errorf("table is partitioned but event id enforcement was not restored: %w", err)
	}
	return nil
}

// createForwardPartitions builds a daily partition per project from the cutoff
// forward. Deliberately after the swap commits: the adopted heap accepts
// everything before the cutoff, so nothing needs these until the cutoff passes,
// and creating thousands of partitions inside the exclusive lock would make the
// swap proportional to the number of projects.
//
// Each creation would normally scan the default partition to prove no row in it
// belongs to the new range. The validated bounds constraint refutes that, so the
// scan is skipped and this stays fast however large the heap is.
func (s *Service) createForwardPartitions(ctx context.Context, bound string) error {
	if _, err := s.db.Exec(ctx, fmt.Sprintf(`
        DO $$
        DECLARE
            d        DATE;
            proj     TEXT;
            name     TEXT;
            from_day DATE := '%s'::TIMESTAMPTZ::DATE;
        BEGIN
            FOR proj IN SELECT id FROM convoy.projects WHERE deleted_at IS NULL LOOP
                FOR d IN SELECT generate_series(from_day, from_day + %d, '1 day')::DATE
                LOOP
                    -- Retention only adopts children named
                    -- <parent>_<TENANT>_<YYYYMMDD> with TENANT in [A-Z0-9]. A
                    -- folded or lower-case id parses as part of the parent name,
                    -- drifts against the bound, and is never dropped.
                    name := 'event_deliveries_'
                            || pg_catalog.UPPER(pg_catalog.REPLACE(proj, '-', ''))
                            || '_' || pg_catalog.REPLACE(d::TEXT, '-', '');
                    EXECUTE FORMAT(
                        'CREATE TABLE IF NOT EXISTS convoy.%%I PARTITION OF convoy.event_deliveries FOR VALUES FROM (%%L, %%L) TO (%%L, %%L)',
                        name, proj, d, proj, d + 1
                    );
                END LOOP;
            END LOOP;
        END $$;`, bound, attachPremakeDays)); err != nil {
		return fmt.Errorf("table is partitioned but forward partitions were not created, writes will fail after the cutoff: %w", err)
	}
	return nil
}

// notice reports which phase the conversion has reached. The conversion is
// driven from here rather than by one statement, so the phases are real
// committed boundaries; raising them as notices is what carries them to the
// partition run record and to the CLI, which both already read this stream.
//
// Progress reporting must never fail a conversion, so the error is dropped.
func (s *Service) notice(ctx context.Context, message string) {
	_, _ = s.db.Exec(ctx, fmt.Sprintf(`DO $$ BEGIN RAISE NOTICE '%s'; END $$;`,
		strings.ReplaceAll(message, "'", "''")))
}

// createPartitionedTableSQL builds the parent from the adopted table rather than
// from a column list.
//
// ATTACH requires the two to agree on every column, so a literal list is a copy
// of the schema that has to be updated by whoever adds the next column, and the
// failure if they do not is a mismatch raised at the swap, after the long
// validation phase has already run. LIKE cannot drift.
//
// What LIKE does not copy is foreign keys, which are named here. They are
// declared with the parent, while it still has no partitions, so they cost
// nothing to add; adding them afterwards would validate against the adopted
// table. Constraints are excluded deliberately: the only one on the source is
// the bounds CHECK, which belongs to history and would reject every future row
// if the parent handed it to new partitions.
const createPartitionedTableSQL = `
CREATE TABLE convoy.event_deliveries
(
    LIKE convoy.event_deliveries_default
        INCLUDING DEFAULTS
        INCLUDING GENERATED
        INCLUDING STORAGE
        INCLUDING COMPRESSION
        INCLUDING COMMENTS,
    FOREIGN KEY (project_id) REFERENCES convoy.projects,
    FOREIGN KEY (endpoint_id) REFERENCES convoy.endpoints,
    FOREIGN KEY (device_id) REFERENCES convoy.devices,
    FOREIGN KEY (subscription_id) REFERENCES convoy.subscriptions,
    PRIMARY KEY (id, created_at, project_id)
) PARTITION BY RANGE (project_id, created_at)`
