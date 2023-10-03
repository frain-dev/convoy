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
	"github.com/stretchr/testify/require"
)

func Test_CreateDevice(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	deviceRepo := NewDeviceRepo(db, nil)
	device := generateDevice(t, db)

	require.NoError(t, deviceRepo.CreateDevice(context.Background(), device))

	newDevice, err := deviceRepo.FetchDeviceByID(context.Background(), device.UID, device.EndpointID, device.ProjectID)
	require.NoError(t, err)

	require.InDelta(t, device.LastSeenAt.Unix(), newDevice.LastSeenAt.Unix(), float64(time.Hour))
	newDevice.CreatedAt, newDevice.UpdatedAt = time.Time{}, time.Time{}
	device.LastSeenAt, newDevice.LastSeenAt = time.Time{}, time.Time{}

	require.Equal(t, device, newDevice)
}

func Test_UpdateDevice(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	deviceRepo := NewDeviceRepo(db, nil)
	device := generateDevice(t, db)

	require.NoError(t, deviceRepo.CreateDevice(context.Background(), device))

	device.Status = datastore.DeviceStatusOffline
	err := deviceRepo.UpdateDevice(context.Background(), device, device.EndpointID, device.ProjectID)
	require.NoError(t, err)

	newDevice, err := deviceRepo.FetchDeviceByID(context.Background(), device.UID, device.EndpointID, device.ProjectID)
	require.NoError(t, err)

	require.InDelta(t, device.LastSeenAt.Unix(), newDevice.LastSeenAt.Unix(), float64(time.Hour))
	newDevice.CreatedAt, newDevice.UpdatedAt = time.Time{}, time.Time{}
	device.LastSeenAt, newDevice.LastSeenAt = time.Time{}, time.Time{}

	require.Equal(t, device, newDevice)
}

func Test_UpdateDeviceLastSeen(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	deviceRepo := NewDeviceRepo(db, nil)
	device := generateDevice(t, db)

	require.NoError(t, deviceRepo.CreateDevice(context.Background(), device))

	device.Status = datastore.DeviceStatusOffline
	err := deviceRepo.UpdateDeviceLastSeen(context.Background(), device, device.EndpointID, device.ProjectID, datastore.DeviceStatusOffline)
	require.NoError(t, err)

	newDevice, err := deviceRepo.FetchDeviceByID(context.Background(), device.UID, device.EndpointID, device.ProjectID)
	require.NoError(t, err)

	require.InDelta(t, device.LastSeenAt.Unix(), newDevice.LastSeenAt.Unix(), float64(time.Hour))
	newDevice.CreatedAt, newDevice.UpdatedAt = time.Time{}, time.Time{}
	device.LastSeenAt, newDevice.LastSeenAt = time.Time{}, time.Time{}

	require.Equal(t, device, newDevice)
}

func Test_DeleteDevice(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	deviceRepo := NewDeviceRepo(db, nil)
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

	deviceRepo := NewDeviceRepo(db, nil)
	device := generateDevice(t, db)

	require.NoError(t, deviceRepo.CreateDevice(context.Background(), device))

	newDevice, err := deviceRepo.FetchDeviceByID(context.Background(), device.UID, device.EndpointID, device.ProjectID)
	require.NoError(t, err)

	require.InDelta(t, device.LastSeenAt.Unix(), newDevice.LastSeenAt.Unix(), float64(time.Hour))
	newDevice.CreatedAt, newDevice.UpdatedAt = time.Time{}, time.Time{}
	device.LastSeenAt, newDevice.LastSeenAt = time.Time{}, time.Time{}

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
		expected Expected
	}{
		{
			name:     "Load Devices Paged - 10 records",
			pageData: datastore.Pageable{PerPage: 3},
			count:    10,
			expected: Expected{
				paginationData: datastore.PaginationData{
					PerPage: 3,
				},
			},
		},

		{
			name:     "Load Devices Paged - 12 records",
			pageData: datastore.Pageable{PerPage: 4},
			count:    12,
			expected: Expected{
				paginationData: datastore.PaginationData{
					PerPage: 4,
				},
			},
		},

		{
			name:     "Load Devices Paged - 5 records",
			pageData: datastore.Pageable{PerPage: 3},
			count:    5,
			expected: Expected{
				paginationData: datastore.PaginationData{
					PerPage: 3,
				},
			},
		},

		{
			name:     "Load Devices Paged - 1 record",
			pageData: datastore.Pageable{PerPage: 3},
			count:    1,
			expected: Expected{
				paginationData: datastore.PaginationData{
					PerPage: 3,
				},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			db, closeFn := getDB(t)
			defer closeFn()

			deviceRepo := NewDeviceRepo(db, nil)
			project := seedProject(t, db)

			for i := 0; i < tc.count; i++ {
				device := &datastore.Device{
					UID:        ulid.Make().String(),
					ProjectID:  project.UID,
					HostName:   "",
					Status:     datastore.DeviceStatusOnline,
					LastSeenAt: time.Now(),
				}

				require.NoError(t, deviceRepo.CreateDevice(context.Background(), device))
			}

			_, pageable, err := deviceRepo.LoadDevicesPaged(context.Background(), project.UID, &datastore.ApiKeyFilter{}, tc.pageData)
			require.NoError(t, err)

			require.Equal(t, tc.expected.paginationData.PerPage, pageable.PerPage)
		})
	}
}

func generateDevice(t *testing.T, db database.Database) *datastore.Device {
	project := seedProject(t, db)

	return &datastore.Device{
		UID:        ulid.Make().String(),
		ProjectID:  project.UID,
		HostName:   "",
		Status:     datastore.DeviceStatusOnline,
		LastSeenAt: time.Now(),
	}
}
