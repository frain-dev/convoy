package retention

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/oklog/ulid/v2"

	"github.com/frain-dev/convoy/database"
)

const (
	runColumns       = `id, status, retention_period, details, error, started_at, completed_at`
	retentionHistory = 90 * 24 * time.Hour
)

type RunStatus string

const (
	RunStatusRunning   RunStatus = "running"
	RunStatusCompleted RunStatus = "completed"
	RunStatusFailed    RunStatus = "failed"
	RunStatusSkipped   RunStatus = "skipped"
)

// TableDropDetail is one retention-managed table's outcome for a nightly run.
type TableDropDetail struct {
	Table                string           `json:"table"`
	DroppedPartitions    []string         `json:"dropped_partitions"`
	DroppedDefault       bool             `json:"dropped_default"`
	DroppedRows          int64            `json:"dropped_rows,omitempty"`
	DroppedDefaultRows   int64            `json:"dropped_default_rows,omitempty"`
	DroppedPartitionRows map[string]int64 `json:"dropped_partition_rows,omitempty"`
	MaintainError        *string          `json:"maintain_error,omitempty"`
	// NotPartitioned is set on skipped runs when the table still needs conversion
	// before partman retention can manage it.
	NotPartitioned bool `json:"not_partitioned,omitempty"`
}

type RunDetails []TableDropDetail

type Run struct {
	UID             string     `json:"uid" db:"id"`
	Status          RunStatus  `json:"status" db:"status"`
	RetentionPeriod string     `json:"retention_period" db:"retention_period"`
	Details         RunDetails `json:"details" db:"details"`
	Error           *string    `json:"error" db:"error"`
	StartedAt       time.Time  `json:"started_at" db:"started_at"`
	CompletedAt     *time.Time `json:"completed_at" db:"completed_at"`
}

var ErrRunNotFound = errors.New("retention run not found")

func (d *RunDetails) Scan(src any) error {
	var data []byte
	switch v := src.(type) {
	case nil:
		*d = nil
		return nil
	case []byte:
		data = v
	case string:
		data = []byte(v)
	default:
		return fmt.Errorf("retention run details: unsupported type %T", src)
	}
	if len(data) == 0 {
		*d = RunDetails{}
		return nil
	}
	return json.Unmarshal(data, d)
}

// Value implements driver.Valuer so Finish can bind details into JSONB.
func (d RunDetails) Value() (driver.Value, error) {
	if d == nil {
		return []byte("[]"), nil
	}
	b, err := json.Marshal(d)
	if err != nil {
		return nil, err
	}
	return b, nil
}

type RunStore struct {
	db database.Database
}

func NewRunStore(db database.Database) *RunStore {
	return &RunStore{db: db}
}

func (s *RunStore) Begin(ctx context.Context, period time.Duration) (string, error) {
	id := ulid.Make().String()
	_, err := s.db.GetDB().ExecContext(ctx, `
        INSERT INTO convoy.retention_runs (id, status, retention_period, details)
        VALUES ($1, $2, $3, '[]'::JSONB)`,
		id, RunStatusRunning, period.String())
	if err != nil {
		return "", err
	}
	return id, nil
}

func (s *RunStore) Finish(ctx context.Context, id string, status RunStatus, details RunDetails, runErr error) error {
	var message *string
	if runErr != nil {
		text := runErr.Error()
		message = &text
	}
	if details == nil {
		details = RunDetails{}
	}

	ctx = context.WithoutCancel(ctx)
	_, err := s.db.GetDB().ExecContext(ctx, `
        UPDATE convoy.retention_runs
        SET status = $2, details = $3, error = $4, completed_at = NOW()
        WHERE id = $1`,
		id, status, details, message)
	if err != nil {
		return err
	}
	return s.prune(ctx)
}

func (s *RunStore) Get(ctx context.Context, id string) (*Run, error) {
	var run Run
	err := s.db.GetDB().QueryRowxContext(ctx, `
        SELECT `+runColumns+`
        FROM convoy.retention_runs WHERE id = $1`, id).StructScan(&run)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrRunNotFound
	}
	if err != nil {
		return nil, err
	}
	return &run, nil
}

