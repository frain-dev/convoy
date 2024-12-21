package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/frain-dev/convoy/database"
	"github.com/frain-dev/convoy/datastore"
	"github.com/jmoiron/sqlx"
)

var (
	ErrJobNotFound   = errors.New("job not found")
	ErrJobNotCreated = errors.New("job could not be created")
	ErrJobNotUpdated = errors.New("job could not be updated")
	ErrJobNotDeleted = errors.New("job could not be deleted")
)

const (
	createJob = `
	INSERT INTO convoy.jobs (id, type, status, project_id)
	VALUES ($1, $2, $3, $4)
	`

	updateJobStartedAt = `
	UPDATE convoy.jobs SET
	status = 'running',
	started_at = NOW(),
	updated_at = NOW()
	WHERE id = $1 AND project_id = $2 AND deleted_at IS NULL;
	`

	updateJobCompletedAt = `
	UPDATE convoy.jobs SET
	status = 'completed',
	completed_at = NOW(),
	updated_at = NOW()
	WHERE id = $1 AND project_id = $2 AND deleted_at IS NULL;
	`

	updateJobFailedAt = `
	UPDATE convoy.jobs SET
	status = 'failed',
	failed_at = NOW(),
	updated_at = NOW()
	WHERE id = $1 AND project_id = $2 AND deleted_at IS NULL;
	`

	deleteJob = `
	UPDATE convoy.jobs SET
	deleted_at = NOW()
	WHERE id = $1 AND project_id = $2 AND deleted_at IS NULL;
	`

	fetchJobById = `
	SELECT * FROM convoy.jobs
	WHERE id = $1 AND project_id = $2 AND deleted_at IS NULL;
	`

	fetchRunningJobsByProjectId = `
	SELECT * FROM convoy.jobs
	WHERE status = 'running'
	AND project_id = $1
	AND deleted_at IS NULL;
	`

	fetchJobsByProjectId = `
	SELECT * FROM convoy.jobs WHERE project_id = $1 AND deleted_at IS NULL;
	`

	fetchJobsPaginated = `
	SELECT * FROM convoy.jobs WHERE deleted_at IS NULL`

	baseJobsFilter = `
	AND project_id = :project_id`

	baseFetchJobsPagedForward = `
	%s
	%s
	AND id <= :cursor
	GROUP BY id
	ORDER BY id DESC
	LIMIT :limit
	`

	baseFetchJobsPagedBackward = `
	WITH jobs AS (
		%s
		%s
		AND id >= :cursor
		GROUP BY id
		ORDER BY id ASC
		LIMIT :limit
	)

	SELECT * FROM jobs ORDER BY id DESC
	`

	countPrevJobs = `
	SELECT COUNT(DISTINCT(id)) AS count
	FROM convoy.jobs
	WHERE deleted_at IS NULL
	%s
	AND id > :cursor GROUP BY id ORDER BY id DESC LIMIT 1`
)

type jobRepo struct {
	db database.Database
}

func NewJobRepo(db database.Database) datastore.JobRepository {
	return &jobRepo{db: db}
}

func (j *jobRepo) CreateJob(ctx context.Context, job *datastore.Job) error {
	r, err := j.db.GetDB().ExecContext(ctx, createJob,
		job.UID,
		job.Type,
		job.Status,
		job.ProjectID,
	)
	if err != nil {
		return err
	}

	rowsAffected, err := r.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected < 1 {
		return ErrJobNotCreated
	}

	return nil
}

func (j *jobRepo) MarkJobAsStarted(ctx context.Context, uid, projectID string) error {
	r, err := j.db.GetDB().ExecContext(ctx, updateJobStartedAt, uid, projectID)
	if err != nil {
		return err
	}

	rowsAffected, err := r.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected < 1 {
		return ErrJobNotUpdated
	}

	return nil
}

func (j *jobRepo) MarkJobAsCompleted(ctx context.Context, uid, projectID string) error {
	r, err := j.db.GetDB().ExecContext(ctx, updateJobCompletedAt, uid, projectID)
	if err != nil {
		return err
	}

	rowsAffected, err := r.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected < 1 {
		return ErrJobNotUpdated
	}

	return nil
}

func (j *jobRepo) MarkJobAsFailed(ctx context.Context, uid, projectID string) error {
	r, err := j.db.GetDB().ExecContext(ctx, updateJobFailedAt, uid, projectID)
	if err != nil {
		return err
	}

	rowsAffected, err := r.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected < 1 {
		return ErrJobNotUpdated
	}

	return nil
}

