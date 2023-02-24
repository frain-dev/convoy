//go:build integration
// +build integration

package mongo

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/util"
	"github.com/stretchr/testify/require"
)

func Test_CreateDevice(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	store := getStore(db)

	deviceRepo := NewDeviceRepository(store)
	device := &datastore.Device{
		UID:        uuid.NewString(),
		ProjectID:  uuid.NewString(),
		EndpointID: uuid.NewString(),
		HostName:   "",
		Status:     datastore.DeviceStatusOnline,
	}

	require.NoError(t, deviceRepo.CreateDevice(context.Background(), device))

	d, err := deviceRepo.FetchDeviceByID(context.Background(), device.UID, device.EndpointID, device.ProjectID)
	require.NoError(t, err)

	require.Equal(t, device.UID, d.UID)
	require.Equal(t, device.EndpointID, d.EndpointID)
	require.Equal(t, device.ProjectID, d.ProjectID)
}

func Test_UpdateDevice(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	store := getStore(db)

	deviceRepo := NewDeviceRepository(store)
	device := &datastore.Device{
		UID:        uuid.NewString(),
		ProjectID:  uuid.NewString(),
		EndpointID: uuid.NewString(),
		HostName:   "",
		Status:     datastore.DeviceStatusOnline,
	}

	require.NoError(t, deviceRepo.CreateDevice(context.Background(), device))

	device.Status = datastore.DeviceStatusOffline
	err := deviceRepo.UpdateDevice(context.Background(), device, device.EndpointID, device.ProjectID)
	require.NoError(t, err)

	d, err := deviceRepo.FetchDeviceByID(context.Background(), device.UID, device.EndpointID, device.ProjectID)
	require.NoError(t, err)

	require.Equal(t, device.UID, d.UID)
	require.Equal(t, device.EndpointID, d.EndpointID)
	require.Equal(t, device.ProjectID, d.ProjectID)
	require.Equal(t, datastore.DeviceStatusOffline, d.Status)
}

func Test_UpdateDeviceLastSeen(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	store := getStore(db)

	deviceRepo := NewDeviceRepository(store)
	device := &datastore.Device{
		UID:        uuid.NewString(),
		ProjectID:  uuid.NewString(),
		EndpointID: uuid.NewString(),
		HostName:   "",
		Status:     datastore.DeviceStatusOnline,
	}

	require.NoError(t, deviceRepo.CreateDevice(context.Background(), device))

	err := deviceRepo.UpdateDeviceLastSeen(context.Background(), device, device.EndpointID, device.ProjectID, datastore.DeviceStatusOffline)
	require.NoError(t, err)

	d, err := deviceRepo.FetchDeviceByID(context.Background(), device.UID, device.EndpointID, device.ProjectID)
	require.NoError(t, err)

	require.Equal(t, device.UID, d.UID)
	require.Equal(t, device.EndpointID, d.EndpointID)
	require.Equal(t, device.ProjectID, d.ProjectID)
	require.Equal(t, device.LastSeenAt, d.LastSeenAt)
	require.Equal(t, datastore.DeviceStatusOffline, d.Status)
}

func Test_DeleteDevice(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	store := getStore(db)

	deviceRepo := NewDeviceRepository(store)
	device := &datastore.Device{
		UID:        uuid.NewString(),
		ProjectID:  uuid.NewString(),
		EndpointID: uuid.NewString(),
		HostName:   "",
		Status:     datastore.DeviceStatusOnline,
	}

	require.NoError(t, deviceRepo.CreateDevice(context.Background(), device))

	err := deviceRepo.DeleteDevice(context.Background(), device.UID, device.EndpointID, device.ProjectID)
	require.NoError(t, err)

	_, err = deviceRepo.FetchDeviceByID(context.Background(), device.UID, device.EndpointID, device.ProjectID)
	require.Equal(t, datastore.ErrDeviceNotFound, err)
}

func Test_FetchDeviceByID(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	store := getStore(db)

	deviceRepo := NewDeviceRepository(store)
	device := &datastore.Device{
		UID:        uuid.NewString(),
		ProjectID:  uuid.NewString(),
		EndpointID: uuid.NewString(),
		HostName:   "",
		Status:     datastore.DeviceStatusOnline,
	}

	require.NoError(t, deviceRepo.CreateDevice(context.Background(), device))

	d, err := deviceRepo.FetchDeviceByID(context.Background(), device.UID, device.EndpointID, device.ProjectID)
	require.NoError(t, err)
	require.Equal(t, device, d)
}

func Test_LoadDevicesPaged(t *testing.T) {
	type Expected struct {
		paginationData datastore.PaginationData
	}

	tests := []struct {
		name      string
		pageData  datastore.Pageable
		count     int
		projectID string
		filter    *datastore.ApiKeyFilter
		expected  Expected
	}{
		{
			name:      "Load Devices Paged - 10 records",
			pageData:  datastore.Pageable{Page: 1, PerPage: 3, Sort: -1},
			count:     10,
			projectID: uuid.NewString(),
			filter:    &datastore.ApiKeyFilter{EndpointID: ""},
			expected: Expected{
				paginationData: datastore.PaginationData{
					Total:     10,
					TotalPage: 4,
					Page:      1,
					PerPage:   3,
					Prev:      0,
					Next:      2,
				},
			},
		},

		{
			name:      "Load Devices Paged - 12 records",
			pageData:  datastore.Pageable{Page: 2, PerPage: 4, Sort: -1},
			count:     12,
			projectID: uuid.NewString(),
			filter:    &datastore.ApiKeyFilter{EndpointID: ""},
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
			name:      "Load Devices Paged - 5 records",
			pageData:  datastore.Pageable{Page: 1, PerPage: 3, Sort: -1},
			count:     5,
			projectID: uuid.NewString(),
			filter:    &datastore.ApiKeyFilter{EndpointID: ""},
			expected: Expected{
				paginationData: datastore.PaginationData{
					Total:     5,
					TotalPage: 2,
					Page:      1,
					PerPage:   3,
					Prev:      0,
					Next:      2,
				},
			},
		},

		{
			name:      "Load Devices Paged - 1 record",
			pageData:  datastore.Pageable{Page: 1, PerPage: 3, Sort: -1},
			count:     1,
			projectID: uuid.NewString(),
			filter:    &datastore.ApiKeyFilter{EndpointID: uuid.NewString()},
			expected: Expected{
				paginationData: datastore.PaginationData{
					Total:     1,
					TotalPage: 1,
					Page:      1,
					PerPage:   3,
					Prev:      0,
					Next:      0,
				},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			db, closeFn := getDB(t)
			defer closeFn()

			store := getStore(db)
			deviceRepo := NewDeviceRepository(store)

			for i := 0; i < tc.count; i++ {
				device := &datastore.Device{
					UID:        uuid.NewString(),
					ProjectID:  tc.projectID,
					EndpointID: uuid.NewString(),
					HostName:   "",
					Status:     datastore.DeviceStatusOnline,
				}

				if !util.IsStringEmpty(tc.filter.EndpointID) {
					device.EndpointID = tc.filter.EndpointID
				}

				require.NoError(t, deviceRepo.CreateDevice(context.Background(), device))
			}

			_, pageable, err := deviceRepo.LoadDevicesPaged(context.Background(), tc.projectID, tc.filter, tc.pageData)
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
