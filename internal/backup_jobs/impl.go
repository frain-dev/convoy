package backup_jobs

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/frain-dev/convoy/database"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/internal/backup_jobs/repo"
	"github.com/frain-dev/convoy/internal/common"
	log "github.com/frain-dev/convoy/pkg/logger"
)

type Service struct {
	logger log.Logger
	repo   repo.Querier
	db     *pgxpool.Pool
}

func New(logger log.Logger, db database.Database) *Service {
	return &Service{
		logger: logger,
		repo:   repo.New(db.GetConn()),
		db:     db.GetConn(),
	}
}

func (s *Service) EnqueueBackupJob(ctx context.Context, projectID string, hourStart, hourEnd time.Time) error {
	return s.repo.EnqueueBackupJob(ctx, repo.EnqueueBackupJobParams{
		ProjectID: common.StringToPgText(projectID),
		HourStart: common.TimeToPgTimestamptz(hourStart),
		HourEnd:   common.TimeToPgTimestamptz(hourEnd),
	})
}

func (s *Service) ClaimBackupJob(ctx context.Context, workerID string) (*datastore.BackupJob, error) {
	row, err := s.repo.ClaimBackupJob(ctx, common.StringToPgText(workerID))
	if err != nil {
		return nil, err
	}

	return rowToBackupJob(row), nil
}

func (s *Service) CompleteBackupJob(ctx context.Context, jobID string, recordCounts map[string]int64) error {
	countsJSON, err := json.Marshal(recordCounts)
	if err != nil {
		return fmt.Errorf("marshal record counts: %w", err)
	}

	return s.repo.CompleteBackupJob(ctx, repo.CompleteBackupJobParams{
		ID:           jobID,
		RecordCounts: countsJSON,
	})
}

func (s *Service) FailBackupJob(ctx context.Context, jobID, errMsg string) error {
	return s.repo.FailBackupJob(ctx, repo.FailBackupJobParams{
		ID:    jobID,
		Error: pgtype.Text{String: errMsg, Valid: true},
	})
}

func (s *Service) ReclaimStaleJobs(ctx context.Context, staleMinutes int32) (int64, error) {
	tag, err := s.repo.ReclaimStaleJobs(ctx, pgtype.Int4{Int32: staleMinutes, Valid: true})
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}

func (s *Service) FindLatestCompletedBackup(ctx context.Context, projectID string) (*datastore.BackupJob, error) {
	row, err := s.repo.FindLatestCompletedBackup(ctx, common.StringToPgText(projectID))
	if err != nil {
		return nil, err
	}

	return rowToLatestBackupJob(row), nil
}

func rowToBackupJob(row repo.ClaimBackupJobRow) *datastore.BackupJob {
	job := &datastore.BackupJob{
		ID:        row.ID,
		ProjectID: row.ProjectID,
		Status:    row.Status,
		WorkerID:  common.PgTextToString(row.WorkerID),
		Error:     common.PgTextToString(row.Error),
	}

	if row.HourStart.Valid {
		job.HourStart = row.HourStart.Time
	}
	if row.HourEnd.Valid {
		job.HourEnd = row.HourEnd.Time
	}
	if row.ClaimedAt.Valid {
		t := row.ClaimedAt.Time
		job.ClaimedAt = &t
	}
	if row.CompletedAt.Valid {
		t := row.CompletedAt.Time
		job.CompletedAt = &t
	}
	if row.CreatedAt.Valid {
		job.CreatedAt = row.CreatedAt.Time
	}
	if row.UpdatedAt.Valid {
		job.UpdatedAt = row.UpdatedAt.Time
	}
	if row.RecordCounts != nil {
		_ = json.Unmarshal(row.RecordCounts, &job.RecordCounts)
	}

	return job
}

func rowToLatestBackupJob(row repo.FindLatestCompletedBackupRow) *datastore.BackupJob {
	job := &datastore.BackupJob{
		ID:        row.ID,
		ProjectID: row.ProjectID,
		Status:    row.Status,
		WorkerID:  common.PgTextToString(row.WorkerID),
		Error:     common.PgTextToString(row.Error),
	}

	if row.HourStart.Valid {
		job.HourStart = row.HourStart.Time
	}
	if row.HourEnd.Valid {
		job.HourEnd = row.HourEnd.Time
	}
	if row.ClaimedAt.Valid {
		t := row.ClaimedAt.Time
		job.ClaimedAt = &t
	}
	if row.CompletedAt.Valid {
		t := row.CompletedAt.Time
		job.CompletedAt = &t
	}
	if row.CreatedAt.Valid {
		job.CreatedAt = row.CreatedAt.Time
	}
	if row.UpdatedAt.Valid {
		job.UpdatedAt = row.UpdatedAt.Time
	}
	if row.RecordCounts != nil {
		_ = json.Unmarshal(row.RecordCounts, &job.RecordCounts)
	}

	return job
}
