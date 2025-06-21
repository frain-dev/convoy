//go:build integration
// +build integration

package postgres

import (
	"context"
	"fmt"
	"math"
	"testing"
	"time"

	"github.com/frain-dev/convoy/database"
	"github.com/frain-dev/convoy/datastore"
	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/require"
)

func generateSubscription(project *datastore.Project, source *datastore.Source, endpoint *datastore.Endpoint, device *datastore.Device) *datastore.Subscription {
	uid := ulid.Make().String()
	return &datastore.Subscription{
		UID:        uid,
		Name:       "Subscription-" + uid,
		Type:       datastore.SubscriptionTypeAPI,
		ProjectID:  project.UID,
		SourceID:   source.UID,
		EndpointID: endpoint.UID,
		DeviceID:   device.UID,
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
			Filter: datastore.FilterSchema{
				IsFlattened: false,
				Headers:     datastore.M{},
				Body:        datastore.M{},
				RawHeaders:  datastore.M{},
				RawBody:     datastore.M{},
			},
		},
	}
}

func seedSubscription(t *testing.T, db database.Database, project *datastore.Project, source *datastore.Source, endpoint *datastore.Endpoint, device *datastore.Device) *datastore.Subscription {
	// If no endpoint is provided, create a new one to avoid unique constraint violations
	if endpoint == nil {
		endpoint = seedEndpoint(t, db)
	}

	s := generateSubscription(project, source, endpoint, device)
	require.NoError(t, NewSubscriptionRepo(db).CreateSubscription(context.Background(), project.UID, s))
	return s
}

func Test_LoadSubscriptionsPaged(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()
	ctx := context.Background()

	subRepo := NewSubscriptionRepo(db)
	project := seedProject(t, db)

	source := seedSource(t, db)
	device := seedDevice(t, db)
	subMap := map[string]*datastore.Subscription{}
	newSub := &datastore.Subscription{}
	// Create a reference endpoint to use for filtering tests
	referenceEndpoint := seedEndpoint(t, db)

	for i := 0; i < 100; i++ {
		// Create a unique endpoint for each subscription to avoid unique constraint violation
		endpoint := seedEndpoint(t, db)
		newSub = generateSubscription(project, source, endpoint, device)
		require.NoError(t, subRepo.CreateSubscription(ctx, project.UID, newSub))
		subMap[newSub.UID] = newSub
	}

	// Create one more subscription with the reference endpoint for endpoint filtering tests
	refSub := generateSubscription(project, source, referenceEndpoint, device)
	require.NoError(t, subRepo.CreateSubscription(ctx, project.UID, refSub))
	subMap[refSub.UID] = refSub

	type Expected struct {
		paginationData datastore.PaginationData
	}

	tests := []struct {
		name             string
		EndpointIDs      []string
		SubscriptionName string
		pageData         datastore.Pageable
		expected         Expected
		expectedCount    int64 // Added to check specific counts
	}{
		{
			name:     "Load Subscriptions Paged - 10 records",
			pageData: datastore.Pageable{PerPage: 3, Direction: datastore.Next, NextCursor: fmt.Sprintf("%d", math.MaxInt)},
			expected: Expected{
				paginationData: datastore.PaginationData{
					PerPage: 3,
				},
			},
			expectedCount: 3,
		},

		{
			name:             "Load Subscriptions Paged - 1 record - filter by name",
			SubscriptionName: newSub.Name,
			pageData:         datastore.Pageable{PerPage: 1, Direction: datastore.Next, NextCursor: fmt.Sprintf("%d", math.MaxInt)},
			expected: Expected{
				paginationData: datastore.PaginationData{
					PerPage: 1,
				},
			},
			expectedCount: 1,
		},

		{
			name:     "Load Subscriptions Paged - 12 records",
			pageData: datastore.Pageable{PerPage: 4, Direction: datastore.Next, NextCursor: fmt.Sprintf("%d", math.MaxInt)},
			expected: Expected{
				paginationData: datastore.PaginationData{
					PerPage: 4,
				},
			},
			expectedCount: 4,
		},

		{
			name:     "Load Subscriptions Paged - 0 records",
			pageData: datastore.Pageable{PerPage: 10, Direction: datastore.Next, NextCursor: fmt.Sprintf("%d", math.MaxInt)},
			expected: Expected{
				paginationData: datastore.PaginationData{
					PerPage: 10,
				},
			},
			expectedCount: 10,
		},

		{
			name:        "Load Subscriptions Paged with Endpoint ID - 1 record",
			EndpointIDs: []string{referenceEndpoint.UID},
			pageData:    datastore.Pageable{PerPage: 3, Direction: datastore.Next, NextCursor: fmt.Sprintf("%d", math.MaxInt)},
			expected: Expected{
				paginationData: datastore.PaginationData{
					PerPage: 3,
				},
			},
			expectedCount: 1, // We only created one subscription with the reference endpoint
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			subs, pageable, err := subRepo.LoadSubscriptionsPaged(ctx, project.UID, &datastore.FilterBy{EndpointIDs: tc.EndpointIDs, SubscriptionName: tc.SubscriptionName}, tc.pageData)
			require.NoError(t, err)

			require.Equal(t, tc.expected.paginationData.PerPage, pageable.PerPage)

			// Check against the expected count instead of the per page value
			require.Equal(t, tc.expectedCount, int64(len(subs)))

			for _, dbSub := range subs {
				require.NotEmpty(t, dbSub.CreatedAt)
				require.NotEmpty(t, dbSub.UpdatedAt)

				dbSub.CreatedAt, dbSub.UpdatedAt = time.Time{}, time.Time{}

				// The endpoint might be different for each subscription now
				if tc.EndpointIDs != nil && len(tc.EndpointIDs) > 0 {
					require.Equal(t, dbSub.Endpoint.UID, referenceEndpoint.UID)
					require.Equal(t, dbSub.Endpoint.Name, referenceEndpoint.Name)
					require.Equal(t, dbSub.Endpoint.ProjectID, referenceEndpoint.ProjectID)
					require.Equal(t, dbSub.Endpoint.SupportEmail, referenceEndpoint.SupportEmail)
				}

				require.Equal(t, dbSub.Source.UID, source.UID)
				require.Equal(t, dbSub.Source.Name, source.Name)
				require.Equal(t, dbSub.Source.Type, source.Type)
				require.Equal(t, dbSub.Source.MaskID, source.MaskID)
				require.Equal(t, dbSub.Source.ProjectID, source.ProjectID)
				require.Equal(t, dbSub.Source.IsDisabled, source.IsDisabled)

				dbSub.Source, dbSub.Endpoint, dbSub.Device = nil, nil, nil

				require.Equal(t, dbSub.UID, subMap[dbSub.UID].UID)
			}
		})
	}
}

