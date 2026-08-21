// Package partitions records the progress of partition conversions.
//
// A conversion takes minutes to hours on a large instance, and progress has to
// reach a session other than the one running it. Writing to a table directly
// does not achieve that for every conversion: copy-based unpartitioning still
// runs as a single statement, so nothing it wrote would be visible until the
// whole thing committed, which is the exact window that needs reporting. What
// does escape a running statement is its RAISE NOTICE stream, which pgx
// delivers as each notice arrives.
//
// Attach conversions commit per phase, so their notices mark real boundaries
// rather than points inside an uncommitted rewrite. Both directions still
// report the same way, so this package has one mechanism to maintain rather
// than two.
package partitions

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
	"github.com/oklog/ulid/v2"

	"github.com/frain-dev/convoy/database"
	"github.com/frain-dev/convoy/internal/delivery_attempts"
	"github.com/frain-dev/convoy/internal/event_deliveries"
	"github.com/frain-dev/convoy/internal/events"
	"github.com/frain-dev/convoy/internal/pkg/indexes"
	log "github.com/frain-dev/convoy/pkg/logger"
)

type Table string

const (
	TableEvents           Table = "events"
	TableEventsSearch     Table = "events_search"
	TableEventDeliveries  Table = "event_deliveries"
	TableDeliveryAttempts Table = "delivery_attempts"
)

type Operation string

const (
	OperationPartition   Operation = "partition"
	OperationUnpartition Operation = "unpartition"

	// OperationRebuildIndex builds back an index a migration dropped for being
	// invalid. It shares this runner because it is the same kind of work as a
	// conversion, on the same tables, and must not run beside one.
	OperationRebuildIndex Operation = "rebuild_index"
)

type Status string

const (
	StatusRunning   Status = "running"
	StatusCompleted Status = "completed"
	StatusFailed    Status = "failed"
)

// runColumns is every column a Run scans, written once so the insert's RETURNING
// and the reads cannot drift apart when a column is added.
const runColumns = `id, table_name, operation, index_name, status, phase, steps,
                    notice_count, error, triggered_by, started_at, updated_at, completed_at`

// singleActiveGuard is the partial unique index that makes one run at a time
// true. Nothing else enforces it: a second start is refused by that index
// rejecting the insert, so the name is needed here to check the index is there.
const singleActiveGuard = "idx_partition_runs_single_active"

// guardRebuildLock serializes the one start the guard cannot: a rebuild of the
// guard itself. Any constant works as long as nothing else in the schema picks
// the same one, so it is defined here beside its only user.
const guardRebuildLock int64 = 8267341982

// maxSteps bounds the step list a run carries. A conversion reports around ten,
// so this is only reached by a loop nobody intended, and it keeps one row from
// growing without limit.
const maxSteps = 200

var (
	// ErrRunInProgress is returned when a conversion is already running. One at a
	// time is deliberate: each conversion rewrites a table and saturates disk
	// doing it, and the caller must not queue behind a multi-hour operation
	// without being told.
	ErrRunInProgress = errors.New("a partition run is already in progress")

	ErrRunNotFound = errors.New("partition run not found")

	// ErrGuardMissing is returned when the unique index that enforces one run at
	// a time is not valid, so a start cannot be refused by it.
	ErrGuardMissing = errors.New("the single-active guard on convoy.partition_runs is missing, " +
		"so a second run could not be refused; rebuild it with: convoy utils indexes --rebuild")

	ErrUnknownTable = errors.New("unknown table")

	// ErrAlreadyPartitioned and ErrNotPartitioned are returned when the table is
	// already in the shape the operation would produce, so the conversion has
	// nothing to do.
	ErrAlreadyPartitioned = errors.New("table is already partitioned")
	ErrNotPartitioned     = errors.New("table is not partitioned")
)

// failedIndexRebuilds are names whose rebuild already failed in this process.
// Handlers call New() per request, so this cannot live on Service or a
// conversion finishing on a fresh runner would retry a boot failure.
// StartIndexRebuild still tries a caller-supplied name (dashboard retry).
var failedIndexRebuilds sync.Map

// Tables lists what can be converted, in the order the CLI converts them when
// given no argument.
func Tables() []Table {
	return []Table{TableEvents, TableEventsSearch, TableEventDeliveries, TableDeliveryAttempts}
}

