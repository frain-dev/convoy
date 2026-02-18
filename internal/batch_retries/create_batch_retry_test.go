package batch_retries

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/require"
	"gopkg.in/guregu/null.v4"

	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/database"
	"github.com/frain-dev/convoy/database/hooks"
	"github.com/frain-dev/convoy/database/postgres"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/internal/organisations"
	"github.com/frain-dev/convoy/internal/pkg/keys"
	"github.com/frain-dev/convoy/internal/projects"
	"github.com/frain-dev/convoy/internal/users"
	"github.com/frain-dev/convoy/pkg/log"
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

func setupTestDB(t *testing.T) (database.Database, context.Context) {
	t.Helper()

	ctx := context.Background()

	err := config.LoadConfig("")
	require.NoError(t, err)

	conn, err := testEnv.CloneTestDatabase(t, "convoy")
	require.NoError(t, err)

	dbHooks := hooks.Init()
	dbHooks.RegisterHook(datastore.EndpointCreated, func(ctx context.Context, data interface{}, changelog interface{}) {})

	db := postgres.NewFromConnection(conn)

	// Load config again and ensure it's set properly
	err = config.LoadConfig("")
	require.NoError(t, err)

	_, err = config.Get()
	require.NoError(t, err)

	// Initialize KeyManager
	km, err := keys.NewLocalKeyManager("test")
	require.NoError(t, err)

	if km.IsSet() {
		_, err = km.GetCurrentKeyFromCache()
		require.NoError(t, err)
	}

	err = keys.Set(km)
	require.NoError(t, err)

	return db, ctx
}

func createBatchRetryService(t *testing.T, db database.Database) *Service {
	t.Helper()
	return New(log.NewLogger(os.Stdout), db)
}

func seedProjectForBatchRetry(t *testing.T, db database.Database) *datastore.Project {
	ctx := context.Background()
	logger := log.NewLogger(os.Stdout)

	// Create user
	userRepo := users.New(logger, db)
	user := &datastore.User{
		UID:       ulid.Make().String(),
		FirstName: "Test",
		LastName:  "User",
		Email:     fmt.Sprintf("test-%s@example.com", ulid.Make().String()),
	}
	err := userRepo.CreateUser(ctx, user)
	require.NoError(t, err)

	// Create organisation
	orgRepo := organisations.New(logger, db)
	org := &datastore.Organisation{
		UID:     ulid.Make().String(),
		Name:    "Test Org",
		OwnerID: user.UID,
	}
	err = orgRepo.CreateOrganisation(ctx, org)
	require.NoError(t, err)

	// Create project
	projectRepo := projects.New(log.NewLogger(os.Stdout), db)
	projectConfig := datastore.DefaultProjectConfig
	project := &datastore.Project{
		UID:            ulid.Make().String(),
		Name:           "Test Project",
		Type:           datastore.OutgoingProject,
		OrganisationID: org.UID,
		Config:         &projectConfig,
	}
	err = projectRepo.CreateProject(ctx, project)
	require.NoError(t, err)

	return project
}

// ============================================================================
// CreateBatchRetry Tests
// ============================================================================

func TestCreateBatchRetry_ValidRequest(t *testing.T) {
	db, ctx := setupTestDB(t)
	project := seedProjectForBatchRetry(t, db)
	service := createBatchRetryService(t, db)

	now := time.Now().UTC().Truncate(time.Microsecond)
	batchRetry := &datastore.BatchRetry{
		ID:              ulid.Make().String(),
		ProjectID:       project.UID,
		Status:          datastore.BatchRetryStatusPending,
		TotalEvents:     100,
		ProcessedEvents: 0,
		FailedEvents:    0,
		Filter: datastore.RetryFilter{
			"ProjectID": project.UID,
			"Status":    []string{"Failed"},
		},
		CreatedAt: now,
		UpdatedAt: now,
	}

	err := service.CreateBatchRetry(ctx, batchRetry)
	require.NoError(t, err)

	// Verify the batch retry was created
	fetched, err := service.FindBatchRetryByID(ctx, batchRetry.ID)
	require.NoError(t, err)
	require.Equal(t, batchRetry.ID, fetched.ID)
	require.Equal(t, batchRetry.ProjectID, fetched.ProjectID)
	require.Equal(t, batchRetry.Status, fetched.Status)
	require.Equal(t, batchRetry.TotalEvents, fetched.TotalEvents)
	require.Equal(t, batchRetry.ProcessedEvents, fetched.ProcessedEvents)
	require.Equal(t, batchRetry.FailedEvents, fetched.FailedEvents)
}

