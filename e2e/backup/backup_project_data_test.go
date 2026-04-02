package backup

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/hibiken/asynq"
	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/require"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/internal/configuration"
	"github.com/frain-dev/convoy/internal/delivery_attempts"
	"github.com/frain-dev/convoy/internal/endpoints"
	"github.com/frain-dev/convoy/internal/event_deliveries"
	"github.com/frain-dev/convoy/internal/events"
	"github.com/frain-dev/convoy/internal/organisations"
	"github.com/frain-dev/convoy/internal/projects"
	log "github.com/frain-dev/convoy/pkg/logger"
	"github.com/frain-dev/convoy/worker/task"
)

func TestE2E_BackupProjectData_MinIO(t *testing.T) {
	env := SetupE2EWithoutWorker(t)
	ctx := context.Background()

	// Get MinIO client from test infrastructure
	minioClient, minioEndpoint, err := (*infra.NewMinIOClient)(t)
	require.NoError(t, err, "failed to create MinIO client")

	// Get database and repositories
	db := env.App.DB
	logger := log.New("convoy", log.LevelInfo)
	projectRepo := projects.New(logger, db)
	configRepo := configuration.New(logger, db)
	eventRepo := events.New(logger, db)
	eventDeliveryRepo := event_deliveries.New(logger, db)
	attemptsRepo := delivery_attempts.New(logger, db)

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
	endpointRepo := endpoints.New(log.New("convoy", log.LevelInfo), db)
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
		logger,
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
	events := parseJSONL(t, eventsData)
	require.GreaterOrEqual(t, len(events), 1, "should have at least 1 old event exported")
	require.True(t, containsUID(events, oldEvent.UID), "exported events should contain the old event")

	// Verify time filtering and project isolation for events
	verifyTimeFiltering(t, eventsData)

	// Find and verify event deliveries export
	deliveriesObj := findObject(objects, "eventdeliveries")
	require.NotNil(t, deliveriesObj, "should have event deliveries export in MinIO")

	deliveriesData := downloadMinIOObject(t, minioClient, "convoy-test-exports", deliveriesObj.Key)
	deliveries := parseJSONL(t, deliveriesData)
	require.GreaterOrEqual(t, len(deliveries), 1, "should have at least 1 old event delivery exported")
	require.True(t, containsUID(deliveries, oldDelivery.UID), "exported deliveries should contain the old delivery")

	verifyTimeFiltering(t, deliveriesData)

	// Find and verify delivery attempts export
	attemptsObj := findObject(objects, "deliveryattempts")
	require.NotNil(t, attemptsObj, "should have delivery attempts export in MinIO")

	attemptsData := downloadMinIOObject(t, minioClient, "convoy-test-exports", attemptsObj.Key)
	attempts := parseJSONL(t, attemptsData)
	require.GreaterOrEqual(t, len(attempts), 1, "should have at least 1 old delivery attempt exported")

	verifyTimeFiltering(t, attemptsData)
}

func TestE2E_BackupProjectData_OnPrem(t *testing.T) {
	env := SetupE2EWithoutWorker(t)
	ctx := context.Background()

	// Create temporary directory for OnPrem exports
	tmpDir := t.TempDir()

	// Get database and repositories
	db := env.App.DB
	logger := log.New("convoy", log.LevelInfo)
	projectRepo := projects.New(logger, db)
	configRepo := configuration.New(logger, db)
	eventRepo := events.New(logger, db)
	eventDeliveryRepo := event_deliveries.New(logger, db)
	attemptsRepo := delivery_attempts.New(logger, db)

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
	endpointRepo := endpoints.New(log.New("convoy", log.LevelInfo), db)
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
		logger,
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
	events := parseJSONL(t, eventsData)
	require.GreaterOrEqual(t, len(events), 1, "should have at least 1 old event exported")
	require.True(t, containsUID(events, oldEvent.UID), "exported events should contain the old event")

	// Verify time filtering - all events should be older than 24 hours
	verifyTimeFiltering(t, eventsData)

	// Verify event deliveries export content
	deliveriesData := readExportFile(t, deliveriesFiles[0])
	deliveries := parseJSONL(t, deliveriesData)
	require.GreaterOrEqual(t, len(deliveries), 1, "should have at least 1 old event delivery exported")
	require.True(t, containsUID(deliveries, oldDelivery.UID), "exported deliveries should contain the old delivery")

	verifyTimeFiltering(t, deliveriesData)

	// Verify delivery attempts export content
	attemptsData := readExportFile(t, attemptsFiles[0])
	attempts := parseJSONL(t, attemptsData)
	require.GreaterOrEqual(t, len(attempts), 1, "should have at least 1 old delivery attempt exported")

	verifyTimeFiltering(t, attemptsData)
}

