// Package indexes reports indexes Postgres left invalid, and rebuilds the ones a
// migration dropped.
//
// An index whose build died partway stays in the catalog marked invalid, and the
// planner ignores it from then on, so the table performs as if the index had
// never been created. Migrations cannot repair that themselves: they build
// CONCURRENTLY, which cannot run in a transaction, so a failed build is not
// recorded and the retry's CREATE INDEX CONCURRENTLY IF NOT EXISTS is skipped by
// the invalid index still holding the name. The migration is then recorded as
// applied.
//
// Migrations drop what they find instead, which is instant, and record the
// definition in convoy.dropped_indexes. Rebuilding is the expensive half: hours
// on a large table. Server and agent start every owed rebuild in the background
// after boot, unique indexes first, one at a time. The dashboard and CLI are
// progress and retry, not the only way a rebuild starts.
package indexes

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

// maxIdentifier is Postgres's identifier length. A name built past it is
// truncated by the server, silently, which would make two derived names collide.
const maxIdentifier = 63

// tableLockTimeout bounds the statements that take a lock on the table: creating
// the parent index, attaching to it, and clearing an invalid leftover. A rebuild
// is elective, so it gives way to traffic rather than queueing ahead of it.
const tableLockTimeout = "3s"

// buildLockTimeout bounds a concurrent build, which needs a budget of a
// different order to the one above.
//
// CREATE INDEX CONCURRENTLY waits twice for every transaction that was already
// open to finish, and it takes that wait as a lock on each transaction's virtual
// id, so lock_timeout cancels it exactly as it cancels a wait for the table.
// Those are not the same risk. Waiting for the table means queueing ahead of
// traffic; waiting for transactions to drain costs nothing but time, since the
// build holds ShareUpdateExclusive throughout, which blocks other DDL and
// autovacuum on that table but not reads or writes.
//
// Sharing one budget made the drain wait fail against Convoy's own daily counts
// rollup, whose transaction runs for tens of seconds every minute, so a build
// only succeeded if it happened to start in a gap between rollup runs. Postgres
// 17 prunes transactions that cannot see the table from that wait; 15, which is
// what the observed instance runs, waits for all of them.
//
// Ten minutes, not longer: the wait is for transactions to end, and a
// transaction still open after ten minutes is stuck rather than slow. There is
// one rebuild slot for the instance, so waiting past that point costs every
// index behind this one. The failure names what it waited for.
const buildLockTimeout = "10min"

// resetTimeout bounds undoing whichever of the budgets above is still set when
// the connection goes back to the pool. It is short because the alternative to
// waiting is closing the connection, which is cheap next to leaving a
// session-wide timeout on a pooled connection.
const resetTimeout = 5 * time.Second

// PayloadGIN is the events payload search index. sql/1787200001.sql inserts this
// name into dropped_indexes instead of CREATE INDEX at migrate.
const PayloadGIN = "idx_events_payload_gin"

// PayloadGINDefinition is the statement the rebuild must execute. It has to match
// the row sql/1787200001.sql inserts, including USING so indexShape can read it.
const PayloadGINDefinition = `CREATE INDEX idx_events_payload_gin ON convoy.events USING gin (convoy.event_payload_jsonb(data) jsonb_path_ops) WHERE (deleted_at IS NULL)`

// EventDeliveriesProjectCreated is the Event Deliveries list/chart index.
// sql/1787251200.sql inserts this name into dropped_indexes instead of CREATE INDEX at migrate.
const EventDeliveriesProjectCreated = "idx_event_deliveries_project_created_id_deleted"

// EventDeliveriesProjectCreatedDefinition is the statement the rebuild must execute.
// It has to match the row sql/1787251200.sql inserts.
const EventDeliveriesProjectCreatedDefinition = `CREATE INDEX idx_event_deliveries_project_created_id_deleted ON convoy.event_deliveries USING btree (project_id, created_at DESC, id DESC) WHERE (deleted_at IS NULL)`

// EventDeliveriesProjectEventTypeCreated is the Observed event-type dropdown index.
// sql/1787702400.sql inserts this name into dropped_indexes instead of CREATE INDEX at migrate.
const EventDeliveriesProjectEventTypeCreated = "idx_event_deliveries_project_event_type_created"

