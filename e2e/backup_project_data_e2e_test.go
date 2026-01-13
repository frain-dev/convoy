package e2e

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/hibiken/asynq"
	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/require"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/database/postgres"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/internal/configuration"
	"github.com/frain-dev/convoy/internal/delivery_attempts"
	"github.com/frain-dev/convoy/internal/organisations"
	"github.com/frain-dev/convoy/internal/projects"
	"github.com/frain-dev/convoy/pkg/log"
	"github.com/frain-dev/convoy/worker/task"
)

func TestE2E_BackupProjectData_MinIO(t *testing.T) {
	env := SetupE2EWithoutWorker(t)
	ctx := context.Background()

	// Get MinIO client from test infrastructure
	minioClient, minioEndpoint, err := infra.NewMinIOClient(t)
	require.NoError(t, err, "failed to create MinIO client")

	// Get database and repositories
	db := env.App.DB
	projectRepo := projects.New(log.NewLogger(os.Stdout), db)
	configRepo := configuration.New(log.NewLogger(os.Stdout), db)
	eventRepo := postgres.NewEventRepo(db)
	eventDeliveryRepo := postgres.NewEventDeliveryRepo(db)
	attemptsRepo := delivery_attempts.New(log.NewLogger(os.Stdout), db)

	// Create organization and project
	org := env.Organisation
	project := env.Project

	// Create MinIO storage configuration
	_ = createMinIOConfig(t, db, ctx, minioEndpoint)

	// Seed an endpoint
	endpoint := &datastore.Endpoint{
		UID:       ulid.Make().String(),
		ProjectID: project.UID,
		OwnerID:   project.UID,
		Url:       "https://example.com/webhook",
		Name:      "Test Endpoint",
		Secrets: []datastore.Secret{
			{UID: ulid.Make().String(), Value: "test-secret"},
		},
		Status:         datastore.ActiveEndpointStatus,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
		Authentication: &datastore.EndpointAuthentication{},
	}
	endpointRepo := postgres.NewEndpointRepo(db)
	err = endpointRepo.CreateEndpoint(ctx, endpoint, project.UID)
	require.NoError(t, err)

	// Create old data (26 hours old - should be exported)
	oldEvent := seedOldEvent(t, db, ctx, project, endpoint, 26)
	oldDelivery := seedOldEventDelivery(t, db, ctx, oldEvent, endpoint, 26)
	seedOldDeliveryAttempt(t, db, ctx, oldDelivery, endpoint, 26)

	// Create recent data (12 hours old - should NOT be exported)
	recentEvent := seedOldEvent(t, db, ctx, project, endpoint, 12)
	recentDelivery := seedOldEventDelivery(t, db, ctx, recentEvent, endpoint, 12)
	seedOldDeliveryAttempt(t, db, ctx, recentDelivery, endpoint, 12)

	// Invoke BackupProjectData task
	backupTask := asynq.NewTask(string(convoy.BackupProjectData), nil,
		asynq.Queue(string(convoy.ScheduleQueue)))

	err = task.BackupProjectData(
		configRepo,
		projectRepo,
		eventRepo,
		eventDeliveryRepo,
		attemptsRepo,
		env.App.Redis,
	)(ctx, backupTask)
	require.NoError(t, err)

	// List objects in MinIO
	prefix := getMinIOPrefix(org.UID, project.UID)
	objects := listMinIOObjects(t, minioClient, "convoy-test-exports", prefix)
	require.Len(t, objects, 3, "should have 3 export files (events, deliveries, attempts)")

	// Find and verify events export
	eventsObj := findObject(objects, "events")
	require.NotNil(t, eventsObj, "should have events export in MinIO")

	eventsData := downloadMinIOObject(t, minioClient, "convoy-test-exports", eventsObj.Key)
	var events []map[string]interface{}
	err = json.Unmarshal(eventsData, &events)
	require.NoError(t, err)
	require.Len(t, events, 1, "should have 1 old event exported")
	require.Equal(t, oldEvent.UID, events[0]["uid"], "exported event should be the old one")

	// Verify time filtering and project isolation for events
	verifyTimeFiltering(t, eventsData)
	verifyProjectIsolation(t, eventsData, project.UID)

	// Find and verify event deliveries export
	deliveriesObj := findObject(objects, "eventdeliveries")
	require.NotNil(t, deliveriesObj, "should have event deliveries export in MinIO")

	deliveriesData := downloadMinIOObject(t, minioClient, "convoy-test-exports", deliveriesObj.Key)
	var deliveries []map[string]interface{}
	err = json.Unmarshal(deliveriesData, &deliveries)
	require.NoError(t, err)
	require.Len(t, deliveries, 1, "should have 1 old event delivery exported")
	require.Equal(t, oldDelivery.UID, deliveries[0]["uid"], "exported delivery should be the old one")

	verifyTimeFiltering(t, deliveriesData)
	verifyProjectIsolation(t, deliveriesData, project.UID)

	// Find and verify delivery attempts export
	attemptsObj := findObject(objects, "deliveryattempts")
	require.NotNil(t, attemptsObj, "should have delivery attempts export in MinIO")

	attemptsData := downloadMinIOObject(t, minioClient, "convoy-test-exports", attemptsObj.Key)
	var attempts []map[string]interface{}
	err = json.Unmarshal(attemptsData, &attempts)
	require.NoError(t, err)
	require.Len(t, attempts, 1, "should have 1 old delivery attempt exported")

	verifyTimeFiltering(t, attemptsData)
	verifyProjectIsolation(t, attemptsData, project.UID)
}

