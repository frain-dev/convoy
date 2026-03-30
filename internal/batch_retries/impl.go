package batch_retries

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/frain-dev/convoy/database"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/internal/batch_retries/repo"
	"github.com/frain-dev/convoy/internal/common"
	log "github.com/frain-dev/convoy/pkg/logger"
	"github.com/frain-dev/convoy/util"
)

// Service implements the BatchRetryRepository using SQLc-generated queries
type Service struct {
	logger log.Logger
	repo   repo.Querier  // SQLc-generated interface
	db     *pgxpool.Pool // Connection pool
}

// Ensure Service implements datastore.BatchRetryRepository at compile time
var _ datastore.BatchRetryRepository = (*Service)(nil)

// New creates a new Batch Retry Service
func New(logger log.Logger, db database.Database) *Service {
	return &Service{
		logger: logger,
		repo:   repo.New(db.GetConn()),
		db:     db.GetConn(),
	}
}

// rowToBatchRetry converts an SQLc-generated row struct to datastore.BatchRetry
func rowToBatchRetry(row interface{}) (*datastore.BatchRetry, error) {
	var (
		id, projectID, status                      string
		totalEvents, processedEvents, failedEvents int32
		filter                                     []byte
		createdAt, updatedAt, completedAt          pgtype.Timestamptz
		errorMsg                                   pgtype.Text
	)

	switch r := row.(type) {
	case repo.FindBatchRetryByIDRow:
		id, projectID, status = r.ID, r.ProjectID, r.Status
		totalEvents, processedEvents, failedEvents = r.TotalEvents, r.ProcessedEvents, r.FailedEvents
		filter = r.Filter
		createdAt, updatedAt, completedAt = r.CreatedAt, r.UpdatedAt, r.CompletedAt
		errorMsg = r.Error
	case repo.FindActiveBatchRetryRow:
		id, projectID, status = r.ID, r.ProjectID, r.Status
		totalEvents, processedEvents, failedEvents = r.TotalEvents, r.ProcessedEvents, r.FailedEvents
		filter = r.Filter
		createdAt, updatedAt, completedAt = r.CreatedAt, r.UpdatedAt, r.CompletedAt
		errorMsg = r.Error
	default:
		return nil, fmt.Errorf("unsupported row type: %T", row)
	}

	retryFilter, err := common.JSONBToRetryFilter(filter)
	if err != nil {
		return nil, fmt.Errorf("failed to parse filter: %w", err)
	}

	return &datastore.BatchRetry{
		ID:              id,
		ProjectID:       projectID,
		Status:          datastore.BatchRetryStatus(status),
		TotalEvents:     int(totalEvents),
		ProcessedEvents: int(processedEvents),
		FailedEvents:    int(failedEvents),
		Filter:          retryFilter,
		CreatedAt:       createdAt.Time,
		UpdatedAt:       updatedAt.Time,
		CompletedAt:     common.PgTimestamptzToNullTime(completedAt),
		Error:           errorMsg.String,
	}, nil
}

// Legacy rowToBatchRetry - keeping for reference but not used
// ============================================================================
// Service Implementation
// ============================================================================

// CreateBatchRetry creates a new batch retry record
func (s *Service) CreateBatchRetry(ctx context.Context, batchRetry *datastore.BatchRetry) error {
	if batchRetry == nil {
		return util.NewServiceError(http.StatusBadRequest, errors.New("batch retry cannot be nil"))
	}

	filterBytes, err := common.RetryFilterToJSONB(batchRetry.Filter)
	if err != nil {
		if s.logger != nil {
			s.logger.Error("failed to marshal filter", "error", err)
		}
		return util.NewServiceError(http.StatusInternalServerError, err)
	}

	err = s.repo.CreateBatchRetry(ctx, repo.CreateBatchRetryParams{
		ID:              common.StringToPgText(batchRetry.ID),
		ProjectID:       common.StringToPgText(batchRetry.ProjectID),
		Status:          common.StringToPgText(string(batchRetry.Status)),
		TotalEvents:     pgtype.Int4{Int32: int32(batchRetry.TotalEvents), Valid: true},
		ProcessedEvents: pgtype.Int4{Int32: int32(batchRetry.ProcessedEvents), Valid: true},
		FailedEvents:    pgtype.Int4{Int32: int32(batchRetry.FailedEvents), Valid: true},
		Filter:          filterBytes,
		CreatedAt:       pgtype.Timestamptz{Time: batchRetry.CreatedAt, Valid: true},
		UpdatedAt:       pgtype.Timestamptz{Time: batchRetry.UpdatedAt, Valid: true},
		CompletedAt:     common.NullTimeToPgTimestamptz(batchRetry.CompletedAt),
		Error:           common.StringToPgTextNullable(batchRetry.Error),
	})

	if err != nil {
		if s.logger != nil {
			s.logger.Error("failed to create batch retry", "error", err)
		}
		return util.NewServiceError(http.StatusInternalServerError, err)
	}

	return nil
}

