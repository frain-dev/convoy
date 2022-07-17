//go:build integration
// +build integration

package mongo

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/frain-dev/convoy/datastore"
	"github.com/stretchr/testify/require"
)

func Test_CreateDevice(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	deviceRepo := NewDeviceRepository(db, datastore.New(db, DeviceCollection))
	device := &datastore.Device{
		UID:            uuid.NewString(),
		GroupID:        uuid.NewString(),
		AppID:          uuid.NewString(),
		HostName:       "",
		Status:         datastore.DeviceStatusOnline,
		DocumentStatus: datastore.ActiveDocumentStatus,
		LastSeenAt:     primitive.NewDateTimeFromTime(time.Now()),
		CreatedAt:      primitive.NewDateTimeFromTime(time.Now()),
		UpdatedAt:      primitive.NewDateTimeFromTime(time.Now()),
	}
	require.NoError(t, deviceRepo.CreateDevice(context.Background(), device))

	d, err := deviceRepo.FetchDeviceByID(context.Background(), device.UID, device.AppID, device.GroupID)
	require.NoError(t, err)

	require.Equal(t, device.UID, d.UID)
	require.Equal(t, device.AppID, d.AppID)
	require.Equal(t, device.GroupID, d.GroupID)
}

func Test_UpdateDevice(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	deviceRepo := NewDeviceRepository(db, datastore.New(db, DeviceCollection))
	device := &datastore.Device{
		UID:            uuid.NewString(),
		GroupID:        uuid.NewString(),
		AppID:          uuid.NewString(),
		HostName:       "",
		Status:         datastore.DeviceStatusOnline,
		DocumentStatus: datastore.ActiveDocumentStatus,
		LastSeenAt:     primitive.NewDateTimeFromTime(time.Now()),
		CreatedAt:      primitive.NewDateTimeFromTime(time.Now()),
		UpdatedAt:      primitive.NewDateTimeFromTime(time.Now()),
	}
	require.NoError(t, deviceRepo.CreateDevice(context.Background(), device))

	device.Status = datastore.DeviceStatusOffline
	err := deviceRepo.UpdateDevice(context.Background(), device, device.AppID, device.GroupID)
	require.NoError(t, err)

	d, err := deviceRepo.FetchDeviceByID(context.Background(), device.UID, device.AppID, device.GroupID)
	require.NoError(t, err)

	require.Equal(t, device.UID, d.UID)
	require.Equal(t, device.AppID, d.AppID)
	require.Equal(t, device.GroupID, d.GroupID)
	require.Equal(t, datastore.DeviceStatusOffline, d.Status)
}

func Test_UpdateDeviceLastSeen(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	deviceRepo := NewDeviceRepository(db, datastore.New(db, DeviceCollection))
	device := &datastore.Device{
		UID:            uuid.NewString(),
		GroupID:        uuid.NewString(),
		AppID:          uuid.NewString(),
		HostName:       "",
		Status:         datastore.DeviceStatusOnline,
		DocumentStatus: datastore.ActiveDocumentStatus,
		LastSeenAt:     primitive.NewDateTimeFromTime(time.Now()),
		CreatedAt:      primitive.NewDateTimeFromTime(time.Now()),
		UpdatedAt:      primitive.NewDateTimeFromTime(time.Now()),
	}
	require.NoError(t, deviceRepo.CreateDevice(context.Background(), device))

	device.Status = datastore.DeviceStatusOffline
	err := deviceRepo.UpdateDeviceLastSeen(context.Background(), device, device.AppID, device.GroupID)
	require.NoError(t, err)

	d, err := deviceRepo.FetchDeviceByID(context.Background(), device.UID, device.AppID, device.GroupID)
	require.NoError(t, err)

	require.Equal(t, device.UID, d.UID)
	require.Equal(t, device.AppID, d.AppID)
	require.Equal(t, device.GroupID, d.GroupID)
	require.Equal(t, device.LastSeenAt, d.LastSeenAt)
	require.Equal(t, datastore.DeviceStatusOffline, d.Status)
}

func Test_DeleteDevice(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	deviceRepo := NewDeviceRepository(db, datastore.New(db, DeviceCollection))
	device := &datastore.Device{
		UID:            uuid.NewString(),
		GroupID:        uuid.NewString(),
		AppID:          uuid.NewString(),
		HostName:       "",
		Status:         datastore.DeviceStatusOnline,
		DocumentStatus: datastore.ActiveDocumentStatus,
		LastSeenAt:     primitive.NewDateTimeFromTime(time.Now()),
		CreatedAt:      primitive.NewDateTimeFromTime(time.Now()),
		UpdatedAt:      primitive.NewDateTimeFromTime(time.Now()),
	}
	require.NoError(t, deviceRepo.CreateDevice(context.Background(), device))

	err := deviceRepo.DeleteDevice(context.Background(), device.UID, device.AppID, device.GroupID)
	require.NoError(t, err)

	_, err = deviceRepo.FetchDeviceByID(context.Background(), device.UID, device.AppID, device.GroupID)
	require.Equal(t, datastore.ErrDeviceNotFound, err)
}

func Test_FetchDeviceByID(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	deviceRepo := NewDeviceRepository(db, datastore.New(db, DeviceCollection))
	device := &datastore.Device{
		UID:            uuid.NewString(),
		GroupID:        uuid.NewString(),
		AppID:          uuid.NewString(),
		HostName:       "",
		Status:         datastore.DeviceStatusOnline,
		DocumentStatus: datastore.ActiveDocumentStatus,
		LastSeenAt:     primitive.NewDateTimeFromTime(time.Now()),
		CreatedAt:      primitive.NewDateTimeFromTime(time.Now()),
		UpdatedAt:      primitive.NewDateTimeFromTime(time.Now()),
	}
	require.NoError(t, deviceRepo.CreateDevice(context.Background(), device))

	d, err := deviceRepo.FetchDeviceByID(context.Background(), device.UID, device.AppID, device.GroupID)
	require.NoError(t, err)
	require.Equal(t, device, d)
}
