package loader

import (
	"context"
	"os"
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
}