func TestE2E_BackupProjectData_MultiTenant(t *testing.T) {
	env := SetupE2EWithoutWorker(t)
	ctx := context.Background()

	// Create temporary directory for OnPrem exports
	tmpDir := t.TempDir()

	// Get database and repositories
	db := env.App.DB
	logger := log.New("convoy", log.LevelInfo)
	projectRepo := projects.New(logger, db)
	configRepo := configuration.New(logger, db)
	eventRepo := events.New(logger, db)
	eventDeliveryRepo := event_deliveries.New(logger, db)
	attemptsRepo := delivery_attempts.New(logger, db)
	endpointRepo := endpoints.New(logger, db)
	orgService := organisations.New(logger, db)

	// Create first organization and project
	_ = env.Organisation
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
		logger,
	)(ctx, backupTask)
	require.NoError(t, err)

	// Export is global — all events from both projects in one file
	eventsFiles := findExportFiles(t, tmpDir, "events")
	require.NotEmpty(t, eventsFiles, "should have events export file")

	eventsData := readExportFile(t, eventsFiles[0])
	allEvents := parseJSONL(t, eventsData)
	// 3 from project1 + 2 from project2 = at least 5
	require.GreaterOrEqual(t, len(allEvents), 5, "should have at least 5 total events from both projects")
}

func TestE2E_BackupProjectData_TimeFiltering(t *testing.T) {
	env := SetupE2EWithoutWorker(t)
	ctx := context.Background()

	// Create temporary directory for OnPrem exports
	tmpDir := t.TempDir()

	// Get database and repositories
	db := env.App.DB
	logger := log.New("convoy", log.LevelInfo)
	projectRepo := projects.New(logger, db)
	configRepo := configuration.New(logger, db)
	eventRepo := events.New(logger, db)
	eventDeliveryRepo := event_deliveries.New(logger, db)
	attemptsRepo := delivery_attempts.New(logger, db)

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
	endpointRepo := endpoints.New(log.New("convoy", log.LevelInfo), db)
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
		logger,
	)(ctx, backupTask)
	require.NoError(t, err)

	// Verify only old events (>24h) were exported
	eventsFiles := findExportFiles(t, tmpDir, "events")
	require.Len(t, eventsFiles, 1, "should have 1 events export file")

	eventsData := readExportFile(t, eventsFiles[0])
	events := parseJSONL(t, eventsData)
	require.GreaterOrEqual(t, len(events), 2, "should have at least 2 old events")

	// Verify all exported events are older than 24 hours
	verifyTimeFiltering(t, eventsData)

	// Verify delivery attempts - should also have exactly 2
	attemptsFiles := findExportFiles(t, tmpDir, "deliveryattempts")
	require.Len(t, attemptsFiles, 1, "should have 1 delivery attempts export file")

	attemptsData := readExportFile(t, attemptsFiles[0])
	attempts := parseJSONL(t, attemptsData)
	require.GreaterOrEqual(t, len(attempts), 2, "should have at least 2 old delivery attempts")

	verifyTimeFiltering(t, attemptsData)
}

func TestE2E_BackupProjectData_AllTables(t *testing.T) {
	env := SetupE2EWithoutWorker(t)
	ctx := context.Background()

	// Create temporary directory for OnPrem exports
	tmpDir := t.TempDir()

	// Get database and repositories
	db := env.App.DB
	logger := log.New("convoy", log.LevelInfo)
	projectRepo := projects.New(logger, db)
	configRepo := configuration.New(logger, db)
	eventRepo := events.New(logger, db)
	eventDeliveryRepo := event_deliveries.New(logger, db)
	attemptsRepo := delivery_attempts.New(logger, db)

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
	endpointRepo := endpoints.New(log.New("convoy", log.LevelInfo), db)
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
		logger,
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
	verifyJSONLStructure(t, eventsData, 1)

	deliveriesData := readExportFile(t, deliveriesFiles[0])
	verifyJSONLStructure(t, deliveriesData, 1)

	attemptsData := readExportFile(t, attemptsFiles[0])
	verifyJSONLStructure(t, attemptsData, 1)

	// Verify directory structure is correct
	expectedEventsPath := filepath.Join(tmpDir, "orgs", org.UID, "projects", project.UID, "events")
	expectedDeliveriesPath := filepath.Join(tmpDir, "orgs", org.UID, "projects", project.UID, "eventdeliveries")
	expectedAttemptsPath := filepath.Join(tmpDir, "orgs", org.UID, "projects", project.UID, "deliveryattempts")

	require.Contains(t, eventsFiles[0], expectedEventsPath, "events file should be in correct directory")
	require.Contains(t, deliveriesFiles[0], expectedDeliveriesPath, "deliveries file should be in correct directory")
	require.Contains(t, attemptsFiles[0], expectedAttemptsPath, "attempts file should be in correct directory")
}

