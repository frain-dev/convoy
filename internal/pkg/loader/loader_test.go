package loader

import (
	"context"
	"os"
	"testing"

	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/require"

	"github.com/frain-dev/convoy/api/testdb"
	"github.com/frain-dev/convoy/database"
	"github.com/frain-dev/convoy/database/postgres"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/internal/pkg/memorystore"
	"github.com/frain-dev/convoy/internal/projects"
	"github.com/frain-dev/convoy/pkg/log"
)

// TestData holds common test data for integration tests
type TestData struct {
	DB            database.Database
	Project       *datastore.Project
	Project2      *datastore.Project
	Endpoint      *datastore.Endpoint
	Endpoint2     *datastore.Endpoint
	Source        *datastore.Source
	Source2       *datastore.Source
	Subscription1 *datastore.Subscription
	Subscription2 *datastore.Subscription
	Subscription3 *datastore.Subscription
	Subscription4 *datastore.Subscription
	Subscription5 *datastore.Subscription
}

// setupTestData creates common test data for integration tests
func setupTestData(t *testing.T) *TestData {
	_, l := newLoader(t)
	db := l.Database

	// Create user and organization
	user, err := testdb.SeedDefaultUser(db)
	require.NoError(t, err)

	org, err := testdb.SeedDefaultOrganisation(db, user)
	require.NoError(t, err)

	// Create projects
	project, err := testdb.SeedDefaultProject(db, org.UID)
	require.NoError(t, err)

	project2, err := testdb.SeedProject(db, ulid.Make().String(), "test-project-2", org.UID, datastore.OutgoingProject, &datastore.DefaultProjectConfig)
	require.NoError(t, err)

	// Create endpoints
	endpoint, err := testdb.SeedEndpoint(db, project, ulid.Make().String(), "test-endpoint-1", user.UID, false, datastore.ActiveEndpointStatus)
	require.NoError(t, err)

	endpoint2, err := testdb.SeedEndpoint(db, project2, ulid.Make().String(), "test-endpoint-2", user.UID, false, datastore.ActiveEndpointStatus)
	require.NoError(t, err)

	// Create sources
	source, err := testdb.SeedSource(db, project, ulid.Make().String(), ulid.Make().String(), "test-source-1", nil, "", "")
	require.NoError(t, err)

	source2, err := testdb.SeedSource(db, project2, ulid.Make().String(), ulid.Make().String(), "test-source-2", nil, "", "")
	require.NoError(t, err)

	// Create subscriptions with different event types
	subscription1, err := testdb.SeedSubscription(db, project, ulid.Make().String(), datastore.OutgoingProject, source, endpoint, nil, nil, &datastore.FilterConfiguration{
		EventTypes: []string{"user.created"},
		Filter: datastore.FilterSchema{
			IsFlattened: false,
			Headers:     datastore.M{},
			Body:        datastore.M{},
			RawHeaders:  datastore.M{},
			RawBody:     datastore.M{},
		},
	})
	require.NoError(t, err)

	subscription2, err := testdb.SeedSubscription(db, project, ulid.Make().String(), datastore.OutgoingProject, source, endpoint, nil, nil, &datastore.FilterConfiguration{
		EventTypes: []string{"user.updated"},
		Filter: datastore.FilterSchema{
			IsFlattened: false,
			Headers:     datastore.M{},
			Body:        datastore.M{},
			RawHeaders:  datastore.M{},
			RawBody:     datastore.M{},
		},
	})
	require.NoError(t, err)

	subscription3, err := testdb.SeedSubscription(db, project, ulid.Make().String(), datastore.OutgoingProject, source, endpoint, nil, nil, &datastore.FilterConfiguration{
		EventTypes: []string{"user.created", "user.updated"},
		Filter: datastore.FilterSchema{
			IsFlattened: false,
			Headers:     datastore.M{},
			Body:        datastore.M{},
			RawHeaders:  datastore.M{},
			RawBody:     datastore.M{},
		},
	})
	require.NoError(t, err)

	subscription4, err := testdb.SeedSubscription(db, project2, ulid.Make().String(), datastore.OutgoingProject, source2, endpoint2, nil, nil, &datastore.FilterConfiguration{
		EventTypes: []string{"order.created"},
		Filter: datastore.FilterSchema{
			IsFlattened: false,
			Headers:     datastore.M{},
			Body:        datastore.M{},
			RawHeaders:  datastore.M{},
			RawBody:     datastore.M{},
		},
	})
	require.NoError(t, err)

	subscription5, err := testdb.SeedSubscription(db, project, ulid.Make().String(), datastore.OutgoingProject, source, endpoint, nil, nil, &datastore.FilterConfiguration{
		EventTypes: []string{"user.deleted"},
		Filter: datastore.FilterSchema{
			IsFlattened: false,
			Headers:     datastore.M{},
			Body:        datastore.M{},
			RawHeaders:  datastore.M{},
			RawBody:     datastore.M{},
		},
	})
	require.NoError(t, err)

	return &TestData{
		DB:            db,
		Project:       project,
		Project2:      project2,
		Endpoint:      endpoint,
		Endpoint2:     endpoint2,
		Source:        source,
		Source2:       source2,
		Subscription1: subscription1,
		Subscription2: subscription2,
		Subscription3: subscription3,
		Subscription4: subscription4,
		Subscription5: subscription5,
	}
}