// EventDeliveriesProjectEventTypeCreatedDefinition is the statement the rebuild must execute.
// It has to match the row sql/1787702400.sql inserts.
const EventDeliveriesProjectEventTypeCreatedDefinition = `CREATE INDEX idx_event_deliveries_project_event_type_created ON convoy.event_deliveries USING btree (project_id, event_type, created_at) WHERE (deleted_at IS NULL)`

// BootQueuedNames are the indexes migrate always inserts into dropped_indexes
// instead of CREATE INDEX, so every upgraded instance owes them a rebuild.
var BootQueuedNames = []string{EventDeliveriesProjectCreated, EventDeliveriesProjectEventTypeCreated, PayloadGIN}

// BootQueued reports whether migrate always queues this name. Repair tests use
// it to look past those rows when counting operator-dropped indexes.
func BootQueued(name string) bool {
	for _, n := range BootQueuedNames {
		if n == name {
			return true
		}
	}
	return false
}

// ErrNotDropped means the name does not identify an index awaiting a rebuild,
// either because nothing dropped it or because it has already been rebuilt.
var ErrNotDropped = errors.New("index is not awaiting a rebuild")

// Invalid is an index the planner is ignoring.
type Invalid struct {
	Table string
	Name  string

	// Busy reports a live build or another session working on the index. It is
	// not a failure and must not be touched: an index under construction is
	// marked invalid until it finishes.
	Busy bool
}

// Dropped is an index a migration removed because it was invalid, held with the
// definition needed to build it again.
type Dropped struct {
	Table         string
	Name          string
	Definition    string
	DroppedAt     time.Time
	BlockedAt     *time.Time
	BlockedReason string
}

func (d Dropped) Blocked() bool {
	return d.BlockedAt != nil
}

// Unique reports whether the index enforced uniqueness, which is the one thing a
// missing index can cost beyond speed.
//
// An invalid index is ignored by the planner but is still maintained by writes
// if it got as far as the validation scan, and a unique one enforces its key
// while it is there. Dropping it takes that enforcement away, so these are the
// rebuilds to run first.
func (d Dropped) Unique() bool {
	return strings.HasPrefix(d.Definition, "CREATE UNIQUE ")
}

// ListInvalid reads the catalog, not convoy.dropped_indexes: these are indexes
// that are invalid right now that nothing has taken responsibility for yet.
//
// One already owed a rebuild is left out. A rebuild that fails leaves a fresh
// invalid index behind under the same name, and the next attempt drops it before
// building, so reporting it here as well would show one index as two pieces of
// work: a rebuild to run, and an abandoned index to deal with separately. The
// debt row is the one an operator can act on.
//
// Partitioned indexes are left out. One of those is invalid until every
// partition has an index attached, which is an ordinary intermediate state of a
// conversion rather than a failure.
func ListInvalid(ctx context.Context, db *pgxpool.Pool) ([]Invalid, error) {
	rows, err := db.Query(ctx, `
        SELECT t.relname,
               c.relname,
               EXISTS (SELECT 1 FROM pg_stat_progress_create_index p WHERE p.index_relid = i.indexrelid)
                   OR EXISTS (SELECT 1 FROM pg_locks l
                               WHERE l.relation = i.indexrelid AND l.granted AND l.pid <> pg_backend_pid())
          FROM pg_index i
          JOIN pg_class c ON c.oid = i.indexrelid
          JOIN pg_class t ON t.oid = i.indrelid
          JOIN pg_namespace n ON n.oid = c.relnamespace
         WHERE n.nspname = 'convoy'
           AND c.relkind = 'i'
           AND NOT i.indisvalid
           AND NOT EXISTS (SELECT 1 FROM convoy.dropped_indexes d
                            WHERE d.index_name = c.relname AND d.rebuilt_at IS NULL)
         ORDER BY t.relname, c.relname`)
	if err != nil {
		return nil, fmt.Errorf("reading invalid indexes: %w", err)
	}
	defer rows.Close()

	var invalid []Invalid
	for rows.Next() {
		var i Invalid
		if err = rows.Scan(&i.Table, &i.Name, &i.Busy); err != nil {
			return nil, fmt.Errorf("reading invalid indexes: %w", err)
		}
		invalid = append(invalid, i)
	}
	return invalid, rows.Err()
}

