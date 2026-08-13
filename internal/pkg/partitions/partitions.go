// Package partitions records the progress of partition conversions.
//
// A conversion takes minutes to hours on a large instance, and progress has to
// reach a session other than the one running it. Writing to a table directly
// does not achieve that for every conversion: unpartitioning still runs as a
// single statement, so nothing it wrote would be visible until the whole thing
// committed, which is the exact window that needs reporting. What does escape a
// running statement is its RAISE NOTICE stream, which pgx delivers as each
// notice arrives.
//
// Partitioning event_deliveries is no longer one statement. It attaches the
// existing table as a partition instead of copying it, and its phases are
// separate committed steps, so its notices mark real boundaries rather than
// points inside an uncommitted rewrite. Both directions still report the same
// way, so this package has one mechanism to maintain rather than two.
package partitions

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/lib/pq"
	"github.com/oklog/ulid/v2"

	"github.com/frain-dev/convoy/database"
	"github.com/frain-dev/convoy/internal/delivery_attempts"
	"github.com/frain-dev/convoy/internal/event_deliveries"
	"github.com/frain-dev/convoy/internal/events"
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
)

type Status string

const (
	StatusRunning   Status = "running"
	StatusCompleted Status = "completed"
	StatusFailed    Status = "failed"
)

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

	ErrUnknownTable = errors.New("unknown table")

	// ErrAlreadyPartitioned and ErrNotPartitioned are returned when the table is
	// already in the shape the operation would produce, so the conversion has
	// nothing to do.
	ErrAlreadyPartitioned = errors.New("table is already partitioned")
	ErrNotPartitioned     = errors.New("table is not partitioned")
)

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
	UID         string     `json:"uid" db:"id"`
	TableName   Table      `json:"table_name" db:"table_name"`
	Operation   Operation  `json:"operation" db:"operation"`
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

type Service struct {
	db        database.Database
	logger    log.Logger
	converter converter
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
	// half-converted table, which is worse than letting it finish.
	go s.convert(context.WithoutCancel(ctx), run)

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

func (s *Service) record(ctx context.Context, table Table, op Operation, triggeredBy string) (*Run, error) {
	if err := s.checkOperation(ctx, table, op); err != nil {
		return nil, err
	}

	run := &Run{
		UID:         ulid.Make().String(),
		TableName:   table,
		Operation:   op,
		Status:      StatusRunning,
		TriggeredBy: triggeredBy,
	}

	// RETURNING rather than a second read: once the conversion is running, a failed
	// read must not be reported as a failure to start, because the caller would be
	// told nothing happened while a table was being rewritten.
	err := s.db.GetDB().QueryRowxContext(ctx, `
        INSERT INTO convoy.partition_runs (id, table_name, operation, status, triggered_by)
        VALUES ($1, $2, $3, $4, $5)
        RETURNING id, table_name, operation, status, phase, steps, notice_count, error,
                  triggered_by, started_at, updated_at, completed_at`,
		run.UID, run.TableName, run.Operation, run.Status, run.TriggeredBy).StructScan(run)
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
	if observer, ok := s.db.(noticeObserver); ok {
		observer.OnNotice(func(n *pgconn.Notice) { s.recordPhase(ctx, run.UID, n) })
		defer observer.OnNotice(nil)
	}

	// A panic here would otherwise leave the row at running forever, and one
	// running row blocks every later conversion, so an operator would have to
	// find and clear it by hand before the instance could convert anything
	// again. Recorded as failed with the panic, then rethrown so it still
	// reaches the process's handler and the stack is not swallowed.
	defer func() {
		if p := recover(); p != nil {
			s.finish(ctx, run.UID, fmt.Errorf("conversion panicked: %v", p))
			panic(p)
		}
	}()

	err := s.converter.run(ctx, run.TableName, run.Operation)
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
        SELECT id, table_name, operation, status, phase, steps, notice_count, error,
               triggered_by, started_at, updated_at, completed_at
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
        SELECT id, table_name, operation, status, phase, steps, notice_count, error,
               triggered_by, started_at, updated_at, completed_at
        FROM convoy.partition_runs ORDER BY started_at DESC LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	return runs, nil
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