func Test_DeleteSubscription(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	subRepo := NewSubscriptionRepo(db)

	project := seedProject(t, db)
	source := seedSource(t, db)
	endpoint := seedEndpoint(t, db)

	newSub := generateSubscription(project, source, endpoint, &datastore.Device{})

	err := subRepo.CreateSubscription(context.Background(), project.UID, newSub)
	require.NoError(t, err)

	// delete the sub
	err = subRepo.DeleteSubscription(context.Background(), project.UID, newSub)
	require.NoError(t, err)

	// Fetch sub again
	_, err = subRepo.FindSubscriptionByID(context.Background(), project.UID, newSub.UID)
	require.Equal(t, err, datastore.ErrSubscriptionNotFound)
}

func Test_CreateSubscription(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	subRepo := NewSubscriptionRepo(db)

	project := seedProject(t, db)
	source := seedSource(t, db)
	endpoint := seedEndpoint(t, db)

	newSub := generateSubscription(project, source, endpoint, &datastore.Device{})
	require.NoError(t, subRepo.CreateSubscription(context.Background(), newSub.ProjectID, newSub))

	dbSub, err := subRepo.FindSubscriptionByID(context.Background(), newSub.ProjectID, newSub.UID)
	require.NoError(t, err)

	require.NotEmpty(t, dbSub.CreatedAt)
	require.NotEmpty(t, dbSub.UpdatedAt)

	dbSub.CreatedAt, dbSub.UpdatedAt = time.Time{}, time.Time{}
	dbSub.Source, dbSub.Endpoint, dbSub.Device = nil, nil, nil

	require.Equal(t, dbSub.UID, newSub.UID)
}

func Test_CountEndpointSubscriptions(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	subRepo := NewSubscriptionRepo(db)

	project := seedOutgoingProject(t, db)
	source := seedSource(t, db)
	endpoint := seedEndpoint(t, db)
	endpoint2 := seedEndpoint(t, db)

	newSub1 := generateSubscription(project, source, endpoint, &datastore.Device{})
	require.NoError(t, subRepo.CreateSubscription(context.Background(), project.UID, newSub1))

	// Create a subscription with a different endpoint - should succeed
	newSub2 := generateSubscription(project, source, endpoint2, &datastore.Device{})
	require.NoError(t, subRepo.CreateSubscription(context.Background(), newSub2.ProjectID, newSub2))

	endpointSubscriptions, err := subRepo.CountEndpointSubscriptions(context.Background(), newSub1.ProjectID, endpoint.UID, "")
	require.NoError(t, err)

	require.Equal(t, int64(1), endpointSubscriptions)
}

