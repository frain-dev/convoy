package delivery_attempts

import (
	"bufio"
	"bytes"
	"encoding/json"
	"testing"
	"time"

	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/require"

	"github.com/frain-dev/convoy/datastore"
	log "github.com/frain-dev/convoy/pkg/logger"
)

// parseJSONL parses JSONL (newline-delimited JSON) into a slice of maps.
func parseJSONL(t *testing.T, data []byte) []map[string]interface{} {
	t.Helper()
	var results []map[string]interface{}
	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var record map[string]interface{}
		err := json.Unmarshal(line, &record)
		require.NoError(t, err, "each JSONL line must be valid JSON")
		results = append(results, record)
	}
	require.NoError(t, scanner.Err())
	return results
}

func TestExportRecords_EmptyResult(t *testing.T) {
	db, ctx := setupTestDB(t)
	defer db.Close()

	_ = seedTestData(t, db, ctx)
	service := New(log.New("convoy", log.LevelInfo), db)

	// Create a buffer to write exported data
	var buf bytes.Buffer

	// Export with a future date as end (should return empty since no data seeded)
	futureDate := time.Now().Add(24 * time.Hour)
	count, err := service.ExportRecords(ctx, time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC), futureDate, &buf)

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
	service := New(log.New("convoy", log.LevelInfo), db)

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
	count, err := service.ExportRecords(ctx, time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC), futureDate, &buf)

	require.NoError(t, err)
	require.Equal(t, int64(5), count)

	// Verify JSONL structure
	exported := parseJSONL(t, buf.Bytes())
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

func TestExportRecords_TimeFiltering(t *testing.T) {
	db, ctx := setupTestDB(t)
	defer db.Close()

	project := seedTestData(t, db, ctx)
	endpoint := seedEndpoint(t, db, ctx, project)
	eventDelivery := seedEventDelivery(t, db, ctx, project, endpoint)
	service := New(log.New("convoy", log.LevelInfo), db)

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

	// Export with past date as end (should return 0)
	var buf bytes.Buffer
	pastDate := time.Now().Add(-1 * time.Hour)
	count, err := service.ExportRecords(ctx, time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC), pastDate, &buf)

	require.NoError(t, err)
	require.Equal(t, int64(0), count)
	require.Empty(t, buf.String())

	// Export with future date as end (should return all 3)
	buf.Reset()
	futureDate := time.Now().Add(24 * time.Hour)
	count, err = service.ExportRecords(ctx, time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC), futureDate, &buf)

	require.NoError(t, err)
	require.Equal(t, int64(3), count)

	exported := parseJSONL(t, buf.Bytes())
	require.Len(t, exported, 3)
}

func TestExportRecords_TimeWindow(t *testing.T) {
	db, ctx := setupTestDB(t)
	defer db.Close()

	project := seedTestData(t, db, ctx)
	endpoint := seedEndpoint(t, db, ctx, project)
	eventDelivery := seedEventDelivery(t, db, ctx, project, endpoint)
	service := New(log.New("convoy", log.LevelInfo), db)

	// Create 5 attempts
	for i := 0; i < 5; i++ {
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
	}

	// Export with window [1h ago, now+1h) — should include all
	var buf bytes.Buffer
	start := time.Now().Add(-1 * time.Hour)
	end := time.Now().Add(1 * time.Hour)
	count, err := service.ExportRecords(ctx, start, end, &buf)
	require.NoError(t, err)
	require.Equal(t, int64(5), count)

	exported := parseJSONL(t, buf.Bytes())
	require.Len(t, exported, 5)

	// Each record should have a uid field
	for _, record := range exported {
		_, ok := record["uid"].(string)
		require.True(t, ok, "each record should have uid")
	}

	// Export with window that excludes all: [2h ago, 1h ago)
	buf.Reset()
	count, err = service.ExportRecords(ctx, time.Now().Add(-2*time.Hour), time.Now().Add(-1*time.Hour), &buf)
	require.NoError(t, err)
	require.Equal(t, int64(0), count)
	require.Empty(t, buf.String())
}