func ParseTable(s string) (Table, error) {
	for _, t := range Tables() {
		if Table(s) == t {
			return t, nil
		}
	}
	return "", fmt.Errorf("%w %q", ErrUnknownTable, s)
}

// TableState is a table's current shape. It decides which operation is valid:
// an ordinary table can be partitioned, a partitioned parent can be
// unpartitioned, and neither can be asked to become what it already is.
type TableState struct {
	Name        Table `json:"name" db:"name"`
	Partitioned bool  `json:"partitioned" db:"partitioned"`

	// Adopted reports that the table was converted by attaching rather than
	// copying, which is what decides whether unpartitioning it has to copy.
	// Partitioned says which operation applies; this says what that operation
	// will cost, and whether ingestion has to stop for it.
	Adopted bool `json:"adopted" db:"adopted"`
}

// ValidOperation is the only operation that changes this table.
func (t TableState) ValidOperation() Operation {
	if t.Partitioned {
		return OperationUnpartition
	}
	return OperationPartition
}

type Run struct {
	UID       string    `json:"uid" db:"id"`
	TableName Table     `json:"table_name" db:"table_name"`
	Operation Operation `json:"operation" db:"operation"`

	// IndexName is set only on a rebuild, where the table alone does not say
	// what the run is doing. The database enforces the pairing.
	IndexName *string `json:"index_name" db:"index_name"`

	Status      Status     `json:"status" db:"status"`
	Phase       *string    `json:"phase" db:"phase"`
	Steps       Steps      `json:"steps" db:"steps"`
	NoticeCount int64      `json:"notice_count" db:"notice_count"`
	Error       *string    `json:"error" db:"error"`
	TriggeredBy string     `json:"triggered_by" db:"triggered_by"`
	StartedAt   time.Time  `json:"started_at" db:"started_at"`
	UpdatedAt   time.Time  `json:"updated_at" db:"updated_at"`
	CompletedAt *time.Time `json:"completed_at" db:"completed_at"`
}

// Step is one line of a conversion's progress, with the time it arrived. The
// stream is otherwise only in the server log, which an operator watching the
// dashboard during a multi-hour conversion cannot see.
type Step struct {
	Message string    `json:"message"`
	At      time.Time `json:"at"`
}

type Steps []Step

// Scan reads the jsonb column. Without it the driver hands the field raw bytes
// and the scan fails.
func (s *Steps) Scan(src any) error {
	var data []byte
	switch v := src.(type) {
	case nil:
		*s = nil
		return nil
	case []byte:
		data = v
	case string:
		data = []byte(v)
	default:
		return fmt.Errorf("cannot read partition run steps from %T", src)
	}
	return json.Unmarshal(data, s)
}

// noticeObserver is implemented by the Postgres database. It is asserted rather
// than required because the test helper builds a database from a pool that was
// created without a notice handler, and a run there still records start and
// finish.
type noticeObserver interface {
	OnNotice(func(*pgconn.Notice))
}

type converter interface {
	run(ctx context.Context, table Table, op Operation) error
}

// rebuilder is the index half of the work this runner drives. It is a seam of its
// own rather than a widening of converter, because the two take different
// subjects: a conversion is named by its table, a rebuild by its index.
type rebuilder interface {
	// dropped resolves the name to the index awaiting a rebuild, and reports
	// which table it is on so the run row names that table.
	dropped(ctx context.Context, name string) (indexes.Dropped, error)
	// listDropped is the owed rebuilds in ListDropped order: guard, unique,
	// deliveries list index, then the rest by when they were dropped.
	listDropped(ctx context.Context) ([]indexes.Dropped, error)
	rebuild(ctx context.Context, d indexes.Dropped) error
}

type Service struct {
	db        database.Database
	logger    log.Logger
	converter converter
	rebuilder rebuilder
}

func New(db database.Database, logger log.Logger) *Service {
	return &Service{
		db:     db,
		logger: logger,
		converter: &repoConverter{
			events:     events.New(logger, db),
			deliveries: event_deliveries.New(logger, db),
			attempts:   delivery_attempts.New(logger, db),
		},
		rebuilder: &indexRebuilder{db: db},
	}
}

