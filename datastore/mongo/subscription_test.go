//go:build integration
// +build integration

package mongo

import (
	"context"
	"fmt"
	"testing"

	"github.com/frain-dev/convoy/datastore"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func createSubscription() *datastore.Subscription {
	return &datastore.Subscription{
		UID:        uuid.NewString(),
		Name:       "Subscription",
		Type:       datastore.SubscriptionTypeAPI,
		AppID:      "app-id-1",
		GroupID:    "group-id-1",
		SourceID:   "source-id-1",
		EndpointID: "endpoint-id-1",
		AlertConfig: &datastore.AlertConfiguration{
			Count:     10,
			Threshold: "1m",
		},
		RetryConfig: &datastore.RetryConfiguration{
			Type:       "linear",
			Duration:   3,
			RetryCount: 10,
		},
		FilterConfig: &datastore.FilterConfiguration{
			EventTypes: []string{"some.event"},
		},
	}
}

func Test_LoadSubscriptionsPaged(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	store := getStore(db)

	subRepo := NewSubscriptionRepo(store)

	for i := 0; i < 20; i++ {
		subscription := &datastore.Subscription{
			UID:        uuid.NewString(),
			Name:       fmt.Sprintf("Subscription %d", i),
			Type:       datastore.SubscriptionTypeAPI,
			GroupID:    "group-id-1",
			SourceID:   uuid.NewString(),
			EndpointID: uuid.NewString(),
		}

		if i == 0 {
			subscription.AppID = "app-id-1"
		}

		require.NoError(t, subRepo.CreateSubscription(context.Background(), subscription.GroupID, subscription))
	}

	type Expected struct {
		paginationData datastore.PaginationData
	}

	tests := []struct {
		name     string
		appId    string
		pageData datastore.Pageable
		expected Expected
	}{
		{
			name:     "Load Subscriptions Paged - 10 records",
			pageData: datastore.Pageable{Page: 1, PerPage: 3},
			expected: Expected{
				paginationData: datastore.PaginationData{
					Total:     20,
					TotalPage: 7,
					Page:      1,
					PerPage:   3,
					Prev:      1,
					Next:      2,
				},
			},
		},

		{
			name:     "Load Subscriptions Paged - 12 records",
			pageData: datastore.Pageable{Page: 2, PerPage: 4},
			expected: Expected{
				paginationData: datastore.PaginationData{
					Total:     20,
					TotalPage: 5,
					Page:      2,
					PerPage:   4,
					Prev:      1,
					Next:      3,
				},
			},
		},

		{
			name:     "Load Subscriptions Paged - 0 records",
			pageData: datastore.Pageable{Page: 1, PerPage: 10},
			expected: Expected{
				paginationData: datastore.PaginationData{
					Total:     20,
					TotalPage: 2,
					Page:      1,
					PerPage:   10,
					Prev:      1,
					Next:      2,
				},
			},
		},

		{
			name:     "Load Subscriptions Paged with App ID - 1 record",
			appId:    "app-id-1",
			pageData: datastore.Pageable{Page: 1, PerPage: 3},
			expected: Expected{
				paginationData: datastore.PaginationData{
					Total:     1,
					TotalPage: 1,
					Page:      1,
					PerPage:   3,
					Prev:      1,
					Next:      2,
				},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, pageable, err := subRepo.LoadSubscriptionsPaged(context.Background(), "group-id-1", &datastore.FilterBy{AppID: tc.appId}, tc.pageData)

			require.NoError(t, err)

			require.Equal(t, tc.expected.paginationData.Total, pageable.Total)
			require.Equal(t, tc.expected.paginationData.TotalPage, pageable.TotalPage)
			require.Equal(t, tc.expected.paginationData.Page, pageable.Page)
			require.Equal(t, tc.expected.paginationData.PerPage, pageable.PerPage)
			require.Equal(t, tc.expected.paginationData.Prev, pageable.Prev)
			require.Equal(t, tc.expected.paginationData.Next, pageable.Next)
		})
	}
}

func Test_DeleteSubscription(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	store := getStore(db)

	subRepo := NewSubscriptionRepo(store)
	newSub := createSubscription()

	require.NoError(t, subRepo.CreateSubscription(context.Background(), newSub.GroupID, newSub))

	// delete the sub
	err := subRepo.DeleteSubscription(context.Background(), newSub.GroupID, newSub)
	require.NoError(t, err)

	// Fetch sub again
	_, err = subRepo.FindSubscriptionByID(context.Background(), newSub.GroupID, newSub.UID)
	require.Error(t, err)
	require.EqualError(t, err, datastore.ErrSubscriptionNotFound.Error())
}

func Test_CreateSubscription(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	store := getStore(db)

	subRepo := NewSubscriptionRepo(store)
	newSub := createSubscription()

	require.NoError(t, subRepo.CreateSubscription(context.Background(), newSub.GroupID, newSub))

	sub, err := subRepo.FindSubscriptionByID(context.Background(), newSub.GroupID, newSub.UID)
	require.NoError(t, err)

	require.Equal(t, sub.UID, newSub.UID)
	require.Equal(t, sub.SourceID, newSub.SourceID)
	require.Equal(t, sub.EndpointID, newSub.EndpointID)
}

func Test_FindSubscriptionByID(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	store := getStore(db)

	subRepo := NewSubscriptionRepo(store)
	newSub := createSubscription()

	// Fetch sub again
	_, err := subRepo.FindSubscriptionByID(context.Background(), newSub.GroupID, newSub.UID)
	require.Error(t, err)
	require.EqualError(t, err, datastore.ErrSubscriptionNotFound.Error())

	require.NoError(t, subRepo.CreateSubscription(context.Background(), newSub.GroupID, newSub))

	// Fetch sub again
	sub, err := subRepo.FindSubscriptionByID(context.Background(), newSub.GroupID, newSub.UID)
	require.NoError(t, err)

	require.Equal(t, sub.UID, newSub.UID)
	require.Equal(t, sub.SourceID, newSub.SourceID)
	require.Equal(t, sub.EndpointID, newSub.EndpointID)
}

func Test_FindSubscriptionByAppID(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	store := getStore(db)

	subRepo := NewSubscriptionRepo(store)

	for i := 0; i < 20; i++ {
		subscription := &datastore.Subscription{
			UID:        uuid.NewString(),
			Name:       fmt.Sprintf("Subscription %d", i),
			Type:       datastore.SubscriptionTypeAPI,
			AppID:      "app-id-1",
			GroupID:    "group-id-1",
			SourceID:   uuid.NewString(),
			EndpointID: uuid.NewString(),
		}
		require.NoError(t, subRepo.CreateSubscription(context.Background(), subscription.GroupID, subscription))
	}

	// Fetch sub again
	subs, err := subRepo.FindSubscriptionsByAppID(context.Background(), "group-id-1", "app-id-1")
	require.NoError(t, err)

	for _, sub := range subs {
		require.NotEmpty(t, sub.UID)
		require.Equal(t, sub.AppID, "app-id-1")
		require.Equal(t, sub.GroupID, "group-id-1")
	}
}

func Test_FindSubscriptionByDeviceID(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	store := getStore(db)
	subRepo := NewSubscriptionRepo(store)

	subscription := &datastore.Subscription{
		UID:      uuid.NewString(),
		Name:     "test_subscription",
		Type:     datastore.SubscriptionTypeAPI,
		SourceID: "source-id-1",
		DeviceID: "device-id-1",
		GroupID:  "group-id-1",
	}
	require.NoError(t, subRepo.CreateSubscription(context.Background(), subscription.GroupID, subscription))

	// Fetch sub again
	sub, err := subRepo.FindSubscriptionByDeviceID(context.Background(), "group-id-1", "device-id-1")
	require.NoError(t, err)

	require.NotEmpty(t, sub.UID)
	require.Equal(t, sub.DeviceID, "device-id-1")
	require.Equal(t, sub.GroupID, "group-id-1")
	require.Equal(t, sub.SourceID, "source-id-1")
}