// ListDropped returns the indexes still owed a rebuild, in the order a rebuild
// should run them.
//
// The run guard comes first, ahead of even the other unique indexes: while it is
// missing nothing refuses a second conversion or rebuild, and the runner refuses
// to start anything else until it is back, so a rebuild that took another index
// first would fail on every one of them. Unique indexes follow, because a missing
// unique index is not just slower, it is not enforcing its key, and hours spent
// on a large non-unique index first leaves that gap open for those hours. Among
// non-unique indexes the Event Deliveries list index is next: that query times
// out until it is valid. The Observed-types index follows it: the same page
// 504s the type dropdown until that btree is valid. dropped_at alone would put
// the hours-long payload GIN first because migrate queued GIN earlier.
//
// This is the only place the order is decided, so the list an operator reads and
// the list --rebuild works through cannot disagree.
func ListDropped(ctx context.Context, db *pgxpool.Pool) ([]Dropped, error) {
	rows, err := db.Query(ctx, `
        SELECT table_name, index_name, definition, dropped_at, blocked_at, blocked_reason
          FROM convoy.dropped_indexes
         WHERE rebuilt_at IS NULL
         ORDER BY (index_name = 'idx_partition_runs_single_active') DESC,
                  (definition LIKE 'CREATE UNIQUE %') DESC,
                  (index_name = '`+EventDeliveriesProjectCreated+`') DESC,
                  (index_name = '`+EventDeliveriesProjectEventTypeCreated+`') DESC,
                  dropped_at`)
	if err != nil {
		return nil, fmt.Errorf("reading dropped indexes: %w", err)
	}
	defer rows.Close()

	var dropped []Dropped
	for rows.Next() {
		var d Dropped
		var blockedAt pgtype.Timestamptz
		var blockedReason pgtype.Text
		if err = rows.Scan(&d.Table, &d.Name, &d.Definition, &d.DroppedAt, &blockedAt, &blockedReason); err != nil {
			return nil, fmt.Errorf("reading dropped indexes: %w", err)
		}
		if blockedAt.Valid {
			t := blockedAt.Time
			d.BlockedAt = &t
		}
		if blockedReason.Valid {
			d.BlockedReason = blockedReason.String
		}
		dropped = append(dropped, d)
	}
	return dropped, rows.Err()
}

// GetDropped reads one index still owed a rebuild.
//
// This is the read a caller-supplied name has to pass before any rebuild is
// recorded or started. It returns ErrNotDropped for a name that is unknown and
// for one already rebuilt alike, because neither is work this can do and the
// caller has nothing different to offer for either. It is also what keeps the
// definition out of the request: the SQL that gets executed comes from the
// catalog capture, never from the caller.
func GetDropped(ctx context.Context, db *pgxpool.Pool, name string) (Dropped, error) {
	var d Dropped
	var blockedAt pgtype.Timestamptz
	var blockedReason pgtype.Text
	err := db.QueryRow(ctx, `
        SELECT table_name, index_name, definition, dropped_at, blocked_at, blocked_reason
          FROM convoy.dropped_indexes
         WHERE index_name = $1 AND rebuilt_at IS NULL`, name).
		Scan(&d.Table, &d.Name, &d.Definition, &d.DroppedAt, &blockedAt, &blockedReason)
	if errors.Is(err, pgx.ErrNoRows) {
		return Dropped{}, fmt.Errorf("%w: %s", ErrNotDropped, name)
	}
	if err != nil {
		return Dropped{}, fmt.Errorf("reading dropped index %s: %w", name, err)
	}
	if blockedAt.Valid {
		t := blockedAt.Time
		d.BlockedAt = &t
	}
	if blockedReason.Valid {
		d.BlockedReason = blockedReason.String
	}
	return d, nil
}

// Rebuild builds one dropped index again and marks it rebuilt.
//
// The whole rebuild runs on a single connection so that lock_timeout applies to
// every statement in it, and so the parent index and the children attached to it
// cannot be split across connections mid-way.
//
// Every statement is written to be safe to run again, because a rebuild on a
// large table can be interrupted and has to resume rather than start over.
func Rebuild(ctx context.Context, db *pgxpool.Pool, d Dropped) error {
	conn, err := db.Acquire(ctx)
	if err != nil {
		return err
	}
	defer release(ctx, conn)

	// SET, not SET LOCAL: a concurrent index build cannot run in a transaction,
	// so there is no transaction for the setting to be local to. That makes it
	// session state on a pooled connection, which release has to undo.
	if err := setLockTimeout(ctx, conn, tableLockTimeout); err != nil {
		return err
	}

	partitioned, err := isPartitioned(ctx, conn, d.Table)
	if err != nil {
		return err
	}

	if partitioned {
		err = rebuildPartitioned(ctx, conn, d)
	} else {
		err = rebuildHeap(ctx, conn, d)
	}
	if err != nil {
		return err
	}

	if _, err = conn.Exec(ctx, `
        UPDATE convoy.dropped_indexes
        SET rebuilt_at = NOW(), blocked_at = NULL, blocked_reason = NULL
         WHERE index_name = $1 AND rebuilt_at IS NULL`, d.Name); err != nil {
		return fmt.Errorf("index %s was rebuilt but recording that failed, it will be offered again: %w", d.Name, err)
	}
	return nil
}

