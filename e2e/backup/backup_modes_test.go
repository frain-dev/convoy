package backup

import (
	"context"
	"fmt"
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
	"github.com/frain-dev/convoy/internal/pkg/backup_collector"
	blobstore "github.com/frain-dev/convoy/internal/pkg/blob-store"
	"github.com/frain-dev/convoy/internal/projects"
	log "github.com/frain-dev/convoy/pkg/logger"
	"github.com/frain-dev/convoy/worker/task"
)

// seedTestData seeds events, deliveries, and attempts for backup testing.
// Returns the counts seeded.
func seedTestData(t *testing.T, env *E2ETestEnv, n int) (int, int, int) {
	t.Helper()
	ctx := context.Background()
	db := env.App.DB
	project := env.Project
	logger := log.New("convoy", log.LevelInfo)

	endpoint := &datastore.Endpoint{
		UID:            ulid.Make().String(),
		ProjectID:      project.UID,
		OwnerID:        project.UID,
		Url:            "https://example.com/webhook",
		Name:           "Backup Test Endpoint",
		Secrets:        []datastore.Secret{{UID: ulid.Make().String(), Value: "test-secret"}},
		Status:         datastore.ActiveEndpointStatus,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
		Authentication: &datastore.EndpointAuthentication{},
	}
	endpointRepo := endpoints.New(logger, db)
	err := endpointRepo.CreateEndpoint(ctx, endpoint, project.UID)
	require.NoError(t, err)

	// Seed old data (26 hours old — eligible for export)
	evtCount, dlvCount, attCount := 0, 0, 0
	for range n {
		evt := seedOldEvent(t, db, ctx, project, endpoint, 26)
		sub := seedSubscription(t, db, ctx, project, endpoint)
		dlv := seedOldEventDelivery(t, db, ctx, evt, endpoint, 26)
		seedOldDeliveryAttempt(t, db, ctx, dlv, endpoint, 26)
		evtCount++
		dlvCount++
		attCount++
		_ = sub
	}

	return evtCount, dlvCount, attCount
}

// ============================================================================
// CDC Mode Tests
// ============================================================================

func TestBackup_CDC_OnPrem(t *testing.T) {
	env := SetupE2EWithoutWorker(t)
	ctx := context.Background()
	tmpDir := t.TempDir()
	logger := log.New("convoy", log.LevelInfo)

	// Configure on-prem storage
	createOnPremConfig(t, env.App.DB, ctx, tmpDir)

	// Seed data
	evtCount, _, _ := seedTestData(t, env, 3)
	require.Equal(t, 3, evtCount)

	// Build replication DSN from the pool config
	pool := env.App.DB.GetConn()
	cfg := pool.Config().ConnConfig
	replDSN := fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=disable",
		cfg.User, cfg.Password, cfg.Host, cfg.Port, cfg.Database)

	// Create publication
	_, err := pool.Exec(ctx, `
		DO $$ BEGIN
			IF NOT EXISTS (SELECT 1 FROM pg_publication WHERE pubname = 'convoy_backup') THEN
				CREATE PUBLICATION convoy_backup FOR TABLE convoy.events, convoy.event_deliveries, convoy.delivery_attempts;
			END IF;
		END $$;
	`)
	require.NoError(t, err)

	// Start collector with short flush interval
	store, err := blobstore.NewOnPremClient(blobstore.BlobStoreOptions{OnPremStorageDir: tmpDir}, logger)
	require.NoError(t, err)

	collector := backup_collector.NewBackupCollector(pool, replDSN, store, 3*time.Second, logger)
	err = collector.Start(ctx)
	require.NoError(t, err)

	// Insert more events AFTER collector starts (CDC captures new INSERTs)
	db := env.App.DB
	endpoint := &datastore.Endpoint{
		UID:            ulid.Make().String(),
		ProjectID:      env.Project.UID,
		OwnerID:        env.Project.UID,
		Url:            "https://example.com/cdc-test",
		Name:           "CDC Test Endpoint",
		Secrets:        []datastore.Secret{{UID: ulid.Make().String(), Value: "s"}},
		Status:         datastore.ActiveEndpointStatus,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
		Authentication: &datastore.EndpointAuthentication{},
	}
	endpointRepo := endpoints.New(logger, db)
	err = endpointRepo.CreateEndpoint(ctx, endpoint, env.Project.UID)
	require.NoError(t, err)

	for range 5 {
		seedOldEvent(t, db, ctx, env.Project, endpoint, 0) // brand new events
	}

	// Wait for flush
	time.Sleep(5 * time.Second)

	collector.Stop(ctx)
	defer func() { _, _ = pool.Exec(ctx, "SELECT pg_drop_replication_slot('convoy_backup')") }()

	// Verify files and record counts
	files := findExportFiles(t, tmpDir, "events")
	require.NotEmpty(t, files, "should have CDC events backup files")

	var totalRecords int
	for _, f := range files {
		data := readExportFile(t, f)
		records := parseJSONL(t, data)
		totalRecords += len(records)
	}
	require.GreaterOrEqual(t, totalRecords, 5, "should have at least 5 CDC-captured events")
}

