package event_deliveries

import (
	"context"
	"fmt"
)

// Unpartitioning is the escape hatch, and the copy path makes it one that costs
// as much as the problem it is escaping: it rewrites every row into a new table
// and drops the source, so it has the same lock profile and the same lost-write
// window as the copy-based conversion this package replaced.
//
// A table converted by attaching can be unconverted by detaching, and then the
// only rows that move are the ones written since the conversion. Minutes after a
// conversion someone regrets, that is nearly nothing, which is exactly when this
// gets reached for. Months later it converges on the cost of the copy path.
//
// This applies only to the attach layout. A table converted by the copy path has
// no adopted heap to rename back, so it still unconverts by copying.
func (s *Service) UnPartitionEventDeliveriesTable(ctx context.Context) error {
	adopted, err := s.hasAdoptedTable(ctx)
	if err != nil {
		return err
	}

	if !adopted {
		_, err = s.db.Exec(ctx, unPartitionEventDeliveriesTableSQL)
		return err
	}

	return s.detachEventDeliveries(ctx)
}

// hasAdoptedTable reports whether convoy.event_deliveries was converted by
// attaching, which is what decides how it can be unconverted.
//
// The marker is the bounds constraint rather than the presence of a partition
// named event_deliveries_default, because a default partition alone does not
// mean the table was attached: gopartman provisions its own default on tables it
// manages, and that one is an empty catch-all rather than the original table.
// Detaching and renaming that back would strand every row.
//
// Anything unrecognised routes to the copy path, which works on any layout.
func (s *Service) hasAdoptedTable(ctx context.Context) (bool, error) {
	var adopted bool
	err := s.db.QueryRow(ctx, `
        SELECT EXISTS (
            SELECT 1
            FROM pg_constraint con
            JOIN pg_class c ON c.oid = con.conrelid
            JOIN pg_namespace n ON n.oid = c.relnamespace
            WHERE n.nspname = 'convoy'
              AND c.relname = 'event_deliveries_default'
              AND con.conname = 'event_deliveries_default_bounds'
        )`).Scan(&adopted)
	if err != nil {
		return false, fmt.Errorf("checking how the table was converted: %w", err)
	}
	return adopted, nil
}

func (s *Service) detachEventDeliveries(ctx context.Context) error {
	// Read first: detaching the partition also detaches its indexes from the
	// parent's, and dropping the parent takes the canonical names with it, so
	// after either step there is nothing left to say which index was which.
	names, err := s.adoptedIndexNames(ctx)
	if err != nil {
		return err
	}

	s.notice(ctx, "Rebuilding the unpartitioned primary key...")
	// Built on the partition while it is still attached, and concurrently, so the
	// swap below does not have to. An ordinary table keys on id alone; the
	// partitioned form had to widen that to include the partition key columns.
	if _, err := s.db.Exec(ctx, `
        CREATE UNIQUE INDEX CONCURRENTLY IF NOT EXISTS event_deliveries_id_key
            ON convoy.event_deliveries_default (id)`); err != nil {
		return fmt.Errorf("rebuilding the unpartitioned primary key: %w", err)
	}

	s.notice(ctx, "Swapping the adopted table back...")
	if err := s.detachSwap(ctx); err != nil {
		return err
	}

	// From here the live table is an ordinary table again and taking writes.
	// Everything below repairs what the partitioned form still holds.
	s.notice(ctx, "Migrating rows written since the conversion...")
	if err := s.drainPartitioned(ctx); err != nil {
		return err
	}

	s.notice(ctx, "Restoring index names...")
	if err := s.restoreIndexNames(ctx, names); err != nil {
		return err
	}

	s.notice(ctx, "Restoring enforcement...")
	if err := s.restoreUnpartitionedEnforcement(ctx); err != nil {
		return err
	}

	s.notice(ctx, "Successfully un-partitioned event_deliveries table...")
	return nil
}

