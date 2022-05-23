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
		Type:       "incoming",
		GroupID:    "group-id-1",
		SourceID:   "source-id-1",
		EndpointID: "endpoint-id-1",
		AlertConfig: datastore.AlertConfiguration{
			Count: 10,
			Time:  "1m",
		},
		RetryConfig: datastore.RetryConfiguration{
			Type: "linear",
			Linear: datastore.LinearStrategyConfiguration{
				IntervalSeconds: 10,
				RetryLimit:      10,
			},
		},
		FilterConfig: datastore.FilterConfiguration{
			Events: []string{"some.event"},
		},
	}
}

func Test_CreateSubscription(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	subRepo := NewSubscriptionRepo(db)
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

	subRepo := NewSubscriptionRepo(db)
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

func Test_LoadSubscriptionsPaged(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	subRepo := NewSubscriptionRepo(db)

	for i := 0; i < 20; i++ {
		subscription := &datastore.Subscription{
			UID:            uuid.NewString(),
			Name:           fmt.Sprintf("Subscription %d", i),
			Type:           "incoming",
			GroupID:        "group-id-1",
			SourceID:       "source-id-1",
			EndpointID:     "endpoint-id-1",
			DocumentStatus: datastore.ActiveDocumentStatus,
		}
		require.NoError(t, subRepo.CreateSubscription(context.Background(), subscription.GroupID, subscription))
	}

	type Expected struct {
		paginationData datastore.PaginationData
	}

	tests := []struct {
		name     string
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
					Prev:      0,
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
			pageData: datastore.Pageable{Page: 0, PerPage: 10},
			expected: Expected{
				paginationData: datastore.PaginationData{
					Total:     20,
					TotalPage: 2,
					Page:      1,
					PerPage:   10,
					Prev:      0,
					Next:      2,
				},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, pageable, err := subRepo.LoadSubscriptionsPaged(context.Background(), "group-id-1", tc.pageData)

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

	subRepo := NewSubscriptionRepo(db)
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
