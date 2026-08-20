package indexes

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	"github.com/frain-dev/convoy/database/postgres"
	"github.com/frain-dev/convoy/testenv"
)

var testEnv *testenv.Environment

func TestMain(m *testing.M) {
	res, cleanup, err := testenv.Launch(context.Background())
	if err != nil {
		fmt.Printf("Failed to launch test environment: %v\n", err)
		os.Exit(1)
	}

	testEnv = res

	code := m.Run()

	if err := cleanup(); err != nil {
		fmt.Printf("Failed to cleanup test infrastructure: %v\n", err)
	}

	os.Exit(code)
}

// The repair reads and writes the catalog and one table, so it needs a migrated
// database and nothing else from the application's config.
func setupDB(t *testing.T) *pgxpool.Pool {
	t.Helper()

	conn, err := testEnv.CloneTestDatabase(t, "convoy")
	require.NoError(t, err)

	return postgres.NewFromConnection(conn).GetConn()
}

// A unique build over duplicate rows is how an invalid index is produced here.
// It fails the way an interrupted build does, leaving the index in the catalog
// marked invalid, and it does so without writing to the catalog by hand, so the
// test exercises the state Postgres actually leaves behind.
func TestInvalidIndexIsReportedDroppedAndRebuilt(t *testing.T) {
	db := setupDB(t)
	ctx := context.Background()

	_, err := db.Exec(ctx, `
        CREATE TABLE convoy.idx_test_heap (project_id TEXT NOT NULL, created_at TIMESTAMPTZ NOT NULL DEFAULT NOW())`)
	require.NoError(t, err)

	_, err = db.Exec(ctx, `INSERT INTO convoy.idx_test_heap (project_id) VALUES ('dup'), ('dup')`)
	require.NoError(t, err)

	_, err = db.Exec(ctx, `CREATE UNIQUE INDEX CONCURRENTLY idx_test_heap_project ON convoy.idx_test_heap (project_id)`)
	require.Error(t, err, "the build has to fail for there to be an invalid index to repair")

	invalid, err := ListInvalid(ctx, db)
	require.NoError(t, err)
	require.Contains(t, names(invalid), "idx_test_heap_project")

	// The build is over, so nothing should read as busy. Reporting it busy would
	// make the migration refuse to drop it.
	for _, i := range invalid {
		if i.Name == "idx_test_heap_project" {
			require.False(t, i.Busy, "a failed build is not a build in progress")
		}
	}

	var dropped bool
	require.NoError(t, db.QueryRow(ctx,
		`SELECT convoy.drop_invalid_index('idx_test_heap_project')`).Scan(&dropped))
	require.True(t, dropped)

	owed, err := ListDropped(ctx, db)
	require.NoError(t, err)
	require.Len(t, owed, 1)
	require.Equal(t, "idx_test_heap_project", owed[0].Name)
	require.Equal(t, "idx_test_heap", owed[0].Table)
	require.Contains(t, owed[0].Definition, "CREATE UNIQUE INDEX")

	// The definition is what the rebuild is built from, so it has to describe the
	// index that was dropped, not a plain one. While the duplicates are still
	// there a unique build cannot succeed, and the debt has to survive that.
	require.Error(t, Rebuild(ctx, db, owed[0]))
	owed, err = ListDropped(ctx, db)
	require.NoError(t, err)
	require.Len(t, owed, 1, "a rebuild that failed still owes the index")

	// The failed build left an invalid index behind under the same name. It is
	// one piece of work, and the debt row is the actionable one, so it must not
	// also be reported as an abandoned index to deal with separately.
	invalid, err = ListInvalid(ctx, db)
	require.NoError(t, err)
	require.NotContains(t, names(invalid), "idx_test_heap_project",
		"an index that already owes a rebuild is reported once, as debt")

	_, err = db.Exec(ctx, `DELETE FROM convoy.idx_test_heap WHERE ctid = (
        SELECT MIN(ctid) FROM convoy.idx_test_heap WHERE project_id = 'dup')`)
	require.NoError(t, err)

	require.NoError(t, Rebuild(ctx, db, owed[0]))
	require.True(t, valid(t, db, "idx_test_heap_project"))

	owed, err = ListDropped(ctx, db)
	require.NoError(t, err)
	require.Empty(t, owed, "a rebuilt index must not be offered again")
}