// release hands the connection back with the lock_timeout undone.
//
// The pool this came from serves ordinary traffic, where either of the rebuild's
// budgets is wrong: a row-lock wait that is supposed to queue would abort after
// three seconds, or hold a request open for ten minutes. The
// reset gets a context of its own because it also has to run when the rebuild's
// context is already cancelled, which is exactly when the setting would
// otherwise be left behind. If the reset cannot be confirmed, the connection
// leaves the pool rather than going back carrying it.
func release(ctx context.Context, conn *pgxpool.Conn) {
	reset, cancel := context.WithTimeout(context.WithoutCancel(ctx), resetTimeout)
	defer cancel()

	if _, err := conn.Exec(reset, `RESET lock_timeout`); err != nil {
		_ = conn.Hijack().Close(reset)
		return
	}
	conn.Release()
}

// rebuildHeap builds the index as it was, online.
func rebuildHeap(ctx context.Context, conn *pgxpool.Conn, d Dropped) error {
	// An earlier attempt that died leaves an invalid index holding the name,
	// which IF NOT EXISTS would skip: the same trap that produced the dropped
	// index in the first place.
	if err := dropIfInvalid(ctx, conn, d.Name); err != nil {
		return err
	}

	stmt, err := concurrently(d.Definition)
	if err != nil {
		return err
	}
	if err := buildConcurrently(ctx, conn, d.Table, stmt,
		fmt.Sprintf("building %s on convoy.%s", d.Name, d.Table)); err != nil {
		return err
	}

	return assertValid(ctx, conn, d.Name,
		"the name is still held by an index the drop declined to remove, a build in progress or one a constraint owns")
}

// rebuildPartitioned builds the index across a partitioned table.
//
// CREATE INDEX CONCURRENTLY is not supported on a partitioned table, and the
// plain form recurses into every partition while holding the parent under an
// exclusive lock for the whole build, which a live instance cannot give. The
// supported online route is an empty parent index, a concurrent build on each
// partition, and an attach: the parent turns valid by itself once every partition
// is covered.
func rebuildPartitioned(ctx context.Context, conn *pgxpool.Conn, d Dropped) error {
	parent, err := parentStatement(d.Name, d.Table, d.Definition)
	if err != nil {
		return err
	}
	if _, err = conn.Exec(ctx, parent); err != nil {
		return fmt.Errorf("creating the parent index %s on convoy.%s: %w", d.Name, d.Table, err)
	}

	partitions, err := unindexedPartitions(ctx, conn, d.Table, d.Name)
	if err != nil {
		return err
	}

	for _, partition := range partitions {
		child := childName(d.Name, partition)
		create, attach, err := childStatements(d.Name, partition, d.Definition)
		if err != nil {
			return err
		}

		if err := dropIfInvalid(ctx, conn, child); err != nil {
			return err
		}

		if err := buildConcurrently(ctx, conn, partition, create,
			fmt.Sprintf("building %s on partition convoy.%s", child, partition)); err != nil {
			return err
		}

		if _, err = conn.Exec(ctx, attach); err != nil {
			return fmt.Errorf("attaching %s to %s: %w", child, d.Name, err)
		}
	}

	// The parent turning valid is the only proof every partition is covered. A
	// partition added while this ran leaves it invalid, and saying nothing would
	// record a rebuild that did not finish.
	return assertValid(ctx, conn, d.Name,
		fmt.Sprintf("%d partitions were covered, run the rebuild again to pick up the rest", len(partitions)))
}