func TestBackup_CDC_S3(t *testing.T) {
	if infra.NewMinIOClient == nil {
		t.Skip("MinIO not available")
	}

	env := SetupE2EWithoutWorker(t)
	ctx := context.Background()
	logger := log.New("convoy", log.LevelInfo)

	minioClient, minioEndpoint, err := (*infra.NewMinIOClient)(t)
	require.NoError(t, err)

	pool := env.App.DB.GetConn()
	cfg := pool.Config().ConnConfig
	replDSN := fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=disable",
		cfg.User, cfg.Password, cfg.Host, cfg.Port, cfg.Database)

	_, err = pool.Exec(ctx, `
		DO $$ BEGIN
			IF NOT EXISTS (SELECT 1 FROM pg_publication WHERE pubname = 'convoy_backup') THEN
				CREATE PUBLICATION convoy_backup FOR TABLE convoy.events, convoy.event_deliveries, convoy.delivery_attempts;
			END IF;
		END $$;
	`)
	require.NoError(t, err)

	store, err := blobstore.NewS3Client(blobstore.BlobStoreOptions{
		Bucket:    "convoy-test-exports",
		AccessKey: "minioadmin",
		SecretKey: "minioadmin",
		Region:    "us-east-1",
		Endpoint:  "http://" + minioEndpoint,
	}, logger)
	require.NoError(t, err)

	collector := backup_collector.NewBackupCollector(pool, replDSN, store, 3*time.Second, logger)
	err = collector.Start(ctx)
	require.NoError(t, err)

	// Insert events after collector starts
	db := env.App.DB
	endpoint := &datastore.Endpoint{
		UID: ulid.Make().String(), ProjectID: env.Project.UID, OwnerID: env.Project.UID,
		Url: "https://example.com/s3-cdc", Name: "S3 CDC", Status: datastore.ActiveEndpointStatus,
		Secrets:   []datastore.Secret{{UID: ulid.Make().String(), Value: "s"}},
		CreatedAt: time.Now(), UpdatedAt: time.Now(), Authentication: &datastore.EndpointAuthentication{},
	}
	endpointRepo := endpoints.New(logger, db)
	require.NoError(t, endpointRepo.CreateEndpoint(ctx, endpoint, env.Project.UID))

	for range 5 {
		seedOldEvent(t, db, ctx, env.Project, endpoint, 0)
	}

	time.Sleep(5 * time.Second)
	collector.Stop(ctx)
	defer func() { _, _ = pool.Exec(ctx, "SELECT pg_drop_replication_slot('convoy_backup')") }()

	// Verify objects and record counts in MinIO
	objects := listMinIOObjects(t, minioClient, "convoy-test-exports", "backup/")
	require.NotEmpty(t, objects, "should have CDC backup objects in MinIO")

	eventsObj := findObject(objects, "events")
	require.NotNil(t, eventsObj, "should have events backup in MinIO")

	eventsData := downloadMinIOObject(t, minioClient, "convoy-test-exports", eventsObj.Key)
	eventsRecords := parseJSONL(t, eventsData)
	require.GreaterOrEqual(t, len(eventsRecords), 5, "should have at least 5 CDC-captured events in S3")
}