// GetDropped is what a caller-supplied index name has to pass before a rebuild is
// recorded, so it has to refuse a name that identifies no pending work. An
// unknown name and an already-rebuilt one are the same answer: there is nothing
// to rebuild, and the caller has nothing different to do about either.
func TestGetDroppedRefusesANameThatOwesNothing(t *testing.T) {
	db := setupDB(t)
	ctx := context.Background()

	_, err := GetDropped(ctx, db, "idx_never_existed")
	require.ErrorIs(t, err, ErrNotDropped)

	_, err = db.Exec(ctx, `
        CREATE TABLE convoy.idx_test_lookup (project_id TEXT NOT NULL)`)
	require.NoError(t, err)

	_, err = db.Exec(ctx, `INSERT INTO convoy.idx_test_lookup (project_id) VALUES ('dup'), ('dup')`)
	require.NoError(t, err)

	_, err = db.Exec(ctx, `CREATE UNIQUE INDEX CONCURRENTLY idx_test_lookup_project ON convoy.idx_test_lookup (project_id)`)
	require.Error(t, err, "the build has to fail for there to be an invalid index to repair")

	var dropped bool
	require.NoError(t, db.QueryRow(ctx,
		`SELECT convoy.drop_invalid_index('idx_test_lookup_project')`).Scan(&dropped))
	require.True(t, dropped)

	owed, err := GetDropped(ctx, db, "idx_test_lookup_project")
	require.NoError(t, err)
	require.Equal(t, "idx_test_lookup", owed.Table)
	require.True(t, owed.Unique())

	// Whitespace is trimmed by the caller, not here: the name is matched against
	// the recorded row exactly, so a padded name is a miss.
	_, err = GetDropped(ctx, db, " idx_test_lookup_project ")
	require.ErrorIs(t, err, ErrNotDropped)

	_, err = db.Exec(ctx, `DELETE FROM convoy.idx_test_lookup WHERE ctid = (
        SELECT MIN(ctid) FROM convoy.idx_test_lookup WHERE project_id = 'dup')`)
	require.NoError(t, err)
	require.NoError(t, Rebuild(ctx, db, owed))

	// Once rebuilt it owes nothing, so a second request for it must not start
	// hours of work on an index that is already valid.
	_, err = GetDropped(ctx, db, "idx_test_lookup_project")
	require.ErrorIs(t, err, ErrNotDropped)
}

// The index has to be rebuildable after the table is partitioned, which is the
// order the repair happens in: the drop unblocks the conversion, and the rebuild
// comes later, once retention has dropped the largest partition.
func TestRebuildOnPartitionedTableAttachesEveryPartition(t *testing.T) {
	db := setupDB(t)
	ctx := context.Background()

	_, err := db.Exec(ctx, `
        CREATE TABLE convoy.idx_test_part (project_id TEXT NOT NULL, created_at TIMESTAMPTZ NOT NULL)
            PARTITION BY RANGE (project_id, created_at)`)
	require.NoError(t, err)

	for _, day := range []struct{ suffix, from, to string }{
		{"19", "2026-08-19", "2026-08-20"},
		{"20", "2026-08-20", "2026-08-21"},
	} {
		_, err = db.Exec(ctx, fmt.Sprintf(`
            CREATE TABLE convoy.idx_test_part_%s PARTITION OF convoy.idx_test_part
                FOR VALUES FROM ('p1', '%s') TO ('p1', '%s')`, day.suffix, day.from, day.to))
		require.NoError(t, err)
	}

	_, err = db.Exec(ctx, `
        INSERT INTO convoy.idx_test_part (project_id, created_at)
        SELECT 'p1', '2026-08-19'::TIMESTAMPTZ FROM generate_series(1, 50)`)
	require.NoError(t, err)

	owed := Dropped{
		Table: "idx_test_part",
		Name:  "idx_test_part_created",
		Definition: `CREATE INDEX idx_test_part_created ON convoy.idx_test_part ` +
			`USING btree (project_id, created_at)`,
	}
	_, err = db.Exec(ctx, `
        INSERT INTO convoy.dropped_indexes (index_name, table_name, definition)
        VALUES ($1, $2, $3)`, owed.Name, owed.Table, owed.Definition)
	require.NoError(t, err)

	require.NoError(t, Rebuild(ctx, db, owed))

	// The parent index only turns valid once every partition has one attached, so
	// this single assertion covers the whole recipe.
	require.True(t, valid(t, db, owed.Name),
		"the parent index is invalid, so a partition was left uncovered")

	var attached int
	require.NoError(t, db.QueryRow(ctx, `
        SELECT COUNT(*)
          FROM pg_inherits h
          JOIN pg_class parent ON parent.oid = h.inhparent
         WHERE parent.relname = $1`, owed.Name).Scan(&attached))
	require.Equal(t, 2, attached)

	// A rebuild that is interrupted has to be safe to run again. Nothing is left
	// to cover, so this must neither fail nor duplicate an attach.
	require.NoError(t, Rebuild(ctx, db, owed))
	require.True(t, valid(t, db, owed.Name))
}

