package e2e

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dchest/uniuri"
	"github.com/minio/minio-go/v7"
	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/require"
	"gopkg.in/guregu/null.v4"

	"github.com/frain-dev/convoy/database"
	"github.com/frain-dev/convoy/database/postgres"
	"github.com/frain-dev/convoy/datastore"
)

// MinIO Operations

// listMinIOObjects lists all objects in a MinIO bucket with the given prefix
func listMinIOObjects(t *testing.T, client *minio.Client, bucket, prefix string) []minio.ObjectInfo {
	t.Helper()

	ctx := context.Background()
	objects := make([]minio.ObjectInfo, 0)

	objectCh := client.ListObjects(ctx, bucket, minio.ListObjectsOptions{
		Prefix:    prefix,
		Recursive: true,
	})

	for object := range objectCh {
		require.NoError(t, object.Err, "error listing objects")
		objects = append(objects, object)
	}

	return objects
}

// downloadMinIOObject downloads an object from MinIO and returns its contents
func downloadMinIOObject(t *testing.T, client *minio.Client, bucket, key string) []byte {
	t.Helper()

	ctx := context.Background()
	object, err := client.GetObject(ctx, bucket, key, minio.GetObjectOptions{})
	require.NoError(t, err, "failed to get object from MinIO")
	defer object.Close()

	data, err := io.ReadAll(object)
	require.NoError(t, err, "failed to read object data")

	return data
}

// findObject finds an object in the list that contains the given path substring
func findObject(objects []minio.ObjectInfo, pathSubstring string) *minio.ObjectInfo {
	for i := range objects {
		if strings.Contains(objects[i].Key, pathSubstring) {
			return &objects[i]
		}
	}
	return nil
}

// OnPrem Operations

// readExportFile reads an export file from the filesystem
func readExportFile(t *testing.T, filePath string) []byte {
	t.Helper()

	data, err := os.ReadFile(filePath)
	require.NoError(t, err, "failed to read export file")

	return data
}

// findExportFiles finds export files in the base directory that contain the table name
func findExportFiles(t *testing.T, baseDir, tableName string) []string {
	t.Helper()

	var files []string
	err := filepath.Walk(baseDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && strings.Contains(path, tableName) && strings.HasSuffix(path, ".json") {
			files = append(files, path)
		}
		return nil
	})
	require.NoError(t, err, "failed to walk directory")

	return files
}

// Data Seeding with Specific Timestamps

