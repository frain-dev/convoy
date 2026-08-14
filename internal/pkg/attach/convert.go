package attach

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type adoptedIndex struct {
	name string
	def  string
}

type rowQuerier interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}

// Convert adopts the existing heap as the parent's DEFAULT partition.
func Convert(ctx context.Context, db *pgxpool.Pool, spec Spec) error {
	bound := Cutoff(time.Now()).Format(time.RFC3339)

	if err := preflight(ctx, db, spec); err != nil {
		return err
	}

	for _, stmt := range spec.Prepare {
		if _, err := db.Exec(ctx, stmt); err != nil {
			return fmt.Errorf("preparing convoy.%s: %w", spec.Table, err)
		}
	}

	notice(ctx, db, "Building the partitioned primary key...")
	if _, err := db.Exec(ctx, fmt.Sprintf(`
        CREATE UNIQUE INDEX CONCURRENTLY IF NOT EXISTS %s
            ON convoy.%s (id, created_at, project_id)`,
		quoteIdent(spec.pkIndex()), quoteIdent(spec.Table))); err != nil {
		return fmt.Errorf("building partitioned primary key: %w", err)
	}

	notice(ctx, db, "Declaring the range the existing table covers...")
	if err := declareBounds(ctx, db, spec, bound); err != nil {
		return err
	}

	notice(ctx, db, "Proving it (this is the long phase, writes continue)...")
	if _, err := db.Exec(ctx, fmt.Sprintf(`
        ALTER TABLE convoy.%s VALIDATE CONSTRAINT %s`,
		quoteIdent(spec.Table), quoteIdent(spec.bounds()))); err != nil {
		if spec.ValidateHint != "" {
			return fmt.Errorf("validating history bounds: %w. %s", err, spec.ValidateHint)
		}
		return fmt.Errorf("validating history bounds: %w", err)
	}

	notice(ctx, db, "Swapping in the partitioned table...")
	if err := swap(ctx, db, spec); err != nil {
		return err
	}

	// Enforcement has to land before createForwardPartitions. That step is the
	// one that can fail after the table is already partitioned; AfterAttach is
	// a trigger/function and does not depend on the new children existing.
	if len(spec.AfterAttach) > 0 {
		notice(ctx, db, "Restoring enforcement...")
		for _, stmt := range spec.AfterAttach {
			if _, err := db.Exec(ctx, stmt); err != nil {
				return fmt.Errorf("table is partitioned but a follow-up step failed: %w", err)
			}
		}
	}

	notice(ctx, db, "Creating partitions...")
	if err := createForwardPartitions(ctx, db, spec, bound); err != nil {
		return err
	}

	notice(ctx, db, "Migration complete!")
	return nil
}

func preflight(ctx context.Context, db *pgxpool.Pool, spec Spec) error {
	notice(ctx, db, "Checking the table can be converted...")

	table := spec.Table
	_, err := db.Exec(ctx, fmt.Sprintf(`
        DO $$
        DECLARE
            kind CHAR;
        BEGIN
            SELECT c.relkind INTO kind
            FROM pg_class c JOIN pg_namespace n ON n.oid = c.relnamespace
            WHERE n.nspname = 'convoy' AND c.relname = %s;

            IF kind IS NULL THEN
                RAISE EXCEPTION 'convoy.%s does not exist';
            END IF;

            IF kind = 'p' THEN
                RAISE EXCEPTION 'convoy.%s is already partitioned';
            END IF;

            IF EXISTS (SELECT 1 FROM pg_class c JOIN pg_namespace n ON n.oid = c.relnamespace
                       WHERE n.nspname = 'convoy' AND c.relname = %s) THEN
                RAISE EXCEPTION
                    'convoy.%s_new exists: a copy-based conversion died partway. Drop it before converting';
            END IF;

            IF EXISTS (SELECT 1 FROM pg_class c JOIN pg_namespace n ON n.oid = c.relnamespace
                       WHERE n.nspname = 'convoy' AND c.relname = %s) THEN
                RAISE EXCEPTION
                    'convoy.%s_default exists: an attach-based conversion died partway. Rename it back before converting';
            END IF;

            IF EXISTS (
                SELECT 1 FROM pg_class c
                JOIN pg_namespace n ON n.oid = c.relnamespace
                JOIN pg_index i ON i.indexrelid = c.oid
                WHERE n.nspname = 'convoy' AND c.relname = %s AND NOT i.indisvalid
            ) THEN
                DROP INDEX convoy.%s;
            END IF;
        END $$;`,
		pgLit(table), table, table,
		pgLit(table+"_new"), table,
		pgLit(spec.defaultName()), table,
		pgLit(spec.pkIndex()), quoteIdent(spec.pkIndex())))
	if err != nil {
		return fmt.Errorf("preflight: %w", err)
	}

	var invalid []string
	rows, err := db.Query(ctx, `
        SELECT c.relname
        FROM pg_index i
        JOIN pg_class c ON c.oid = i.indexrelid
        JOIN pg_class t ON t.oid = i.indrelid
        JOIN pg_namespace n ON n.oid = t.relnamespace
        WHERE n.nspname = 'convoy' AND t.relname = $1 AND NOT i.indisvalid
        ORDER BY c.relname`, table)
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
		return fmt.Errorf("convoy.%s has invalid indexes: %s. Rebuild them with REINDEX INDEX CONCURRENTLY, or drop them, before converting",
			table, strings.Join(invalid, ", "))
	}
	return nil
}

