package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/frain-dev/convoy/database"
	"github.com/frain-dev/convoy/datastore"
)

type batchRetryRepo struct {
	db database.Database
}

func NewBatchRetryRepo(db database.Database) datastore.BatchRetryRepository {
	return &batchRetryRepo{db: db}
}

func (r *batchRetryRepo) CreateBatchRetry(ctx context.Context, batchRetry *datastore.BatchRetry) error {
	batchRetryFilter, err := json.Marshal(batchRetry.Filter)
	if err != nil {
		return err
	}

	_, err = r.db.GetDB().NamedExecContext(ctx, insertBatchRetry, map[string]interface{}{
		"id":               batchRetry.ID,
		"project_id":       batchRetry.ProjectID,
		"status":           batchRetry.Status,
		"total_events":     batchRetry.TotalEvents,
		"processed_events": batchRetry.ProcessedEvents,
		"failed_events":    batchRetry.FailedEvents,
		"filter":           batchRetryFilter,
		"created_at":       batchRetry.CreatedAt,
		"updated_at":       batchRetry.UpdatedAt,
		"completed_at":     batchRetry.CompletedAt.Time,
		"error":            batchRetry.Error,
	})

	return err
}

func (r *batchRetryRepo) UpdateBatchRetry(ctx context.Context, batchRetry *datastore.BatchRetry) error {

	result, err := r.db.GetDB().NamedExecContext(ctx, updateBatchRetry, map[string]interface{}{
		"id":               batchRetry.ID,
		"project_id":       batchRetry.ProjectID,
		"status":           batchRetry.Status,
		"total_events":     batchRetry.TotalEvents,
		"processed_events": batchRetry.ProcessedEvents,
		"failed_events":    batchRetry.FailedEvents,
		"filter":           batchRetry.Filter,
		"updated_at":       batchRetry.UpdatedAt,
		"completed_at":     batchRetry.CompletedAt.Time,
		"error":            batchRetry.Error,
	})
	if err != nil {
		return err
	}

	rowCount, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowCount < 1 {
		return fmt.Errorf("no rows affected")
	}

	return nil
}

func (r *batchRetryRepo) FindBatchRetryByID(ctx context.Context, id string) (*datastore.BatchRetry, error) {
	var batchRetry datastore.BatchRetry
	err := r.db.GetDB().GetContext(ctx, &batchRetry, findBatchRetryByID, id)
	if err != nil {
		return nil, err
	}

	return &batchRetry, nil
}

func (r *batchRetryRepo) FindActiveBatchRetry(ctx context.Context, projectID string) (*datastore.BatchRetry, error) {
	var batchRetry datastore.BatchRetry
	err := r.db.GetDB().GetContext(ctx, &batchRetry,
		fetchActiveRetry, projectID,
		datastore.BatchRetryStatusPending,
		datastore.BatchRetryStatusProcessing)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}

		return nil, err
	}

	return &batchRetry, nil
}

const (
	insertBatchRetry = `
	INSERT INTO convoy.batch_retries (
		id, project_id, status, total_events, processed_events, failed_events,
		filter, created_at, updated_at, completed_at, error
	) VALUES (
		:id, :project_id, :status, :total_events, :processed_events, :failed_events,
		:filter, :created_at, :updated_at, :completed_at, :error
	)`

	fetchActiveRetry = `
	SELECT * FROM convoy.batch_retries 
	WHERE project_id = $1
	AND status IN ($2, $3)
	ORDER BY created_at DESC
	LIMIT 1`

	findBatchRetryByID = `SELECT * FROM convoy.batch_retries WHERE id = $1`

	updateBatchRetry = `
	UPDATE convoy.batch_retries SET
		status = :status,
		processed_events = :processed_events,
		failed_events = :failed_events,
		updated_at = :updated_at,
		filter = :filter,
		total_events = :total_events,
		completed_at = :completed_at,
		error = :error
	WHERE id = :id and project_id = :project_id`
)