// A partition added after the rebuild leaves the parent invalid again, and the
// next run has to cover it rather than report success.
func TestRebuildCoversPartitionsAddedLater(t *testing.T) {
	db := setupDB(t)
	ctx := context.Background()

	_, err := db.Exec(ctx, `
        CREATE TABLE convoy.idx_test_late (project_id TEXT NOT NULL, created_at TIMESTAMPTZ NOT NULL)
            PARTITION BY RANGE (project_id, created_at)`)
	require.NoError(t, err)

	_, err = db.Exec(ctx, `
        CREATE TABLE convoy.idx_test_late_19 PARTITION OF convoy.idx_test_late
            FOR VALUES FROM ('p1', '2026-08-19') TO ('p1', '2026-08-20')`)
	require.NoError(t, err)

	owed := Dropped{
		Table:      "idx_test_late",
		Name:       "idx_test_late_created",
		Definition: `CREATE INDEX idx_test_late_created ON convoy.idx_test_late USING btree (project_id, created_at)`,
	}
	_, err = db.Exec(ctx, `
        INSERT INTO convoy.dropped_indexes (index_name, table_name, definition)
        VALUES ($1, $2, $3)`, owed.Name, owed.Table, owed.Definition)
	require.NoError(t, err)

	require.NoError(t, Rebuild(ctx, db, owed))

	_, err = db.Exec(ctx, `
        CREATE TABLE convoy.idx_test_late_20 PARTITION OF convoy.idx_test_late
            FOR VALUES FROM ('p1', '2026-08-20') TO ('p1', '2026-08-21')`)
	require.NoError(t, err)

	require.NoError(t, Rebuild(ctx, db, owed))
	require.True(t, valid(t, db, owed.Name))
}

// Every build statement carries IF NOT EXISTS so a rebuild can resume, which
// means Postgres says nothing when the name is already held by an index it
// declined to drop: a build in progress, or one a constraint owns. Without this
// check the rebuild would return success and mark the debt paid while the
// planner still ignores the index.
func TestARebuildIsNotDoneWhileTheIndexIsStillInvalid(t *testing.T) {
	db := setupDB(t)
	ctx := context.Background()

	_, err := db.Exec(ctx, `
        CREATE TABLE convoy.idx_test_guard (project_id TEXT NOT NULL)`)
	require.NoError(t, err)

	_, err = db.Exec(ctx, `INSERT INTO convoy.idx_test_guard (project_id) VALUES ('dup'), ('dup')`)
	require.NoError(t, err)

	_, err = db.Exec(ctx, `CREATE UNIQUE INDEX CONCURRENTLY idx_test_guard_project ON convoy.idx_test_guard (project_id)`)
	require.Error(t, err)

	conn, err := db.Acquire(ctx)
	require.NoError(t, err)
	defer conn.Release()

	require.ErrorContains(t, assertValid(ctx, conn, "idx_test_guard_project", "because"), "still invalid")
	require.ErrorContains(t, assertValid(ctx, conn, "idx_test_guard_absent", "because"), "not there")
}

