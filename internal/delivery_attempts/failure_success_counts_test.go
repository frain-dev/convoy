package delivery_attempts

import (
	"os"
	"testing"
	"time"

	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/require"

	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/pkg/log"
)

func TestGetFailureAndSuccessCounts_BasicLookback(t *testing.T) {
	db, ctx := setupTestDB(t)
	defer db.Close()

	project := seedTestData(t, db, ctx)
	endpoint := seedEndpoint(t, db, ctx, project)
	eventDelivery := seedEventDelivery(t, db, ctx, project, endpoint)

	service := New(log.NewLogger(os.Stdout), db)

	// Create multiple delivery attempts with different statuses
	// 3 successful, 2 failed
	for i := 0; i < 5; i++ {
		attempt := &datastore.DeliveryAttempt{
			UID:             ulid.Make().String(),
			URL:             "https://example.com/webhook",
			Method:          "POST",
			APIVersion:      "2023.12.25",
			EndpointID:      endpoint.UID,
			EventDeliveryId: eventDelivery.UID,
			ProjectId:       project.UID,
			Status:          i < 3, // First 3 are successful
		}
		err := service.CreateDeliveryAttempt(ctx, attempt)
		require.NoError(t, err)
	}

	// Get counts with 60 minute lookback
	results, err := service.GetFailureAndSuccessCounts(ctx, 60, nil)
	require.NoError(t, err)
	require.NotEmpty(t, results)

	// Verify counts for our endpoint
	result, exists := results[endpoint.UID]
	require.True(t, exists)
	require.Equal(t, uint64(3), result.Successes)
	require.Equal(t, uint64(2), result.Failures)
	require.Equal(t, endpoint.UID, result.Key)
	require.Equal(t, project.UID, result.TenantId)
}

func TestGetFailureAndSuccessCounts_WithResetTimes(t *testing.T) {
	db, ctx := setupTestDB(t)
	defer db.Close()

	project := seedTestData(t, db, ctx)
	endpoint1 := seedEndpoint(t, db, ctx, project)
	endpoint2 := seedEndpoint(t, db, ctx, project)
	eventDelivery := seedEventDelivery(t, db, ctx, project, endpoint1)

	service := New(log.NewLogger(os.Stdout), db)

	// Create old attempts for endpoint1 (before reset time)
	for i := 0; i < 3; i++ {
		attempt := &datastore.DeliveryAttempt{
			UID:             ulid.Make().String(),
			URL:             "https://example.com/webhook",
			Method:          "POST",
			APIVersion:      "2023.12.25",
			EndpointID:      endpoint1.UID,
			EventDeliveryId: eventDelivery.UID,
			ProjectId:       project.UID,
			Status:          false, // Old failures
		}
		err := service.CreateDeliveryAttempt(ctx, attempt)
		require.NoError(t, err)
	}

	// Set reset time to now
	resetTime := time.Now()

	// Create new attempts for endpoint1 (after reset time)
	// Wait to ensure created_at is definitely after reset time
	time.Sleep(100 * time.Millisecond)

	for i := 0; i < 2; i++ {
		attempt := &datastore.DeliveryAttempt{
			UID:             ulid.Make().String(),
			URL:             "https://example.com/webhook",
			Method:          "POST",
			APIVersion:      "2023.12.25",
			EndpointID:      endpoint1.UID,
			EventDeliveryId: eventDelivery.UID,
			ProjectId:       project.UID,
			Status:          true, // New successes
		}
		err := service.CreateDeliveryAttempt(ctx, attempt)
		require.NoError(t, err)
	}

	// Create attempts for endpoint2
	eventDelivery2 := seedEventDelivery(t, db, ctx, project, endpoint2)
	for i := 0; i < 4; i++ {
		attempt := &datastore.DeliveryAttempt{
			UID:             ulid.Make().String(),
			URL:             "https://example.com/webhook",
			Method:          "POST",
			APIVersion:      "2023.12.25",
			EndpointID:      endpoint2.UID,
			EventDeliveryId: eventDelivery2.UID,
			ProjectId:       project.UID,
			Status:          i%2 == 0, // 2 successes, 2 failures
		}
		err := service.CreateDeliveryAttempt(ctx, attempt)
		require.NoError(t, err)
	}

	// Get counts with reset time for endpoint1 only
	resetTimes := map[string]time.Time{
		endpoint1.UID: resetTime,
	}

	results, err := service.GetFailureAndSuccessCounts(ctx, 60, resetTimes)
	require.NoError(t, err)
	require.NotEmpty(t, results)

	// Verify endpoint1 only counts attempts after reset time
	result1, exists1 := results[endpoint1.UID]
	require.True(t, exists1)
	require.Equal(t, uint64(2), result1.Successes, "Should only count successes after reset")
	require.Equal(t, uint64(0), result1.Failures, "Should not count old failures")

	// Verify endpoint2 counts all attempts (no reset time)
	result2, exists2 := results[endpoint2.UID]
	require.True(t, exists2)
	require.Equal(t, uint64(2), result2.Successes)
	require.Equal(t, uint64(2), result2.Failures)
}