// seedSource creates a source for testing
func seedSource(t *testing.T, db database.Database, ctx context.Context, project *datastore.Project) *datastore.Source {
	t.Helper()

	sourceRepo := postgres.NewSourceRepo(db)

	source := &datastore.Source{
		UID:       ulid.Make().String(),
		ProjectID: project.UID,
		Name:      "Test Source",
		MaskID:    uniuri.NewLen(16),
		Type:      datastore.HTTPSource,
		Verifier: &datastore.VerifierConfig{
			Type: datastore.HMacVerifier,
			HMac: &datastore.HMac{
				Header: "X-Test-Signature",
				Hash:   "SHA512",
				Secret: "test-secret",
			},
			ApiKey:    &datastore.ApiKey{},
			BasicAuth: &datastore.BasicAuth{},
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	err := sourceRepo.CreateSource(ctx, source)
	require.NoError(t, err, "failed to create source")

	return source
}

// seedOldEvent creates an event with a timestamp in the past
func seedOldEvent(t *testing.T, db database.Database, ctx context.Context, project *datastore.Project, endpoint *datastore.Endpoint, hoursOld int) *datastore.Event {
	t.Helper()

	// Create a source first (required for events)
	source := seedSource(t, db, ctx, project)

	eventRepo := postgres.NewEventRepo(db)

	event := &datastore.Event{
		UID:              ulid.Make().String(),
		EventType:        datastore.EventType("test.event"),
		ProjectID:        project.UID,
		SourceID:         source.UID,
		Endpoints:        []string{endpoint.UID},
		Data:             []byte(`{"test": "data"}`),
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
		AcknowledgedAt:   null.TimeFrom(time.Now()),
		Raw:              `{"test": "data"}`,
		IsDuplicateEvent: false,
	}

	err := eventRepo.CreateEvent(ctx, event)
	require.NoError(t, err, "failed to create event")

	// Update timestamp to be old
	targetTime := time.Now().Add(-time.Duration(hoursOld) * time.Hour)
	_, err = db.GetDB().ExecContext(ctx,
		"UPDATE convoy.events SET created_at=$1, updated_at=$1 WHERE id=$2",
		targetTime, event.UID)
	require.NoError(t, err, "failed to update event timestamp")

	// Reload event to get updated timestamps
	event, err = eventRepo.FindEventByID(ctx, project.UID, event.UID)
	require.NoError(t, err, "failed to reload event")

	return event
}

// seedSubscription creates a subscription for testing
func seedSubscription(t *testing.T, db database.Database, ctx context.Context, project *datastore.Project, endpoint *datastore.Endpoint) *datastore.Subscription {
	t.Helper()

	subscriptionRepo := postgres.NewSubscriptionRepo(db)

	subscription := &datastore.Subscription{
		UID:        ulid.Make().String(),
		ProjectID:  project.UID,
		Name:       "Test Subscription",
		Type:       datastore.SubscriptionTypeAPI,
		EndpointID: endpoint.UID,
		FilterConfig: &datastore.FilterConfiguration{
			EventTypes: []string{"*"},
		},
		RateLimitConfig: &datastore.RateLimitConfiguration{
			Count:    100,
			Duration: 60,
		},
		RetryConfig: &datastore.RetryConfiguration{
			Type:       datastore.LinearStrategyProvider,
			Duration:   10,
			RetryCount: 3,
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	err := subscriptionRepo.CreateSubscription(ctx, project.UID, subscription)
	require.NoError(t, err, "failed to create subscription")

	return subscription
}

// seedOldEventDelivery creates an event delivery with a timestamp in the past
func seedOldEventDelivery(t *testing.T, db database.Database, ctx context.Context, event *datastore.Event, endpoint *datastore.Endpoint, hoursOld int) *datastore.EventDelivery {
	t.Helper()

	// Create a subscription first (required for event deliveries)
	project := &datastore.Project{UID: event.ProjectID}
	subscription := seedSubscription(t, db, ctx, project, endpoint)

	eventDeliveryRepo := postgres.NewEventDeliveryRepo(db)

	eventDelivery := &datastore.EventDelivery{
		UID:            ulid.Make().String(),
		ProjectID:      event.ProjectID,
		EventID:        event.UID,
		EndpointID:     endpoint.UID,
		SubscriptionID: subscription.UID,
		Status:         datastore.SuccessEventStatus,
		Metadata: &datastore.Metadata{
			Data:            []byte(`{"test": "metadata"}`),
			Strategy:        datastore.ExponentialStrategyProvider,
			NextSendTime:    time.Now(),
			NumTrials:       1,
			IntervalSeconds: 10,
			RetryLimit:      3,
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	err := eventDeliveryRepo.CreateEventDelivery(ctx, eventDelivery)
	require.NoError(t, err, "failed to create event delivery")

	// Update timestamp to be old
	targetTime := time.Now().Add(-time.Duration(hoursOld) * time.Hour)
	_, err = db.GetDB().ExecContext(ctx,
		"UPDATE convoy.event_deliveries SET created_at=$1, updated_at=$1 WHERE id=$2",
		targetTime, eventDelivery.UID)
	require.NoError(t, err, "failed to update event delivery timestamp")

	// Reload event delivery to get updated timestamps
	eventDelivery, err = eventDeliveryRepo.FindEventDeliveryByID(ctx, event.ProjectID, eventDelivery.UID)
	require.NoError(t, err, "failed to reload event delivery")

	return eventDelivery
}

// seedOldDeliveryAttempt creates a delivery attempt with a timestamp in the past
func seedOldDeliveryAttempt(t *testing.T, db database.Database, ctx context.Context, delivery *datastore.EventDelivery, endpoint *datastore.Endpoint, hoursOld int) {
	t.Helper()

	attempt := &datastore.DeliveryAttempt{
		UID:             ulid.Make().String(),
		URL:             endpoint.Url,
		Method:          "POST",
		APIVersion:      "2024.01.01",
		EndpointID:      endpoint.UID,
		EventDeliveryId: delivery.UID,
		ProjectId:       delivery.ProjectID,
		Status:          true,
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
	}

	_, err := db.GetDB().ExecContext(ctx,
		`INSERT INTO convoy.delivery_attempts (id, event_delivery_id, url, method, endpoint_id, api_version, project_id, status, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`,
		attempt.UID, attempt.EventDeliveryId, attempt.URL, attempt.Method,
		attempt.EndpointID, attempt.APIVersion, attempt.ProjectId, attempt.Status,
		attempt.CreatedAt, attempt.UpdatedAt)
	require.NoError(t, err, "failed to create delivery attempt")

	// Update timestamp to be old
	targetTime := time.Now().Add(-time.Duration(hoursOld) * time.Hour)
	_, err = db.GetDB().ExecContext(ctx,
		"UPDATE convoy.delivery_attempts SET created_at=$1, updated_at=$1 WHERE id=$2",
		targetTime, attempt.UID)
	require.NoError(t, err, "failed to update delivery attempt timestamp")
}

// Configuration Creation

// createMinIOConfig updates the existing configuration with MinIO storage settings
func createMinIOConfig(t *testing.T, db database.Database, ctx context.Context, endpoint string) *datastore.Configuration {
	t.Helper()

	configRepo := postgres.NewConfigRepo(db)

	// Load existing configuration (created by test setup)
	config, err := configRepo.LoadConfiguration(ctx)
	require.NoError(t, err, "failed to load existing configuration")

	// MinIO testcontainer uses HTTP, so prepend http:// to endpoint
	if !strings.HasPrefix(endpoint, "http://") && !strings.HasPrefix(endpoint, "https://") {
		endpoint = "http://" + endpoint
	}

	// Update with MinIO storage settings
	config.StoragePolicy = &datastore.StoragePolicyConfiguration{
		Type: datastore.S3,
		S3: &datastore.S3Storage{
			Prefix:       null.NewString("", false),
			Bucket:       null.NewString("convoy-test-exports", true),
			AccessKey:    null.NewString("minioadmin", true),
			SecretKey:    null.NewString("minioadmin", true),
			Region:       null.NewString("us-east-1", true),
			SessionToken: null.NewString("", false),
			Endpoint:     null.NewString(endpoint, true),
		},
	}
	config.RetentionPolicy = &datastore.RetentionPolicyConfiguration{
		IsRetentionPolicyEnabled: true,
		Policy:                   "720h",
	}

	err = configRepo.UpdateConfiguration(ctx, config)
	require.NoError(t, err, "failed to update MinIO configuration")

	return config
}

// createOnPremConfig updates the existing configuration with OnPrem storage settings
func createOnPremConfig(t *testing.T, db database.Database, ctx context.Context, exportPath string) *datastore.Configuration {
	t.Helper()

	configRepo := postgres.NewConfigRepo(db)

	// Load existing configuration (created by test setup)
	config, err := configRepo.LoadConfiguration(ctx)
	require.NoError(t, err, "failed to load existing configuration")

	// Update with OnPrem storage settings
	config.StoragePolicy = &datastore.StoragePolicyConfiguration{
		Type: datastore.OnPrem,
		S3: &datastore.S3Storage{
			Prefix:       null.NewString("", false),
			Bucket:       null.NewString("", false),
			AccessKey:    null.NewString("", false),
			SecretKey:    null.NewString("", false),
			Region:       null.NewString("", false),
			SessionToken: null.NewString("", false),
			Endpoint:     null.NewString("", false),
		},
		OnPrem: &datastore.OnPremStorage{
			Path: null.NewString(exportPath, true),
		},
	}
	config.RetentionPolicy = &datastore.RetentionPolicyConfiguration{
		IsRetentionPolicyEnabled: true,
		Policy:                   "720h",
	}

	err = configRepo.UpdateConfiguration(ctx, config)
	require.NoError(t, err, "failed to update OnPrem configuration")

	return config
}

// Verification Functions

// verifyTimeFiltering verifies that all records in the data are older than the specified cutoff hours
func verifyTimeFiltering(t *testing.T, data []byte) {
	t.Helper()

	cutoffTime := time.Now().Add(-time.Duration(24) * time.Hour)

	// Try to unmarshal as a slice of maps to handle generic JSON
	var records []map[string]interface{}
	err := json.Unmarshal(data, &records)
	require.NoError(t, err, "failed to unmarshal records for time filtering verification")

	for i, record := range records {
		// Check for the created_at field
		createdAtStr, ok := record["created_at"].(string)
		require.True(t, ok, "record %d missing or invalid created_at field", i)

		createdAt, err := time.Parse(time.RFC3339, createdAtStr)
		require.NoError(t, err, "failed to parse created_at for record %d", i)

		require.True(t, createdAt.Before(cutoffTime),
			"record %d created_at (%v) should be before cutoff (%v)",
			i, createdAt, cutoffTime)
	}
}

// verifyProjectIsolation verifies that all records belong to the specified project
func verifyProjectIsolation(t *testing.T, data []byte, projectID string) {
	t.Helper()

	// Try to unmarshal as a slice of maps to handle generic JSON
	var records []map[string]interface{}
	err := json.Unmarshal(data, &records)
	require.NoError(t, err, "failed to unmarshal records for project isolation verification")

	for i, record := range records {
		// Check for project_id field
		recordProjectID, ok := record["project_id"].(string)
		require.True(t, ok, "record %d missing or invalid project_id field", i)

		require.Equal(t, projectID, recordProjectID,
			"record %d project_id should match expected project ID", i)
	}
}

// verifyJSONStructure verifies that the data is valid JSON and has the expected structure
func verifyJSONStructure(t *testing.T, data []byte, expectedCount int) {
	t.Helper()

	var records []map[string]interface{}
	err := json.Unmarshal(data, &records)
	require.NoError(t, err, "failed to unmarshal JSON")

	if expectedCount >= 0 {
		require.Len(t, records, expectedCount, "unexpected number of records")
	}

	// Verify each record has required fields
	for i, record := range records {
		require.Contains(t, record, "uid", "record %d missing uid field", i)
		require.Contains(t, record, "created_at", "record %d missing created_at field", i)
		require.Contains(t, record, "project_id", "record %d missing project_id field", i)
	}
}

// getExportPath constructs the expected export path for a table
func getExportPath(baseDir, orgID, projectID, tableName string) string {
	return filepath.Join(baseDir, "orgs", orgID, "projects", projectID, tableName)
}

// getMinIOPrefix constructs the MinIO prefix for listing objects
func getMinIOPrefix(orgID, projectID string) string {
	return fmt.Sprintf("convoy/export/orgs/%s/projects/%s/", orgID, projectID)
}
