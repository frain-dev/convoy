//go:build integration

package postgres

import (
	"context"
	"gopkg.in/guregu/null.v4"
	"testing"
	"time"

	"github.com/oklog/ulid/v2"

	"github.com/frain-dev/convoy/database"
	"github.com/frain-dev/convoy/datastore"
	"github.com/stretchr/testify/require"
)

func Test_CreateJob(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	jobRepo := NewJobRepo(db, nil)
	job := generateJob(t, db)

	require.NoError(t, jobRepo.CreateJob(context.Background(), job))

	jobById, err := jobRepo.FetchJobById(context.Background(), job.UID, job.ProjectID)
	require.NoError(t, err)

	require.NotNil(t, jobById)
	require.Equal(t, datastore.JobStatusReady, jobById.Status)
}

func TestJobRepo_FetchJobsByProjectId(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	org := seedOrg(t, db)
	jobRepo := NewJobRepo(db, nil)

	p1 := &datastore.Project{
		UID:            ulid.Make().String(),
		Name:           "P1",
		OrganisationID: org.UID,
		Type:           datastore.IncomingProject,
		Config:         &datastore.DefaultProjectConfig,
	}

	p2 := &datastore.Project{
		UID:            ulid.Make().String(),
		Name:           "P2",
		OrganisationID: org.UID,
		Type:           datastore.IncomingProject,
		Config:         &datastore.DefaultProjectConfig,
	}

	err := NewProjectRepo(db, nil).CreateProject(context.Background(), p1)
	require.NoError(t, err)

	err = NewProjectRepo(db, nil).CreateProject(context.Background(), p2)
	require.NoError(t, err)

	require.NoError(t, jobRepo.CreateJob(context.Background(), &datastore.Job{
		UID:       ulid.Make().String(),
		Type:      "create",
		Status:    datastore.JobStatusRunning,
		StartedAt: null.TimeFrom(time.Now()),
		ProjectID: p1.UID,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}))

	require.NoError(t, jobRepo.CreateJob(context.Background(), &datastore.Job{
		UID:         ulid.Make().String(),
		Type:        "update",
		Status:      datastore.JobStatusCompleted,
		StartedAt:   null.TimeFrom(time.Now()),
		CompletedAt: null.TimeFrom(time.Now()),
		ProjectID:   p2.UID,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}))

	require.NoError(t, jobRepo.CreateJob(context.Background(), &datastore.Job{
		UID:       ulid.Make().String(),
		Type:      "update",
		Status:    datastore.JobStatusFailed,
		StartedAt: null.TimeFrom(time.Now()),
		FailedAt:  null.TimeFrom(time.Now()),
		ProjectID: p2.UID,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}))

	jobs, err := jobRepo.FetchJobsByProjectId(context.Background(), p2.UID)
	require.NoError(t, err)

	require.Equal(t, 2, len(jobs))
}

func TestJobRepo_FetchRunningJobsByProjectId(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	org := seedOrg(t, db)
	jobRepo := NewJobRepo(db, nil)

	p1 := &datastore.Project{
		UID:            ulid.Make().String(),
		Name:           "P1",
		OrganisationID: org.UID,
		Type:           datastore.IncomingProject,
		Config:         &datastore.DefaultProjectConfig,
	}

	p2 := &datastore.Project{
		UID:            ulid.Make().String(),
		Name:           "P2",
		OrganisationID: org.UID,
		Type:           datastore.IncomingProject,
		Config:         &datastore.DefaultProjectConfig,
	}

	err := NewProjectRepo(db, nil).CreateProject(context.Background(), p1)
	require.NoError(t, err)

	err = NewProjectRepo(db, nil).CreateProject(context.Background(), p2)
	require.NoError(t, err)

	require.NoError(t, jobRepo.CreateJob(context.Background(), &datastore.Job{
		UID:       ulid.Make().String(),
		Type:      "create",
		Status:    datastore.JobStatusRunning,
		StartedAt: null.TimeFrom(time.Now()),
		ProjectID: p1.UID,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}))

	require.NoError(t, jobRepo.CreateJob(context.Background(), &datastore.Job{
		UID:       ulid.Make().String(),
		Type:      "update",
		Status:    datastore.JobStatusRunning,
		StartedAt: null.TimeFrom(time.Now()),
		ProjectID: p2.UID,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}))

	require.NoError(t, jobRepo.CreateJob(context.Background(), &datastore.Job{
		UID:       ulid.Make().String(),
		Type:      "update",
		Status:    datastore.JobStatusFailed,
		StartedAt: null.TimeFrom(time.Now()),
		FailedAt:  null.TimeFrom(time.Now()),
		ProjectID: p2.UID,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}))

	jobs, err := jobRepo.FetchRunningJobsByProjectId(context.Background(), p2.UID)
	require.NoError(t, err)

	require.Equal(t, 1, len(jobs))
}