// A partition's copy of an index has no meaning on its own. The rebuild drops an
// invalid one on its way past, and recording that would queue a name whose
// rebuild leaves the parent no closer to valid.
func TestAPartitionsCopyIsNotQueuedAsItsOwnDebt(t *testing.T) {
	db := setupDB(t)
	ctx := context.Background()

	_, err := db.Exec(ctx, `
        CREATE TABLE convoy.idx_test_copy (project_id TEXT NOT NULL, created_at TIMESTAMPTZ NOT NULL)
            PARTITION BY RANGE (project_id, created_at)`)
	require.NoError(t, err)

	_, err = db.Exec(ctx, `
        CREATE TABLE convoy.idx_test_copy_19 PARTITION OF convoy.idx_test_copy
            FOR VALUES FROM ('p1', '2026-08-19') TO ('p1', '2026-08-20')`)
	require.NoError(t, err)

	_, err = db.Exec(ctx, `
        INSERT INTO convoy.idx_test_copy (project_id, created_at)
        VALUES ('p1', '2026-08-19'), ('p1', '2026-08-19')`)
	require.NoError(t, err)

	owed := Dropped{
		Table: "idx_test_copy",
		Name:  "idx_test_copy_created",
		Definition: `CREATE UNIQUE INDEX idx_test_copy_created ON convoy.idx_test_copy ` +
			`USING btree (project_id, created_at)`,
	}
	_, err = db.Exec(ctx, `
        INSERT INTO convoy.dropped_indexes (index_name, table_name, definition)
        VALUES ($1, $2, $3)`, owed.Name, owed.Table, owed.Definition)
	require.NoError(t, err)

	// An earlier run that died on this partition is what leaves a copy behind
	// under the name the next run will want.
	child := childName(owed.Name, "idx_test_copy_19")
	_, err = db.Exec(ctx, fmt.Sprintf(
		`CREATE UNIQUE INDEX CONCURRENTLY %s ON convoy.idx_test_copy_19 (project_id, created_at)`, child))
	require.Error(t, err, "the duplicate rows are there to make this build fail")

	_, err = db.Exec(ctx, `
        DELETE FROM convoy.idx_test_copy
         WHERE ctid = (SELECT MIN(ctid) FROM convoy.idx_test_copy)`)
	require.NoError(t, err)

	require.NoError(t, Rebuild(ctx, db, owed))
	require.True(t, valid(t, db, owed.Name))

	var queued []string
	rows, err := db.Query(ctx, `SELECT index_name FROM convoy.dropped_indexes WHERE rebuilt_at IS NULL`)
	require.NoError(t, err)
	defer rows.Close()
	for rows.Next() {
		var name string
		require.NoError(t, rows.Scan(&name))
		queued = append(queued, name)
	}
	require.NoError(t, rows.Err())
	require.Empty(t, queued, "the parent is rebuilt and its partitions were never debts of their own")
}

// A rebuild works through this list in order, and a unique index that is missing
// is not enforcing its key, so it cannot wait behind hours of work on a large
// non-unique one.
func TestDroppedIndexesAreOfferedUniqueFirst(t *testing.T) {
	db := setupDB(t)
	ctx := context.Background()

	_, err := db.Exec(ctx, `
        INSERT INTO convoy.dropped_indexes (index_name, table_name, definition, dropped_at)
        VALUES ('idx_test_plain', 'events', 'CREATE INDEX idx_test_plain ON convoy.events USING btree (created_at)',
                NOW() - INTERVAL '2 days'),
               ('idx_test_uniq', 'partition_runs', 'CREATE UNIQUE INDEX idx_test_uniq ON convoy.partition_runs USING btree (status)',
                NOW())`)
	require.NoError(t, err)

	owed, err := ListDropped(ctx, db)
	require.NoError(t, err)
	require.Len(t, owed, 2)
	require.Equal(t, "idx_test_uniq", owed[0].Name, "the unique index waited two days less and still goes first")
	require.True(t, owed[0].Unique())
}