// Start records a run and converts the table on a goroutine of its own, because
// the conversion outlives any request that asked for it. The returned run is the
// handle a caller polls.
func (s *Service) Start(ctx context.Context, table Table, op Operation, triggeredBy string) (*Run, error) {
	run, err := s.record(ctx, table, op, triggeredBy)
	if err != nil {
		return nil, err
	}

	// The conversion is detached from the request that started it, so it keeps
	// running after the response is written. Cancelling it midway would leave a
	// half-converted table, which is worse than letting it finish. When it
	// frees the slot, start any owed index rebuilds that lost the race at boot.
	go func() {
		detached := context.WithoutCancel(ctx)
		_ = s.convert(detached, run)
		s.StartQueuedDroppedIndexes(detached)
	}()

	return run, nil
}

// Run converts on the caller's own goroutine, for the CLI, which has nothing to
// poll with and exits when the command returns.
//
// It goes through the same record step as Start rather than calling the
// repository directly. That record is what takes the instance-wide single-active
// lock, so a CLI conversion that skipped it could run beside one started from
// the dashboard, each rewriting a table while the other holds locks on it. It
// also means a conversion run from a shell shows up in the same history.
func (s *Service) Run(ctx context.Context, table Table, op Operation, triggeredBy string) error {
	run, err := s.record(ctx, table, op, triggeredBy)
	if err != nil {
		return err
	}
	return s.convert(ctx, run)
}

// StartIndexRebuild records a rebuild and runs it detached, for the dashboard.
func (s *Service) StartIndexRebuild(ctx context.Context, indexName, triggeredBy string) (*Run, error) {
	run, d, err := s.recordRebuild(ctx, indexName, triggeredBy)
	if err != nil {
		return nil, err
	}

	go func() {
		detached := context.WithoutCancel(ctx)
		if err := s.rebuild(detached, run, d); err != nil {
			failedIndexRebuilds.Store(d.Name, struct{}{})
		}
		s.startQueuedDroppedIndexes(detached, d.Name)
	}()

	return run, nil
}

// RunIndexRebuild rebuilds on the caller's goroutine, for the CLI.
//
// It goes through the same record step for the reason Run does: that record takes
// the instance-wide single-active slot, and a rebuild started from a shell must
// not run beside a conversion of the table the index is on.
func (s *Service) RunIndexRebuild(ctx context.Context, indexName, triggeredBy string) error {
	run, d, err := s.recordRebuild(ctx, indexName, triggeredBy)
	if err != nil {
		return err
	}
	return s.rebuild(ctx, run, d)
}

const bootTriggeredBy = "boot"

// StartQueuedDroppedIndexes starts the concurrent rebuild of every index still
// owed, in ListDropped order (guard, unique, deliveries list, then the rest). Failure policy: fail open. Queries seq-scan until
// an index is valid; ingest and HTTP must not wait on the build. A replica that
// finds the slot taken, or a name that is already rebuilt, is success. Any
// other error is logged and ignored so the process still listens.
func (s *Service) StartQueuedDroppedIndexes(ctx context.Context) {
	s.startQueuedDroppedIndexes(ctx, "")
}

func (s *Service) startQueuedDroppedIndexes(ctx context.Context, skip string) {
	if s.rebuilder == nil {
		return
	}
	owed, err := s.rebuilder.listDropped(ctx)
	if err != nil {
		s.logger.Error("dropped index list did not load", "error", err.Error())
		return
	}
	for _, d := range owed {
		if d.Name == skip {
			continue
		}
		if _, failed := failedIndexRebuilds.Load(d.Name); failed {
			continue
		}
		s.startQueuedBootIndex(ctx, d.Name, bootIndexLogName(d.Name))
	}
}

func (s *Service) startQueuedBootIndex(ctx context.Context, name, logName string) {
	_, err := s.StartIndexRebuild(ctx, name, bootTriggeredBy)
	if err == nil {
		s.logger.Info("started "+logName, "index", name)
		return
	}
	if errors.Is(err, indexes.ErrNotDropped) || errors.Is(err, ErrRunInProgress) {
		return
	}
	s.logger.Error(logName+" did not start", "index", name, "error", err.Error())
}

