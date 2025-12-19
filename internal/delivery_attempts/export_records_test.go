package delivery_attempts

import (
	"bytes"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/require"

	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/pkg/log"
)

func TestExportRecords_EmptyResult(t *testing.T) {
	db, ctx := setupTestDB(t)
	defer db.Close()

	project := seedTestData(t, db, ctx)
	service := New(log.NewLogger(os.Stdout), db)

	// Create a buffer to write exported data
	var buf bytes.Buffer

	// Export with a future date (should return empty)
	futureDate := time.Now().Add(24 * time.Hour)
	count, err := service.ExportRecords(ctx, project.UID, futureDate, &buf)

	require.NoError(t, err)
	require.Equal(t, int64(0), count)
	require.Empty(t, buf.String())
}

func TestExportRecords_WithData(t *testing.T) {
	db, ctx := setupTestDB(t)
	defer db.Close()

	project := seedTestData(t, db, ctx)
	endpoint := seedEndpoint(t, db, ctx, project)
	eventDelivery := seedEventDelivery(t, db, ctx, project, endpoint)
	service := New(log.NewLogger(os.Stdout), db)

	// Create 5 delivery attempts
	attemptIDs := make([]string, 5)
	for i := 0; i < 5; i++ {
		attempt := &datastore.DeliveryAttempt{
			UID:             ulid.Make().String(),
			URL:             "https://example.com/webhook",
			Method:          "POST",
			APIVersion:      "2023.12.25",
			EndpointID:      endpoint.UID,
			EventDeliveryId: eventDelivery.UID,
			ProjectId:       project.UID,
			Status:          i%2 == 0, // Alternating success/failure
		}
		err := service.CreateDeliveryAttempt(ctx, attempt)
		require.NoError(t, err)
		attemptIDs[i] = attempt.UID
	}

	// Export all attempts
	var buf bytes.Buffer
	futureDate := time.Now().Add(24 * time.Hour)
	count, err := service.ExportRecords(ctx, project.UID, futureDate, &buf)

	require.NoError(t, err)
	require.Equal(t, int64(5), count)

	// Verify JSON structure
	var exported []map[string]interface{}
	err = json.Unmarshal(buf.Bytes(), &exported)
	require.NoError(t, err)
	require.Len(t, exported, 5)

	// Verify all UIDs are present
	exportedUIDs := make(map[string]bool)
	for _, record := range exported {
		uid, ok := record["uid"].(string)
		require.True(t, ok, "uid should be present in exported record")
		exportedUIDs[uid] = true
	}

	for _, expectedUID := range attemptIDs {
		require.True(t, exportedUIDs[expectedUID], "Expected UID %s should be in exported data", expectedUID)
	}
}

func TestExportRecords_ProjectIsolation(t *testing.T) {
	db, ctx := setupTestDB(t)
	defer db.Close()

	service := New(log.NewLogger(os.Stdout), db)

	// Create two projects with delivery attempts
	project1 := seedTestData(t, db, ctx)
	endpoint1 := seedEndpoint(t, db, ctx, project1)
	eventDelivery1 := seedEventDelivery(t, db, ctx, project1, endpoint1)

	project2 := seedTestData(t, db, ctx)
	endpoint2 := seedEndpoint(t, db, ctx, project2)
	eventDelivery2 := seedEventDelivery(t, db, ctx, project2, endpoint2)

	// Create 3 attempts for project1
	for i := 0; i < 3; i++ {
		attempt := &datastore.DeliveryAttempt{
			UID:             ulid.Make().String(),
			URL:             "https://example.com/webhook",
			Method:          "POST",
			APIVersion:      "2023.12.25",
			EndpointID:      endpoint1.UID,
			EventDeliveryId: eventDelivery1.UID,
			ProjectId:       project1.UID,
			Status:          true,
		}
		err := service.CreateDeliveryAttempt(ctx, attempt)
		require.NoError(t, err)
	}

	// Create 2 attempts for project2
	for i := 0; i < 2; i++ {
		attempt := &datastore.DeliveryAttempt{
			UID:             ulid.Make().String(),
			URL:             "https://example.com/webhook",
			Method:          "POST",
			APIVersion:      "2023.12.25",
			EndpointID:      endpoint2.UID,
			EventDeliveryId: eventDelivery2.UID,
			ProjectId:       project2.UID,
			Status:          true,
		}
		err := service.CreateDeliveryAttempt(ctx, attempt)
		require.NoError(t, err)
	}

	// Export project1 only
	var buf bytes.Buffer
	futureDate := time.Now().Add(24 * time.Hour)
	count, err := service.ExportRecords(ctx, project1.UID, futureDate, &buf)

	require.NoError(t, err)
	require.Equal(t, int64(3), count, "Should only export project1's attempts")

	// Verify no project2 data is included
	var exported []map[string]interface{}
	err = json.Unmarshal(buf.Bytes(), &exported)
	require.NoError(t, err)
	require.Len(t, exported, 3)

	for _, record := range exported {
		projectID, ok := record["project_id"].(string)
		require.True(t, ok)
		require.Equal(t, project1.UID, projectID, "All records should belong to project1")
	}
}

func TestExportRecords_TimeFiltering(t *testing.T) {
	db, ctx := setupTestDB(t)
	defer db.Close()

	project := seedTestData(t, db, ctx)
	endpoint := seedEndpoint(t, db, ctx, project)
	eventDelivery := seedEventDelivery(t, db, ctx, project, endpoint)
	service := New(log.NewLogger(os.Stdout), db)

	// Create 3 attempts
	for i := 0; i < 3; i++ {
		attempt := &datastore.DeliveryAttempt{
			UID:             ulid.Make().String(),
			URL:             "https://example.com/webhook",
			Method:          "POST",
			APIVersion:      "2023.12.25",
			EndpointID:      endpoint.UID,
			EventDeliveryId: eventDelivery.UID,
			ProjectId:       project.UID,
			Status:          true,
		}
		err := service.CreateDeliveryAttempt(ctx, attempt)
		require.NoError(t, err)
		time.Sleep(10 * time.Millisecond) // Small delay to ensure different timestamps
	}

	// Export with past date (should return 0)
	var buf bytes.Buffer
	pastDate := time.Now().Add(-1 * time.Hour)
	count, err := service.ExportRecords(ctx, project.UID, pastDate, &buf)

	require.NoError(t, err)
	require.Equal(t, int64(0), count)
	require.Empty(t, buf.String())

	// Export with future date (should return all 3)
	buf.Reset()
	futureDate := time.Now().Add(24 * time.Hour)
	count, err = service.ExportRecords(ctx, project.UID, futureDate, &buf)

	require.NoError(t, err)
	require.Equal(t, int64(3), count)

	var exported []map[string]interface{}
	err = json.Unmarshal(buf.Bytes(), &exported)
	require.NoError(t, err)
	require.Len(t, exported, 3)
}