func (s *RunStore) List(ctx context.Context, limit int) ([]Run, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}

	runs := make([]Run, 0)
	err := s.db.GetDB().SelectContext(ctx, &runs, `
        SELECT `+runColumns+`
        FROM convoy.retention_runs ORDER BY started_at DESC LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	return runs, nil
}

func (s *RunStore) prune(ctx context.Context) error {
	_, err := s.db.GetDB().ExecContext(ctx, `
        DELETE FROM convoy.retention_runs
        WHERE started_at < NOW() - $1::INTERVAL`,
		fmt.Sprintf("%f seconds", retentionHistory.Seconds()))
	return err
}

type partitionSnapshot map[string]map[string]struct{}

func snapshotManagedPartitions(ctx context.Context, db database.Database) (partitionSnapshot, error) {
	rows, err := db.GetDB().QueryxContext(ctx, `
        SELECT t.table_name, p.name
        FROM partman.partitions p
        JOIN partman.parent_tables t ON t.id = p.parent_table_id
        WHERE t.schema_name = $1 AND t.table_name = ANY($2) AND p.status = 'active'`,
		retentionSchema, RetentionTables)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "42P01" {
			return partitionSnapshot{}, nil
		}
		return nil, err
	}
	defer rows.Close()

	out := make(partitionSnapshot, len(RetentionTables))
	for rows.Next() {
		var table, name string
		if err := rows.Scan(&table, &name); err != nil {
			return nil, err
		}
		if out[table] == nil {
			out[table] = make(map[string]struct{})
		}
		out[table][name] = struct{}{}
	}
	return out, rows.Err()
}

// countExpiredPartitionRowCounts counts rows only in partitions partman will
// drop this run (same filter as gopartman ListExpiredPartitions). Best-effort:
// a failed count on one partition must not block Maintain.
func countExpiredPartitionRowCounts(ctx context.Context, db database.Database) map[string]map[string]int64 {
	rows, err := db.GetDB().QueryxContext(ctx, `
        SELECT t.table_name, p.name
        FROM partman.partitions p
        JOIN partman.parent_tables t ON t.id = p.parent_table_id
        WHERE t.schema_name = $1 AND t.table_name = ANY($2)
          AND p.partition_bounds_to <= NOW() - t.retention_period
          AND p.is_default = false AND p.status = 'active'`,
		retentionSchema, RetentionTables)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "42P01" {
			return nil
		}
		return nil
	}
	defer rows.Close()

	out := make(map[string]map[string]int64, len(RetentionTables))
	for rows.Next() {
		var table, name string
		if err := rows.Scan(&table, &name); err != nil {
			return out
		}
		count, err := countQualifiedRelation(ctx, db, name)
		if err != nil || count == 0 {
			continue
		}
		if out[table] == nil {
			out[table] = make(map[string]int64)
		}
		out[table][name] = count
	}
	return out
}

func countQualifiedRelation(ctx context.Context, db database.Database, qualifiedName string) (int64, error) {
	schema, rel, ok := strings.Cut(qualifiedName, ".")
	if !ok {
		schema = retentionSchema
		rel = qualifiedName
	}

	var count int64
	err := db.GetDB().QueryRowContext(ctx, fmt.Sprintf(`SELECT count(*) FROM %s.%s`, schema, rel)).Scan(&count)
	if err != nil {
		var pgErr *pgconn.PgError
		// partman can list premade partitions before the child relation exists.
		if errors.As(err, &pgErr) && pgErr.Code == "42P01" {
			return 0, nil
		}
	}
	return count, err
}

func emptyDetails() RunDetails {
	details := make(RunDetails, 0, len(RetentionTables))
	for _, table := range RetentionTables {
		details = append(details, TableDropDetail{
			Table:             table,
			DroppedPartitions: []string{},
		})
	}
	return details
}

func diffPartitionDrops(before, after partitionSnapshot, beforeCounts map[string]map[string]int64) RunDetails {
	details := emptyDetails()
	for i, table := range RetentionTables {
		b := before[table]
		a := after[table]
		var (
			dropped   []string
			rowTotal  int64
			rowByPart map[string]int64
		)
		for name := range b {
			if _, ok := a[name]; !ok {
				dropped = append(dropped, name)
				if rows := beforeCounts[table][name]; rows > 0 {
					rowTotal += rows
					if rowByPart == nil {
						rowByPart = make(map[string]int64)
					}
					rowByPart[name] = rows
				}
			}
		}
		sort.Strings(dropped)
		if dropped == nil {
			dropped = []string{}
		}
		details[i].DroppedPartitions = dropped
		details[i].DroppedRows = rowTotal
		details[i].DroppedPartitionRows = rowByPart
	}
	return details
}

func detailsIndex(details RunDetails) map[string]int {
	idx := make(map[string]int, len(details))
	for i, d := range details {
		idx[d.Table] = i
	}
	return idx
}