func TestCreateBatchRetry_WithCompletedAt(t *testing.T) {
	db, ctx := setupTestDB(t)
	project := seedProjectForBatchRetry(t, db)
	service := createBatchRetryService(t, db)

	now := time.Now().UTC().Truncate(time.Microsecond)
	completedAt := now.Add(5 * time.Minute)
	batchRetry := &datastore.BatchRetry{
		ID:              ulid.Make().String(),
		ProjectID:       project.UID,
		Status:          datastore.BatchRetryStatusCompleted,
		TotalEvents:     50,
		ProcessedEvents: 50,
		FailedEvents:    0,
		Filter:          datastore.RetryFilter{},
		CreatedAt:       now,
		UpdatedAt:       completedAt,
		CompletedAt:     null.NewTime(completedAt, true),
	}

	err := service.CreateBatchRetry(ctx, batchRetry)
	require.NoError(t, err)

	// Verify completed_at was set
	fetched, err := service.FindBatchRetryByID(ctx, batchRetry.ID)
	require.NoError(t, err)
	require.True(t, fetched.CompletedAt.Valid)
	require.WithinDuration(t, completedAt, fetched.CompletedAt.Time, time.Second)
}

func TestCreateBatchRetry_WithError(t *testing.T) {
	db, ctx := setupTestDB(t)
	project := seedProjectForBatchRetry(t, db)
	service := createBatchRetryService(t, db)

	now := time.Now().UTC().Truncate(time.Microsecond)
	batchRetry := &datastore.BatchRetry{
		ID:              ulid.Make().String(),
		ProjectID:       project.UID,
		Status:          datastore.BatchRetryStatusFailed,
		TotalEvents:     100,
		ProcessedEvents: 50,
		FailedEvents:    50,
		Filter:          datastore.RetryFilter{},
		CreatedAt:       now,
		UpdatedAt:       now,
		Error:           "processing failed: connection timeout",
	}

	err := service.CreateBatchRetry(ctx, batchRetry)
	require.NoError(t, err)

	// Verify error was stored
	fetched, err := service.FindBatchRetryByID(ctx, batchRetry.ID)
	require.NoError(t, err)
	require.Equal(t, batchRetry.Error, fetched.Error)
}

func TestCreateBatchRetry_NilBatchRetry(t *testing.T) {
	db, ctx := setupTestDB(t)
	service := createBatchRetryService(t, db)

	err := service.CreateBatchRetry(ctx, nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "cannot be nil")
}

func TestCreateBatchRetry_WithFilter(t *testing.T) {
	db, ctx := setupTestDB(t)
	project := seedProjectForBatchRetry(t, db)
	service := createBatchRetryService(t, db)

	now := time.Now().UTC().Truncate(time.Microsecond)
	filter := datastore.RetryFilter{
		"ProjectID":   project.UID,
		"EndpointIDs": []string{"endpoint-1", "endpoint-2"},
		"Status":      []string{"Failed", "Discarded"},
		"SearchParams": map[string]any{
			"created_at_start": 1704067200,
			"created_at_end":   1704153600,
		},
	}

	batchRetry := &datastore.BatchRetry{
		ID:              ulid.Make().String(),
		ProjectID:       project.UID,
		Status:          datastore.BatchRetryStatusPending,
		TotalEvents:     200,
		ProcessedEvents: 0,
		FailedEvents:    0,
		Filter:          filter,
		CreatedAt:       now,
		UpdatedAt:       now,
	}

	err := service.CreateBatchRetry(ctx, batchRetry)
	require.NoError(t, err)

	// Verify filter was stored correctly
	fetched, err := service.FindBatchRetryByID(ctx, batchRetry.ID)
	require.NoError(t, err)
	require.NotNil(t, fetched.Filter)
	require.Equal(t, project.UID, fetched.Filter["ProjectID"])
}
