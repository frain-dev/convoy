// Package partitions records the progress of partition conversions.
//
// Converting a table rewrites it, which takes minutes to hours on a large
// instance. The DDL runs as a single statement, so it cannot report progress by
// writing to a table: nothing it wrote would be visible to another session until
// the whole conversion committed, which is the exact window that needs
// reporting. What does escape the statement is its RAISE NOTICE stream, which
// pgx delivers as each notice arrives. This package turns that stream into a row
// other sessions can read, so progress survives the process and reaches a UI.
package partitions

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
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

var (
	// ErrRunInProgress is returned when a conversion is already running. One at a
	// time is deliberate: each conversion rewrites a table and saturates disk
	// doing it, and the caller must not queue behind a multi-hour operation
	// without being told.
	ErrRunInProgress = errors.New("a partition run is already in progress")

	ErrRunNotFound = errors.New("partition run not found")

	ErrUnknownTable = errors.New("unknown table")
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

type Run struct {
	UID         string     `json:"uid" db:"id"`
	TableName   Table      `json:"table_name" db:"table_name"`
	Operation   Operation  `json:"operation" db:"operation"`
	Status      Status     `json:"status" db:"status"`
	Phase       *string    `json:"phase" db:"phase"`
	NoticeCount int64      `json:"notice_count" db:"notice_count"`
	Error       *string    `json:"error" db:"error"`
	TriggeredBy string     `json:"triggered_by" db:"triggered_by"`
	StartedAt   time.Time  `json:"started_at" db:"started_at"`
	UpdatedAt   time.Time  `json:"updated_at" db:"updated_at"`
	CompletedAt *time.Time `json:"completed_at" db:"completed_at"`
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
        RETURNING id, table_name, operation, status, phase, notice_count, error,
                  triggered_by, started_at, updated_at, completed_at`,
		run.UID, run.TableName, run.Operation, run.Status, run.TriggeredBy).StructScan(run)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return nil, ErrRunInProgress
		}
		return nil, err
	}

	// The conversion is detached from the request that started it, so it keeps
	// running after the response is written. Cancelling it midway would leave a
	// half-converted table, which is worse than letting it finish.
	go s.convert(context.WithoutCancel(ctx), run)

	return run, nil
}

// convert runs the DDL with this run observing the notice stream.
//
// The observer is pool-wide, so a notice raised by an unrelated statement during
// the conversion is recorded as this run's phase. That is a reporting
// imprecision only, and it cannot affect the conversion or the run's status:
// phase and notice_count are display fields. Attributing notices exactly needs
// the DDL pinned to a known connection, which means the repository exposing the
// statement it runs rather than only a method that runs it.
func (s *Service) convert(ctx context.Context, run *Run) {
	if observer, ok := s.db.(noticeObserver); ok {
		observer.OnNotice(func(n *pgconn.Notice) { s.recordPhase(ctx, run.UID, n) })
		defer observer.OnNotice(nil)
	}

	err := s.converter.run(ctx, run.TableName, run.Operation)
	s.finish(ctx, run.UID, err)
}

// recordPhase writes the latest notice to the run. Only the newest phase is kept
// because the full stream is already in the log; this row exists so another
// session can see where a conversion has reached.
func (s *Service) recordPhase(ctx context.Context, id string, n *pgconn.Notice) {
	if n == nil {
		return
	}

	_, err := s.db.GetDB().ExecContext(ctx, `
        UPDATE convoy.partition_runs
        SET phase = $2, notice_count = notice_count + 1, updated_at = NOW()
        WHERE id = $1 AND status = $3`,
		id, n.Message, StatusRunning)
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
        SELECT id, table_name, operation, status, phase, notice_count, error,
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
        SELECT id, table_name, operation, status, phase, notice_count, error,
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