func TestJobRepo_MarkJobAsStarted(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	jobRepo := NewJobRepo(db, nil)
	job := generateJob(t, db)

	ctx := context.Background()

	require.NoError(t, jobRepo.CreateJob(ctx, job))

	require.NoError(t, jobRepo.MarkJobAsStarted(ctx, job.UID, job.ProjectID))

	jobById, err := jobRepo.FetchJobById(ctx, job.UID, job.ProjectID)
	require.NoError(t, err)

	require.Equal(t, datastore.JobStatusRunning, jobById.Status)
	require.Less(t, time.Time{}.Unix(), jobById.StartedAt.Time.Unix())
	require.True(t, time.Now().After(jobById.StartedAt.Time))
	require.Equal(t, time.Time{}.Unix(), jobById.FailedAt.Time.Unix())
	require.Equal(t, time.Time{}.Unix(), jobById.CompletedAt.Time.Unix())
}

func TestJobRepo_MarkJobAsCompleted(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	jobRepo := NewJobRepo(db, nil)
	job := generateJob(t, db)

	ctx := context.Background()

	require.NoError(t, jobRepo.CreateJob(ctx, job))

	require.NoError(t, jobRepo.MarkJobAsStarted(ctx, job.UID, job.ProjectID))
	require.NoError(t, jobRepo.MarkJobAsCompleted(ctx, job.UID, job.ProjectID))

	jobById, err := jobRepo.FetchJobById(ctx, job.UID, job.ProjectID)
	require.NoError(t, err)

	require.Equal(t, datastore.JobStatusCompleted, jobById.Status)
	require.Less(t, time.Time{}.Unix(), jobById.StartedAt.Time.Unix())
	require.True(t, time.Now().After(jobById.StartedAt.Time))
	require.Equal(t, time.Time{}.Unix(), jobById.FailedAt.Time.Unix())
}

func TestJobRepo_MarkJobAsFailed(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	jobRepo := NewJobRepo(db, nil)
	job := generateJob(t, db)

	ctx := context.Background()

	require.NoError(t, jobRepo.CreateJob(ctx, job))

	require.NoError(t, jobRepo.MarkJobAsStarted(ctx, job.UID, job.ProjectID))
	require.NoError(t, jobRepo.MarkJobAsFailed(ctx, job.UID, job.ProjectID))

	jobById, err := jobRepo.FetchJobById(ctx, job.UID, job.ProjectID)
	require.NoError(t, err)

	require.Equal(t, datastore.JobStatusFailed, jobById.Status)
	require.Less(t, time.Time{}.Unix(), jobById.StartedAt.Time.Unix())
	require.True(t, time.Now().After(jobById.StartedAt.Time))
	require.Equal(t, time.Time{}.Unix(), jobById.CompletedAt.Time.Unix())
}

func TestJobRepo_DeleteJob(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	jobRepo := NewJobRepo(db, nil)
	job := generateJob(t, db)

	require.NoError(t, jobRepo.CreateJob(context.Background(), job))

	err := jobRepo.DeleteJob(context.Background(), job.UID, job.ProjectID)
	require.NoError(t, err)

	_, err = jobRepo.FetchJobById(context.Background(), job.UID, job.ProjectID)
	require.Equal(t, ErrJobNotFound, err)
}

func Test_LoadJobsPaged(t *testing.T) {
	type Expected struct {
		paginationData datastore.PaginationData
	}

	tests := []struct {
		name     string
		pageData datastore.Pageable
		count    int
		expected Expected
	}{
		{
			name:     "Load Jobs Paged - 10 records",
			pageData: datastore.Pageable{PerPage: 3},
			count:    10,
			expected: Expected{
				paginationData: datastore.PaginationData{
					PerPage: 3,
				},
			},
		},

		{
			name:     "Load Jobs Paged - 12 records",
			pageData: datastore.Pageable{PerPage: 4},
			count:    12,
			expected: Expected{
				paginationData: datastore.PaginationData{
					PerPage: 4,
				},
			},
		},

		{
			name:     "Load Jobs Paged - 5 records",
			pageData: datastore.Pageable{PerPage: 3},
			count:    5,
			expected: Expected{
				paginationData: datastore.PaginationData{
					PerPage: 3,
				},
			},
		},

		{
			name:     "Load Jobs Paged - 1 record",
			pageData: datastore.Pageable{PerPage: 3},
			count:    1,
			expected: Expected{
				paginationData: datastore.PaginationData{
					PerPage: 3,
				},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			db, closeFn := getDB(t)
			defer closeFn()

			jobRepository := NewJobRepo(db, nil)
			project := seedProject(t, db)

			for i := 0; i < tc.count; i++ {
				job := &datastore.Job{
					UID:       ulid.Make().String(),
					ProjectID: project.UID,
					Status:    datastore.JobStatusReady,
				}

				require.NoError(t, jobRepository.CreateJob(context.Background(), job))
			}

			_, pageable, err := jobRepository.LoadJobsPaged(context.Background(), project.UID, tc.pageData)
			require.NoError(t, err)

			require.Equal(t, tc.expected.paginationData.PerPage, pageable.PerPage)
		})
	}
}

func generateJob(t *testing.T, db database.Database) *datastore.Job {
	project := seedProject(t, db)

	return &datastore.Job{
		UID:       ulid.Make().String(),
		Type:      "search_tokenizer",
		Status:    datastore.JobStatusReady,
		ProjectID: project.UID,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
}
