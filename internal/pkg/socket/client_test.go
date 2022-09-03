package socket

import (
	"testing"
	"time"

	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/mocks"
	"github.com/frain-dev/convoy/pkg/httpheader"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func TestGoOffline(t *testing.T) {
	ctrl := gomock.NewController(t)
	conn := mocks.NewMockWebSocketConnection(ctrl)
	r := provideRepo(ctrl)

	c := &Client{
		conn:              conn,
		deviceID:          "1234",
		deviceRepo:        r.DeviceRepo,
		EventTypes:        []string{"*"},
		eventDeliveryRepo: r.EventDeliveryRepo,
		Device: &datastore.Device{
			UID:    "1234",
			Status: datastore.DeviceStatusOnline,
		},
	}

	dev := r.DeviceRepo.(*mocks.MockDeviceRepository)
	dev.EXPECT().UpdateDevice(gomock.Any(), c.Device, gomock.Any(), gomock.Any()).
		Return(nil)

	c.GoOffline()

	require.Equal(t, false, c.IsOnline())
	require.Equal(t, datastore.DeviceStatusOffline, c.Device.Status)
}

func TestIsOnline(t *testing.T) {
	ctrl := gomock.NewController(t)
	conn := mocks.NewMockWebSocketConnection(ctrl)
	r := provideRepo(ctrl)

	c := &Client{
		conn:              conn,
		deviceID:          "1234",
		deviceRepo:        r.DeviceRepo,
		EventTypes:        []string{"*"},
		eventDeliveryRepo: r.EventDeliveryRepo,
		Device: &datastore.Device{
			UID:        "1234",
			Status:     datastore.DeviceStatusOnline,
			LastSeenAt: primitive.DateTime(time.Now().UnixMilli()),
		},
	}

	require.Equal(t, true, c.IsOnline())

	c.Device.LastSeenAt = primitive.DateTime(time.Now().Add(-time.Minute).UnixMilli())

	require.Equal(t, false, c.IsOnline())
}

func TestUpdateEventDeliveryStatus(t *testing.T) {
	ctrl := gomock.NewController(t)
	conn := mocks.NewMockWebSocketConnection(ctrl)
	r := provideRepo(ctrl)

	c := &Client{
		conn:              conn,
		deviceID:          "1234",
		deviceRepo:        r.DeviceRepo,
		EventTypes:        []string{"*"},
		eventDeliveryRepo: r.EventDeliveryRepo,
		Device: &datastore.Device{
			UID:        "1234",
			Status:     datastore.DeviceStatusOnline,
			LastSeenAt: primitive.DateTime(time.Now().UnixMilli()),
		},
	}

	evd := r.EventDeliveryRepo.(*mocks.MockEventDeliveryRepository)
	evd.EXPECT().FindEventDeliveryByID(gomock.Any(), gomock.Any()).
		Return(&datastore.EventDelivery{UID: "ed-id"}, nil)

	evd.EXPECT().UpdateStatusOfEventDelivery(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(nil)

	c.UpdateEventDeliveryStatus(c.deviceID)
}

func TestResendEventDeliveries(t *testing.T) {
	ctrl := gomock.NewController(t)
	conn := mocks.NewMockWebSocketConnection(ctrl)
	r := provideRepo(ctrl)
	evts := make(chan *CLIEvent, 1)
	defer close(evts)

	c := &Client{
		conn:              conn,
		deviceID:          "1234",
		deviceRepo:        r.DeviceRepo,
		EventTypes:        []string{"*"},
		eventDeliveryRepo: r.EventDeliveryRepo,
		Device: &datastore.Device{
			UID:        "1234",
			Status:     datastore.DeviceStatusOnline,
			LastSeenAt: primitive.DateTime(time.Now().UnixMilli()),
		},
	}

	wantEd := datastore.EventDelivery{
		UID:      "evd-1",
		AppID:    "app-1",
		GroupID:  "group-1",
		DeviceID: "device-1",
		Headers: httpheader.HTTPHeader{
			"key": []string{"value"},
		},
		Metadata: &datastore.Metadata{
			Data: []byte("data"),
		},
		CLIMetadata: &datastore.CLIMetadata{
			EventType: "*",
		},
	}

	evd := r.EventDeliveryRepo.(*mocks.MockEventDeliveryRepository)
	evd.EXPECT().FindDiscardedEventDeliveries(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return([]datastore.EventDelivery{wantEd}, nil)

	c.ResendEventDeliveries(time.Now(), evts)

	ev := <-evts
	require.Equal(t, wantEd.UID, ev.UID)
	require.Equal(t, wantEd.Metadata.Data, ev.Data)
}

func TestClose(t *testing.T) {
	ctrl := gomock.NewController(t)
	conn := mocks.NewMockWebSocketConnection(ctrl)
	unregister := make(chan *Client, 1)
	defer close(unregister)

	c := &Client{conn: conn, deviceID: "124"}

	conn.EXPECT().Close()

	c.Close(unregister)

	client := <-unregister
	require.Equal(t, c.deviceID, client.deviceID)
}

func TestPingHandler_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	conn := mocks.NewMockWebSocketConnection(ctrl)
	r := provideRepo(ctrl)

	c := &Client{
		conn:              conn,
		deviceID:          "1234",
		deviceRepo:        r.DeviceRepo,
		EventTypes:        []string{"*"},
		eventDeliveryRepo: r.EventDeliveryRepo,
		Device: &datastore.Device{
			UID:        "1234",
			Status:     datastore.DeviceStatusOnline,
			LastSeenAt: primitive.DateTime(time.Now().Add(-time.Minute).UnixMilli()),
		},
	}

	conn.EXPECT().WriteMessage(gomock.Any(), gomock.Any()).Return(nil)

	dev := r.DeviceRepo.(*mocks.MockDeviceRepository)
	dev.EXPECT().UpdateDeviceLastSeen(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(nil)

	err := c.pingHandler("")
	require.NoError(t, err)
}

func TestPingHandler_FailedToUpdateDevice(t *testing.T) {
	ctrl := gomock.NewController(t)
	conn := mocks.NewMockWebSocketConnection(ctrl)
	r := provideRepo(ctrl)

	c := &Client{
		conn:              conn,
		deviceID:          "1234",
		deviceRepo:        r.DeviceRepo,
		EventTypes:        []string{"*"},
		eventDeliveryRepo: r.EventDeliveryRepo,
		Device: &datastore.Device{
			UID:        "1234",
			Status:     datastore.DeviceStatusOnline,
			LastSeenAt: primitive.DateTime(time.Now().Add(-time.Minute).UnixMilli()),
		},
	}

	// conn.EXPECT().WriteMessage(gomock.Any(), gomock.Any()).Return(nil)

	dev := r.DeviceRepo.(*mocks.MockDeviceRepository)
	dev.EXPECT().UpdateDeviceLastSeen(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(ErrFailedToUpdateDevice)

	err := c.pingHandler("")
	require.Error(t, ErrFailedToUpdateDevice, err)
}

func TestPingHandler_FailedToSendPongMessage(t *testing.T) {
	ctrl := gomock.NewController(t)
	conn := mocks.NewMockWebSocketConnection(ctrl)
	r := provideRepo(ctrl)

	c := &Client{
		conn:              conn,
		deviceID:          "1234",
		deviceRepo:        r.DeviceRepo,
		EventTypes:        []string{"*"},
		eventDeliveryRepo: r.EventDeliveryRepo,
		Device: &datastore.Device{
			UID:        "1234",
			Status:     datastore.DeviceStatusOnline,
			LastSeenAt: primitive.DateTime(time.Now().Add(-time.Minute).UnixMilli()),
		},
	}

	conn.EXPECT().WriteMessage(gomock.Any(), gomock.Any()).
		Return(ErrFailedToSendPongMessage)

	dev := r.DeviceRepo.(*mocks.MockDeviceRepository)
	dev.EXPECT().UpdateDeviceLastSeen(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(nil)

	err := c.pingHandler("")
	require.Error(t, ErrFailedToSendPongMessage, err)
}
