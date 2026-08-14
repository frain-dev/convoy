package attach

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Revert detaches an attach-converted table. Copy-converted tables still go
// through CopyUnpartition, which is the only path that can unconvert them.
func Revert(ctx context.Context, db *pgxpool.Pool, spec Spec) error {
	adopted, err := Adopted(ctx, db, spec.Table)
	if err != nil {
		return err
	}
	if !adopted {
		if _, err = db.Exec(ctx, spec.CopyUnpartition); err != nil {
			return err
		}
		// Copy SQL drops inbound FKs and appends the stand-in so the
		// rewrite cannot commit without enforcement. DuringDetach is
		// the same SQL again (CREATE OR REPLACE). AfterDetach then
		// upgrades to a real FK when both sides are heaps.
		if err := installDuringDetach(ctx, db, spec); err != nil {
			return err
		}
		return restoreAfterDetach(ctx, db, spec)
	}
	return detach(ctx, db, spec)
}

// Adopted reports whether the table was converted by attaching, via the bounds
// constraint on <table>_default. A default partition alone is not enough:
// gopartman provisions its own empty catch-all under the same name.
func Adopted(ctx context.Context, db *pgxpool.Pool, table string) (bool, error) {
	var adopted bool
	err := db.QueryRow(ctx, `
        SELECT EXISTS (
            SELECT 1
            FROM pg_constraint con
            JOIN pg_class c ON c.oid = con.conrelid
            JOIN pg_namespace n ON n.oid = c.relnamespace
            WHERE n.nspname = 'convoy'
              AND c.relname = $1
              AND con.conname = $2
        )`, table+"_default", table+"_default_bounds").Scan(&adopted)
	if err != nil {
		return false, fmt.Errorf("checking how the table was converted: %w", err)
	}
	return adopted, nil
}

func detach(ctx context.Context, db *pgxpool.Pool, spec Spec) error {
	names, err := adoptedIndexNames(ctx, db, spec)
	if err != nil {
		return err
	}

	notice(ctx, db, "Rebuilding the unpartitioned primary key...")
	if err := dropInvalidIndex(ctx, db, spec.idIndex()); err != nil {
		return err
	}
	if _, err := db.Exec(ctx, fmt.Sprintf(`
        CREATE UNIQUE INDEX CONCURRENTLY IF NOT EXISTS %s
            ON convoy.%s (id)`,
		quoteIdent(spec.idIndex()), quoteIdent(spec.defaultName()))); err != nil {
		return fmt.Errorf("rebuilding the unpartitioned primary key: %w", err)
	}

	notice(ctx, db, "Swapping the adopted table back...")
	if err := detachSwap(ctx, db, spec); err != nil {
		return err
	}

	// Swap renames the partitioned parent to _partitioned and takes its
	// stand-in triggers with it. AfterDetach restores a real FK only after
	// drain, when both names hold the full row set. Reinstall the stand-in
	// on the live name now, or the drain window writes orphans and child
	// lookups miss rows that still live only on _partitioned.
	if err := installDuringDetach(ctx, db, spec); err != nil {
		return err
	}

	notice(ctx, db, "Migrating rows written since the conversion...")
	if err := drainPartitioned(ctx, db, spec); err != nil {
		return err
	}

	notice(ctx, db, "Restoring index names...")
	if err := restoreIndexNames(ctx, db, names); err != nil {
		return err
	}

	if err := restoreAfterDetach(ctx, db, spec); err != nil {
		return err
	}

	notice(ctx, db, "Successfully un-partitioned "+spec.Table+" table...")
	return nil
}

func installDuringDetach(ctx context.Context, db *pgxpool.Pool, spec Spec) error {
	if len(spec.DuringDetach) == 0 {
		return nil
	}
	notice(ctx, db, "Keeping enforcement...")
	for _, stmt := range spec.DuringDetach {
		if _, err := db.Exec(ctx, stmt); err != nil {
			return fmt.Errorf("table is unpartitioned but enforcement was not restored: %w", err)
		}
	}
	return nil
}

func restoreAfterDetach(ctx context.Context, db *pgxpool.Pool, spec Spec) error {
	if len(spec.AfterDetach) == 0 {
		return nil
	}
	notice(ctx, db, "Restoring enforcement...")
	for _, stmt := range spec.AfterDetach {
		if _, err := db.Exec(ctx, stmt); err != nil {
			return fmt.Errorf("table is unpartitioned but enforcement was not restored: %w", err)
		}
	}
	return nil
}