// verifySubscriptionInTable verifies that a subscription exists in the table for its event types
func verifySubscriptionInTable(t *testing.T, table *memorystore.Table, subscription *datastore.Subscription, shouldExist bool) {
	if subscription.FilterConfig == nil || len(subscription.FilterConfig.EventTypes) == 0 || (len(subscription.FilterConfig.EventTypes) == 1 && subscription.FilterConfig.EventTypes[0] == "*") {
		return
	}

	for _, eventType := range subscription.FilterConfig.EventTypes {
		key := memorystore.NewKey(subscription.ProjectID, eventType)
		row := table.Get(key)

		require.NotNil(t, row, "Row should exist for event type: %s", eventType)

		values, ok := row.Value().([]datastore.Subscription)
		require.True(t, ok, "Row value should be []datastore.Subscription for event type: %s", eventType)

		found := false
		for _, sub := range values {
			if sub.UID == subscription.UID {
				found = true
				break
			}
		}

		if shouldExist {
			require.True(t, found, "Subscription %s should be found in table for event type: %s", subscription.UID, eventType)
		} else {
			require.False(t, found, "Subscription %s should not be found in table for event type: %s", subscription.UID, eventType)
		}
	}
}

// countSubscriptionsInTable counts the total number of subscriptions in the table
func countSubscriptionsInTable(t *testing.T, table *memorystore.Table) int {
	seenSubscriptions := make(map[string]struct{})
	total := 0
	for _, key := range table.GetKeys() {
		row := table.Get(key)
		if row != nil {
			values, ok := row.Value().([]datastore.Subscription)
			if ok {
				for _, sub := range values {
					if _, exists := seenSubscriptions[sub.UID]; !exists {
						seenSubscriptions[sub.UID] = struct{}{}
						total++
					}
				}
			}
		}
	}
	return total
}

