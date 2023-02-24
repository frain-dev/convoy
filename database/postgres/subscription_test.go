//go:build integration
// +build integration

package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/frain-dev/convoy/database"
	"github.com/frain-dev/convoy/datastore"
	"github.com/google/uuid"
	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/require"
)

func generateSubscription(project *datastore.Project, source *datastore.Source, endpoint *datastore.Endpoint, device *datastore.Device) *datastore.Subscription {
	return &datastore.Subscription{
		UID:        uuid.NewString(),
		Name:       "Subscription",
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
				Headers: datastore.M{},
				Body:    datastore.M{},
			},
		},
	}
}

func Test_LoadSubscriptionsPaged(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	subRepo := NewSubscriptionRepo(db)

	source := seedSource(t, db)
	project := seedProject(t, db)
	endpoint := seedEndpoint(t, db)
	device := seedDevice(t, db)
	subMap := map[string]*datastore.Subscription{}
	for i := 0; i < 100; i++ {
		newSub := generateSubscription(project, source, endpoint, device)
		require.NoError(t, subRepo.CreateSubscription(context.Background(), project.UID, newSub))
		subMap[newSub.UID] = newSub
	}

	type Expected struct {
		paginationData datastore.PaginationData
	}

	tests := []struct {
		name        string
		EndpointIDs []string
		pageData    datastore.Pageable
		expected    Expected
	}{
		{
			name:     "Load Subscriptions Paged - 10 records",
			pageData: datastore.Pageable{Page: 1, PerPage: 3},
			expected: Expected{
				paginationData: datastore.PaginationData{
					Total:     100,
					TotalPage: 34,
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
					Total:     100,
					TotalPage: 25,
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
					Total:     100,
					TotalPage: 10,
					Page:      1,
					PerPage:   10,
					Prev:      1,
					Next:      2,
				},
			},
		},

		{
			name:        "Load Subscriptions Paged with Endpoint ID - 1 record",
			EndpointIDs: []string{endpoint.UID},
			pageData:    datastore.Pageable{Page: 1, PerPage: 3},
			expected: Expected{
				paginationData: datastore.PaginationData{
					Total:     100,
					TotalPage: 34,
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
			subs, pageable, err := subRepo.LoadSubscriptionsPaged(context.Background(), project.UID, &datastore.FilterBy{EndpointIDs: tc.EndpointIDs}, tc.pageData)
			require.NoError(t, err)

			require.Equal(t, tc.expected.paginationData.Total, pageable.Total)
			require.Equal(t, tc.expected.paginationData.TotalPage, pageable.TotalPage)
			require.Equal(t, tc.expected.paginationData.Page, pageable.Page)
			require.Equal(t, tc.expected.paginationData.PerPage, pageable.PerPage)
			require.Equal(t, tc.expected.paginationData.Prev, pageable.Prev)
			require.Equal(t, tc.expected.paginationData.Next, pageable.Next)

			require.Equal(t, tc.expected.paginationData.PerPage, int64(len(subs)))

			for _, dbSub := range subs {

				require.NotEmpty(t, dbSub.CreatedAt)
				require.NotEmpty(t, dbSub.UpdatedAt)

				dbSub.CreatedAt, dbSub.UpdatedAt = time.Time{}, time.Time{}

				require.Equal(t, dbSub.Endpoint.UID, endpoint.UID)
				require.Equal(t, dbSub.Endpoint.Title, endpoint.Title)
				require.Equal(t, dbSub.Endpoint.ProjectID, endpoint.ProjectID)
				require.Equal(t, dbSub.Endpoint.SupportEmail, endpoint.SupportEmail)

				require.Equal(t, dbSub.Source.UID, source.UID)
				require.Equal(t, dbSub.Source.Name, source.Name)
				require.Equal(t, dbSub.Source.Type, source.Type)
				require.Equal(t, dbSub.Source.MaskID, source.MaskID)
				require.Equal(t, dbSub.Source.ProjectID, source.ProjectID)
				require.Equal(t, dbSub.Source.IsDisabled, source.IsDisabled)

				dbSub.Source, dbSub.Endpoint = nil, nil

				require.Equal(t, dbSub, *subMap[dbSub.UID])
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
	dbSub.Source, dbSub.Endpoint = nil, nil

	require.Equal(t, dbSub, newSub)
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

	update := &datastore.Subscription{
		UID:             newSub.UID,
		Name:            "tyne&wear",
		ProjectID:       newSub.ProjectID,
		Type:            newSub.Type,
		SourceID:        seedSource(t, db).UID,
		EndpointID:      seedEndpoint(t, db).UID,
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
	dbSub.Source, dbSub.Endpoint = nil, nil

	require.Equal(t, dbSub, update)
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
	require.Equal(t, dbSub.Endpoint.Title, endpoint.Title)
	require.Equal(t, dbSub.Endpoint.ProjectID, endpoint.ProjectID)
	require.Equal(t, dbSub.Endpoint.SupportEmail, endpoint.SupportEmail)

	require.Equal(t, dbSub.Source.UID, source.UID)
	require.Equal(t, dbSub.Source.Name, source.Name)
	require.Equal(t, dbSub.Source.Type, source.Type)
	require.Equal(t, dbSub.Source.MaskID, source.MaskID)
	require.Equal(t, dbSub.Source.ProjectID, source.ProjectID)
	require.Equal(t, dbSub.Source.IsDisabled, source.IsDisabled)

	dbSub.Source, dbSub.Endpoint = nil, nil

	require.Equal(t, dbSub, newSub)
}

func Test_FindSubscriptionsBySourceID(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	subRepo := NewSubscriptionRepo(db)

	source := seedSource(t, db)
	project := seedProject(t, db)
	endpoint := seedEndpoint(t, db)

	subMap := map[string]*datastore.Subscription{}
	for i := 0; i < 5; i++ {
		var newSub *datastore.Subscription
		if i == 3 {
			newSub = generateSubscription(project, seedSource(t, db), endpoint, &datastore.Device{})
		} else {
			newSub = generateSubscription(project, source, endpoint, &datastore.Device{})
		}

		require.NoError(t, subRepo.CreateSubscription(context.Background(), project.UID, newSub))
		subMap[newSub.UID] = newSub
	}

	// Fetch sub again
	dbSubs, err := subRepo.FindSubscriptionsBySourceID(context.Background(), project.UID, source.UID)
	require.NoError(t, err)
	require.Equal(t, 4, len(dbSubs))

	for _, dbSub := range dbSubs {

		require.NotEmpty(t, dbSub.CreatedAt)
		require.NotEmpty(t, dbSub.UpdatedAt)

		dbSub.CreatedAt, dbSub.UpdatedAt = time.Time{}, time.Time{}
		require.Equal(t, dbSub.Endpoint.UID, endpoint.UID)
		require.Equal(t, dbSub.Endpoint.Title, endpoint.Title)
		require.Equal(t, dbSub.Endpoint.ProjectID, endpoint.ProjectID)
		require.Equal(t, dbSub.Endpoint.SupportEmail, endpoint.SupportEmail)

		require.Equal(t, dbSub.Source.UID, source.UID)
		require.Equal(t, dbSub.Source.Name, source.Name)
		require.Equal(t, dbSub.Source.Type, source.Type)
		require.Equal(t, dbSub.Source.MaskID, source.MaskID)
		require.Equal(t, dbSub.Source.ProjectID, source.ProjectID)
		require.Equal(t, dbSub.Source.IsDisabled, source.IsDisabled)

		dbSub.Source, dbSub.Endpoint = nil, nil

		require.Equal(t, dbSub, *subMap[dbSub.UID])
	}
}

func Test_FindSubscriptionByEndpointID(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	subRepo := NewSubscriptionRepo(db)

	source := seedSource(t, db)
	project := seedProject(t, db)
	endpoint := seedEndpoint(t, db)

	subMap := map[string]*datastore.Subscription{}
	for i := 0; i < 8; i++ {
		var newSub *datastore.Subscription
		if i == 3 {
			newSub = generateSubscription(project, source, seedEndpoint(t, db), &datastore.Device{})
		} else {
			newSub = generateSubscription(project, source, endpoint, &datastore.Device{})
		}

		require.NoError(t, subRepo.CreateSubscription(context.Background(), project.UID, newSub))
		subMap[newSub.UID] = newSub
	}

	// Fetch sub again
	dbSubs, err := subRepo.FindSubscriptionsByEndpointID(context.Background(), project.UID, endpoint.UID)
	require.NoError(t, err)
	require.Equal(t, 7, len(dbSubs))

	for _, dbSub := range dbSubs {

		require.NotEmpty(t, dbSub.CreatedAt)
		require.NotEmpty(t, dbSub.UpdatedAt)

		dbSub.CreatedAt, dbSub.UpdatedAt = time.Time{}, time.Time{}
		require.Equal(t, dbSub.Endpoint.UID, endpoint.UID)
		require.Equal(t, dbSub.Endpoint.Title, endpoint.Title)
		require.Equal(t, dbSub.Endpoint.ProjectID, endpoint.ProjectID)
		require.Equal(t, dbSub.Endpoint.SupportEmail, endpoint.SupportEmail)

		require.Equal(t, dbSub.Source.UID, source.UID)
		require.Equal(t, dbSub.Source.Name, source.Name)
		require.Equal(t, dbSub.Source.Type, source.Type)
		require.Equal(t, dbSub.Source.MaskID, source.MaskID)
		require.Equal(t, dbSub.Source.ProjectID, source.ProjectID)
		require.Equal(t, dbSub.Source.IsDisabled, source.IsDisabled)

		dbSub.Source, dbSub.Endpoint = nil, nil

		require.Equal(t, dbSub, *subMap[dbSub.UID])
	}
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

	// Fetch sub again
	dbSub, err := subRepo.FindSubscriptionByID(context.Background(), project.UID, newSub.UID)
	require.NoError(t, err)

	require.NotEmpty(t, dbSub.CreatedAt)
	require.NotEmpty(t, dbSub.UpdatedAt)

	dbSub.CreatedAt, dbSub.UpdatedAt = time.Time{}, time.Time{}
	require.Equal(t, dbSub.Endpoint.UID, endpoint.UID)
	require.Equal(t, dbSub.Endpoint.Title, endpoint.Title)
	require.Equal(t, dbSub.Endpoint.ProjectID, endpoint.ProjectID)
	require.Equal(t, dbSub.Endpoint.SupportEmail, endpoint.SupportEmail)

	require.Equal(t, dbSub.Source.UID, source.UID)
	require.Equal(t, dbSub.Source.Name, source.Name)
	require.Equal(t, dbSub.Source.Type, source.Type)
	require.Equal(t, dbSub.Source.MaskID, source.MaskID)
	require.Equal(t, dbSub.Source.ProjectID, source.ProjectID)
	require.Equal(t, dbSub.Source.IsDisabled, source.IsDisabled)

	dbSub.Source, dbSub.Endpoint = nil, nil

	require.Equal(t, dbSub, newSub)
}

func TestTestSubscriptionFilter(t *testing.T) {
	t.Skip()
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
