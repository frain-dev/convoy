//go:build integration
// +build integration

package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/oklog/ulid/v2"

	"github.com/frain-dev/convoy/database"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/util"
	"github.com/stretchr/testify/require"
)

func Test_CreateDevice(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	deviceRepo := NewDeviceRepo(db)
	device := generateDevice(t, db)

	require.NoError(t, deviceRepo.CreateDevice(context.Background(), device))

	newDevice, err := deviceRepo.FetchDeviceByID(context.Background(), device.UID, device.EndpointID, device.ProjectID)
	require.NoError(t, err)

	device.LastSeenAt = device.LastSeenAt.UTC()
	newDevice.LastSeenAt = newDevice.LastSeenAt.UTC()
	newDevice.CreatedAt = time.Time{}
	newDevice.UpdatedAt = time.Time{}

	require.Equal(t, device, newDevice)
}

func Test_UpdateDevice(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	deviceRepo := NewDeviceRepo(db)
	device := generateDevice(t, db)

	require.NoError(t, deviceRepo.CreateDevice(context.Background(), device))

	device.Status = datastore.DeviceStatusOffline
	err := deviceRepo.UpdateDevice(context.Background(), device, device.EndpointID, device.ProjectID)
	require.NoError(t, err)

	newDevice, err := deviceRepo.FetchDeviceByID(context.Background(), device.UID, device.EndpointID, device.ProjectID)
	require.NoError(t, err)

	device.LastSeenAt = device.LastSeenAt.UTC()
	newDevice.LastSeenAt = newDevice.LastSeenAt.UTC()
	newDevice.CreatedAt = time.Time{}
	newDevice.UpdatedAt = time.Time{}

	require.Equal(t, device, newDevice)
}

func Test_UpdateDeviceLastSeen(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	deviceRepo := NewDeviceRepo(db)
	device := generateDevice(t, db)

	require.NoError(t, deviceRepo.CreateDevice(context.Background(), device))

	device.Status = datastore.DeviceStatusOffline
	err := deviceRepo.UpdateDeviceLastSeen(context.Background(), device, device.EndpointID, device.ProjectID, datastore.DeviceStatusOffline)
	require.NoError(t, err)

	newDevice, err := deviceRepo.FetchDeviceByID(context.Background(), device.UID, device.EndpointID, device.ProjectID)
	require.NoError(t, err)

	device.LastSeenAt = device.LastSeenAt.UTC()
	newDevice.LastSeenAt = newDevice.LastSeenAt.UTC()
	newDevice.CreatedAt = time.Time{}
	newDevice.UpdatedAt = time.Time{}

	require.Equal(t, device, newDevice)
}

func Test_DeleteDevice(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	deviceRepo := NewDeviceRepo(db)
	device := generateDevice(t, db)

	require.NoError(t, deviceRepo.CreateDevice(context.Background(), device))

	err := deviceRepo.DeleteDevice(context.Background(), device.UID, device.EndpointID, device.ProjectID)
	require.NoError(t, err)

	_, err = deviceRepo.FetchDeviceByID(context.Background(), device.UID, device.EndpointID, device.ProjectID)
	require.Equal(t, datastore.ErrDeviceNotFound, err)
}

func Test_FetchDeviceByID(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	deviceRepo := NewDeviceRepo(db)
	device := generateDevice(t, db)

	require.NoError(t, deviceRepo.CreateDevice(context.Background(), device))

	newDevice, err := deviceRepo.FetchDeviceByID(context.Background(), device.UID, device.EndpointID, device.ProjectID)
	require.NoError(t, err)

	device.LastSeenAt = device.LastSeenAt.UTC()
	newDevice.LastSeenAt = newDevice.LastSeenAt.UTC()
	newDevice.CreatedAt = time.Time{}
	newDevice.UpdatedAt = time.Time{}

	require.Equal(t, device, newDevice)
}

func Test_LoadDevicesPaged(t *testing.T) {
	type Expected struct {
		paginationData datastore.PaginationData
	}

	tests := []struct {
		name     string
		pageData datastore.Pageable
		count    int
		filter   *datastore.ApiKeyFilter
		expected Expected
	}{
		{
			name:     "Load Devices Paged - 10 records",
			pageData: datastore.Pageable{Page: 1, PerPage: 3, Sort: -1},
			count:    10,
			filter:   &datastore.ApiKeyFilter{EndpointID: ""},
			expected: Expected{
				paginationData: datastore.PaginationData{
					Total:     10,
					TotalPage: 4,
					Page:      1,
					PerPage:   3,
					Prev:      1,
					Next:      2,
				},
			},
		},

		{
			name:     "Load Devices Paged - 12 records",
			pageData: datastore.Pageable{Page: 2, PerPage: 4, Sort: -1},
			count:    12,
			filter:   &datastore.ApiKeyFilter{EndpointID: ""},
			expected: Expected{
				paginationData: datastore.PaginationData{
					Total:     12,
					TotalPage: 3,
					Page:      2,
					PerPage:   4,
					Prev:      1,
					Next:      3,
				},
			},
		},

		{
			name:     "Load Devices Paged - 5 records",
			pageData: datastore.Pageable{Page: 1, PerPage: 3, Sort: -1},
			count:    5,
			filter:   &datastore.ApiKeyFilter{EndpointID: ""},
			expected: Expected{
				paginationData: datastore.PaginationData{
					Total:     5,
					TotalPage: 2,
					Page:      1,
					PerPage:   3,
					Prev:      1,
					Next:      2,
				},
			},
		},

		{
			name:     "Load Devices Paged - 1 record",
			pageData: datastore.Pageable{Page: 1, PerPage: 3, Sort: -1},
			count:    1,
			filter:   &datastore.ApiKeyFilter{EndpointID: ulid.Make().String()},
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
			db, closeFn := getDB(t)
			defer closeFn()

			deviceRepo := NewDeviceRepo(db)
			project := seedProject(t, db)
			endpoint := generateEndpoint(project)

			if !util.IsStringEmpty(tc.filter.EndpointID) {
				endpoint.UID = tc.filter.EndpointID
			}

			err := NewEndpointRepo(db).CreateEndpoint(context.Background(), endpoint, project.UID)
			require.NoError(t, err)

			for i := 0; i < tc.count; i++ {
				device := &datastore.Device{
					UID:        ulid.Make().String(),
					ProjectID:  project.UID,
					EndpointID: endpoint.UID,
					HostName:   "",
					Status:     datastore.DeviceStatusOnline,
					LastSeenAt: time.Now(),
				}

				if !util.IsStringEmpty(tc.filter.EndpointID) {
					device.EndpointID = tc.filter.EndpointID
				}

				require.NoError(t, deviceRepo.CreateDevice(context.Background(), device))
			}

			_, pageable, err := deviceRepo.LoadDevicesPaged(context.Background(), project.UID, tc.filter, tc.pageData)
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

func generateDevice(t *testing.T, db database.Database) *datastore.Device {
	project := seedProject(t, db)
	endpoint := generateEndpoint(project)

	err := NewEndpointRepo(db).CreateEndpoint(context.Background(), endpoint, project.UID)
	require.NoError(t, err)

	return &datastore.Device{
		UID:        ulid.Make().String(),
		ProjectID:  project.UID,
		EndpointID: endpoint.UID,
		HostName:   "",
		Status:     datastore.DeviceStatusOnline,
		LastSeenAt: time.Now(),
	}
}