func adoptedIndexes(ctx context.Context, q rowQuerier, spec Spec) ([]adoptedIndex, error) {
	rows, err := q.Query(ctx, `
        SELECT c.relname, pg_get_indexdef(i.indexrelid)
        FROM pg_index i
        JOIN pg_class c ON c.oid = i.indexrelid
        JOIN pg_class t ON t.oid = i.indrelid
        JOIN pg_namespace n ON n.oid = t.relnamespace
        WHERE n.nspname = 'convoy'
          AND t.relname = $1
          AND i.indisvalid
          AND NOT i.indisprimary
          AND c.relname <> $2
          AND (NOT i.indisunique OR (
                SELECT ARRAY['project_id', 'created_at']::NAME[] <@ COALESCE(array_agg(a.attname), '{}')
                FROM pg_attribute a
                WHERE a.attrelid = t.oid AND a.attnum = ANY (i.indkey)
          ))
        ORDER BY c.relname`, spec.Table, spec.pkIndex())
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

func declareBounds(ctx context.Context, db *pgxpool.Pool, spec Spec, bound string) error {
	tx, err := db.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err = tx.Exec(ctx, `SET LOCAL lock_timeout = '`+lockTimeout+`'`); err != nil {
		return err
	}

	if _, err = tx.Exec(ctx, fmt.Sprintf(
		`ALTER TABLE convoy.%s DROP CONSTRAINT IF EXISTS %s`,
		quoteIdent(spec.Table), quoteIdent(spec.bounds()))); err != nil {
		return fmt.Errorf("clearing history bounds: %w", err)
	}

	if _, err = tx.Exec(ctx, fmt.Sprintf(`
        ALTER TABLE convoy.%s
            ADD CONSTRAINT %s
            CHECK (%s) NOT VALID`,
		quoteIdent(spec.Table), quoteIdent(spec.bounds()), boundsExpr(spec.ExtraNotNull, bound))); err != nil {
		return fmt.Errorf("declaring history bounds: %w", err)
	}

	return tx.Commit(ctx)
}

func boundsExpr(extra []string, bound string) string {
	parts := make([]string, 0, 2+len(extra))
	parts = append(parts, "created_at IS NOT NULL")
	for _, col := range extra {
		parts = append(parts, quoteIdent(col)+" IS NOT NULL")
	}
	parts = append(parts, "created_at < '"+bound+"'::TIMESTAMPTZ")
	return strings.Join(parts, " AND ")
}

func swap(ctx context.Context, db *pgxpool.Pool, spec Spec) error {
	tx, err := db.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	indexes, err := adoptedIndexes(ctx, tx, spec)
	if err != nil {
		return err
	}
	attachIndexes, err := attachIndexStatements(spec.Table, indexes)
	if err != nil {
		return err
	}

	notNull := []string{`ALTER COLUMN created_at SET NOT NULL`}
	for _, col := range spec.ExtraNotNull {
		notNull = append(notNull, fmt.Sprintf(`ALTER COLUMN %s SET NOT NULL`, quoteIdent(col)))
	}

	fks := spec.ParentForeignKeys
	if fks != "" {
		fks += ","
	}

	statements := append([]string{
		`SET LOCAL lock_timeout = '` + lockTimeout + `'`,
		fmt.Sprintf(`ALTER TABLE convoy.%s RENAME TO %s`, quoteIdent(spec.Table), quoteIdent(spec.defaultName())),
		fmt.Sprintf(`ALTER TABLE convoy.%s %s`, quoteIdent(spec.defaultName()), strings.Join(notNull, ", ")),
	}, spec.Swap...)

	statements = append(statements,
		dropExistingPrimaryKeySQL(spec.defaultName()),
		fmt.Sprintf(`ALTER TABLE convoy.%s ADD CONSTRAINT %s PRIMARY KEY USING INDEX %s`,
			quoteIdent(spec.defaultName()), quoteIdent(spec.defaultName()+"_pkey"), quoteIdent(spec.pkIndex())),
		fmt.Sprintf(`
CREATE TABLE convoy.%s
(
    LIKE convoy.%s
        INCLUDING DEFAULTS
        INCLUDING GENERATED
        INCLUDING STORAGE
        INCLUDING COMPRESSION
        INCLUDING COMMENTS,
    %s
    PRIMARY KEY (id, created_at, project_id)
) PARTITION BY RANGE (project_id, created_at)`,
			quoteIdent(spec.Table), quoteIdent(spec.defaultName()), fks),
		fmt.Sprintf(`ALTER TABLE convoy.%s ATTACH PARTITION convoy.%s DEFAULT`,
			quoteIdent(spec.Table), quoteIdent(spec.defaultName())),
	)
	statements = append(statements, attachIndexes...)
	statements = append(statements, spec.SwapEnd...)

	for _, statement := range statements {
		if _, err = tx.Exec(ctx, statement); err != nil {
			return fmt.Errorf("swapping in the partitioned table: %w", err)
		}
	}
	return tx.Commit(ctx)
}

func attachIndexStatements(table string, indexes []adoptedIndex) ([]string, error) {
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

		child := table + "_default_idx" + strconv.Itoa(i)
		statements = append(statements,
			fmt.Sprintf(`ALTER INDEX convoy.%s RENAME TO %s`, quoteIdent(index.name), quoteIdent(child)),
			fmt.Sprintf(`CREATE %sINDEX %s ON ONLY convoy.%s%s`, unique, quoteIdent(index.name), quoteIdent(table), index.def[shape:]),
			fmt.Sprintf(`ALTER INDEX convoy.%s ATTACH PARTITION convoy.%s`, quoteIdent(index.name), quoteIdent(child)),
		)
	}
	return statements, nil
}

