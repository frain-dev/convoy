package jobs

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/frain-dev/convoy/database"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/internal/common"
	"github.com/frain-dev/convoy/internal/jobs/repo"
	log "github.com/frain-dev/convoy/pkg/logger"
)

var (
	ErrJobNotCreated = errors.New("job could not be created")
	ErrJobNotUpdated = errors.New("job could not be updated")
	ErrJobNotDeleted = errors.New("job could not be deleted")
)

// Service implements datastore.JobRepository using sqlc-generated queries
type Service struct {
	logger log.Logger
	repo   repo.Querier
	db     *pgxpool.Pool
}

// Ensure Service implements datastore.JobRepository at compile time
var _ datastore.JobRepository = (*Service)(nil)

// New creates a new jobs service
func New(logger log.Logger, db database.Database) *Service {
	return &Service{
		logger: logger,
		repo:   repo.New(db.GetConn()),
		db:     db.GetConn(),
	}
}

// rowToJob converts any sqlc-generated row struct to datastore.Job
func rowToJob(row interface{}) datastore.Job {
	switch r := row.(type) {
	case repo.FetchJobByIdRow:
		return datastore.Job{
			UID:         r.ID,
			Type:        r.Type,
			Status:      datastore.JobStatus(r.Status),
			ProjectID:   r.ProjectID,
			StartedAt:   common.PgTimestamptzToNullTime(r.StartedAt),
			CompletedAt: common.PgTimestamptzToNullTime(r.CompletedAt),
			FailedAt:    common.PgTimestamptzToNullTime(r.FailedAt),
			CreatedAt:   common.PgTimestamptzToTime(r.CreatedAt),
			UpdatedAt:   common.PgTimestamptzToTime(r.UpdatedAt),
			DeletedAt:   common.PgTimestamptzToNullTime(r.DeletedAt),
		}
	case repo.FetchRunningJobsByProjectIdRow:
		return datastore.Job{
			UID:         r.ID,
			Type:        r.Type,
			Status:      datastore.JobStatus(r.Status),
			ProjectID:   r.ProjectID,
			StartedAt:   common.PgTimestamptzToNullTime(r.StartedAt),
			CompletedAt: common.PgTimestamptzToNullTime(r.CompletedAt),
			FailedAt:    common.PgTimestamptzToNullTime(r.FailedAt),
			CreatedAt:   common.PgTimestamptzToTime(r.CreatedAt),
			UpdatedAt:   common.PgTimestamptzToTime(r.UpdatedAt),
			DeletedAt:   common.PgTimestamptzToNullTime(r.DeletedAt),
		}
	case repo.FetchJobsByProjectIdRow:
		return datastore.Job{
			UID:         r.ID,
			Type:        r.Type,
			Status:      datastore.JobStatus(r.Status),
			ProjectID:   r.ProjectID,
			StartedAt:   common.PgTimestamptzToNullTime(r.StartedAt),
			CompletedAt: common.PgTimestamptzToNullTime(r.CompletedAt),
			FailedAt:    common.PgTimestamptzToNullTime(r.FailedAt),
			CreatedAt:   common.PgTimestamptzToTime(r.CreatedAt),
			UpdatedAt:   common.PgTimestamptzToTime(r.UpdatedAt),
			DeletedAt:   common.PgTimestamptzToNullTime(r.DeletedAt),
		}
	case repo.FetchJobsPaginatedRow:
		return datastore.Job{
			UID:         r.ID,
			Type:        r.Type,
			Status:      datastore.JobStatus(r.Status),
			ProjectID:   r.ProjectID,
			StartedAt:   common.PgTimestamptzToNullTime(r.StartedAt),
			CompletedAt: common.PgTimestamptzToNullTime(r.CompletedAt),
			FailedAt:    common.PgTimestamptzToNullTime(r.FailedAt),
			CreatedAt:   common.PgTimestamptzToTime(r.CreatedAt),
			UpdatedAt:   common.PgTimestamptzToTime(r.UpdatedAt),
		}
	default:
		return datastore.Job{}
	}
}

// CreateJob creates a new job
func (s *Service) CreateJob(ctx context.Context, job *datastore.Job) error {
	err := s.repo.CreateJob(ctx, repo.CreateJobParams{
		ID:        common.StringToPgText(job.UID),
		Type:      common.StringToPgText(job.Type),
		Status:    common.StringToPgText(string(job.Status)),
		ProjectID: common.StringToPgText(job.ProjectID),
	})
	if err != nil {
		s.logger.Error("failed to create job", "error", err)
		return err
	}

	return nil
}

// MarkJobAsStarted marks a job as started
func (s *Service) MarkJobAsStarted(ctx context.Context, uid, projectID string) error {
	result, err := s.repo.MarkJobAsStarted(ctx, repo.MarkJobAsStartedParams{
		ID:        common.StringToPgText(uid),
		ProjectID: common.StringToPgText(projectID),
	})
	if err != nil {
		s.logger.Error("failed to mark job as started", "error", err)
		return err
	}

	if result.RowsAffected() < 1 {
		return ErrJobNotUpdated
	}

	return nil
}

