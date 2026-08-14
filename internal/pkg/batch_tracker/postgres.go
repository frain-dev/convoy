package batch_tracker

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/jmoiron/sqlx"
	"github.com/oklog/ulid/v2"
)

type Tracker interface {
	GenerateBatchID() string
	CreateBatch(context.Context, string, int64, string, string, string) error
	UpdateProgress(context.Context, string, int64, int64) error
	CompleteBatch(context.Context, string) error
	FailBatch(context.Context, string, string) error
	GetBatch(context.Context, string) (*BatchProgress, error)
	IncrementProcessed(context.Context, string, int64) error
	IncrementFailed(context.Context, string, int64) error
	IncrementTotal(context.Context, string, int64) error
	SyncCounters(context.Context, string) error
	ListBatches(context.Context) ([]*BatchProgress, error)
	DeleteBatch(context.Context, string) error
}

type PostgresTracker struct {
	db *sqlx.DB
}

func NewPostgresTracker(db *sqlx.DB) Tracker {
	return &PostgresTracker{db: db}
}

func (t *PostgresTracker) GenerateBatchID() string {
	return ulid.Make().String()
}

func (t *PostgresTracker) CreateBatch(ctx context.Context, id string, total int64, statusFilter, period, eventID string) error {
	_, err := t.db.ExecContext(ctx, `
		INSERT INTO convoy.batch_retry_progress
			(batch_id, status, total_count, processed_count, failed_count, start_time, status_filter, time_period, event_id, expires_at)
		VALUES ($1, $2, $3, 0, 0, NOW(), $4, $5, $6, NOW() + INTERVAL '24 hours')`,
		id, BatchStatusRunning, total, statusFilter, period, eventID)
	return err
}

func (t *PostgresTracker) UpdateProgress(ctx context.Context, id string, processed, failed int64) error {
	_, err := t.db.ExecContext(ctx, `
		UPDATE convoy.batch_retry_progress
		SET processed_count = $2, failed_count = $3
		WHERE batch_id = $1`, id, processed, failed)
	return err
}

func (t *PostgresTracker) CompleteBatch(ctx context.Context, id string) error {
	_, err := t.db.ExecContext(ctx, `
		UPDATE convoy.batch_retry_progress
		SET status = $2, end_time = NOW()
		WHERE batch_id = $1`, id, BatchStatusCompleted)
	return err
}

func (t *PostgresTracker) FailBatch(ctx context.Context, id, message string) error {
	_, err := t.db.ExecContext(ctx, `
		UPDATE convoy.batch_retry_progress
		SET status = $2, end_time = NOW(), error = $3
		WHERE batch_id = $1`, id, BatchStatusFailed, message)
	return err
}

func (t *PostgresTracker) GetBatch(ctx context.Context, id string) (*BatchProgress, error) {
	var progress BatchProgress
	err := t.db.GetContext(ctx, &progress, `
		SELECT batch_id, status, total_count, processed_count, failed_count,
		       start_time, end_time, error, status_filter, time_period, event_id
		FROM convoy.batch_retry_progress
		WHERE batch_id = $1 AND expires_at > NOW()`, id)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("batch not found: %s", id)
	}
	return &progress, err
}

func (t *PostgresTracker) IncrementProcessed(ctx context.Context, id string, count int64) error {
	return t.increment(ctx, id, "processed_count", count)
}

func (t *PostgresTracker) IncrementFailed(ctx context.Context, id string, count int64) error {
	return t.increment(ctx, id, "failed_count", count)
}

func (t *PostgresTracker) IncrementTotal(ctx context.Context, id string, count int64) error {
	return t.increment(ctx, id, "total_count", count)
}

func (t *PostgresTracker) increment(ctx context.Context, id, column string, count int64) error {
	queries := map[string]string{
		"processed_count": `UPDATE convoy.batch_retry_progress SET processed_count = processed_count + $2 WHERE batch_id = $1`,
		"failed_count":    `UPDATE convoy.batch_retry_progress SET failed_count = failed_count + $2 WHERE batch_id = $1`,
		"total_count":     `UPDATE convoy.batch_retry_progress SET total_count = total_count + $2 WHERE batch_id = $1`,
	}
	query, ok := queries[column]
	if !ok {
		return fmt.Errorf("unsupported batch counter %q", column)
	}
	_, err := t.db.ExecContext(ctx, query, id, count)
	return err
}

func (t *PostgresTracker) SyncCounters(context.Context, string) error {
	return nil
}

func (t *PostgresTracker) ListBatches(ctx context.Context) ([]*BatchProgress, error) {
	var batches []*BatchProgress
	err := t.db.SelectContext(ctx, &batches, `
		SELECT batch_id, status, total_count, processed_count, failed_count,
		       start_time, end_time, error, status_filter, time_period, event_id
		FROM convoy.batch_retry_progress
		WHERE expires_at > NOW()
		ORDER BY start_time DESC`)
	return batches, err
}

func (t *PostgresTracker) DeleteBatch(ctx context.Context, id string) error {
	_, err := t.db.ExecContext(ctx, `DELETE FROM convoy.batch_retry_progress WHERE batch_id = $1`, id)
	return err
}

var _ Tracker = (*BatchTracker)(nil)
var _ Tracker = (*PostgresTracker)(nil)