func TestGetFailureAndSuccessCounts_NoAttempts(t *testing.T) {
	db, ctx := setupTestDB(t)
	defer db.Close()

	seedTestData(t, db, ctx)

	service := New(log.NewLogger(os.Stdout), db)

	// Get counts when there are no delivery attempts
	results, err := service.GetFailureAndSuccessCounts(ctx, 60, nil)
	require.NoError(t, err)
	require.Empty(t, results, "Should return empty map when no attempts exist")
}

func TestGetFailureAndSuccessCounts_MultipleEndpoints(t *testing.T) {
	db, ctx := setupTestDB(t)
	defer db.Close()

	project := seedTestData(t, db, ctx)
	service := New(log.NewLogger(os.Stdout), db)

	// Create 3 endpoints with different success/failure patterns
	endpoints := make([]*datastore.Endpoint, 3)
	expectedSuccesses := []uint64{5, 3, 1}
	expectedFailures := []uint64{0, 2, 4}

	for i := 0; i < 3; i++ {
		endpoints[i] = seedEndpoint(t, db, ctx, project)
		eventDelivery := seedEventDelivery(t, db, ctx, project, endpoints[i])

		// Create successes
		for j := uint64(0); j < expectedSuccesses[i]; j++ {
			attempt := &datastore.DeliveryAttempt{
				UID:             ulid.Make().String(),
				URL:             "https://example.com/webhook",
				Method:          "POST",
				APIVersion:      "2023.12.25",
				EndpointID:      endpoints[i].UID,
				EventDeliveryId: eventDelivery.UID,
				ProjectId:       project.UID,
				Status:          true,
			}
			err := service.CreateDeliveryAttempt(ctx, attempt)
			require.NoError(t, err)
		}

		// Create failures
		for j := uint64(0); j < expectedFailures[i]; j++ {
			attempt := &datastore.DeliveryAttempt{
				UID:             ulid.Make().String(),
				URL:             "https://example.com/webhook",
				Method:          "POST",
				APIVersion:      "2023.12.25",
				EndpointID:      endpoints[i].UID,
				EventDeliveryId: eventDelivery.UID,
				ProjectId:       project.UID,
				Status:          false,
			}
			err := service.CreateDeliveryAttempt(ctx, attempt)
			require.NoError(t, err)
		}
	}

	// Get counts for all endpoints
	results, err := service.GetFailureAndSuccessCounts(ctx, 60, nil)
	require.NoError(t, err)
	require.Len(t, results, 3)

	// Verify each endpoint's counts
	for i, endpoint := range endpoints {
		result, exists := results[endpoint.UID]
		require.True(t, exists, "Endpoint %d should exist in results", i)
		require.Equal(t, expectedSuccesses[i], result.Successes, "Endpoint %d successes mismatch", i)
		require.Equal(t, expectedFailures[i], result.Failures, "Endpoint %d failures mismatch", i)
	}
}

func TestGetFailureAndSuccessCounts_ShortLookback(t *testing.T) {
	db, ctx := setupTestDB(t)
	defer db.Close()

	project := seedTestData(t, db, ctx)
	endpoint := seedEndpoint(t, db, ctx, project)
	eventDelivery := seedEventDelivery(t, db, ctx, project, endpoint)

	service := New(log.NewLogger(os.Stdout), db)

	// Create an attempt
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

	// Get counts with 1 minute lookback (should include the recent attempt)
	results, err := service.GetFailureAndSuccessCounts(ctx, 1, nil)
	require.NoError(t, err)
	require.NotEmpty(t, results)

	result, exists := results[endpoint.UID]
	require.True(t, exists)
	require.Equal(t, uint64(1), result.Successes)
	require.Equal(t, uint64(0), result.Failures)
}