func TestBackup_CDC_Azure(t *testing.T) {
	if infra.NewAzuriteClient == nil {
		t.Skip("Azurite not available")
	}

	env := SetupE2EWithoutWorker(t)
	ctx := context.Background()
	logger := log.New("convoy", log.LevelInfo)

	azClient, azEndpoint, err := (*infra.NewAzuriteClient)(t)
	require.NoError(t, err)

	pool := env.App.DB.GetConn()
	cfg := pool.Config().ConnConfig
	replDSN := fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=disable",
		cfg.User, cfg.Password, cfg.Host, cfg.Port, cfg.Database)

	_, err = pool.Exec(ctx, `
		DO $$ BEGIN
			IF NOT EXISTS (SELECT 1 FROM pg_publication WHERE pubname = 'convoy_backup') THEN
				CREATE PUBLICATION convoy_backup FOR TABLE convoy.events, convoy.event_deliveries, convoy.delivery_attempts;
			END IF;
		END $$;
	`)
	require.NoError(t, err)

	store, err := blobstore.NewAzureBlobClient(blobstore.BlobStoreOptions{
		AzureAccountName:   "devstoreaccount1",
		AzureAccountKey:    "Eby8vdM02xNOcqFlqUwJPLlmEtlCDXJ1OUzFT50uSRZ6IFsuFq2UVErCz4I6tq/K1SZFPTOtr/KBHBeksoGMGw==",
		AzureContainerName: "convoy-test-exports",
		AzureEndpoint:      azEndpoint,
	}, logger)
	require.NoError(t, err)

	collector := backup_collector.NewBackupCollector(pool, replDSN, store, 3*time.Second, logger)
	err = collector.Start(ctx)
	require.NoError(t, err)

	db := env.App.DB
	endpoint := &datastore.Endpoint{
		UID: ulid.Make().String(), ProjectID: env.Project.UID, OwnerID: env.Project.UID,
		Url: "https://example.com/az-cdc", Name: "Azure CDC", Status: datastore.ActiveEndpointStatus,
		Secrets:   []datastore.Secret{{UID: ulid.Make().String(), Value: "s"}},
		CreatedAt: time.Now(), UpdatedAt: time.Now(), Authentication: &datastore.EndpointAuthentication{},
	}
	endpointRepo := endpoints.New(logger, db)
	require.NoError(t, endpointRepo.CreateEndpoint(ctx, endpoint, env.Project.UID))

	for range 5 {
		seedOldEvent(t, db, ctx, env.Project, endpoint, 0)
	}

	time.Sleep(5 * time.Second)
	collector.Stop(ctx)
	defer func() { _, _ = pool.Exec(ctx, "SELECT pg_drop_replication_slot('convoy_backup')") }()

	blobs := listAzuriteBlobs(t, azClient, "convoy-test-exports", "backup/")
	require.NotEmpty(t, blobs, "should have CDC backup blobs in Azurite")

	var eventsBlob string
	for _, b := range blobs {
		if strings.Contains(b, "/events/") {
			eventsBlob = b
			break
		}
	}
	require.NotEmpty(t, eventsBlob, "should have events backup in Azurite")

	eventsData := downloadAzuriteBlob(t, azClient, "convoy-test-exports", eventsBlob)
	eventsRecords := parseJSONL(t, eventsData)
	require.GreaterOrEqual(t, len(eventsRecords), 5, "should have at least 5 CDC-captured events in Azure")
}

// ============================================================================
// Export (Cron) Mode Tests
// ============================================================================

func TestBackup_Export_OnPrem(t *testing.T) {
	env := SetupE2EWithoutWorker(t)
	ctx := context.Background()
	tmpDir := t.TempDir()
	logger := log.New("convoy", log.LevelInfo)

	db := env.App.DB
	createOnPremConfig(t, db, ctx, tmpDir)
	seedTestData(t, env, 3)

	configRepo := configuration.New(logger, db)
	projectRepo := projects.New(logger, db)
	eventRepo := events.New(logger, db)
	eventDeliveryRepo := event_deliveries.New(logger, db)
	attemptsRepo := delivery_attempts.New(logger, db)

	backupTask := asynq.NewTask(string(convoy.ExportTableData), nil,
		asynq.Queue(string(convoy.ScheduleQueue)))
	err := task.ExportTableData(configRepo, projectRepo, eventRepo, eventDeliveryRepo, attemptsRepo, env.App.Redis, logger)(ctx, backupTask)
	require.NoError(t, err)

	// Verify files and record counts
	eventsFiles := findExportFiles(t, tmpDir, "events")
	require.NotEmpty(t, eventsFiles, "should have exported events files")
	eventsData := readExportFile(t, eventsFiles[0])
	require.GreaterOrEqual(t, len(parseJSONL(t, eventsData)), 3, "should have at least 3 exported events")

	deliveriesFiles := findExportFiles(t, tmpDir, "eventdeliveries")
	require.NotEmpty(t, deliveriesFiles, "should have exported deliveries files")
	deliveriesData := readExportFile(t, deliveriesFiles[0])
	require.GreaterOrEqual(t, len(parseJSONL(t, deliveriesData)), 3, "should have at least 3 exported deliveries")

	attemptsFiles := findExportFiles(t, tmpDir, "deliveryattempts")
	require.NotEmpty(t, attemptsFiles, "should have exported attempts files")
	attemptsData := readExportFile(t, attemptsFiles[0])
	require.GreaterOrEqual(t, len(parseJSONL(t, attemptsData)), 3, "should have at least 3 exported attempts")
}