// buildConcurrently runs one concurrent build under the longer budget, and only
// that statement, so the statements around it still give way to traffic.
//
// The table lock is probed first because the longer budget would otherwise hide
// real contention: a table someone holds a conflicting lock on would occupy the
// one rebuild slot for half an hour instead of being left for the next pass. The
// probe also separates the two failures for the operator reading the message,
// since lock_timeout alone cannot say which wait expired.
//
// A lock taken and released proves the table was free a moment ago, not that it
// still is. That is enough to stop the common case from wasting the slot.
func buildConcurrently(ctx context.Context, conn *pgxpool.Conn, relation, stmt, what string) error {
	if err := probeTableLock(ctx, conn, relation); err != nil {
		return fmt.Errorf("%s: %w", what, err)
	}

	if err := setLockTimeout(ctx, conn, buildLockTimeout); err != nil {
		return err
	}
	_, err := conn.Exec(ctx, stmt)

	// Restore the short budget whether or not the build worked: the attach that
	// follows a partition build, and the row the rebuild records, both take
	// ordinary locks and must not inherit half an hour of patience.
	if resetErr := setLockTimeout(ctx, conn, tableLockTimeout); resetErr != nil && err == nil {
		return resetErr
	}

	if err == nil {
		return nil
	}
	if isLockTimeout(err) {
		if waiting := openTransactions(ctx, conn); waiting != "" {
			return fmt.Errorf("%s: %w, after waiting %s for transactions to finish. Still open: %s",
				what, err, buildLockTimeout, waiting)
		}
	}
	return fmt.Errorf("%s: %w", what, err)
}

// probeTableLock reports whether the relation can be locked the way a concurrent
// build locks it, without waiting longer than an elective rebuild should.
//
// The lock is taken in its own transaction and rolled back, so it is held for
// the round trip and nothing else.
func probeTableLock(ctx context.Context, conn *pgxpool.Conn, relation string) error {
	err := func() error {
		tx, err := conn.Begin(ctx)
		if err != nil {
			return err
		}
		// Rolled back before this returns, so the caller can query the
		// connection: a statement that failed leaves the transaction aborted and
		// refusing everything until it ends.
		defer func() { _ = tx.Rollback(ctx) }()

		if _, err = tx.Exec(ctx, `SET LOCAL lock_timeout = '`+tableLockTimeout+`'`); err != nil {
			return fmt.Errorf("bounding the table lock wait: %w", err)
		}
		_, err = tx.Exec(ctx, `LOCK TABLE convoy.`+pgx.Identifier{relation}.Sanitize()+
			` IN SHARE UPDATE EXCLUSIVE MODE`)
		return err
	}()

	if err != nil && isLockTimeout(err) {
		return fmt.Errorf("convoy.%s could not be locked within %s%s", relation, tableLockTimeout,
			held(ctx, conn, relation))
	}
	return err
}

// held names the sessions holding a lock on the relation, for a message that
// would otherwise say only that a lock could not be taken.
func held(ctx context.Context, conn *pgxpool.Conn, relation string) string {
	rows, err := conn.Query(ctx, `
        SELECT l.pid, l.mode, COALESCE(NULLIF(LEFT(REGEXP_REPLACE(a.query, '\s+', ' ', 'g'), 80), ''), '(not visible)')
          FROM pg_locks l
          LEFT JOIN pg_stat_activity a ON a.pid = l.pid
         WHERE l.relation = ('convoy.' || QUOTE_IDENT($1))::regclass
           AND l.granted
           AND l.pid <> pg_backend_pid()
           AND l.mode IN ('ShareUpdateExclusiveLock', 'ShareLock', 'ShareRowExclusiveLock',
                          'ExclusiveLock', 'AccessExclusiveLock')
         ORDER BY l.pid
         LIMIT 3`, relation)
	if err != nil {
		return ""
	}
	defer rows.Close()

	var holders []string
	for rows.Next() {
		var pid int
		var mode, query string
		if err = rows.Scan(&pid, &mode, &query); err != nil {
			return ""
		}
		holders = append(holders, fmt.Sprintf("pid %d holds %s running %s", pid, mode, query))
	}
	if rows.Err() != nil || len(holders) == 0 {
		return ""
	}
	return ". Held by " + strings.Join(holders, "; ")
}