func Test_UpdateSubscription(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	subRepo := NewSubscriptionRepo(db)

	project := seedProject(t, db)
	source := seedSource(t, db)
	endpoint := seedEndpoint(t, db)

	newSub := generateSubscription(project, source, endpoint, &datastore.Device{})
	require.NoError(t, subRepo.CreateSubscription(context.Background(), newSub.ProjectID, newSub))

	// Create a new endpoint for the update to avoid unique constraint violation
	newEndpoint := seedEndpoint(t, db)
	newSource := seedSource(t, db)

	update := &datastore.Subscription{
		UID:             newSub.UID,
		Name:            "tyne&wear",
		ProjectID:       newSub.ProjectID,
		Type:            newSub.Type,
		SourceID:        newSource.UID,
		EndpointID:      newEndpoint.UID,
		AlertConfig:     &datastore.DefaultAlertConfig,
		RetryConfig:     &datastore.DefaultRetryConfig,
		FilterConfig:    newSub.FilterConfig,
		RateLimitConfig: &datastore.DefaultRateLimitConfig,
	}

	err := subRepo.UpdateSubscription(context.Background(), project.UID, update)
	require.NoError(t, err)

	dbSub, err := subRepo.FindSubscriptionByID(context.Background(), newSub.ProjectID, newSub.UID)
	require.NoError(t, err)

	require.NotEmpty(t, dbSub.CreatedAt)
	require.NotEmpty(t, dbSub.UpdatedAt)

	dbSub.CreatedAt, dbSub.UpdatedAt = time.Time{}, time.Time{}
	dbSub.Source, dbSub.Endpoint, dbSub.Device = nil, nil, nil

	require.Equal(t, dbSub.UID, update.UID)
	require.Equal(t, dbSub.Name, update.Name)
	require.Equal(t, dbSub.EndpointID, update.EndpointID)
	require.Equal(t, dbSub.SourceID, update.SourceID)
}

func Test_FindSubscriptionByID(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	subRepo := NewSubscriptionRepo(db)

	project := seedProject(t, db)
	source := seedSource(t, db)
	endpoint := seedEndpoint(t, db)

	newSub := generateSubscription(project, source, endpoint, &datastore.Device{})

	// Fetch sub again
	_, err := subRepo.FindSubscriptionByID(context.Background(), project.UID, newSub.UID)
	require.Error(t, err)
	require.EqualError(t, err, datastore.ErrSubscriptionNotFound.Error())

	require.NoError(t, subRepo.CreateSubscription(context.Background(), project.UID, newSub))

	// Fetch sub again
	dbSub, err := subRepo.FindSubscriptionByID(context.Background(), project.UID, newSub.UID)
	require.NoError(t, err)

	require.NotEmpty(t, dbSub.CreatedAt)
	require.NotEmpty(t, dbSub.UpdatedAt)

	dbSub.CreatedAt, dbSub.UpdatedAt = time.Time{}, time.Time{}
	require.Equal(t, dbSub.Endpoint.UID, endpoint.UID)
	require.Equal(t, dbSub.Endpoint.Name, endpoint.Name)
	require.Equal(t, dbSub.Endpoint.ProjectID, endpoint.ProjectID)
	require.Equal(t, dbSub.Endpoint.SupportEmail, endpoint.SupportEmail)

	require.Equal(t, dbSub.Source.UID, source.UID)
	require.Equal(t, dbSub.Source.Name, source.Name)
	require.Equal(t, dbSub.Source.Type, source.Type)
	require.Equal(t, dbSub.Source.MaskID, source.MaskID)
	require.Equal(t, dbSub.Source.ProjectID, source.ProjectID)
	require.Equal(t, dbSub.Source.IsDisabled, source.IsDisabled)

	dbSub.Source, dbSub.Endpoint, dbSub.Device = nil, nil, nil

	require.Equal(t, dbSub.UID, newSub.UID)
}