func TestE2E_BackupProjectData_AzureBlob(t *testing.T) {
	if infra.NewAzuriteClient == nil {
		t.Skip("Azurite not available")
	}

	env := SetupE2EWithoutWorker(t)
	ctx := context.Background()

	// Get Azurite client
	azClient, azEndpoint, err := (*infra.NewAzuriteClient)(t)
	require.NoError(t, err)

	// Get database and repositories
	db := env.App.DB
	logger := log.New("convoy", log.LevelInfo)
	projectRepo := projects.New(logger, db)
	configRepo := configuration.New(logger, db)
	eventRepo := events.New(logger, db)
	eventDeliveryRepo := event_deliveries.New(logger, db)
	attemptsRepo := delivery_attempts.New(logger, db)

	org := env.Organisation
	project := env.Project

	// Configure Azure Blob storage
	createAzuriteConfig(t, db, ctx, azEndpoint)

	// Seed an endpoint
	endpoint := &datastore.Endpoint{
		UID:       ulid.Make().String(),
		ProjectID: project.UID,
		OwnerID:   project.UID,
		Url:       "https://example.com/webhook",
		Name:      "Test Endpoint Azure",
		Secrets: []datastore.Secret{
			{UID: ulid.Make().String(), Value: "test-secret"},
		},
		Status:         datastore.ActiveEndpointStatus,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
		Authentication: &datastore.EndpointAuthentication{},
	}
	endpointRepo := endpoints.New(logger, db)
	err = endpointRepo.CreateEndpoint(ctx, endpoint, project.UID)
	require.NoError(t, err)

	// Seed old data (26 hours old - should be exported)
	oldEvent := seedOldEvent(t, db, ctx, project, endpoint, 26)
	oldDelivery := seedOldEventDelivery(t, db, ctx, oldEvent, endpoint, 26)
	seedOldDeliveryAttempt(t, db, ctx, oldDelivery, endpoint, 26)

	// Seed recent data (12 hours old - should NOT be exported)
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
		logger,
	)(ctx, backupTask)
	require.NoError(t, err)

	// List exported blobs
	prefix := fmt.Sprintf("orgs/%s/projects/%s/", org.UID, project.UID)
	blobs := listAzuriteBlobs(t, azClient, "convoy-test-exports", prefix)
	require.Len(t, blobs, 3, "should have 3 export files (events, deliveries, attempts)")

	// Find blobs by path
	var eventsBlob, deliveriesBlob, attemptsBlob string
	for _, b := range blobs {
		switch {
		case strings.Contains(b, "/events/"):
			eventsBlob = b
		case strings.Contains(b, "/eventdeliveries/"):
			deliveriesBlob = b
		case strings.Contains(b, "/deliveryattempts/"):
			attemptsBlob = b
		}
	}
	require.NotEmpty(t, eventsBlob, "should have events export")
	require.NotEmpty(t, deliveriesBlob, "should have deliveries export")
	require.NotEmpty(t, attemptsBlob, "should have attempts export")

	// Download and verify events
	eventsData := downloadAzuriteBlob(t, azClient, "convoy-test-exports", eventsBlob)
	evts := parseJSONL(t, eventsData)
	require.GreaterOrEqual(t, len(evts), 1, "should have at least 1 old event exported")
	require.Equal(t, oldEvent.UID, evts[0]["uid"], "exported event should be the old one")

	verifyTimeFiltering(t, eventsData)

	// Download and verify deliveries
	deliveriesData := downloadAzuriteBlob(t, azClient, "convoy-test-exports", deliveriesBlob)
	dlvrs := parseJSONL(t, deliveriesData)
	require.GreaterOrEqual(t, len(dlvrs), 1, "should have at least 1 old delivery exported")

	// Download and verify attempts
	attemptsData := downloadAzuriteBlob(t, azClient, "convoy-test-exports", attemptsBlob)
	atmpts := parseJSONL(t, attemptsData)
	require.GreaterOrEqual(t, len(atmpts), 1, "should have at least 1 old delivery attempt exported")
}