// MarkJobAsCompleted marks a job as completed
func (s *Service) MarkJobAsCompleted(ctx context.Context, uid, projectID string) error {
	result, err := s.repo.MarkJobAsCompleted(ctx, repo.MarkJobAsCompletedParams{
		ID:        common.StringToPgText(uid),
		ProjectID: common.StringToPgText(projectID),
	})
	if err != nil {
		s.logger.Error("failed to mark job as completed", "error", err)
		return err
	}

	if result.RowsAffected() < 1 {
		return ErrJobNotUpdated
	}

	return nil
}

// MarkJobAsFailed marks a job as failed
func (s *Service) MarkJobAsFailed(ctx context.Context, uid, projectID string) error {
	result, err := s.repo.MarkJobAsFailed(ctx, repo.MarkJobAsFailedParams{
		ID:        common.StringToPgText(uid),
		ProjectID: common.StringToPgText(projectID),
	})
	if err != nil {
		s.logger.Error("failed to mark job as failed", "error", err)
		return err
	}

	if result.RowsAffected() < 1 {
		return ErrJobNotUpdated
	}

	return nil
}

// DeleteJob soft-deletes a job
func (s *Service) DeleteJob(ctx context.Context, uid, projectID string) error {
	result, err := s.repo.DeleteJob(ctx, repo.DeleteJobParams{
		ID:        common.StringToPgText(uid),
		ProjectID: common.StringToPgText(projectID),
	})
	if err != nil {
		s.logger.Error("failed to delete job", "error", err)
		return err
	}

	if result.RowsAffected() < 1 {
		return ErrJobNotDeleted
	}

	return nil
}

// FetchJobById retrieves a job by its ID
func (s *Service) FetchJobById(ctx context.Context, uid, projectID string) (*datastore.Job, error) {
	row, err := s.repo.FetchJobById(ctx, repo.FetchJobByIdParams{
		ID:        common.StringToPgText(uid),
		ProjectID: common.StringToPgText(projectID),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, datastore.ErrJobNotFound
		}
		s.logger.Error("failed to fetch job by id", "error", err)
		return nil, err
	}

	job := rowToJob(row)
	return &job, nil
}

// FetchRunningJobsByProjectId retrieves all running jobs for a project
func (s *Service) FetchRunningJobsByProjectId(ctx context.Context, projectID string) ([]datastore.Job, error) {
	rows, err := s.repo.FetchRunningJobsByProjectId(ctx, common.StringToPgText(projectID))
	if err != nil {
		s.logger.Error("failed to fetch running jobs", "error", err)
		return nil, err
	}

	jobs := make([]datastore.Job, 0, len(rows))
	for _, row := range rows {
		jobs = append(jobs, rowToJob(row))
	}

	return jobs, nil
}

// FetchJobsByProjectId retrieves all jobs for a project
func (s *Service) FetchJobsByProjectId(ctx context.Context, projectID string) ([]datastore.Job, error) {
	rows, err := s.repo.FetchJobsByProjectId(ctx, common.StringToPgText(projectID))
	if err != nil {
		s.logger.Error("failed to fetch jobs by project id", "error", err)
		return nil, err
	}

	jobs := make([]datastore.Job, 0, len(rows))
	for _, row := range rows {
		jobs = append(jobs, rowToJob(row))
	}

	return jobs, nil
}

// LoadJobsPaged retrieves jobs with pagination
func (s *Service) LoadJobsPaged(ctx context.Context, projectID string, pageable datastore.Pageable) ([]datastore.Job, datastore.PaginationData, error) {
	direction := "next"
	if pageable.Direction == datastore.Prev {
		direction = "prev"
	}

	rows, err := s.repo.FetchJobsPaginated(ctx, repo.FetchJobsPaginatedParams{
		Direction: direction,
		ProjectID: common.StringToPgText(projectID),
		Cursor:    common.StringToPgText(pageable.Cursor()),
		LimitVal:  int64(pageable.Limit()),
	})
	if err != nil {
		s.logger.Error("failed to load jobs paged", "error", err)
		return nil, datastore.PaginationData{}, err
	}

	jobs := make([]datastore.Job, 0, len(rows))
	for _, row := range rows {
		jobs = append(jobs, rowToJob(row))
	}

	// Count previous rows for pagination metadata
	var prevRowCount datastore.PrevRowCount
	if len(jobs) > 0 {
		first := jobs[0]
		count, err := s.repo.CountPrevJobs(ctx, repo.CountPrevJobsParams{
			ProjectID: common.StringToPgText(projectID),
			Cursor:    common.StringToPgText(first.UID),
		})
		if err != nil {
			s.logger.Error("failed to count prev jobs", "error", err)
			return nil, datastore.PaginationData{}, err
		}
		prevRowCount.Count = int(count.Int64)
	}

	// Build pagination metadata with untrimmed ids
	ids := make([]string, len(jobs))
	for i := range jobs {
		ids[i] = jobs[i].UID
	}

	pagination := &datastore.PaginationData{PrevRowCount: prevRowCount}
	pagination = pagination.Build(pageable, ids)

	// Trim LIMIT+1 after building pagination
	if len(jobs) > pageable.PerPage {
		jobs = jobs[:len(jobs)-1]
	}

	return jobs, *pagination, nil
}