// While the run guard is missing the runner refuses to start anything else, so a
// rebuild that took another index first would fail on every one of them.
func TestTheRunGuardIsOfferedAheadOfOtherUniqueIndexes(t *testing.T) {
	db := setupDB(t)
	ctx := context.Background()

	_, err := db.Exec(ctx, `
        INSERT INTO convoy.dropped_indexes (index_name, table_name, definition, dropped_at)
        VALUES ('idx_test_uniq', 'events', 'CREATE UNIQUE INDEX idx_test_uniq ON convoy.events USING btree (id)',
                NOW() - INTERVAL '2 days'),
               ('idx_partition_runs_single_active', 'partition_runs',
                'CREATE UNIQUE INDEX idx_partition_runs_single_active ON convoy.partition_runs USING btree (status) WHERE (status = ''running'')',
                NOW())`)
	require.NoError(t, err)

	owed, err := ListDropped(ctx, db)
	require.NoError(t, err)
	require.Len(t, owed, 2)
	require.Equal(t, "idx_partition_runs_single_active", owed[0].Name,
		"the guard blocks every other rebuild, so it cannot be second")
}

// A concurrent build cannot run in a transaction, so the rebuild's lock_timeout
// is session state on a pooled connection. Left behind, it would abort row-lock
// waits in ordinary traffic that are supposed to queue.
func TestRebuildLeavesNoLockTimeoutOnThePool(t *testing.T) {
	db := setupDB(t)
	ctx := context.Background()

	_, err := db.Exec(ctx, `CREATE TABLE convoy.idx_test_reset (k TEXT)`)
	require.NoError(t, err)

	owed := Dropped{
		Table:      "idx_test_reset",
		Name:       "idx_test_reset_k",
		Definition: `CREATE INDEX idx_test_reset_k ON convoy.idx_test_reset USING btree (k)`,
	}
	_, err = db.Exec(ctx, `
        INSERT INTO convoy.dropped_indexes (index_name, table_name, definition)
        VALUES ($1, $2, $3)`, owed.Name, owed.Table, owed.Definition)
	require.NoError(t, err)

	require.NoError(t, Rebuild(ctx, db, owed))

	// Check every connection the pool is holding, not one, since the rebuild's
	// connection is only the one most likely to be handed back first.
	idle := int(db.Stat().IdleConns())
	require.Positive(t, idle, "the rebuild's connection should be back in the pool")

	held := make([]*pgxpool.Conn, 0, idle)
	defer func() {
		for _, c := range held {
			c.Release()
		}
	}()
	for i := 0; i < idle; i++ {
		conn, err := db.Acquire(ctx)
		require.NoError(t, err)
		held = append(held, conn)

		var timeout string
		require.NoError(t, conn.QueryRow(ctx, `SHOW lock_timeout`).Scan(&timeout))
		require.Equal(t, "0", timeout, "a pooled connection is still carrying the rebuild's lock_timeout")
	}
}

func TestRebuildRejectsAMissingTable(t *testing.T) {
	db := setupDB(t)

	err := Rebuild(context.Background(), db, Dropped{
		Table:      "idx_test_gone",
		Name:       "idx_test_gone_created",
		Definition: `CREATE INDEX idx_test_gone_created ON convoy.idx_test_gone USING btree (created_at)`,
	})
	require.ErrorContains(t, err, "no longer exists")
}

func names(invalid []Invalid) []string {
	out := make([]string, 0, len(invalid))
	for _, i := range invalid {
		out = append(out, i.Name)
	}
	return out
}

func valid(t *testing.T, db *pgxpool.Pool, index string) bool {
	t.Helper()

	var isValid bool
	require.NoError(t, db.QueryRow(context.Background(), `
        SELECT i.indisvalid
          FROM pg_index i
          JOIN pg_class c ON c.oid = i.indexrelid
          JOIN pg_namespace n ON n.oid = c.relnamespace
         WHERE n.nspname = 'convoy' AND c.relname = $1`, index).Scan(&isValid))
	return isValid
}