func TestBackup_Export_S3(t *testing.T) {
	if infra.NewMinIOClient == nil {
		t.Skip("MinIO not available")
	}

	env := SetupE2EWithoutWorker(t)
	ctx := context.Background()
	logger := log.New("convoy", log.LevelInfo)

	_, minioEndpoint, err := (*infra.NewMinIOClient)(t)
	require.NoError(t, err)

	db := env.App.DB
	createMinIOConfig(t, db, ctx, minioEndpoint)
	seedTestData(t, env, 3)

	configRepo := configuration.New(logger, db)
	projectRepo := projects.New(logger, db)
	eventRepo := events.New(logger, db)
	eventDeliveryRepo := event_deliveries.New(logger, db)
	attemptsRepo := delivery_attempts.New(logger, db)

	backupTask := asynq.NewTask(string(convoy.ExportTableData), nil,
		asynq.Queue(string(convoy.ScheduleQueue)))
	err = task.ExportTableData(configRepo, projectRepo, eventRepo, eventDeliveryRepo, attemptsRepo, env.App.Redis, logger)(ctx, backupTask)
	require.NoError(t, err)

	minioClient, _, err := (*infra.NewMinIOClient)(t)
	require.NoError(t, err)

	objects := listMinIOObjects(t, minioClient, "convoy-test-exports", "orgs/")
	require.NotEmpty(t, objects, "should have exported objects in MinIO")

	for _, table := range []string{"events", "eventdeliveries", "deliveryattempts"} {
		obj := findObject(objects, table)
		require.NotNil(t, obj, "should have %s backup in MinIO", table)
		data := downloadMinIOObject(t, minioClient, "convoy-test-exports", obj.Key)
		records := parseJSONL(t, data)
		require.GreaterOrEqual(t, len(records), 3, "should have at least 3 exported %s in S3", table)
	}
}

func TestBackup_Export_Azure(t *testing.T) {
	if infra.NewAzuriteClient == nil {
		t.Skip("Azurite not available")
	}

	env := SetupE2EWithoutWorker(t)
	ctx := context.Background()
	logger := log.New("convoy", log.LevelInfo)

	azClient, azEndpoint, err := (*infra.NewAzuriteClient)(t)
	require.NoError(t, err)

	db := env.App.DB
	createAzuriteConfig(t, db, ctx, azEndpoint)
	seedTestData(t, env, 3)

	configRepo := configuration.New(logger, db)
	projectRepo := projects.New(logger, db)
	eventRepo := events.New(logger, db)
	eventDeliveryRepo := event_deliveries.New(logger, db)
	attemptsRepo := delivery_attempts.New(logger, db)

	backupTask := asynq.NewTask(string(convoy.ExportTableData), nil,
		asynq.Queue(string(convoy.ScheduleQueue)))
	err = task.ExportTableData(configRepo, projectRepo, eventRepo, eventDeliveryRepo, attemptsRepo, env.App.Redis, logger)(ctx, backupTask)
	require.NoError(t, err)

	blobs := listAzuriteBlobs(t, azClient, "convoy-test-exports", "orgs/")
	require.NotEmpty(t, blobs, "should have exported blobs in Azurite")

	for _, table := range []string{"events", "eventdeliveries", "deliveryattempts"} {
		var blobName string
		for _, b := range blobs {
			if strings.Contains(b, "/"+table+"/") {
				blobName = b
				break
			}
		}
		require.NotEmpty(t, blobName, "should have %s backup in Azurite", table)
		data := downloadAzuriteBlob(t, azClient, "convoy-test-exports", blobName)
		records := parseJSONL(t, data)
		require.GreaterOrEqual(t, len(records), 3, "should have at least 3 exported %s in Azure", table)
	}
}