func TestE2E_BackupProjectData_OnPrem(t *testing.T) {
	env := SetupE2EWithoutWorker(t)
	ctx := context.Background()

	// Create temporary directory for OnPrem exports
	tmpDir := t.TempDir()

	// Get database and repositories
	db := env.App.DB
	projectRepo := projects.New(log.NewLogger(os.Stdout), db)
	configRepo := configuration.New(log.NewLogger(os.Stdout), db)
	eventRepo := postgres.NewEventRepo(db)
	eventDeliveryRepo := postgres.NewEventDeliveryRepo(db)
	attemptsRepo := delivery_attempts.New(log.NewLogger(os.Stdout), db)

	// Create organization and project
	project := env.Project

	// Create OnPrem storage configuration
	_ = createOnPremConfig(t, db, ctx, tmpDir)

	// Verify configuration was created correctly
	loadedConfig, err := configRepo.LoadConfiguration(ctx)
	require.NoError(t, err, "should load configuration")
	require.NotNil(t, loadedConfig, "configuration should not be nil")
	require.NotNil(t, loadedConfig.RetentionPolicy, "retention policy should not be nil")
	require.True(t, loadedConfig.RetentionPolicy.IsRetentionPolicyEnabled, "retention policy should be enabled")

	// Seed an endpoint
	endpoint := &datastore.Endpoint{
		UID:       ulid.Make().String(),
		ProjectID: project.UID,
		OwnerID:   project.UID,
		Url:       "https://example.com/webhook",
		Name:      "Test Endpoint",
		Secrets: []datastore.Secret{
			{UID: ulid.Make().String(), Value: "test-secret"},
		},
		Status:         datastore.ActiveEndpointStatus,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
		Authentication: &datastore.EndpointAuthentication{},
	}
	endpointRepo := postgres.NewEndpointRepo(db)
	err = endpointRepo.CreateEndpoint(ctx, endpoint, project.UID)
	require.NoError(t, err)

	// Create old data (26 hours old - should be exported)
	oldEvent := seedOldEvent(t, db, ctx, project, endpoint, 26)
	oldDelivery := seedOldEventDelivery(t, db, ctx, oldEvent, endpoint, 26)
	seedOldDeliveryAttempt(t, db, ctx, oldDelivery, endpoint, 26)

	// Create recent data (12 hours old - should NOT be exported)
	recentEvent := seedOldEvent(t, db, ctx, project, endpoint, 12)
	recentDelivery := seedOldEventDelivery(t, db, ctx, recentEvent, endpoint, 12)
	seedOldDeliveryAttempt(t, db, ctx, recentDelivery, endpoint, 12)

	// Invoke BackupProjectData task
	backupTask := asynq.NewTask(string(convoy.BackupProjectData), nil,
		asynq.Queue(string(convoy.ScheduleQueue)))

	err = task.BackupProjectData(
		configRepo,
		projectRepo,
		eventRepo,
		eventDeliveryRepo,
		attemptsRepo,
		env.App.Redis,
	)(ctx, backupTask)
	require.NoError(t, err)

	// Verify export files were created
	eventsFiles := findExportFiles(t, tmpDir, "events")
	require.Len(t, eventsFiles, 1, "should have 1 events export file")

	deliveriesFiles := findExportFiles(t, tmpDir, "eventdeliveries")
	require.Len(t, deliveriesFiles, 1, "should have 1 event deliveries export file")

	attemptsFiles := findExportFiles(t, tmpDir, "deliveryattempts")
	require.Len(t, attemptsFiles, 1, "should have 1 delivery attempts export file")

	// Verify events export content
	eventsData := readExportFile(t, eventsFiles[0])
	var events []map[string]interface{}
	err = json.Unmarshal(eventsData, &events)
	require.NoError(t, err)
	require.Len(t, events, 1, "should have 1 old event exported")
	require.Equal(t, oldEvent.UID, events[0]["uid"], "exported event should be the old one")

	// Verify time filtering - all events should be older than 24 hours
	verifyTimeFiltering(t, eventsData)
	verifyProjectIsolation(t, eventsData, project.UID)

	// Verify event deliveries export content
	deliveriesData := readExportFile(t, deliveriesFiles[0])
	var deliveries []map[string]interface{}
	err = json.Unmarshal(deliveriesData, &deliveries)
	require.NoError(t, err)
	require.Len(t, deliveries, 1, "should have 1 old event delivery exported")
	require.Equal(t, oldDelivery.UID, deliveries[0]["uid"], "exported delivery should be the old one")

	verifyTimeFiltering(t, deliveriesData)
	verifyProjectIsolation(t, deliveriesData, project.UID)

	// Verify delivery attempts export content
	attemptsData := readExportFile(t, attemptsFiles[0])
	var attempts []map[string]interface{}
	err = json.Unmarshal(attemptsData, &attempts)
	require.NoError(t, err)
	require.Len(t, attempts, 1, "should have 1 old delivery attempt exported")

	verifyTimeFiltering(t, attemptsData)
	verifyProjectIsolation(t, attemptsData, project.UID)
}

