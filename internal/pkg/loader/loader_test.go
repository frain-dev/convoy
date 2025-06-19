package loader

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"gopkg.in/guregu/null.v4"

	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/internal/pkg/memorystore"
	"github.com/frain-dev/convoy/mocks"
	"github.com/frain-dev/convoy/pkg/log"
	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestSyncChanges(t *testing.T) {
	t.Run("should load all valid subscriptions into the store on initial run", func(t *testing.T) {
		table := memorystore.NewTable()
		ctx := context.Background()
		projectID := ulid.Make().String()
		endpointID := "test-endpoint"
		batchSize := int64(5)

		ctrl := gomock.NewController(t)
		subRepo := mocks.NewMockSubscriptionRepository(ctrl)
		projectRepo := mocks.NewMockProjectRepository(ctrl)
		logger := log.NewLogger(os.Stdout)

		totalSubs := 5
		subscriptions := []datastore.Subscription{
			{
				UID:        ulid.Make().String(),
				Name:       "test-subscription-1",
				ProjectID:  projectID,
				EndpointID: endpointID,
				FilterConfig: &datastore.FilterConfiguration{
					EventTypes: []string{"sub-1"},
				},
			},
			{
				UID:        ulid.Make().String(),
				Name:       "test-subscription-2",
				ProjectID:  projectID,
				EndpointID: endpointID,
				FilterConfig: &datastore.FilterConfiguration{
					EventTypes: []string{"sub-2"},
				},
			},
			{
				UID:        ulid.Make().String(),
				Name:       "test-subscription-3",
				ProjectID:  projectID,
				EndpointID: endpointID,
				FilterConfig: &datastore.FilterConfiguration{
					EventTypes: []string{"sub-3"},
				},
			},
			{
				UID:        ulid.Make().String(),
				Name:       "test-subscription-4",
				ProjectID:  projectID,
				EndpointID: endpointID,
				FilterConfig: &datastore.FilterConfiguration{
					EventTypes: []string{"sub-4"},
				},
			},
			{
				UID:        ulid.Make().String(),
				Name:       "test-subscription-5",
				ProjectID:  projectID,
				EndpointID: endpointID,
				FilterConfig: &datastore.FilterConfiguration{
					EventTypes: []string{"sub-5"},
				},
			},
		}

		projects := []*datastore.Project{
			{
				UID:  projectID,
				Name: "test-project",
			},
		}

		// mock subscriptions repo
		subRepo.EXPECT().
			LoadAllSubscriptionConfig(ctx, []string{projectID}, batchSize).
			Times(1).
			Return(subscriptions, nil)

		projectRepo.EXPECT().
			LoadProjects(ctx, gomock.Any()).
			Times(1).
			Return(projects, nil)

		// call subject.
		loader := NewSubscriptionLoader(subRepo, projectRepo, logger, batchSize)
		err := loader.SyncChanges(ctx, table)
		require.NoError(t, err)

		// assert.
		require.Equal(t, totalSubs, len(table.GetKeys()))
	})
	t.Run("should update the store with new subscriptions after initial run", func(t *testing.T) {
		table := memorystore.NewTable()
		ctx := context.Background()
		projectID := ulid.Make().String()
		endpointID := "test-endpoint"
		batchSize := int64(5)

		ctrl := gomock.NewController(t)
		subRepo := mocks.NewMockSubscriptionRepository(ctrl)
		projectRepo := mocks.NewMockProjectRepository(ctrl)
		logger := log.NewLogger(os.Stdout)

		initialSubs := 5
		lastUpdate := time.Now().Add(-5 * time.Second)
		lastDelete := time.Time{}
		subscriptions := []datastore.Subscription{
			{
				UID:        ulid.Make().String(),
				Name:       "test-subscription-1",
				ProjectID:  projectID,
				EndpointID: endpointID,
				FilterConfig: &datastore.FilterConfiguration{
					EventTypes: []string{"sub-1"},
				},
				UpdatedAt: time.Now().Add(-50 * time.Second),
			},
			{
				UID:        ulid.Make().String(),
				Name:       "test-subscription-2",
				ProjectID:  projectID,
				EndpointID: endpointID,
				FilterConfig: &datastore.FilterConfiguration{
					EventTypes: []string{"sub-2"},
				},
				UpdatedAt: time.Now().Add(-45 * time.Second),
			},
			{
				UID:        ulid.Make().String(),
				Name:       "test-subscription-3",
				ProjectID:  projectID,
				EndpointID: endpointID,
				FilterConfig: &datastore.FilterConfiguration{
					EventTypes: []string{"sub-3"},
				},
				UpdatedAt: time.Now().Add(-40 * time.Second),
			},
			{
				UID:        ulid.Make().String(),
				Name:       "test-subscription-4",
				ProjectID:  projectID,
				EndpointID: endpointID,
				FilterConfig: &datastore.FilterConfiguration{
					EventTypes: []string{"sub-4"},
				},
				UpdatedAt: time.Now().Add(-35 * time.Second),
			},
			{
				UID:        ulid.Make().String(),
				Name:       "test-subscription-5",
				ProjectID:  projectID,
				EndpointID: endpointID,
				FilterConfig: &datastore.FilterConfiguration{
					EventTypes: []string{"sub-5"},
				},
				UpdatedAt: lastUpdate,
				DeletedAt: null.TimeFrom(lastDelete),
			},
		}

		newSubsCount := 2
		newSubscriptions := []datastore.Subscription{
			{
				UID:        ulid.Make().String(),
				Name:       "test-subscription-6",
				ProjectID:  projectID,
				EndpointID: endpointID,
				FilterConfig: &datastore.FilterConfiguration{
					EventTypes: []string{"sub-5"},
				},
				UpdatedAt: lastUpdate,
			},
			{
				UID:        ulid.Make().String(),
				Name:       "test-subscription-7",
				ProjectID:  projectID,
				EndpointID: endpointID,
				FilterConfig: &datastore.FilterConfiguration{
					EventTypes: []string{"sub-5"},
				},
				UpdatedAt: lastUpdate,
			},
		}

		projects := []*datastore.Project{
			{
				UID:  projectID,
				Name: "test-project",
			},
		}

		// mock subscriptions repo
		subRepo.EXPECT().
			LoadAllSubscriptionConfig(ctx, []string{projectID}, batchSize).
			Times(1).
			Return(subscriptions, nil)

		projectRepo.EXPECT().
			LoadProjects(ctx, gomock.Any()).
			Times(3).
			Return(projects, nil)

		// call subject.
		loader := NewSubscriptionLoader(subRepo, projectRepo, logger, batchSize)

		// perform initial loading.
		err := loader.SyncChanges(ctx, table)
		require.NoError(t, err)

		// assert initial loading
		require.Equal(t, initialSubs, len(table.GetKeys()))

		subRepo.EXPECT().
			FetchUpdatedSubscriptions(ctx, []string{projectID}, lastUpdate, batchSize).
			Times(1).
			Return(newSubscriptions, nil)

		subRepo.EXPECT().
			FetchDeletedSubscriptions(ctx, []string{projectID}, lastDelete, batchSize).
			Times(1).
			Return(newSubscriptions, nil)

		// perform updates
		err = loader.SyncChanges(ctx, table)
		require.NoError(t, err)

		// assert.
		require.Equal(t, initialSubs+newSubsCount, len(table.GetKeys())+len(newSubscriptions))
	})
	t.Run("should remove deleted subscriptions from the store after initial run", func(t *testing.T) {
		table := memorystore.NewTable()
		ctx := context.Background()
		projectID := ulid.Make().String()
		endpointID := "test-endpoint"
		batchSize := int64(5)

		ctrl := gomock.NewController(t)
		subRepo := mocks.NewMockSubscriptionRepository(ctrl)
		projectRepo := mocks.NewMockProjectRepository(ctrl)
		logger := log.NewLogger(os.Stdout)

		initialSubs := 5
		lastUpdate := time.Now().Add(-5 * time.Second)
		lastDelete := time.Time{}
		subscriptions := []datastore.Subscription{
			{
				UID:        ulid.Make().String(),
				Name:       "test-subscription-1",
				ProjectID:  projectID,
				EndpointID: endpointID,
				FilterConfig: &datastore.FilterConfiguration{
					EventTypes: []string{"sub-1"},
				},
				UpdatedAt: time.Now().Add(-50 * time.Second),
			},
			{
				UID:        ulid.Make().String(),
				Name:       "test-subscription-2",
				ProjectID:  projectID,
				EndpointID: endpointID,
				FilterConfig: &datastore.FilterConfiguration{
					EventTypes: []string{"sub-2"},
				},
				UpdatedAt: time.Now().Add(-45 * time.Second),
			},
			{
				UID:        ulid.Make().String(),
				Name:       "test-subscription-3",
				ProjectID:  projectID,
				EndpointID: endpointID,
				FilterConfig: &datastore.FilterConfiguration{
					EventTypes: []string{"sub-3"},
				},
				UpdatedAt: time.Now().Add(-40 * time.Second),
			},
			{
				UID:        ulid.Make().String(),
				Name:       "test-subscription-4",
				ProjectID:  projectID,
				EndpointID: endpointID,
				FilterConfig: &datastore.FilterConfiguration{
					EventTypes: []string{"sub-4"},
				},
				UpdatedAt: time.Now().Add(-35 * time.Second),
			},
			{
				UID:        ulid.Make().String(),
				Name:       "test-subscription-5",
				ProjectID:  projectID,
				EndpointID: endpointID,
				FilterConfig: &datastore.FilterConfiguration{
					EventTypes: []string{"sub-5"},
				},
				UpdatedAt: lastUpdate,
			},
		}

		deletedSubsCount := 2
		newSubscriptions := make([]datastore.Subscription, 0)
		deletedSubscriptions := make([]datastore.Subscription, 0, deletedSubsCount)

		for i := 0; i < deletedSubsCount; i++ {
			subscription := subscriptions[i]
			subscription.UpdatedAt = lastUpdate.Add(4 * time.Second)
			subscription.DeletedAt = null.TimeFrom(lastDelete)
			deletedSubscriptions = append(deletedSubscriptions, subscription)
		}

		projects := []*datastore.Project{
			{
				UID:  projectID,
				Name: "test-project",
			},
		}

		// mock subscriptions repo
		subRepo.EXPECT().
			LoadAllSubscriptionConfig(ctx, []string{projectID}, batchSize).
			Times(1).
			Return(subscriptions, nil)

		projectRepo.EXPECT().
			LoadProjects(ctx, gomock.Any()).
			Times(3).
			Return(projects, nil)

		// call subject.
		loader := NewSubscriptionLoader(subRepo, projectRepo, logger, batchSize)

		// perform initial loading.
		err := loader.SyncChanges(ctx, table)
		require.NoError(t, err)

		// assert initial loading
		require.Equal(t, initialSubs, len(table.GetKeys()))

		subRepo.EXPECT().
			FetchUpdatedSubscriptions(ctx, []string{projectID}, lastUpdate, batchSize).
			Times(1).
			Return(newSubscriptions, nil)

		subRepo.EXPECT().
			FetchDeletedSubscriptions(ctx, []string{projectID}, lastDelete, batchSize).
			Times(1).
			Return(deletedSubscriptions, nil)

		// perform updates
		err = loader.SyncChanges(ctx, table)
		require.NoError(t, err)

		// assert.
		require.Equal(t, initialSubs-deletedSubsCount, len(table.GetKeys()))
	})

	t.Run("table should have an accurate number of keys", func(t *testing.T) {
		table := memorystore.NewTable()
		ctx := context.Background()
		projectID := "test-project"
		endpointID := "test-endpoint"
		batchSize := int64(5)

		ctrl := gomock.NewController(t)
		subRepo := mocks.NewMockSubscriptionRepository(ctrl)
		projectRepo := mocks.NewMockProjectRepository(ctrl)
		logger := log.NewLogger(os.Stdout)

		uniqueEventTypes := 2
		subscriptions := []datastore.Subscription{
			{
				UID:        ulid.Make().String(),
				Name:       "test-subscription-1",
				ProjectID:  projectID,
				EndpointID: endpointID,
				FilterConfig: &datastore.FilterConfiguration{
					EventTypes: []string{"event.type.1"},
				},
			},
			{
				UID:        ulid.Make().String(),
				Name:       "test-subscription-2",
				ProjectID:  projectID,
				EndpointID: endpointID,
				FilterConfig: &datastore.FilterConfiguration{
					EventTypes: []string{"event.type.2"},
				},
			},
			{
				UID:        ulid.Make().String(),
				Name:       "test-subscription-3",
				ProjectID:  projectID,
				EndpointID: endpointID,
				FilterConfig: &datastore.FilterConfiguration{
					EventTypes: []string{"event.type.1"},
				},
			},
			{
				UID:        ulid.Make().String(),
				Name:       "test-subscription-4",
				ProjectID:  projectID,
				EndpointID: endpointID,
				FilterConfig: &datastore.FilterConfiguration{
					EventTypes: []string{"event.type.1"},
				},
			},
			{
				UID:        ulid.Make().String(),
				Name:       "test-subscription-5",
				ProjectID:  projectID,
				EndpointID: endpointID,
				FilterConfig: &datastore.FilterConfiguration{
					EventTypes: []string{"event.type.2"},
				},
			},
		}

		projects := []*datastore.Project{
			{
				UID:  projectID,
				Name: "test-project",
			},
		}

		// mock subscriptions repo
		subRepo.EXPECT().
			LoadAllSubscriptionConfig(ctx, []string{projectID}, batchSize).
			Times(1).
			Return(subscriptions, nil)

		projectRepo.EXPECT().
			LoadProjects(ctx, gomock.Any()).
			Times(1).
			Return(projects, nil)

		// call subject.
		loader := NewSubscriptionLoader(subRepo, projectRepo, logger, batchSize)
		err := loader.SyncChanges(ctx, table)
		require.NoError(t, err)

		// assert.
		require.Equal(t, uniqueEventTypes, len(table.GetKeys()))
	})

	t.Run("should accurately reflect subscription updates in the table", func(t *testing.T) {
		table := memorystore.NewTable()
		ctx := context.Background()
		projectID := ulid.Make().String()
		endpointID := "test-endpoint"
		batchSize := int64(5)

		ctrl := gomock.NewController(t)
		subRepo := mocks.NewMockSubscriptionRepository(ctrl)
		projectRepo := mocks.NewMockProjectRepository(ctrl)
		logger := log.NewLogger(os.Stdout)

		baseTime := time.Now().Add(-10 * time.Minute)

		// Create subscription UIDs that we can reference later
		updateSub2UID := ulid.Make().String()

		// First batch of subscriptions
		subscriptions := []datastore.Subscription{
			{
				UID:        ulid.Make().String(),
				Name:       "update-subscription-1",
				ProjectID:  projectID,
				EndpointID: endpointID,
				FilterConfig: &datastore.FilterConfiguration{
					EventTypes: []string{"batch.event.1"},
				},
				UpdatedAt: baseTime.Add(1 * time.Second),
			},
			{
				UID:        updateSub2UID,
				Name:       "update-subscription-2",
				ProjectID:  projectID,
				EndpointID: endpointID,
				FilterConfig: &datastore.FilterConfiguration{
					EventTypes: []string{"batch.event.2"},
				},
				UpdatedAt: baseTime.Add(2 * time.Second),
			},
			{
				UID:        ulid.Make().String(),
				Name:       "update-subscription-3",
				ProjectID:  projectID,
				EndpointID: endpointID,
				FilterConfig: &datastore.FilterConfiguration{
					EventTypes: []string{"batch.event.3"},
				},
				UpdatedAt: baseTime.Add(5 * time.Second),
			},
		}

		projects := []*datastore.Project{
			{
				UID:  projectID,
				Name: "test-project",
			},
		}

		// Setup initial load
		subRepo.EXPECT().
			LoadAllSubscriptionConfig(ctx, []string{projectID}, batchSize).
			Times(1).
			Return(subscriptions, nil)

		projectRepo.EXPECT().
			LoadProjects(ctx, gomock.Any()).
			Times(3). // Initial + 2 sync cycles (each does 3 calls: projects, updates, deletes)
			Return(projects, nil)

		loader := NewSubscriptionLoader(subRepo, projectRepo, logger, batchSize)

		// Perform initial load
		err := loader.SyncChanges(ctx, table)
		require.NoError(t, err)
		require.Equal(t, 3, len(table.GetKeys())) // Only initial subscription

		// Retrieve the second item in the subscriptions array and update the updated time to baseTime.Add(4 * time.Second)
		updatedSubscriptions := append(make([]datastore.Subscription, 0, len(subscriptions)), subscriptions...)
		updatedSubscriptions[1].FilterConfig = &datastore.FilterConfiguration{
			EventTypes: []string{"updated.batch.event.2"},
		}
		updatedSubscriptions[1].UpdatedAt = baseTime.Add(4 * time.Second)

		// First sync after initial load - returns empty because the last updated time is after the second subscription updated time.
		subRepo.EXPECT().
			FetchUpdatedSubscriptions(ctx, []string{projectID}, baseTime.Add(5*time.Second), batchSize).
			Times(1).
			Return([]datastore.Subscription{}, nil)

		subRepo.EXPECT().
			FetchDeletedSubscriptions(ctx, []string{projectID}, time.Time{}, batchSize).
			Times(1).
			Return([]datastore.Subscription{}, nil)

		// First sync - processes the batch
		err = loader.SyncChanges(ctx, table)
		require.NoError(t, err)
		require.Equal(t, 3, len(table.GetKeys())) // Initial + 3 batch subscriptions

		// Retrieve all items from the table and assert that the filter event types match each subscription
		allKeys := table.GetKeys()
		for _, key := range allKeys {
			row := table.Get(key)
			if row == nil {
				continue
			}

			var dbSubs []datastore.Subscription
			if row.Value() != nil {
				var ok bool
				dbSubs, ok = row.Value().([]datastore.Subscription)
				if !ok {
					t.Errorf("malformed data in subscriptions memory store with key: %s", key.String())
					continue
				}
			}

			for _, sub := range dbSubs {
				// Find the equivalent subscription in the predefined subscriptions array
				var equivalentSub *datastore.Subscription
				for _, predefinedSub := range updatedSubscriptions {
					if predefinedSub.UID == sub.UID {
						equivalentSub = &predefinedSub
						break
					}
				}

				if equivalentSub == nil {
					t.Errorf("no equivalent subscription found for UID: %s", sub.UID)
					continue
				}

				// Verify the event types match
				for _, eventType := range sub.FilterConfig.EventTypes {
					if eventType != strings.Join(equivalentSub.FilterConfig.EventTypes, "") {
						t.Errorf("subscription event type does not match the predefined event type for UID: %s", sub.UID)
					}
				}
			}
		}

	})
}