// UpdateBatchRetry updates an existing batch retry record
func (s *Service) UpdateBatchRetry(ctx context.Context, batchRetry *datastore.BatchRetry) error {
	if batchRetry == nil {
		return util.NewServiceError(http.StatusBadRequest, errors.New("batch retry cannot be nil"))
	}

	filterBytes, err := common.RetryFilterToJSONB(batchRetry.Filter)
	if err != nil {
		if s.logger != nil {
			s.logger.Error("failed to marshal filter", "error", err)
		}
		return util.NewServiceError(http.StatusInternalServerError, err)
	}

	result, err := s.repo.UpdateBatchRetry(ctx, repo.UpdateBatchRetryParams{
		ID:              common.StringToPgText(batchRetry.ID),
		ProjectID:       common.StringToPgText(batchRetry.ProjectID),
		Status:          common.StringToPgText(string(batchRetry.Status)),
		TotalEvents:     pgtype.Int4{Int32: int32(batchRetry.TotalEvents), Valid: true},
		ProcessedEvents: pgtype.Int4{Int32: int32(batchRetry.ProcessedEvents), Valid: true},
		FailedEvents:    pgtype.Int4{Int32: int32(batchRetry.FailedEvents), Valid: true},
		Filter:          filterBytes,
		UpdatedAt:       pgtype.Timestamptz{Time: batchRetry.UpdatedAt, Valid: true},
		CompletedAt:     common.NullTimeToPgTimestamptz(batchRetry.CompletedAt),
		Error:           common.StringToPgTextNullable(batchRetry.Error),
	})

	if err != nil {
		if s.logger != nil {
			s.logger.Error("failed to update batch retry", "error", err)
		}
		return util.NewServiceError(http.StatusInternalServerError, err)
	}

	if result.RowsAffected() < 1 {
		return fmt.Errorf("no rows affected")
	}

	return nil
}

// FindBatchRetryByID retrieves a batch retry by its ID
func (s *Service) FindBatchRetryByID(ctx context.Context, id string) (*datastore.BatchRetry, error) {
	row, err := s.repo.FindBatchRetryByID(ctx, common.StringToPgText(id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, datastore.ErrBatchRetryNotFound
		}
		if s.logger != nil {
			s.logger.Error("failed to find batch retry by id", "error", err)
		}
		return nil, util.NewServiceError(http.StatusInternalServerError, err)
	}

	return rowToBatchRetry(row)
}

// FindActiveBatchRetry finds an active batch retry for a project (pending or processing status)
func (s *Service) FindActiveBatchRetry(ctx context.Context, projectID string) (*datastore.BatchRetry, error) {
	row, err := s.repo.FindActiveBatchRetry(ctx, repo.FindActiveBatchRetryParams{
		ProjectID: common.StringToPgText(projectID),
		Status1:   common.StringToPgText(string(datastore.BatchRetryStatusPending)),
		Status2:   common.StringToPgText(string(datastore.BatchRetryStatusProcessing)),
	})

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// Return nil, nil for no active batch retry (matches legacy behavior)
			return nil, nil
		}
		if s.logger != nil {
			s.logger.Error("failed to find active batch retry", "error", err)
		}
		return nil, util.NewServiceError(http.StatusInternalServerError, err)
	}

	return rowToBatchRetry(row)
}
