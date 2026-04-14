package jobs

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/require"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/database"
	"github.com/frain-dev/convoy/database/hooks"
	"github.com/frain-dev/convoy/database/postgres"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/internal/organisations"
	"github.com/frain-dev/convoy/internal/pkg/keys"
	"github.com/frain-dev/convoy/internal/projects"
	"github.com/frain-dev/convoy/internal/users"
	log "github.com/frain-dev/convoy/pkg/logger"
	"github.com/frain-dev/convoy/testenv"
)

var testEnv *testenv.Environment

func TestMain(m *testing.M) {
	res, cleanup, err := testenv.Launch(context.Background())
	if err != nil {
		panic(err)
	}
	testEnv = res

	code := m.Run()

	if err := cleanup(); err != nil {
		fmt.Printf("failed to cleanup: %v\n", err)
	}

	os.Exit(code)
}

func setupTestDB(t *testing.T) (database.Database, *Service) {
	t.Helper()

	err := config.LoadConfig("")
	require.NoError(t, err)

	conn, err := testEnv.CloneTestDatabase(t, "convoy")
	require.NoError(t, err)

	dbHooks := hooks.Init()
	dbHooks.RegisterHook(datastore.EndpointCreated, func(ctx context.Context, data interface{}, changelog interface{}) {})

	db := postgres.NewFromConnection(conn)

	km, err := keys.NewLocalKeyManager("test")
	require.NoError(t, err)

	if km.IsSet() {
		_, err = km.GetCurrentKeyFromCache()
		require.NoError(t, err)
	}

	err = keys.Set(km)
	require.NoError(t, err)

	logger := log.New("convoy", log.LevelInfo)
	return db, New(logger, db)
}

func seedProject(t *testing.T, db database.Database) *datastore.Project {
	t.Helper()
	ctx := context.Background()
	logger := log.New("convoy", log.LevelInfo)

	userRepo := users.New(logger, db)
	user := &datastore.User{
		UID:       ulid.Make().String(),
		FirstName: "Test",
		LastName:  "User",
		Email:     fmt.Sprintf("test-%s@example.com", ulid.Make().String()),
	}
	require.NoError(t, userRepo.CreateUser(ctx, user))

	orgRepo := organisations.New(logger, db)
	org := &datastore.Organisation{
		UID:     ulid.Make().String(),
		Name:    "Test Org",
		OwnerID: user.UID,
	}
	require.NoError(t, orgRepo.CreateOrganisation(ctx, org))

	projectRepo := projects.New(logger, db)
	projectConfig := datastore.DefaultProjectConfig
	project := &datastore.Project{
		UID:            ulid.Make().String(),
		Name:           "Test Project",
		Type:           datastore.OutgoingProject,
		OrganisationID: org.UID,
		Config:         &projectConfig,
	}
	require.NoError(t, projectRepo.CreateProject(ctx, project))

	return project
}

func TestCreateJob(t *testing.T) {
	db, svc := setupTestDB(t)
	project := seedProject(t, db)
	ctx := context.Background()

	job := &datastore.Job{
		UID:       ulid.Make().String(),
		Type:      "search_tokenizer",
		Status:    datastore.JobStatusReady,
		ProjectID: project.UID,
	}

	require.NoError(t, svc.CreateJob(ctx, job))

	fetched, err := svc.FetchJobById(ctx, job.UID, job.ProjectID)
	require.NoError(t, err)
	require.Equal(t, job.UID, fetched.UID)
	require.Equal(t, datastore.JobStatusReady, fetched.Status)
	require.Equal(t, "search_tokenizer", fetched.Type)
}

func TestFetchJobById(t *testing.T) {
	db, svc := setupTestDB(t)
	project := seedProject(t, db)
	ctx := context.Background()

	job := &datastore.Job{
		UID:       ulid.Make().String(),
		Type:      "search_tokenizer",
		Status:    datastore.JobStatusReady,
		ProjectID: project.UID,
	}
	require.NoError(t, svc.CreateJob(ctx, job))

	t.Run("found", func(t *testing.T) {
		fetched, err := svc.FetchJobById(ctx, job.UID, job.ProjectID)
		require.NoError(t, err)
		require.Equal(t, job.UID, fetched.UID)
	})

	t.Run("not found", func(t *testing.T) {
		_, err := svc.FetchJobById(ctx, "nonexistent", project.UID)
		require.Equal(t, datastore.ErrJobNotFound, err)
	})
}

func TestFetchJobsByProjectId(t *testing.T) {
	db, svc := setupTestDB(t)
	p1 := seedProject(t, db)
	p2 := seedProject(t, db)
	ctx := context.Background()

	require.NoError(t, svc.CreateJob(ctx, &datastore.Job{
		UID:       ulid.Make().String(),
		Type:      "create",
		Status:    datastore.JobStatusRunning,
		ProjectID: p1.UID,
	}))

	require.NoError(t, svc.CreateJob(ctx, &datastore.Job{
		UID:       ulid.Make().String(),
		Type:      "update",
		Status:    datastore.JobStatusCompleted,
		ProjectID: p2.UID,
	}))

	require.NoError(t, svc.CreateJob(ctx, &datastore.Job{
		UID:       ulid.Make().String(),
		Type:      "update",
		Status:    datastore.JobStatusFailed,
		ProjectID: p2.UID,
	}))

	jobs, err := svc.FetchJobsByProjectId(ctx, p2.UID)
	require.NoError(t, err)
	require.Equal(t, 2, len(jobs))
}