func (j *jobRepo) DeleteJob(ctx context.Context, uid string, projectID string) error {
	r, err := j.db.GetDB().ExecContext(ctx, deleteJob, uid, projectID)
	if err != nil {
		return err
	}

	rowsAffected, err := r.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected < 1 {
		return ErrJobNotDeleted
	}

	return nil
}

func (j *jobRepo) FetchJobById(ctx context.Context, uid string, projectID string) (*datastore.Job, error) {
	job := &datastore.Job{}
	err := j.db.GetDB().QueryRowxContext(ctx, fetchJobById, uid, projectID).StructScan(job)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrJobNotFound
		}
		return nil, err
	}

	return job, nil
}

func (j *jobRepo) FetchRunningJobsByProjectId(ctx context.Context, projectID string) ([]datastore.Job, error) {
	var jobs []datastore.Job
	rows, err := j.db.GetReadDB().QueryxContext(ctx, fetchRunningJobsByProjectId, projectID)
	if err != nil {
		return nil, err
	}
	defer closeWithError(rows)

	for rows.Next() {
		var job datastore.Job

		err = rows.StructScan(&job)
		if err != nil {
			return nil, err
		}

		jobs = append(jobs, job)
	}

	return jobs, nil
}

func (j *jobRepo) FetchJobsByProjectId(ctx context.Context, projectID string) ([]datastore.Job, error) {
	var jobs []datastore.Job
	rows, err := j.db.GetReadDB().QueryxContext(ctx, fetchJobsByProjectId, projectID)
	if err != nil {
		return nil, err
	}
	defer closeWithError(rows)

	for rows.Next() {
		var job datastore.Job

		err = rows.StructScan(&job)
		if err != nil {
			return nil, err
		}

		jobs = append(jobs, job)
	}

	return jobs, nil
}

func (j *jobRepo) LoadJobsPaged(ctx context.Context, projectID string, pageable datastore.Pageable) ([]datastore.Job, datastore.PaginationData, error) {
	var query, filterQuery string
	var args []interface{}
	var err error

	arg := map[string]interface{}{
		"project_id": projectID,
		"limit":      pageable.Limit(),
		"cursor":     pageable.Cursor(),
	}

	if pageable.Direction == datastore.Next {
		query = baseFetchJobsPagedForward
	} else {
		query = baseFetchJobsPagedBackward
	}

	filterQuery = baseJobsFilter

	query = fmt.Sprintf(query, fetchJobsPaginated, filterQuery)

	query, args, err = sqlx.Named(query, arg)
	if err != nil {
		return nil, datastore.PaginationData{}, err
	}

	query, args, err = sqlx.In(query, args...)
	if err != nil {
		return nil, datastore.PaginationData{}, err
	}

	query = j.db.GetReadDB().Rebind(query)

	rows, err := j.db.GetReadDB().QueryxContext(ctx, query, args...)
	if err != nil {
		return nil, datastore.PaginationData{}, err
	}
	defer closeWithError(rows)

	var jobs []datastore.Job
	for rows.Next() {
		var data JobPaginated

		err = rows.StructScan(&data)
		if err != nil {
			return nil, datastore.PaginationData{}, err
		}

		jobs = append(jobs, data.Job)
	}

	var count datastore.PrevRowCount
	if len(jobs) > 0 {
		var countQuery string
		var qargs []interface{}
		first := jobs[0]
		qarg := arg
		qarg["cursor"] = first.UID

		cq := fmt.Sprintf(countPrevJobs, filterQuery)
		countQuery, qargs, err = sqlx.Named(cq, qarg)
		if err != nil {
			return nil, datastore.PaginationData{}, err
		}

		countQuery = j.db.GetReadDB().Rebind(countQuery)

		// count the row number before the first row
		rows, err := j.db.GetReadDB().QueryxContext(ctx, countQuery, qargs...)
		if err != nil {
			return nil, datastore.PaginationData{}, err
		}
		defer closeWithError(rows)

		if rows.Next() {
			err = rows.StructScan(&count)
			if err != nil {
				return nil, datastore.PaginationData{}, err
			}
		}
	}

	ids := make([]string, len(jobs))
	for i := range jobs {
		ids[i] = jobs[i].UID
	}

	if len(jobs) > pageable.PerPage {
		jobs = jobs[:len(jobs)-1]
	}

	pagination := &datastore.PaginationData{PrevRowCount: count}
	pagination = pagination.Build(pageable, ids)

	return jobs, *pagination, nil
}

type JobPaginated struct {
	Count int
	datastore.Job
}