func createForwardPartitions(ctx context.Context, db *pgxpool.Pool, spec Spec, bound string) error {
	if _, err := db.Exec(ctx, fmt.Sprintf(`
        DO $$
        DECLARE
            d        DATE;
            proj     TEXT;
            name     TEXT;
            from_day DATE := ('%s'::TIMESTAMPTZ AT TIME ZONE 'UTC')::DATE;
        BEGIN
            FOR proj IN SELECT id FROM convoy.projects WHERE deleted_at IS NULL LOOP
                FOR d IN SELECT generate_series(from_day, from_day + %d, '1 day')::DATE
                LOOP
                    name := %s
                            || pg_catalog.UPPER(pg_catalog.REPLACE(proj, '-', ''))
                            || '_' || pg_catalog.REPLACE(d::TEXT, '-', '');
                    EXECUTE FORMAT(
                        'CREATE TABLE IF NOT EXISTS convoy.%%I PARTITION OF convoy.%s FOR VALUES FROM (%%L, %%L) TO (%%L, %%L)',
                        name, proj, d::timestamp AT TIME ZONE 'UTC', proj, (d + 1)::timestamp AT TIME ZONE 'UTC'
                    );
                END LOOP;
            END LOOP;
        END $$;`, bound, PremakeDays, pgLit(spec.Table+"_"), spec.Table)); err != nil {
		return fmt.Errorf("table is partitioned but forward partitions were not created, writes will fail after the cutoff: %w", err)
	}
	return nil
}

func dropInvalidIndex(ctx context.Context, db *pgxpool.Pool, name string) error {
	_, err := db.Exec(ctx, fmt.Sprintf(`
        DO $$
        BEGIN
            IF EXISTS (
                SELECT 1 FROM pg_class c
                JOIN pg_namespace n ON n.oid = c.relnamespace
                JOIN pg_index i ON i.indexrelid = c.oid
                WHERE n.nspname = 'convoy' AND c.relname = %s AND NOT i.indisvalid
            ) THEN
                DROP INDEX convoy.%s;
            END IF;
        END $$;`, pgLit(name), quoteIdent(name)))
	if err != nil {
		return fmt.Errorf("dropping invalid index %s: %w", name, err)
	}
	return nil
}

// dropExistingPrimaryKeySQL drops whatever primary key the heap still carries.
// Copy-unpartition creates {table}_new with PRIMARY KEY, then renames the table;
// RENAME TABLE does not rename the constraint, so the leftover name is
// {table}_new_pkey, not {table}_pkey. Dropping only the latter leaves two
// primary keys when swap promotes {table}_pk_part.
func dropExistingPrimaryKeySQL(relname string) string {
	return fmt.Sprintf(`
DO $drop_pk$
DECLARE
    pk TEXT;
BEGIN
    SELECT con.conname INTO pk
    FROM pg_constraint con
    JOIN pg_class c ON c.oid = con.conrelid
    JOIN pg_namespace n ON n.oid = c.relnamespace
    WHERE n.nspname = 'convoy'
      AND c.relname = %s
      AND con.contype = 'p';
    IF pk IS NOT NULL THEN
        EXECUTE format('ALTER TABLE convoy.%s DROP CONSTRAINT %%I', pk);
    END IF;
END
$drop_pk$`, pgLit(relname), quoteIdent(relname))
}

func notice(ctx context.Context, db *pgxpool.Pool, message string) {
	_, _ = db.Exec(ctx, fmt.Sprintf(`DO $$ BEGIN RAISE NOTICE '%s'; END $$;`,
		strings.ReplaceAll(message, "'", "''")))
}

func pgLit(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}