func Test_FindSubscriptionsBySourceID(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	subRepo := NewSubscriptionRepo(db)

	targetSource := seedSource(t, db)
	differentSource := seedSource(t, db)
	project := seedProject(t, db)

	subMap := map[string]*datastore.Subscription{}
	for i := 0; i < 5; i++ {
		var newSub *datastore.Subscription
		// Create a unique endpoint for each subscription to avoid unique constraint violation
		endpoint := seedEndpoint(t, db)

		if i == 3 {
			// Use a different source for one subscription
			newSub = generateSubscription(project, differentSource, endpoint, &datastore.Device{})
		} else {
			// Use the target source for the rest
			newSub = generateSubscription(project, targetSource, endpoint, &datastore.Device{})
		}

		require.NoError(t, subRepo.CreateSubscription(context.Background(), project.UID, newSub))
		subMap[newSub.UID] = newSub
	}

	// Fetch subscriptions by source ID
	dbSubs, err := subRepo.FindSubscriptionsBySourceID(context.Background(), project.UID, targetSource.UID)
	require.NoError(t, err)
	require.Equal(t, 4, len(dbSubs))

	for _, dbSub := range dbSubs {
		require.NotEmpty(t, dbSub.CreatedAt)
		require.NotEmpty(t, dbSub.UpdatedAt)

		dbSub.CreatedAt, dbSub.UpdatedAt = time.Time{}, time.Time{}

		// We can't check for endpoint equality since each subscription has a unique endpoint now
		require.NotEmpty(t, dbSub.Endpoint.UID)
		require.NotEmpty(t, dbSub.Endpoint.Name)
		require.NotEmpty(t, dbSub.Endpoint.ProjectID)
		require.NotEmpty(t, dbSub.Endpoint.SupportEmail)

		require.Equal(t, dbSub.Source.UID, targetSource.UID)
		require.Equal(t, dbSub.Source.Name, targetSource.Name)
		require.Equal(t, dbSub.Source.Type, targetSource.Type)
		require.Equal(t, dbSub.Source.MaskID, targetSource.MaskID)
		require.Equal(t, dbSub.Source.ProjectID, targetSource.ProjectID)
		require.Equal(t, dbSub.Source.IsDisabled, targetSource.IsDisabled)

		dbSub.Source, dbSub.Endpoint, dbSub.Device = nil, nil, nil

		require.Equal(t, dbSub.UID, subMap[dbSub.UID].UID)
	}
}

func Test_FindSubscriptionByEndpointID(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	subRepo := NewSubscriptionRepo(db)

	// Create a single project and endpoint for this test
	project := seedProject(t, db)
	endpoint := seedEndpoint(t, db)

	// Create a single subscription with the endpoint
	source := seedSource(t, db)
	sub := generateSubscription(project, source, endpoint, &datastore.Device{})
	err := subRepo.CreateSubscription(context.Background(), project.UID, sub)
	require.NoError(t, err)

	// Fetch the subscription by endpoint ID
	dbSubs, err := subRepo.FindSubscriptionsByEndpointID(context.Background(), project.UID, endpoint.UID)
	require.NoError(t, err)
	require.Equal(t, 1, len(dbSubs))

	// Verify the subscription details
	dbSub := dbSubs[0]
	require.NotEmpty(t, dbSub.CreatedAt)
	require.NotEmpty(t, dbSub.UpdatedAt)

	// Verify endpoint details
	require.Equal(t, endpoint.UID, dbSub.Endpoint.UID)
	require.Equal(t, endpoint.Name, dbSub.Endpoint.Name)
	require.Equal(t, endpoint.ProjectID, dbSub.Endpoint.ProjectID)
	require.Equal(t, endpoint.SupportEmail, dbSub.Endpoint.SupportEmail)

	// Verify source details
	require.Equal(t, source.UID, dbSub.Source.UID)
	require.Equal(t, source.Name, dbSub.Source.Name)
	require.Equal(t, source.Type, dbSub.Source.Type)

	// Verify subscription details
	require.Equal(t, sub.UID, dbSub.UID)
	require.Equal(t, sub.Name, dbSub.Name)
	require.Equal(t, sub.ProjectID, dbSub.ProjectID)
}

func Test_FindSubscriptionByDeviceID(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	subRepo := NewSubscriptionRepo(db)

	project := seedProject(t, db)
	source := seedSource(t, db)
	endpoint := seedEndpoint(t, db)
	device := seedDevice(t, db)
	newSub := generateSubscription(project, source, endpoint, device)

	require.NoError(t, subRepo.CreateSubscription(context.Background(), project.UID, newSub))

	// Fetch subscription by device ID
	dbSub, err := subRepo.FindSubscriptionByDeviceID(context.Background(), project.UID, device.UID, newSub.Type)
	require.NoError(t, err)

	require.NotEmpty(t, dbSub.CreatedAt)
	require.NotEmpty(t, dbSub.UpdatedAt)

	dbSub.CreatedAt, dbSub.UpdatedAt = time.Time{}, time.Time{}
	require.Nil(t, dbSub.Endpoint)
	require.Nil(t, dbSub.Source)

	require.Equal(t, device.UID, dbSub.Device.UID)
	require.Equal(t, device.HostName, dbSub.Device.HostName)

	dbSub.Source, dbSub.Endpoint, dbSub.Device = nil, nil, nil

	require.Equal(t, dbSub.UID, newSub.UID)
}