// checkGuard refuses a start when the index that enforces one run at a time is
// not valid.
//
// That index is the whole mechanism, and it is created CONCURRENTLY by a
// migration, which puts it in the class of index a killed build leaves invalid
// and the repair migration then drops. While it is gone, insert has nothing to
// fail on, so two conversions of the same table could run at once. Checked here
// rather than trusted, and fail closed on a read error: starting a rewrite
// without knowing whether anything would stop a second one is the worse outcome.
func (s *Service) checkGuard(ctx context.Context) error {
	var valid bool
	err := s.db.GetDB().QueryRowxContext(ctx, `
        SELECT COALESCE(bool_or(x.indisvalid), FALSE)
        FROM pg_class i
        JOIN pg_namespace n ON n.oid = i.relnamespace
        LEFT JOIN pg_index x ON x.indexrelid = i.oid
        WHERE n.nspname = 'convoy' AND i.relname = $1`, singleActiveGuard).Scan(&valid)
	if err != nil {
		return fmt.Errorf("reading the single-active guard: %w", err)
	}
	if !valid {
		return ErrGuardMissing
	}
	return nil
}

// insertGuardRebuild takes the slot for a rebuild of the guard itself, which is
// the one insert the guard cannot cover because it is what is missing.
//
// Reading "nothing is running" and then inserting would not serialize: two
// starts can both read no open run and both insert, which is exactly what the
// index was refusing. So the read and the insert share one transaction holding an
// advisory lock, and the second start waits for the first, sees its row, and is
// refused. The transaction only records the row; the build itself runs after the
// commit, so nothing waits on the lock for longer than an insert.
func (s *Service) insertGuardRebuild(ctx context.Context, run *Run) (*Run, error) {
	tx, err := s.db.GetDB().BeginTxx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("starting the run record: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	// SET LOCAL, so the timeout goes with the transaction rather than back to the
	// pool. Bounded because waiting forever here would look like a hung start.
	if _, err = tx.ExecContext(ctx, `SET LOCAL lock_timeout = '5s'`); err != nil {
		return nil, fmt.Errorf("bounding the run lock wait: %w", err)
	}
	if _, err = tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock($1)`, guardRebuildLock); err != nil {
		// Someone else is holding it, which is the same answer the guard would
		// have given, so it reads as one rather than as a lock error the caller
		// cannot act on.
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "55P03" {
			return nil, ErrRunInProgress
		}
		return nil, fmt.Errorf("taking the run lock: %w", err)
	}

	var running bool
	err = tx.QueryRowxContext(ctx, `
        SELECT EXISTS (SELECT 1 FROM convoy.partition_runs WHERE status = $1)`, StatusRunning).Scan(&running)
	if err != nil {
		return nil, fmt.Errorf("reading open runs: %w", err)
	}
	if running {
		return nil, ErrRunInProgress
	}

	if _, err = insertRun(ctx, tx, run); err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, fmt.Errorf("recording the run: %w", err)
	}
	return run, nil
}

// record checks the conversion can change the table, then takes the slot.
func (s *Service) record(ctx context.Context, table Table, op Operation, triggeredBy string) (*Run, error) {
	if err := s.checkGuard(ctx); err != nil {
		return nil, err
	}

	if err := s.checkOperation(ctx, table, op); err != nil {
		return nil, err
	}

	return s.insert(ctx, &Run{
		UID:         ulid.Make().String(),
		TableName:   table,
		Operation:   op,
		Status:      StatusRunning,
		TriggeredBy: triggeredBy,
	})
}

// recordRebuild resolves the index, then takes the slot.
//
// The subject is read here, at the decision, for the reason a conversion reads
// the table's shape here: a name that identifies no pending rebuild must be
// refused before the instance-wide slot is held, and a read that fails rejects
// the start rather than beginning work whose subject is unconfirmed. The table
// comes from that record, so a run row cannot name a table the index is not on.
func (s *Service) recordRebuild(ctx context.Context, indexName, triggeredBy string) (*Run, indexes.Dropped, error) {
	d, err := s.rebuilder.dropped(ctx, indexName)
	if err != nil {
		return nil, indexes.Dropped{}, err
	}

	// Rebuilding the guard is the exception, because it is the only work that
	// ends the state the check is objecting to. Requiring the guard to rebuild
	// the guard would leave an instance unable to convert or rebuild anything
	// again. It is safe to allow: the guard is a small index on this table, so
	// its build contends with nothing a conversion touches.
	//
	// What the missing guard would have refused is done in its place, because it
	// is the one insert that is not protected by it. Without this, a second
	// guard rebuild, or one started while a row from before the drop is still
	// running, gets as far as a unique build over duplicate running rows and
	// fails on the key rather than on the reason.
	guardRebuild := indexName == singleActiveGuard
	if !guardRebuild {
		if err := s.checkGuard(ctx); err != nil {
			return nil, indexes.Dropped{}, err
		}
	}

	row := &Run{
		UID:         ulid.Make().String(),
		TableName:   Table(d.Table),
		Operation:   OperationRebuildIndex,
		IndexName:   &indexName,
		Status:      StatusRunning,
		TriggeredBy: triggeredBy,
	}

	insert := s.insert
	if guardRebuild {
		insert = s.insertGuardRebuild
	}

	run, err := insert(ctx, row)
	if err != nil {
		return nil, indexes.Dropped{}, err
	}
	return run, d, nil
}

func (s *Service) insert(ctx context.Context, run *Run) (*Run, error) {
	return insertRun(ctx, s.db.GetDB(), run)
}

// rowQuerier is what both the pool and a transaction offer, so the insert has one
// statement whether it runs on its own or inside the guard rebuild's lock.
type rowQuerier interface {
	QueryRowxContext(ctx context.Context, query string, args ...any) *sqlx.Row
}

func insertRun(ctx context.Context, q rowQuerier, run *Run) (*Run, error) {
	// RETURNING rather than a second read: once the conversion is running, a failed
	// read must not be reported as a failure to start, because the caller would be
	// told nothing happened while a table was being rewritten.
	err := q.QueryRowxContext(ctx, `
        INSERT INTO convoy.partition_runs (id, table_name, operation, index_name, status, triggered_by)
        VALUES ($1, $2, $3, $4, $5, $6)
        RETURNING `+runColumns,
		run.UID, run.TableName, run.Operation, run.IndexName, run.Status, run.TriggeredBy).StructScan(run)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return nil, ErrRunInProgress
		}
		return nil, err
	}

	return run, nil
}

// TableStates reads the shape of every convertible table from Postgres, so a
// caller can offer the operation that applies to each one.
func (s *Service) TableStates(ctx context.Context) ([]TableState, error) {
	tables := Tables()
	names := make([]string, 0, len(tables))
	for _, t := range tables {
		names = append(names, string(t))
	}

	partitioned := make([]string, 0, len(tables))
	err := s.db.GetDB().SelectContext(ctx, &partitioned, `
        SELECT c.relname
        FROM pg_class c
        JOIN pg_namespace n ON n.oid = c.relnamespace
        WHERE n.nspname = 'convoy' AND c.relkind = 'p' AND c.relname = ANY($1)`, pq.Array(names))
	if err != nil {
		return nil, err
	}

	isPartitioned := make(map[string]bool, len(partitioned))
	for _, name := range partitioned {
		isPartitioned[name] = true
	}

	isAdopted, err := s.adoptedTables(ctx, tables)
	if err != nil {
		return nil, err
	}

	states := make([]TableState, 0, len(tables))
	for _, t := range tables {
		states = append(states, TableState{
			Name:        t,
			Partitioned: isPartitioned[string(t)],
			Adopted:     isAdopted[string(t)],
		})
	}
	return states, nil
}

// adoptedTables reports which parents hold the table they were converted from,
// rather than a copy of it.
//
// The marker is the bounds constraint the attach conversion writes on the
// partition it adopts, the same one the unpartition path reads to decide which
// way to convert back. A partition named <table>_default is not enough on its
// own: gopartman provisions its own default, and that one is an empty catch-all.
func (s *Service) adoptedTables(ctx context.Context, tables []Table) (map[string]bool, error) {
	defaults := make([]string, 0, len(tables))
	for _, t := range tables {
		defaults = append(defaults, string(t)+"_default")
	}

	adopted := make([]string, 0, len(tables))
	err := s.db.GetDB().SelectContext(ctx, &adopted, `
        SELECT c.relname
        FROM pg_constraint con
        JOIN pg_class c ON c.oid = con.conrelid
        JOIN pg_namespace n ON n.oid = c.relnamespace
        WHERE n.nspname = 'convoy'
          AND c.relname = ANY($1)
          AND con.conname = c.relname || '_bounds'`, pq.Array(defaults))
	if err != nil {
		return nil, err
	}

	isAdopted := make(map[string]bool, len(adopted))
	for _, name := range adopted {
		isAdopted[strings.TrimSuffix(name, "_default")] = true
	}
	return isAdopted, nil
}

// checkOperation rejects an operation whose result the table already has.
// Conversion is a table rewrite that runs for hours and blocks every other
// conversion while it does, so an operation that cannot change anything must be
// refused before a run is recorded. The state is read here, at the decision, and
// a read that fails rejects the start: beginning a rewrite without knowing the
// table's shape is the worse outcome.
func (s *Service) checkOperation(ctx context.Context, table Table, op Operation) error {
	states, err := s.TableStates(ctx)
	if err != nil {
		return err
	}

	for _, state := range states {
		if state.Name != table {
			continue
		}
		if state.ValidOperation() == op {
			return nil
		}
		if state.Partitioned {
			return fmt.Errorf("%w: %s", ErrAlreadyPartitioned, table)
		}
		return fmt.Errorf("%w: %s", ErrNotPartitioned, table)
	}

	return fmt.Errorf("%w %q", ErrUnknownTable, table)
}

// convert runs the DDL with this run observing the notice stream.
//
// The observer is pool-wide, so a notice raised by an unrelated statement during
// the conversion is recorded as this run's phase. That is a reporting
// imprecision only, and it cannot affect the conversion or the run's status:
// phase and notice_count are display fields. Attributing notices exactly needs
// the DDL pinned to a known connection, which means the repository exposing the
// statement it runs rather than only a method that runs it.
func (s *Service) convert(ctx context.Context, run *Run) error {
	return s.execute(ctx, run, func(ctx context.Context) error {
		return s.converter.run(ctx, run.TableName, run.Operation)
	})
}

// rebuild builds the index back under the same reporting and close-out a
// conversion gets. The rebuild raises notices of its own, including the one for
// clearing a leftover from an earlier attempt, so the phase stream is as useful
// here as it is for a conversion.
func (s *Service) rebuild(ctx context.Context, run *Run, d indexes.Dropped) error {
	return s.execute(ctx, run, func(ctx context.Context) error {
		return s.rebuilder.rebuild(ctx, d)
	})
}

func bootIndexLogName(name string) string {
	switch name {
	case indexes.PayloadGIN:
		return "payload search index rebuild"
	case indexes.EventDeliveriesProjectCreated:
		return "event deliveries list index rebuild"
	default:
		return "index rebuild"
	}
}

// execute runs the work with this run observing notices, and closes the row out
// however the work ends.
func (s *Service) execute(ctx context.Context, run *Run, work func(context.Context) error) error {
	if observer, ok := s.db.(noticeObserver); ok {
		observer.OnNotice(func(n *pgconn.Notice) { s.recordPhase(ctx, run.UID, n) })
		defer observer.OnNotice(nil)
	}

	// A panic here would otherwise leave the row at running forever, and one
	// running row blocks every later run, so an operator would have to find and
	// clear it by hand before the instance could convert or rebuild anything
	// again. Recorded as failed with the panic, then rethrown so it still
	// reaches the process's handler and the stack is not swallowed.
	defer func() {
		if p := recover(); p != nil {
			s.finish(ctx, run.UID, fmt.Errorf("run panicked: %v", p))
			panic(p)
		}
	}()

	err := work(ctx)
	s.finish(ctx, run.UID, err)
	return err
}

// recordPhase appends the notice to the run's step list and keeps it as the
// current phase, so another session can see both where a conversion has reached
// and how it got there.
//
// Any notice raised on the pool while a run is open is recorded, not only the
// conversion's own: a notice carries no way to tell which statement raised it.
// In practice the conversion is the only thing raising them, and the alternative
// is reporting nothing.
//
// The list is capped by dropping its oldest entry, rather than by refusing to
// append. A conversion reports around ten steps, so the cap is only reached by
// something unexpected, and in that case the recent steps are the ones worth
// keeping.
func (s *Service) recordPhase(ctx context.Context, id string, n *pgconn.Notice) {
	if n == nil {
		return
	}

	_, err := s.db.GetDB().ExecContext(ctx, `
        UPDATE convoy.partition_runs
        SET phase = $2,
            steps = CASE WHEN jsonb_array_length(steps) >= $4 THEN steps - 0 ELSE steps END
                    || jsonb_build_object('message', $2::TEXT, 'at', NOW()),
            notice_count = notice_count + 1,
            updated_at = NOW()
        WHERE id = $1 AND status = $3`,
		id, n.Message, StatusRunning, maxSteps)
	if err != nil {
		// Losing a progress update must not affect the conversion, which is the
		// thing that matters and is still reporting to the log.
		s.logger.Warn("failed to record partition phase", "run_id", id, "error", err.Error())
	}
}

func (s *Service) finish(ctx context.Context, id string, runErr error) {
	status := StatusCompleted
	var message *string
	if runErr != nil {
		status = StatusFailed
		text := runErr.Error()
		message = &text
	}

	// Closing out survives the cancellation that ended the run. A CLI
	// conversion runs on the command's context, and interrupting it cancels the
	// conversion and this write together, which would leave the row at running.
	// One running row blocks every later conversion, so the interrupt an
	// operator meant to stop one table would lock the instance out of all four.
	ctx = context.WithoutCancel(ctx)

	_, err := s.db.GetDB().ExecContext(ctx, `
        UPDATE convoy.partition_runs
        SET status = $2, error = $3, updated_at = NOW(), completed_at = NOW()
        WHERE id = $1`,
		id, status, message)
	if err != nil {
		// The row stays 'running' and blocks the next conversion until an
		// operator clears it. That is the fail-closed side: the alternative is
		// reporting a conversion finished when nothing confirmed it did.
		s.logger.Error("failed to close out partition run", "run_id", id, "error", err.Error())
		return
	}

	if runErr != nil {
		s.logger.Error("partition run failed", "run_id", id, "error", runErr.Error())
		return
	}
	s.logger.Info("partition run completed", "run_id", id)
}

func (s *Service) Get(ctx context.Context, id string) (*Run, error) {
	var run Run
	err := s.db.GetDB().QueryRowxContext(ctx, `
        SELECT `+runColumns+`
        FROM convoy.partition_runs WHERE id = $1`, id).StructScan(&run)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrRunNotFound
	}
	if err != nil {
		return nil, err
	}
	return &run, nil
}

// List returns recent runs, newest first, so a UI can show what a conversion did
// last time as well as what it is doing now.
func (s *Service) List(ctx context.Context, limit int) ([]Run, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}

	runs := make([]Run, 0)
	err := s.db.GetDB().SelectContext(ctx, &runs, `
        SELECT `+runColumns+`
        FROM convoy.partition_runs ORDER BY started_at DESC LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	return runs, nil
}

// indexRebuilder is the indexes package behind the rebuilder seam. It holds no
// state of its own so that the pool is read at call time, the same way the
// conversion repositories are.
type indexRebuilder struct {
	db database.Database
}

func (r *indexRebuilder) dropped(ctx context.Context, name string) (indexes.Dropped, error) {
	return indexes.GetDropped(ctx, r.db.GetConn(), name)
}

func (r *indexRebuilder) listDropped(ctx context.Context) ([]indexes.Dropped, error) {
	return indexes.ListDropped(ctx, r.db.GetConn())
}

func (r *indexRebuilder) rebuild(ctx context.Context, d indexes.Dropped) error {
	return indexes.Rebuild(ctx, r.db.GetConn(), d)
}

// repoConverter maps a table and operation onto the repository method that owns
// the DDL, so the SQL has one home and this package does not restate it.
type repoConverter struct {
	events     *events.Service
	deliveries *event_deliveries.Service
	attempts   *delivery_attempts.Service
}

func (c *repoConverter) run(ctx context.Context, table Table, op Operation) error {
	partition := op == OperationPartition

	switch table {
	case TableEvents:
		if partition {
			return c.events.PartitionEventsTable(ctx)
		}
		return c.events.UnPartitionEventsTable(ctx)
	case TableEventsSearch:
		if partition {
			return c.events.PartitionEventsSearchTable(ctx)
		}
		return c.events.UnPartitionEventsSearchTable(ctx)
	case TableEventDeliveries:
		if partition {
			return c.deliveries.PartitionEventDeliveriesTable(ctx)
		}
		return c.deliveries.UnPartitionEventDeliveriesTable(ctx)
	case TableDeliveryAttempts:
		if partition {
			return c.attempts.PartitionDeliveryAttemptsTable(ctx)
		}
		return c.attempts.UnPartitionDeliveryAttemptsTable(ctx)
	default:
		return fmt.Errorf("%w %q", ErrUnknownTable, table)
	}
}