func TestSubscriptionLoaderIntegration(t *testing.T) {
	t.Run("TestInitialLoad", func(t *testing.T) {
		t.Run("TestLoadEmptyDatabase", func(t *testing.T) {
			ctx, l := newLoader(t)
			table := memorystore.NewTable()

			subRepo := postgres.NewSubscriptionRepo(l.Database)
			projectRepo := projects.New(log.NewLogger(os.Stdout), l.Database)

			loader := NewSubscriptionLoader(subRepo, projectRepo, l.Logger, 1000)
			err := loader.SyncChanges(ctx, table)
			require.NoError(t, err)

			// Should have no keys in empty database
			require.Equal(t, 0, len(table.GetKeys()))
			require.Equal(t, 0, countSubscriptionsInTable(t, table))
		})

		t.Run("TestLoadSingleProject", func(t *testing.T) {
			testData := setupTestData(t)
			defer testdb.PurgeDB(t, testData.DB)

			table := memorystore.NewTable()
			ctx := context.Background()
			logger := log.NewLogger(os.Stdout)

			subRepo := postgres.NewSubscriptionRepo(testData.DB)
			projectRepo := projects.New(log.NewLogger(os.Stdout), testData.DB)

			loader := NewSubscriptionLoader(subRepo, projectRepo, logger, 1000)
			err := loader.SyncChanges(ctx, table)
			require.NoError(t, err)

			// Should have 4 unique event types: user.created, user.updated, user.deleted, order.created
			require.Equal(t, 4, len(table.GetKeys()))

			// Should have 5 total subscriptions
			require.Equal(t, 5, countSubscriptionsInTable(t, table))

			// Verify each subscription is in the table
			verifySubscriptionInTable(t, table, testData.Subscription1, true)
			verifySubscriptionInTable(t, table, testData.Subscription2, true)
			verifySubscriptionInTable(t, table, testData.Subscription3, true)
			verifySubscriptionInTable(t, table, testData.Subscription4, true)
			verifySubscriptionInTable(t, table, testData.Subscription5, true)
		})

		t.Run("TestLoadWithVariousEventTypes", func(t *testing.T) {
			testData := setupTestData(t)
			defer testdb.PurgeDB(t, testData.DB)

			table := memorystore.NewTable()
			ctx := context.Background()
			logger := log.NewLogger(os.Stdout)

			subRepo := postgres.NewSubscriptionRepo(testData.DB)
			projectRepo := projects.New(log.NewLogger(os.Stdout), testData.DB)

			loader := NewSubscriptionLoader(subRepo, projectRepo, logger, 1000)
			err := loader.SyncChanges(ctx, table)
			require.NoError(t, err)

			// Verify subscription3 is in both user.created and user.updated keys
			key1 := memorystore.NewKey(testData.Project.UID, "user.created")
			key2 := memorystore.NewKey(testData.Project.UID, "user.updated")

			row1 := table.Get(key1)
			require.NotNil(t, row1)
			values1, ok := row1.Value().([]datastore.Subscription)
			require.True(t, ok)

			row2 := table.Get(key2)
			require.NotNil(t, row2)
			values2, ok := row2.Value().([]datastore.Subscription)
			require.True(t, ok)

			// subscription3 should be in both arrays
			found1 := false
			found2 := false
			for _, sub := range values1 {
				if sub.UID == testData.Subscription3.UID {
					found1 = true
					break
				}
			}
			for _, sub := range values2 {
				if sub.UID == testData.Subscription3.UID {
					found2 = true
					break
				}
			}
			require.True(t, found1, "Subscription3 should be in user.created key")
			require.True(t, found2, "Subscription3 should be in user.updated key")
		})
	})

	t.Run("TestIncrementalUpdates", func(t *testing.T) {
		t.Run("TestAddNewSubscriptions", func(t *testing.T) {
			testData := setupTestData(t)
			defer testdb.PurgeDB(t, testData.DB)

			table := memorystore.NewTable()
			ctx := context.Background()
			logger := log.NewLogger(os.Stdout)

			subRepo := postgres.NewSubscriptionRepo(testData.DB)
			projectRepo := projects.New(log.NewLogger(os.Stdout), testData.DB)

			loader := NewSubscriptionLoader(subRepo, projectRepo, logger, 1000)

			// Initial load
			err := loader.SyncChanges(ctx, table)
			require.NoError(t, err)
			initialCount := countSubscriptionsInTable(t, table)

			// Add a new subscription
			newSubscription, err := testdb.SeedSubscription(testData.DB, testData.Project, ulid.Make().String(), datastore.OutgoingProject, testData.Source, testData.Endpoint, nil, nil, &datastore.FilterConfiguration{
				EventTypes: []string{"user.registered"},
				Filter: datastore.FilterSchema{
					IsFlattened: false,
					Headers:     datastore.M{},
					Body:        datastore.M{},
					RawHeaders:  datastore.M{},
					RawBody:     datastore.M{},
				},
			})
			require.NoError(t, err)

			// Sync changes
			err = loader.SyncChanges(ctx, table)
			require.NoError(t, err)

			// Should have one more subscription
			newCount := countSubscriptionsInTable(t, table)
			require.Equal(t, initialCount+1, newCount)

			// Verify new subscription is in table
			verifySubscriptionInTable(t, table, newSubscription, true)
		})

		t.Run("TestUpdateExistingSubscriptions", func(t *testing.T) {
			testData := setupTestData(t)
			defer testdb.PurgeDB(t, testData.DB)

			table := memorystore.NewTable()
			ctx := context.Background()
			logger := log.NewLogger(os.Stdout)

			subRepo := postgres.NewSubscriptionRepo(testData.DB)
			projectRepo := projects.New(log.NewLogger(os.Stdout), testData.DB)

			loader := NewSubscriptionLoader(subRepo, projectRepo, logger, 1000)

			// Initial load
			err := loader.SyncChanges(ctx, table)
			require.NoError(t, err)

			// Update subscription1 to have different event types
			testData.Subscription1.FilterConfig.EventTypes = []string{"user.modified", "user.activated"}
			err = subRepo.UpdateSubscription(ctx, testData.Project.UID, testData.Subscription1)
			require.NoError(t, err)

			// Sync changes
			err = loader.SyncChanges(ctx, table)
			require.NoError(t, err)

			// Verify subscription1 is no longer in user.created
			key := memorystore.NewKey(testData.Project.UID, "user.created")
			row := table.Get(key)
			if row != nil {
				values, ok := row.Value().([]datastore.Subscription)
				if ok {
					for _, sub := range values {
						require.NotEqual(t, testData.Subscription1.UID, sub.UID, "Subscription1 should not be in user.created anymore")
					}
				}
			}

			// Verify subscription1 is now in user.modified and user.activated
			verifySubscriptionInTable(t, table, testData.Subscription1, true)
		})
	})

	t.Run("TestDeletions", func(t *testing.T) {
		t.Run("TestDeleteSubscriptions", func(t *testing.T) {
			testData := setupTestData(t)
			defer testdb.PurgeDB(t, testData.DB)

			table := memorystore.NewTable()
			ctx := context.Background()
			logger := log.NewLogger(os.Stdout)

			subRepo := postgres.NewSubscriptionRepo(testData.DB)
			projectRepo := projects.New(log.NewLogger(os.Stdout), testData.DB)

			loader := NewSubscriptionLoader(subRepo, projectRepo, logger, 1000)

			// Initial load
			err := loader.SyncChanges(ctx, table)
			require.NoError(t, err)
			initialCount := countSubscriptionsInTable(t, table)

			// Delete subscription1
			err = subRepo.DeleteSubscription(ctx, testData.Project.UID, testData.Subscription1)
			require.NoError(t, err)

			// Sync changes
			err = loader.SyncChanges(ctx, table)
			require.NoError(t, err)

			// Should have one less subscription
			newCount := countSubscriptionsInTable(t, table)
			require.Equal(t, initialCount-1, newCount)

			// Verify subscription1 is no longer in table
			verifySubscriptionInTable(t, table, testData.Subscription1, false)
		})
	})

	t.Run("TestEdgeCases", func(t *testing.T) {
		t.Run("TestInvalidSubscriptions", func(t *testing.T) {
			testData := setupTestData(t)
			defer testdb.PurgeDB(t, testData.DB)

			// Create subscription with no filter config
			invalidSub, err := testdb.SeedSubscription(testData.DB, testData.Project, ulid.Make().String(), datastore.OutgoingProject, testData.Source, testData.Endpoint, nil, nil, nil)
			require.NoError(t, err)

			table := memorystore.NewTable()
			ctx := context.Background()
			logger := log.NewLogger(os.Stdout)

			subRepo := postgres.NewSubscriptionRepo(testData.DB)
			projectRepo := projects.New(log.NewLogger(os.Stdout), testData.DB)

			loader := NewSubscriptionLoader(subRepo, projectRepo, logger, 1000)
			err = loader.SyncChanges(ctx, table)
			require.NoError(t, err)

			// Invalid subscription should not be in table
			verifySubscriptionInTable(t, table, invalidSub, false)
		})

		t.Run("TestLargeBatchSizes", func(t *testing.T) {
			testData := setupTestData(t)
			defer testdb.PurgeDB(t, testData.DB)

			table := memorystore.NewTable()
			ctx := context.Background()
			logger := log.NewLogger(os.Stdout)

			subRepo := postgres.NewSubscriptionRepo(testData.DB)
			projectRepo := projects.New(log.NewLogger(os.Stdout), testData.DB)

			// Test with very small batch size
			loader := NewSubscriptionLoader(subRepo, projectRepo, logger, 1)
			err := loader.SyncChanges(ctx, table)
			require.NoError(t, err)

			// Should still load all subscriptions
			require.Equal(t, 5, countSubscriptionsInTable(t, table))
		})
	})
}