func Test_FindCLISubscriptions(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	subRepo := NewSubscriptionRepo(db)

	source := seedSource(t, db)
	project := seedProject(t, db)

	for i := 0; i < 8; i++ {
		// Create a unique endpoint for each subscription to avoid unique constraint violation
		endpoint := seedEndpoint(t, db)

		newSub := &datastore.Subscription{
			UID:        ulid.Make().String(),
			Name:       "Subscription",
			Type:       datastore.SubscriptionTypeCLI,
			ProjectID:  project.UID,
			SourceID:   source.UID,
			EndpointID: endpoint.UID,
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
				Filter: datastore.FilterSchema{
					Headers: datastore.M{},
					Body:    datastore.M{},
				},
			},
		}

		require.NoError(t, subRepo.CreateSubscription(context.Background(), project.UID, newSub))
	}

	// Fetch CLI subscriptions
	dbSubs, err := subRepo.FindCLISubscriptions(context.Background(), project.UID)
	require.NoError(t, err)
	require.Equal(t, 8, len(dbSubs))
}

func Test_DeliveryModes(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	subRepo := NewSubscriptionRepo(db)
	project := seedProject(t, db)
	source := seedSource(t, db)
	endpoint := seedEndpoint(t, db)

	testCases := []struct {
		name                string
		deliveryMode        datastore.DeliveryMode
		expectedInitial     datastore.DeliveryMode
		updateTo            datastore.DeliveryMode
		expectedAfterUpdate datastore.DeliveryMode
	}{
		{
			name:                "At Least Once",
			deliveryMode:        datastore.AtLeastOnceDeliveryMode,
			expectedInitial:     datastore.AtLeastOnceDeliveryMode,
			updateTo:            datastore.AtMostOnceDeliveryMode,
			expectedAfterUpdate: datastore.AtMostOnceDeliveryMode,
		},
		{
			name:                "At Most Once",
			deliveryMode:        datastore.AtMostOnceDeliveryMode,
			expectedInitial:     datastore.AtMostOnceDeliveryMode,
			updateTo:            datastore.AtLeastOnceDeliveryMode,
			expectedAfterUpdate: datastore.AtLeastOnceDeliveryMode,
		},
		{
			name:                "Empty String",
			deliveryMode:        "",
			expectedInitial:     datastore.AtLeastOnceDeliveryMode, // default value
			updateTo:            datastore.AtMostOnceDeliveryMode,
			expectedAfterUpdate: datastore.AtMostOnceDeliveryMode,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create a subscription with the test delivery mode
			sub := generateSubscription(project, source, endpoint, &datastore.Device{})
			sub.DeliveryMode = tc.deliveryMode

			// Create the subscription
			err := subRepo.CreateSubscription(context.Background(), project.UID, sub)
			require.NoError(t, err)

			// Retrieve the subscription
			dbSub, err := subRepo.FindSubscriptionByID(context.Background(), project.UID, sub.UID)
			require.NoError(t, err)

			// Verify the initial delivery mode
			require.Equal(t, tc.expectedInitial, dbSub.DeliveryMode)

			// Update the subscription with a different delivery mode
			sub.DeliveryMode = tc.updateTo
			err = subRepo.UpdateSubscription(context.Background(), project.UID, sub)
			require.NoError(t, err)

			// Verify the update
			dbSub, err = subRepo.FindSubscriptionByID(context.Background(), project.UID, sub.UID)
			require.NoError(t, err)
			require.Equal(t, tc.expectedAfterUpdate, dbSub.DeliveryMode)
		})
	}
}

func seedDevice(t *testing.T, db database.Database) *datastore.Device {
	project := seedProject(t, db)
	endpoint := seedEndpoint(t, db)

	d := &datastore.Device{
		UID:        ulid.Make().String(),
		ProjectID:  project.UID,
		EndpointID: endpoint.UID,
		HostName:   "host1",
		Status:     datastore.DeviceStatusOnline,
	}

	err := NewDeviceRepo(db).CreateDevice(context.Background(), d)
	require.NoError(t, err)
	return d
}