func drainPartitioned(ctx context.Context, db *pgxpool.Pool, spec Spec) error {
	tx, err := db.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err = tx.Exec(ctx, fmt.Sprintf(
		`LOCK TABLE convoy.%s IN ACCESS EXCLUSIVE MODE`, quoteIdent(spec.partitioned()))); err != nil {
		return fmt.Errorf("table is unpartitioned but rows written since the conversion are still in convoy.%s: %w", spec.partitioned(), err)
	}

	cols, err := insertableColumns(ctx, tx, spec.Table)
	if err != nil {
		return err
	}
	list := strings.Join(cols, ", ")

	if _, err = tx.Exec(ctx, fmt.Sprintf(`
        INSERT INTO convoy.%s (%s)
        SELECT %s FROM convoy.%s`,
		quoteIdent(spec.Table), list, list, quoteIdent(spec.partitioned()))); err != nil {
		return fmt.Errorf("table is unpartitioned but rows written since the conversion are still in convoy.%s: %w", spec.partitioned(), err)
	}

	if _, err = tx.Exec(ctx, fmt.Sprintf(`DROP TABLE convoy.%s`, quoteIdent(spec.partitioned()))); err != nil {
		return fmt.Errorf("table is unpartitioned but the empty partitioned table remains: %w", err)
	}

	return tx.Commit(ctx)
}

func insertableColumns(ctx context.Context, q rowQuerier, table string) ([]string, error) {
	rows, err := q.Query(ctx, `
        SELECT a.attname
        FROM pg_attribute a
        JOIN pg_class c ON c.oid = a.attrelid
        JOIN pg_namespace n ON n.oid = c.relnamespace
        WHERE n.nspname = 'convoy' AND c.relname = $1
          AND a.attnum > 0 AND NOT a.attisdropped AND a.attgenerated = ''
        ORDER BY a.attnum`, table)
	if err != nil {
		return nil, fmt.Errorf("reading columns to drain: %w", err)
	}
	defer rows.Close()

	var cols []string
	for rows.Next() {
		var name string
		if err = rows.Scan(&name); err != nil {
			return nil, fmt.Errorf("reading columns to drain: %w", err)
		}
		cols = append(cols, quoteIdent(name))
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(cols) == 0 {
		return nil, fmt.Errorf("convoy.%s has no insertable columns", table)
	}
	return cols, nil
}

func detachSwap(ctx context.Context, db *pgxpool.Pool, spec Spec) error {
	tx, err := db.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	statements := []string{
		`SET LOCAL lock_timeout = '` + lockTimeout + `'`,
		fmt.Sprintf(`ALTER TABLE convoy.%s RENAME TO %s`, quoteIdent(spec.Table), quoteIdent(spec.partitioned())),
		fmt.Sprintf(`ALTER TABLE convoy.%s DETACH PARTITION convoy.%s`, quoteIdent(spec.partitioned()), quoteIdent(spec.defaultName())),
		fmt.Sprintf(`ALTER TABLE convoy.%s RENAME TO %s`, quoteIdent(spec.defaultName()), quoteIdent(spec.Table)),
		fmt.Sprintf(`ALTER TABLE convoy.%s DROP CONSTRAINT %s`, quoteIdent(spec.Table), quoteIdent(spec.bounds())),
		fmt.Sprintf(`ALTER TABLE convoy.%s DROP CONSTRAINT %s`, quoteIdent(spec.Table), quoteIdent(spec.defaultName()+"_pkey")),
		fmt.Sprintf(`ALTER INDEX IF EXISTS convoy.%s RENAME TO %s`,
			quoteIdent(spec.Table+"_pkey"), quoteIdent(spec.partitioned()+"_pkey")),
		fmt.Sprintf(`ALTER TABLE convoy.%s ADD CONSTRAINT %s PRIMARY KEY USING INDEX %s`,
			quoteIdent(spec.Table), quoteIdent(spec.Table+"_pkey"), quoteIdent(spec.idIndex())),
	}

	for _, statement := range statements {
		if _, err = tx.Exec(ctx, statement); err != nil {
			return fmt.Errorf("swapping the adopted table back: %w", err)
		}
	}
	return tx.Commit(ctx)
}

func adoptedIndexNames(ctx context.Context, db *pgxpool.Pool, spec Spec) (map[string]string, error) {
	rows, err := db.Query(ctx, `
        SELECT child.relname, parent.relname
        FROM pg_index i
        JOIN pg_class child ON child.oid = i.indexrelid
        JOIN pg_class t ON t.oid = i.indrelid
        JOIN pg_namespace n ON n.oid = t.relnamespace
        JOIN pg_inherits inh ON inh.inhrelid = child.oid
        JOIN pg_class parent ON parent.oid = inh.inhparent
        WHERE n.nspname = 'convoy'
          AND t.relname = $1
          AND NOT i.indisprimary`, spec.defaultName())
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

func restoreIndexNames(ctx context.Context, db *pgxpool.Pool, names map[string]string) error {
	for child, canonical := range names {
		if _, err := db.Exec(ctx, fmt.Sprintf(
			`ALTER INDEX IF EXISTS convoy.%s RENAME TO %s`, quoteIdent(child), quoteIdent(canonical))); err != nil {
			return fmt.Errorf("table is unpartitioned but index %s kept its partitioned name %s: %w", canonical, child, err)
		}
	}
	return nil
}