// drainPartitioned moves the rows written since the conversion onto the table
// that is now live, and drops what they came from.
//
// Copying and dropping are one transaction, and it locks before it reads. The
// copy path fails exactly here when it does not: it copies a snapshot, and rows
// written after that snapshot are dropped with the source. Running that on the
// lab instance under 3 req/s did not even lose them quietly, it aborted 28
// minutes in restoring the delivery_attempts foreign key, because an attempt
// referenced a delivery the copy had not seen.
//
// Renaming the parent aside already redirected writes, so this lock is
// uncontended in the ordinary case. What it covers is a straggler that resolved
// the name before the swap: it waits, then finds the table gone and fails, which
// is the behaviour to want over a row that was written, acknowledged, and then
// discarded.
func (s *Service) drainPartitioned(ctx context.Context) error {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err = tx.Exec(ctx,
		`LOCK TABLE convoy.event_deliveries_partitioned IN ACCESS EXCLUSIVE MODE`); err != nil {
		return fmt.Errorf("table is unpartitioned but rows written since the conversion are still in convoy.event_deliveries_partitioned: %w", err)
	}

	// Columns are listed rather than SELECT *. Attaching matches a partition to
	// its parent by column name, not position, so the two can disagree on
	// physical order, and a positional copy between them would line up columns of
	// the same type without failing.
	if _, err = tx.Exec(ctx, `
        INSERT INTO convoy.event_deliveries (
            id, status, description, project_id, created_at, updated_at, endpoint_id, event_id,
            device_id, subscription_id, metadata, headers, attempts, cli_metadata, deleted_at,
            target_url, url_query_params, idempotency_key, latency, event_type, acknowledged_at,
            latency_seconds, delivery_mode, event_bytes
        )
        SELECT id, status, description, project_id, created_at, updated_at, endpoint_id, event_id,
               device_id, subscription_id, metadata, headers, attempts, cli_metadata, deleted_at,
               target_url, url_query_params, idempotency_key, latency, event_type, acknowledged_at,
               latency_seconds, delivery_mode, event_bytes
        FROM convoy.event_deliveries_partitioned`); err != nil {
		return fmt.Errorf("table is unpartitioned but rows written since the conversion are still in convoy.event_deliveries_partitioned: %w", err)
	}

	// Drops every remaining partition with it, and only ever sees a table this
	// transaction has already emptied.
	if _, err = tx.Exec(ctx, `DROP TABLE convoy.event_deliveries_partitioned`); err != nil {
		return fmt.Errorf("table is unpartitioned but the empty partitioned table remains: %w", err)
	}

	return tx.Commit(ctx)
}

// detachSwap is the only phase holding ACCESS EXCLUSIVE, and it is catalog-only.
//
// The parent is renamed aside rather than emptied, so the cost does not depend
// on how many partitions it has: detaching every child individually would be
// thousands of statements under the lock on a busy instance. Only the adopted
// partition is detached here; the rest stay attached to the renamed parent and
// are drained afterwards.
func (s *Service) detachSwap(ctx context.Context) error {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	statements := []string{
		`SET LOCAL lock_timeout = '` + attachLockTimeout + `'`,

		`ALTER TABLE convoy.event_deliveries RENAME TO event_deliveries_partitioned`,
		`ALTER TABLE convoy.event_deliveries_partitioned DETACH PARTITION convoy.event_deliveries_default`,
		`ALTER TABLE convoy.event_deliveries_default RENAME TO event_deliveries`,

		// The constraint that made the adoption safe would now reject every write:
		// it caps created_at at the conversion's cutoff, which is in the past.
		`ALTER TABLE convoy.event_deliveries DROP CONSTRAINT event_deliveries_default_bounds`,

		// Back to keying on id alone. The wider key only existed because a
		// partitioned table's primary key must contain its partition key columns.
		`ALTER TABLE convoy.event_deliveries DROP CONSTRAINT event_deliveries_default_pkey`,

		// Renaming a table does not rename its indexes, so the parent renamed
		// aside above still holds the name the key below is about to take. It is
		// dropped with the parent shortly after; this only has to survive until
		// then.
		`ALTER INDEX IF EXISTS convoy.event_deliveries_pkey RENAME TO event_deliveries_partitioned_pkey`,
		`ALTER TABLE convoy.event_deliveries
            ADD CONSTRAINT event_deliveries_pkey PRIMARY KEY USING INDEX event_deliveries_id_key`,
	}

	for _, statement := range statements {
		if _, err = tx.Exec(ctx, statement); err != nil {
			return fmt.Errorf("swapping the adopted table back: %w", err)
		}
	}

	return tx.Commit(ctx)
}

// adoptedIndexNames pairs each index on the adopted partition with the canonical
// name its parent index holds. pg_inherits is the authority on that pairing: the
// conversion renamed the partition's copies to positional names precisely so it
// would not depend on a naming convention to find them again.
//
// The primary key is excluded. The swap replaces it rather than renaming it.
func (s *Service) adoptedIndexNames(ctx context.Context) (map[string]string, error) {
	rows, err := s.db.Query(ctx, `
        SELECT child.relname, parent.relname
        FROM pg_index i
        JOIN pg_class child ON child.oid = i.indexrelid
        JOIN pg_class t ON t.oid = i.indrelid
        JOIN pg_namespace n ON n.oid = t.relnamespace
        JOIN pg_inherits inh ON inh.inhrelid = child.oid
        JOIN pg_class parent ON parent.oid = inh.inhparent
        WHERE n.nspname = 'convoy'
          AND t.relname = 'event_deliveries_default'
          AND NOT i.indisprimary`)
	if err != nil {
		return nil, fmt.Errorf("reading the partition's index names: %w", err)
	}
	defer rows.Close()

	names := make(map[string]string)
	for rows.Next() {
		var child, parent string
		if err = rows.Scan(&child, &parent); err != nil {
			return nil, fmt.Errorf("reading the partition's index names: %w", err)
		}
		names[child] = parent
	}
	return names, rows.Err()
}