// openTransactions names the transactions a concurrent build was still waiting
// for. A build waits for transactions that were open before it started, whatever
// they touch, so the one holding it up is usually nowhere near this table.
func openTransactions(ctx context.Context, conn *pgxpool.Conn) string {
	rows, err := conn.Query(ctx, `
        SELECT pid,
               DATE_TRUNC('second', CLOCK_TIMESTAMP() - xact_start)::TEXT,
               COALESCE(NULLIF(LEFT(REGEXP_REPLACE(query, '\s+', ' ', 'g'), 80), ''), '(not visible)')
          FROM pg_stat_activity
         WHERE pid <> pg_backend_pid()
           AND xact_start IS NOT NULL
           AND CLOCK_TIMESTAMP() - xact_start > INTERVAL '3 seconds'
         ORDER BY xact_start
         LIMIT 3`)
	if err != nil {
		return ""
	}
	defer rows.Close()

	var open []string
	for rows.Next() {
		var pid int
		var age, query string
		if err = rows.Scan(&pid, &age, &query); err != nil {
			return ""
		}
		open = append(open, fmt.Sprintf("pid %d open %s running %s", pid, age, query))
	}
	if rows.Err() != nil || len(open) == 0 {
		return ""
	}
	return strings.Join(open, "; ")
}

func setLockTimeout(ctx context.Context, conn *pgxpool.Conn, value string) error {
	if _, err := conn.Exec(ctx, `SET lock_timeout = '`+value+`'`); err != nil {
		return fmt.Errorf("setting the lock timeout to %s: %w", value, err)
	}
	return nil
}

// isLockTimeout reports a statement Postgres cancelled because a lock it wanted
// was held by someone else. It says nothing about which lock.
func isLockTimeout(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "55P03"
}

// assertValid is what stands between a build that did not happen and a
// rebuilt_at that says the debt is paid. Every build statement here carries
// IF NOT EXISTS, which is what lets a rebuild resume, and the same clause makes
// Postgres silent when the name was already taken by an index it declined to
// drop. Validity is read from the catalog because that is what the planner
// reads: an index it ignores is a table with no index, whatever the build
// returned.
func assertValid(ctx context.Context, conn *pgxpool.Conn, name, because string) error {
	var valid bool
	err := conn.QueryRow(ctx, `
        SELECT i.indisvalid
          FROM pg_index i
          JOIN pg_class c ON c.oid = i.indexrelid
          JOIN pg_namespace n ON n.oid = c.relnamespace
         WHERE n.nspname = 'convoy' AND c.relname = $1`, name).Scan(&valid)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return fmt.Errorf("%s is not there after the rebuild, so it stays on the list", name)
	case err != nil:
		return fmt.Errorf("checking %s after rebuild: %w", name, err)
	case !valid:
		return fmt.Errorf("%s is still invalid after the rebuild: %s", name, because)
	}
	return nil
}

// dropIfInvalid clears an invalid index left by an interrupted attempt, through
// the same function migrations use, so a build here cannot be skipped by the name
// already existing.
//
// Nothing is recorded. The debt is already on the books as the row being
// rebuilt, and a partition's copy of an index has no life of its own: recording
// one would queue a name that only means anything attached to a parent, and
// rebuilding it alone would leave the parent no closer to valid.
func dropIfInvalid(ctx context.Context, conn *pgxpool.Conn, name string) error {
	if _, err := conn.Exec(ctx, `SELECT convoy.drop_invalid_index($1, FALSE)`, name); err != nil {
		return fmt.Errorf("clearing the invalid index %s: %w", name, err)
	}
	return nil
}

func isPartitioned(ctx context.Context, conn *pgxpool.Conn, table string) (bool, error) {
	var kind string
	if err := conn.QueryRow(ctx, `
        SELECT c.relkind
          FROM pg_class c
          JOIN pg_namespace n ON n.oid = c.relnamespace
         WHERE n.nspname = 'convoy' AND c.relname = $1`, table).Scan(&kind); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, fmt.Errorf("convoy.%s no longer exists", table)
		}
		return false, fmt.Errorf("reading the shape of convoy.%s: %w", table, err)
	}
	return kind == "p", nil
}

