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
	"github.com/frain-dev/convoy/pkg/log"
	"github.com/frain-dev/convoy/util"
)

// Service implements the BatchRetryRepository using SQLc-generated queries
type Service struct {
	logger log.StdLogger
	repo   repo.Querier  // SQLc-generated interface
	db     *pgxpool.Pool // Connection pool
}

// Ensure Service implements datastore.BatchRetryRepository at compile time
var _ datastore.BatchRetryRepository = (*Service)(nil)

// New creates a new Batch Retry Service
func New(logger log.StdLogger, db database.Database) *Service {
	return &Service{
		logger: logger,
		repo:   repo.New(db.GetConn()),
		db:     db.GetConn(),
	}
}

// rowToBatchRetry converts an SQLc-generated row struct to datastore.BatchRetry
func rowToBatchRetry(row repo.ConvoyBatchRetry) (*datastore.BatchRetry, error) {
	filter, err := common.JSONBToRetryFilter(row.Filter)
	if err != nil {
		return nil, fmt.Errorf("failed to parse filter: %w", err)
	}

	return &datastore.BatchRetry{
		ID:              row.ID,
		ProjectID:       row.ProjectID,
		Status:          datastore.BatchRetryStatus(row.Status),
		TotalEvents:     int(row.TotalEvents),
		ProcessedEvents: int(row.ProcessedEvents),
		FailedEvents:    int(row.FailedEvents),
		Filter:          filter,
		CreatedAt:       row.CreatedAt.Time,
		UpdatedAt:       row.UpdatedAt.Time,
		CompletedAt:     common.PgTimestamptzToNullTime(row.CompletedAt),
		Error:           row.Error.String,
	}, nil
}

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
			s.logger.WithError(err).Error("failed to marshal filter")
		}
		return util.NewServiceError(http.StatusInternalServerError, err)
	}

	err = s.repo.CreateBatchRetry(ctx, repo.CreateBatchRetryParams{
		ID:              batchRetry.ID,
		ProjectID:       batchRetry.ProjectID,
		Status:          string(batchRetry.Status),
		TotalEvents:     int32(batchRetry.TotalEvents),
		ProcessedEvents: int32(batchRetry.ProcessedEvents),
		FailedEvents:    int32(batchRetry.FailedEvents),
		Filter:          filterBytes,
		CreatedAt:       pgtype.Timestamptz{Time: batchRetry.CreatedAt, Valid: true},
		UpdatedAt:       pgtype.Timestamptz{Time: batchRetry.UpdatedAt, Valid: true},
		CompletedAt:     common.NullTimeToPgTimestamptz(batchRetry.CompletedAt),
		Error:           common.StringToPgText(batchRetry.Error),
	})

	if err != nil {
		if s.logger != nil {
			s.logger.WithError(err).Error("failed to create batch retry")
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
			s.logger.WithError(err).Error("failed to marshal filter")
		}
		return util.NewServiceError(http.StatusInternalServerError, err)
	}

	result, err := s.repo.UpdateBatchRetry(ctx, repo.UpdateBatchRetryParams{
		ID:              batchRetry.ID,
		ProjectID:       batchRetry.ProjectID,
		Status:          string(batchRetry.Status),
		TotalEvents:     int32(batchRetry.TotalEvents),
		ProcessedEvents: int32(batchRetry.ProcessedEvents),
		FailedEvents:    int32(batchRetry.FailedEvents),
		Filter:          filterBytes,
		UpdatedAt:       pgtype.Timestamptz{Time: batchRetry.UpdatedAt, Valid: true},
		CompletedAt:     common.NullTimeToPgTimestamptz(batchRetry.CompletedAt),
		Error:           common.StringToPgText(batchRetry.Error),
	})

	if err != nil {
		if s.logger != nil {
			s.logger.WithError(err).Error("failed to update batch retry")
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
	row, err := s.repo.FindBatchRetryByID(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, datastore.ErrBatchRetryNotFound
		}
		if s.logger != nil {
			s.logger.WithError(err).Error("failed to find batch retry by id")
		}
		return nil, util.NewServiceError(http.StatusInternalServerError, err)
	}

	return rowToBatchRetry(row)
}

// FindActiveBatchRetry finds an active batch retry for a project (pending or processing status)
func (s *Service) FindActiveBatchRetry(ctx context.Context, projectID string) (*datastore.BatchRetry, error) {
	row, err := s.repo.FindActiveBatchRetry(ctx, repo.FindActiveBatchRetryParams{
		ProjectID: projectID,
		Status1:   string(datastore.BatchRetryStatusPending),
		Status2:   string(datastore.BatchRetryStatusProcessing),
	})

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// Return nil, nil for no active batch retry (matches legacy behavior)
			return nil, nil
		}
		if s.logger != nil {
			s.logger.WithError(err).Error("failed to find active batch retry")
		}
		return nil, util.NewServiceError(http.StatusInternalServerError, err)
	}

	return rowToBatchRetry(row)
}