func TestFetchRunningJobsByProjectId(t *testing.T) {
	db, svc := setupTestDB(t)
	p1 := seedProject(t, db)
	p2 := seedProject(t, db)
	ctx := context.Background()

	require.NoError(t, svc.CreateJob(ctx, &datastore.Job{
		UID: ulid.Make().String(), Type: "create", Status: datastore.JobStatusRunning, ProjectID: p1.UID,
	}))

	require.NoError(t, svc.CreateJob(ctx, &datastore.Job{
		UID: ulid.Make().String(), Type: "update", Status: datastore.JobStatusRunning, ProjectID: p2.UID,
	}))

	require.NoError(t, svc.CreateJob(ctx, &datastore.Job{
		UID: ulid.Make().String(), Type: "update", Status: datastore.JobStatusFailed, ProjectID: p2.UID,
	}))

	jobs, err := svc.FetchRunningJobsByProjectId(ctx, p2.UID)
	require.NoError(t, err)
	require.Equal(t, 1, len(jobs))
}

func TestMarkJobAsStarted(t *testing.T) {
	db, svc := setupTestDB(t)
	project := seedProject(t, db)
	ctx := context.Background()

	job := &datastore.Job{
		UID: ulid.Make().String(), Type: "search_tokenizer", Status: datastore.JobStatusReady, ProjectID: project.UID,
	}
	require.NoError(t, svc.CreateJob(ctx, job))
	require.NoError(t, svc.MarkJobAsStarted(ctx, job.UID, job.ProjectID))

	fetched, err := svc.FetchJobById(ctx, job.UID, job.ProjectID)
	require.NoError(t, err)
	require.Equal(t, datastore.JobStatusRunning, fetched.Status)
	require.True(t, fetched.StartedAt.Valid)
	require.True(t, time.Now().After(fetched.StartedAt.Time))
	require.False(t, fetched.FailedAt.Valid)
	require.False(t, fetched.CompletedAt.Valid)
}

func TestMarkJobAsCompleted(t *testing.T) {
	db, svc := setupTestDB(t)
	project := seedProject(t, db)
	ctx := context.Background()

	job := &datastore.Job{
		UID: ulid.Make().String(), Type: "search_tokenizer", Status: datastore.JobStatusReady, ProjectID: project.UID,
	}
	require.NoError(t, svc.CreateJob(ctx, job))
	require.NoError(t, svc.MarkJobAsStarted(ctx, job.UID, job.ProjectID))
	require.NoError(t, svc.MarkJobAsCompleted(ctx, job.UID, job.ProjectID))

	fetched, err := svc.FetchJobById(ctx, job.UID, job.ProjectID)
	require.NoError(t, err)
	require.Equal(t, datastore.JobStatusCompleted, fetched.Status)
	require.True(t, fetched.StartedAt.Valid)
	require.True(t, fetched.CompletedAt.Valid)
	require.False(t, fetched.FailedAt.Valid)
}

func TestMarkJobAsFailed(t *testing.T) {
	db, svc := setupTestDB(t)
	project := seedProject(t, db)
	ctx := context.Background()

	job := &datastore.Job{
		UID: ulid.Make().String(), Type: "search_tokenizer", Status: datastore.JobStatusReady, ProjectID: project.UID,
	}
	require.NoError(t, svc.CreateJob(ctx, job))
	require.NoError(t, svc.MarkJobAsStarted(ctx, job.UID, job.ProjectID))
	require.NoError(t, svc.MarkJobAsFailed(ctx, job.UID, job.ProjectID))

	fetched, err := svc.FetchJobById(ctx, job.UID, job.ProjectID)
	require.NoError(t, err)
	require.Equal(t, datastore.JobStatusFailed, fetched.Status)
	require.True(t, fetched.StartedAt.Valid)
	require.True(t, fetched.FailedAt.Valid)
	require.False(t, fetched.CompletedAt.Valid)
}

func TestDeleteJob(t *testing.T) {
	db, svc := setupTestDB(t)
	project := seedProject(t, db)
	ctx := context.Background()

	job := &datastore.Job{
		UID: ulid.Make().String(), Type: "search_tokenizer", Status: datastore.JobStatusReady, ProjectID: project.UID,
	}
	require.NoError(t, svc.CreateJob(ctx, job))
	require.NoError(t, svc.DeleteJob(ctx, job.UID, job.ProjectID))

	_, err := svc.FetchJobById(ctx, job.UID, job.ProjectID)
	require.Equal(t, datastore.ErrJobNotFound, err)
}

func TestLoadJobsPaged(t *testing.T) {
	tests := []struct {
		name     string
		pageData datastore.Pageable
		count    int
		perPage  int
	}{
		{name: "10 records, page size 3", pageData: datastore.Pageable{PerPage: 3}, count: 10, perPage: 3},
		{name: "12 records, page size 4", pageData: datastore.Pageable{PerPage: 4}, count: 12, perPage: 4},
		{name: "5 records, page size 3", pageData: datastore.Pageable{PerPage: 3}, count: 5, perPage: 3},
		{name: "1 record, page size 3", pageData: datastore.Pageable{PerPage: 3}, count: 1, perPage: 3},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			db, svc := setupTestDB(t)
			project := seedProject(t, db)
			ctx := context.Background()

			for i := 0; i < tc.count; i++ {
				require.NoError(t, svc.CreateJob(ctx, &datastore.Job{
					UID:       ulid.Make().String(),
					ProjectID: project.UID,
					Status:    datastore.JobStatusReady,
				}))
			}

			_, pageable, err := svc.LoadJobsPaged(ctx, project.UID, tc.pageData)
			require.NoError(t, err)
			require.Equal(t, int64(tc.perPage), pageable.PerPage)
		})
	}
}