// restoreIndexNames gives the table back the index names it had before the
// conversion. It runs after the parent is dropped, because until then the parent
// still holds them.
func (s *Service) restoreIndexNames(ctx context.Context, names map[string]string) error {
	for child, canonical := range names {
		if _, err := s.db.Exec(ctx, fmt.Sprintf(
			`ALTER INDEX IF EXISTS convoy.%s RENAME TO %s`, child, canonical)); err != nil {
			return fmt.Errorf("table is unpartitioned but index %s kept its partitioned name %s: %w", canonical, child, err)
		}
	}
	return nil
}

// restoreUnpartitionedEnforcement puts back the constraints an ordinary table
// carries. Both foreign keys are added NOT VALID and then validated, which is
// available here and was not during conversion: NOT VALID is rejected on
// partitioned tables, but this is no longer one. Adding a key validated holds
// SHARE ROW EXCLUSIVE for the length of the scan and blocks writes; adding it
// NOT VALID enforces new rows immediately and moves the scan into VALIDATE,
// which takes SHARE UPDATE EXCLUSIVE and does not.
func (s *Service) restoreUnpartitionedEnforcement(ctx context.Context) error {
	// NOT VALID is only available while delivery_attempts is an ordinary table.
	// It may itself have been partitioned, and Postgres 16 rejects NOT VALID
	// foreign keys there, so that case adds the key validated and pays the
	// blocking scan the copy path has always paid.
	statements := []string{
		// The trigger stood in for this key while the parent was partitioned.
		`DROP TRIGGER IF EXISTS event_delivery_fk_check ON convoy.delivery_attempts`,
		`ALTER TABLE convoy.delivery_attempts DROP CONSTRAINT IF EXISTS delivery_attempts_event_delivery_id_fkey`,
		`DO $$
        BEGIN
            IF EXISTS (
                SELECT 1 FROM pg_catalog.pg_class c
                JOIN pg_catalog.pg_namespace n ON n.oid = c.relnamespace
                WHERE n.nspname = 'convoy' AND c.relname = 'delivery_attempts' AND c.relkind = 'p'
            ) THEN
                ALTER TABLE convoy.delivery_attempts
                    ADD CONSTRAINT delivery_attempts_event_delivery_id_fkey
                        FOREIGN KEY (event_delivery_id) REFERENCES convoy.event_deliveries (id);
            ELSE
                ALTER TABLE convoy.delivery_attempts
                    ADD CONSTRAINT delivery_attempts_event_delivery_id_fkey
                        FOREIGN KEY (event_delivery_id) REFERENCES convoy.event_deliveries (id) NOT VALID;
                ALTER TABLE convoy.delivery_attempts
                    VALIDATE CONSTRAINT delivery_attempts_event_delivery_id_fkey;
            END IF;
        END $$;`,
	}

	for _, statement := range statements {
		if _, err := s.db.Exec(ctx, statement); err != nil {
			return fmt.Errorf("table is unpartitioned but delivery attempt enforcement was not restored: %w", err)
		}
	}

	// Unpartitioning deliveries says nothing about convoy.events, which may still
	// be partitioned and can then only be enforced by the trigger. Where a real
	// key is possible it is preferred, because unlike the trigger it also covers
	// updates and blocks deleting a referenced event.
	if _, err := s.db.Exec(ctx, `
        DO $$
        BEGIN
            IF EXISTS (
                SELECT 1 FROM pg_catalog.pg_class c
                JOIN pg_catalog.pg_namespace n ON n.oid = c.relnamespace
                WHERE n.nspname = 'convoy' AND c.relname = 'events' AND c.relkind = 'p'
            ) THEN
                CREATE OR REPLACE TRIGGER event_fk_check
                    BEFORE INSERT ON convoy.event_deliveries
                    FOR EACH ROW EXECUTE FUNCTION convoy.enforce_event_fk();
            ELSE
                DROP TRIGGER IF EXISTS event_fk_check ON convoy.event_deliveries;

                -- Usually already present. Attaching a table as a partition does
                -- not remove its foreign keys, so a table converted by attaching
                -- comes back still carrying the one it had, validated over all of
                -- history. Adding it again is an error, and replacing it would
                -- mean rescanning the table to prove what it already proved.
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
        END $$;`); err != nil {
		return fmt.Errorf("table is unpartitioned but event id enforcement was not restored: %w", err)
	}

	return nil
}