func TestE2E_BackupProjectData_MultiTenant(t *testing.T) {
	env := SetupE2EWithoutWorker(t)
	ctx := context.Background()

	// Create temporary directory for OnPrem exports
	tmpDir := t.TempDir()

	// Get database and repositories
	db := env.App.DB
	projectRepo := projects.New(log.NewLogger(os.Stdout), db)
	configRepo := configuration.New(log.NewLogger(os.Stdout), db)
	eventRepo := postgres.NewEventRepo(db)
	eventDeliveryRepo := postgres.NewEventDeliveryRepo(db)
	attemptsRepo := delivery_attempts.New(log.NewLogger(os.Stdout), db)
	endpointRepo := postgres.NewEndpointRepo(db)
	orgService := organisations.New(log.NewLogger(os.Stdout), db)

	// Create first organization and project
	org1 := env.Organisation
	project1 := env.Project
	user := env.User

	// Create second organization and project
	org2 := &datastore.Organisation{
		UID:       ulid.Make().String(),
		OwnerID:   user.UID,
		Name:      "Test Org 2",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	err := orgService.CreateOrganisation(ctx, org2)
	require.NoError(t, err)

	project2 := &datastore.Project{
		UID:            ulid.Make().String(),
		Name:           "Test Project 2",
		OrganisationID: org2.UID,
		Type:           datastore.IncomingProject,
		Config:         &datastore.DefaultProjectConfig,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}
	err = projectRepo.CreateProject(ctx, project2)
	require.NoError(t, err)

	// Create OnPrem storage configuration (global, not per-org)
	_ = createOnPremConfig(t, db, ctx, tmpDir)

	// Seed endpoints for both projects
	endpoint1 := &datastore.Endpoint{
		UID:       ulid.Make().String(),
		ProjectID: project1.UID,
		OwnerID:   project1.UID,
		Url:       "https://example.com/webhook1",
		Name:      "Test Endpoint 1",
		Secrets: []datastore.Secret{
			{UID: ulid.Make().String(), Value: "test-secret-1"},
		},
		Status:         datastore.ActiveEndpointStatus,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
		Authentication: &datastore.EndpointAuthentication{},
	}
	err = endpointRepo.CreateEndpoint(ctx, endpoint1, project1.UID)
	require.NoError(t, err)

	endpoint2 := &datastore.Endpoint{
		UID:       ulid.Make().String(),
		ProjectID: project2.UID,
		OwnerID:   project2.UID,
		Url:       "https://example.com/webhook2",
		Name:      "Test Endpoint 2",
		Secrets: []datastore.Secret{
			{UID: ulid.Make().String(), Value: "test-secret-2"},
		},
		Status:         datastore.ActiveEndpointStatus,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
		Authentication: &datastore.EndpointAuthentication{},
	}
	err = endpointRepo.CreateEndpoint(ctx, endpoint2, project2.UID)
	require.NoError(t, err)

	// Create old data for project1 (3 records)
	for i := 0; i < 3; i++ {
		oldEvent := seedOldEvent(t, db, ctx, project1, endpoint1, 26)
		oldDelivery := seedOldEventDelivery(t, db, ctx, oldEvent, endpoint1, 26)
		seedOldDeliveryAttempt(t, db, ctx, oldDelivery, endpoint1, 26)
	}

	// Create old data for project2 (2 records)
	for i := 0; i < 2; i++ {
		oldEvent := seedOldEvent(t, db, ctx, project2, endpoint2, 26)
		oldDelivery := seedOldEventDelivery(t, db, ctx, oldEvent, endpoint2, 26)
		seedOldDeliveryAttempt(t, db, ctx, oldDelivery, endpoint2, 26)
	}

	// Invoke BackupProjectData task
	backupTask := asynq.NewTask(string(convoy.BackupProjectData), nil,
		asynq.Queue(string(convoy.ScheduleQueue)))

	err = task.BackupProjectData(
		configRepo,
		projectRepo,
		eventRepo,
		eventDeliveryRepo,
		attemptsRepo,
		env.App.Redis,
	)(ctx, backupTask)
	require.NoError(t, err)

	// Verify project1 exports (should have 3 records each)
	project1Path := getExportPath(tmpDir, org1.UID, project1.UID, "events")
	project1EventsFiles := findExportFiles(t, project1Path, "")
	require.Len(t, project1EventsFiles, 1, "project1 should have events export file")

	project1EventsData := readExportFile(t, project1EventsFiles[0])
	verifyProjectIsolation(t, project1EventsData, project1.UID)
	var project1Events []map[string]interface{}
	err = json.Unmarshal(project1EventsData, &project1Events)
	require.NoError(t, err)
	require.Len(t, project1Events, 3, "project1 should have 3 events")

	// Verify project2 exports (should have 2 records each)
	project2Path := getExportPath(tmpDir, org2.UID, project2.UID, "events")
	project2EventsFiles := findExportFiles(t, project2Path, "")
	require.Len(t, project2EventsFiles, 1, "project2 should have events export file")

	project2EventsData := readExportFile(t, project2EventsFiles[0])
	verifyProjectIsolation(t, project2EventsData, project2.UID)
	var project2Events []map[string]interface{}
	err = json.Unmarshal(project2EventsData, &project2Events)
	require.NoError(t, err)
	require.Len(t, project2Events, 2, "project2 should have 2 events")
}

func TestE2E_BackupProjectData_TimeFiltering(t *testing.T) {
	env := SetupE2EWithoutWorker(t)
	ctx := context.Background()

	// Create temporary directory for OnPrem exports
	tmpDir := t.TempDir()

	// Get database and repositories
	db := env.App.DB
	projectRepo := projects.New(log.NewLogger(os.Stdout), db)
	configRepo := configuration.New(log.NewLogger(os.Stdout), db)
	eventRepo := postgres.NewEventRepo(db)
	eventDeliveryRepo := postgres.NewEventDeliveryRepo(db)
	attemptsRepo := delivery_attempts.New(log.NewLogger(os.Stdout), db)

	// Create organization and project
	project := env.Project

	// Create OnPrem storage configuration
	_ = createOnPremConfig(t, db, ctx, tmpDir)

	// Seed an endpoint
	endpoint := &datastore.Endpoint{
		UID:       ulid.Make().String(),
		ProjectID: project.UID,
		OwnerID:   project.UID,
		Url:       "https://example.com/webhook",
		Name:      "Test Endpoint",
		Secrets: []datastore.Secret{
			{UID: ulid.Make().String(), Value: "test-secret"},
		},
		Status:         datastore.ActiveEndpointStatus,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
		Authentication: &datastore.EndpointAuthentication{},
	}
	endpointRepo := postgres.NewEndpointRepo(db)
	err := endpointRepo.CreateEndpoint(ctx, endpoint, project.UID)
	require.NoError(t, err)

	// Create events at different timestamps
	// 1. Very old (26 hours) - should be exported
	event26h := seedOldEvent(t, db, ctx, project, endpoint, 26)
	delivery26h := seedOldEventDelivery(t, db, ctx, event26h, endpoint, 26)
	seedOldDeliveryAttempt(t, db, ctx, delivery26h, endpoint, 26)

	// 2. Just past cutoff (25 hours) - should be exported
	event25h := seedOldEvent(t, db, ctx, project, endpoint, 25)
	delivery25h := seedOldEventDelivery(t, db, ctx, event25h, endpoint, 25)
	seedOldDeliveryAttempt(t, db, ctx, delivery25h, endpoint, 25)

	// 3. Recent (12 hours) - should NOT be exported
	event12h := seedOldEvent(t, db, ctx, project, endpoint, 12)
	delivery12h := seedOldEventDelivery(t, db, ctx, event12h, endpoint, 12)
	seedOldDeliveryAttempt(t, db, ctx, delivery12h, endpoint, 12)

	// 4. Very recent (1 hour) - should NOT be exported
	event1h := seedOldEvent(t, db, ctx, project, endpoint, 1)
	delivery1h := seedOldEventDelivery(t, db, ctx, event1h, endpoint, 1)
	seedOldDeliveryAttempt(t, db, ctx, delivery1h, endpoint, 1)

	// Invoke BackupProjectData task
	backupTask := asynq.NewTask(string(convoy.BackupProjectData), nil,
		asynq.Queue(string(convoy.ScheduleQueue)))

	err = task.BackupProjectData(
		configRepo,
		projectRepo,
		eventRepo,
		eventDeliveryRepo,
		attemptsRepo,
		env.App.Redis,
	)(ctx, backupTask)
	require.NoError(t, err)

	// Verify only old events (>24h) were exported
	eventsFiles := findExportFiles(t, tmpDir, "events")
	require.Len(t, eventsFiles, 1, "should have 1 events export file")

	eventsData := readExportFile(t, eventsFiles[0])
	var events []map[string]interface{}
	err = json.Unmarshal(eventsData, &events)
	require.NoError(t, err)
	require.Len(t, events, 2, "should have exactly 2 old events (26h and 25h)")

	// Verify all exported events are older than 24 hours
	verifyTimeFiltering(t, eventsData)

	// Verify delivery attempts - should also have exactly 2
	attemptsFiles := findExportFiles(t, tmpDir, "deliveryattempts")
	require.Len(t, attemptsFiles, 1, "should have 1 delivery attempts export file")

	attemptsData := readExportFile(t, attemptsFiles[0])
	var attempts []map[string]interface{}
	err = json.Unmarshal(attemptsData, &attempts)
	require.NoError(t, err)
	require.Len(t, attempts, 2, "should have exactly 2 old delivery attempts")

	verifyTimeFiltering(t, attemptsData)
}

func TestE2E_BackupProjectData_AllTables(t *testing.T) {
	env := SetupE2EWithoutWorker(t)
	ctx := context.Background()

	// Create temporary directory for OnPrem exports
	tmpDir := t.TempDir()

	// Get database and repositories
	db := env.App.DB
	projectRepo := projects.New(log.NewLogger(os.Stdout), db)
	configRepo := configuration.New(log.NewLogger(os.Stdout), db)
	eventRepo := postgres.NewEventRepo(db)
	eventDeliveryRepo := postgres.NewEventDeliveryRepo(db)
	attemptsRepo := delivery_attempts.New(log.NewLogger(os.Stdout), db)

	// Create organization and project
	org := env.Organisation
	project := env.Project

	// Create OnPrem storage configuration
	_ = createOnPremConfig(t, db, ctx, tmpDir)

	// Seed an endpoint
	endpoint := &datastore.Endpoint{
		UID:       ulid.Make().String(),
		ProjectID: project.UID,
		OwnerID:   project.UID,
		Url:       "https://example.com/webhook",
		Name:      "Test Endpoint",
		Secrets: []datastore.Secret{
			{UID: ulid.Make().String(), Value: "test-secret"},
		},
		Status:         datastore.ActiveEndpointStatus,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
		Authentication: &datastore.EndpointAuthentication{},
	}
	endpointRepo := postgres.NewEndpointRepo(db)
	err := endpointRepo.CreateEndpoint(ctx, endpoint, project.UID)
	require.NoError(t, err)

	// Create old data (26 hours old)
	oldEvent := seedOldEvent(t, db, ctx, project, endpoint, 26)
	oldDelivery := seedOldEventDelivery(t, db, ctx, oldEvent, endpoint, 26)
	seedOldDeliveryAttempt(t, db, ctx, oldDelivery, endpoint, 26)

	// Invoke BackupProjectData task
	backupTask := asynq.NewTask(string(convoy.BackupProjectData), nil,
		asynq.Queue(string(convoy.ScheduleQueue)))

	err = task.BackupProjectData(
		configRepo,
		projectRepo,
		eventRepo,
		eventDeliveryRepo,
		attemptsRepo,
		env.App.Redis,
	)(ctx, backupTask)
	require.NoError(t, err)

	// Verify all 3 tables have export files
	eventsFiles := findExportFiles(t, tmpDir, "events")
	require.Len(t, eventsFiles, 1, "should have events export file")

	deliveriesFiles := findExportFiles(t, tmpDir, "eventdeliveries")
	require.Len(t, deliveriesFiles, 1, "should have event deliveries export file")

	attemptsFiles := findExportFiles(t, tmpDir, "deliveryattempts")
	require.Len(t, attemptsFiles, 1, "should have delivery attempts export file")

	// Verify all files contain valid JSON with at least 1 record
	eventsData := readExportFile(t, eventsFiles[0])
	verifyJSONStructure(t, eventsData, 1)

	deliveriesData := readExportFile(t, deliveriesFiles[0])
	verifyJSONStructure(t, deliveriesData, 1)

	attemptsData := readExportFile(t, attemptsFiles[0])
	verifyJSONStructure(t, attemptsData, 1)

	// Verify directory structure is correct
	expectedEventsPath := filepath.Join(tmpDir, "orgs", org.UID, "projects", project.UID, "events")
	expectedDeliveriesPath := filepath.Join(tmpDir, "orgs", org.UID, "projects", project.UID, "eventdeliveries")
	expectedAttemptsPath := filepath.Join(tmpDir, "orgs", org.UID, "projects", project.UID, "deliveryattempts")

	require.Contains(t, eventsFiles[0], expectedEventsPath, "events file should be in correct directory")
	require.Contains(t, deliveriesFiles[0], expectedDeliveriesPath, "deliveries file should be in correct directory")
	require.Contains(t, attemptsFiles[0], expectedAttemptsPath, "attempts file should be in correct directory")
}