// unindexedPartitions lists the partitions with no index attached to the parent
// yet, which is what makes a rebuild resumable: an interrupted run picks up where
// it stopped instead of re-attaching what is already attached.
func unindexedPartitions(ctx context.Context, conn *pgxpool.Conn, table, index string) ([]string, error) {
	rows, err := conn.Query(ctx, `
        SELECT part.relname
          FROM pg_inherits h
          JOIN pg_class part ON part.oid = h.inhrelid
          JOIN pg_class parent ON parent.oid = h.inhparent
          JOIN pg_namespace n ON n.oid = parent.relnamespace
         WHERE n.nspname = 'convoy'
           AND parent.relname = $1
           AND NOT EXISTS (
               SELECT 1
                 FROM pg_inherits attached
                 JOIN pg_index child ON child.indexrelid = attached.inhrelid
                 JOIN pg_class parent_index ON parent_index.oid = attached.inhparent
                WHERE parent_index.relname = $2
                  AND parent_index.relnamespace = n.oid
                  AND child.indrelid = part.oid
           )
         ORDER BY part.relname`, table, index)
	if err != nil {
		return nil, fmt.Errorf("listing the partitions of convoy.%s: %w", table, err)
	}
	defer rows.Close()

	var partitions []string
	for rows.Next() {
		var name string
		if err = rows.Scan(&name); err != nil {
			return nil, fmt.Errorf("listing the partitions of convoy.%s: %w", table, err)
		}
		partitions = append(partitions, name)
	}
	return partitions, rows.Err()
}

// concurrently turns a recorded definition into an online build.
// pg_get_indexdef never writes CONCURRENTLY, and IF NOT EXISTS makes a resumed
// rebuild skip what an earlier run finished.
func concurrently(def string) (string, error) {
	if rest, ok := strings.CutPrefix(def, "CREATE UNIQUE INDEX "); ok {
		return "CREATE UNIQUE INDEX CONCURRENTLY IF NOT EXISTS " + rest, nil
	}
	if rest, ok := strings.CutPrefix(def, "CREATE INDEX "); ok {
		return "CREATE INDEX CONCURRENTLY IF NOT EXISTS " + rest, nil
	}
	return "", fmt.Errorf("cannot read the recorded definition: %q", def)
}

// parentStatement creates the index on the parent alone. It scans nothing and
// stays invalid until every partition is attached, so it is only a declaration of
// the shape the children must match.
func parentStatement(index, table, def string) (string, error) {
	shape, unique, err := indexShape(def)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf(`CREATE %sINDEX IF NOT EXISTS %s ON ONLY %s%s`,
		unique, ident(index), qualified(table), shape), nil
}

// childStatements build one partition's copy of the index and hand it to the
// parent. The build is concurrent because it is the part that reads the data.
func childStatements(index, partition, def string) (create, attach string, err error) {
	shape, unique, err := indexShape(def)
	if err != nil {
		return "", "", err
	}

	child := childName(index, partition)
	create = fmt.Sprintf(`CREATE %sINDEX CONCURRENTLY IF NOT EXISTS %s ON %s%s`,
		unique, ident(child), qualified(partition), shape)
	attach = fmt.Sprintf(`ALTER INDEX %s ATTACH PARTITION %s`, qualified(index), qualified(child))
	return create, attach, nil
}

// indexShape splits a definition into the part that describes the index, from
// USING onwards, and whether it is unique. Everything before USING names the
// index and the table, which a rebuild is replacing.
func indexShape(def string) (shape, unique string, err error) {
	at := strings.Index(def, " USING ")
	if at < 0 {
		return "", "", fmt.Errorf("cannot read the recorded definition: %q", def)
	}
	if strings.HasPrefix(def, "CREATE UNIQUE ") {
		unique = "UNIQUE "
	}
	return def[at:], unique, nil
}

// childName derives the name of a partition's copy of an index.
//
// Partition names already run long, so the obvious index_partition would be
// truncated by the server to 63 bytes, and truncation cuts the date that
// distinguishes one partition from the next: two partitions would collide on one
// name, and IF NOT EXISTS would then attach the first partition's index to the
// second. A digest of the partition keeps the name both short and distinct, and
// keeps it a function of its inputs so a resumed rebuild derives the same name.
func childName(index, partition string) string {
	sum := sha256.Sum256([]byte(index + "\x00" + partition))
	digest := hex.EncodeToString(sum[:5])

	base := index
	if len(base) > maxIdentifier-len(digest)-1 {
		base = base[:maxIdentifier-len(digest)-1]
	}
	return base + "_" + digest
}

func ident(name string) string {
	return pgx.Identifier{name}.Sanitize()
}

func qualified(name string) string {
	return pgx.Identifier{"convoy", name}.Sanitize()
}
