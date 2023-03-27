package socket

import (
	"errors"
	"testing"
	"time"

	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/mocks"
	"github.com/frain-dev/convoy/pkg/httpheader"
	"github.com/golang/mock/gomock"
	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/require"
)

func provideClient(r *Repo, c WebSocketConnection) *Client {
	return &Client{
		conn:       c,
		deviceID:   "1234",
		deviceRepo: r.DeviceRepo,
		// EventTypes:        []string{"*"},
		eventDeliveryRepo: r.EventDeliveryRepo,
		Device: &datastore.Device{
			UID:        "1234",
			Status:     datastore.DeviceStatusOnline,
			LastSeenAt: time.Now(),
		},
	}
}

func TestGoOffline(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	conn := mocks.NewMockWebSocketConnection(ctrl)
	r := provideRepo(ctrl)

	c := provideClient(r, conn)
	c.Device.Status = datastore.DeviceStatusOnline

	dev := r.DeviceRepo.(*mocks.MockDeviceRepository)
	dev.EXPECT().UpdateDevice(gomock.Any(), c.Device, gomock.Any(), gomock.Any()).
		Return(nil)

	c.GoOffline()

	require.Equal(t, datastore.DeviceStatusOffline, c.Device.Status)
}

func TestIsOnline(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	conn := mocks.NewMockWebSocketConnection(ctrl)
	r := provideRepo(ctrl)

	c := provideClient(r, conn)

	require.Equal(t, true, c.IsOnline())

	c.Device.LastSeenAt = time.Now().Add(-time.Minute)

	require.Equal(t, false, c.IsOnline())
}

func TestUpdateEventDeliveryStatus(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	conn := mocks.NewMockWebSocketConnection(ctrl)
	r := provideRepo(ctrl)

	c := provideClient(r, conn)

	evd := r.EventDeliveryRepo.(*mocks.MockEventDeliveryRepository)
	evd.EXPECT().FindEventDeliveryByID(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(&datastore.EventDelivery{UID: "ed-id"}, nil)

	evd.EXPECT().UpdateStatusOfEventDelivery(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(nil)

	c.UpdateEventDeliveryStatus(c.deviceID, c.Device.ProjectID)
}

func TestResendEventDeliveries(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	conn := mocks.NewMockWebSocketConnection(ctrl)
	r := provideRepo(ctrl)
	evts := make(chan *CLIEvent, 1)
	defer close(evts)

	c := provideClient(r, conn)

	wantEd := datastore.EventDelivery{
		UID:        "evd-1",
		EndpointID: "endpoint-1",
		ProjectID:  "project-1",
		DeviceID:   "device-1",
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
	defer ctrl.Finish()

	r := provideRepo(ctrl)

	conn := mocks.NewMockWebSocketConnection(ctrl)
	unreg := make(chan *Client, 1)
	defer close(unreg)

	c := provideClient(r, conn)

	conn.EXPECT().Close()

	c.Close(unreg)

	client := <-unreg
	require.Equal(t, c.deviceID, client.deviceID)
}

func TestPingHandler_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	conn := mocks.NewMockWebSocketConnection(ctrl)
	r := provideRepo(ctrl)

	c := provideClient(r, conn)
	c.Device.LastSeenAt = time.Now().Add(-time.Minute)

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
		conn:       conn,
		deviceID:   "1234",
		deviceRepo: r.DeviceRepo,
		// EventTypes:        []string{"*"},
		eventDeliveryRepo: r.EventDeliveryRepo,
		Device: &datastore.Device{
			UID:        "1234",
			Status:     datastore.DeviceStatusOnline,
			LastSeenAt: time.Now().Add(-time.Minute),
		},
	}

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
		conn:       conn,
		deviceID:   "1234",
		deviceRepo: r.DeviceRepo,
		// EventTypes:        []string{"*"},
		eventDeliveryRepo: r.EventDeliveryRepo,
		Device: &datastore.Device{
			UID:        "1234",
			Status:     datastore.DeviceStatusOnline,
			LastSeenAt: time.Now().Add(-time.Minute),
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

func TestProcessMessage(t *testing.T) {
	type Args struct {
		messageType int
		message     []byte
		err         error
		unreg       chan *Client
	}

	tests := []struct {
		name string
		args Args
		dbFn func(r *Repo, c WebSocketConnection)
	}{
		{
			name: "should go offline",
			args: Args{
				messageType: -1,
				message:     []byte(""),
				err:         nil,
				unreg:       nil,
			},
			dbFn: func(r *Repo, c WebSocketConnection) {
				dev := r.DeviceRepo.(*mocks.MockDeviceRepository)
				dev.EXPECT().UpdateDevice(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Return(nil)
			},
		},
		{
			name: "should close client",
			args: Args{
				messageType: websocket.CloseMessage,
				message:     []byte(""),
				err:         nil,
				unreg:       make(chan *Client, 1),
			},
			dbFn: func(r *Repo, c WebSocketConnection) {
				conn := c.(*mocks.MockWebSocketConnection)
				conn.EXPECT().Close()
			},
		},
		{
			name: "should disconnect client and go offline",
			args: Args{
				messageType: websocket.TextMessage,
				message:     []byte("disconnect"),
				err:         nil,
				unreg:       nil,
			},
			dbFn: func(r *Repo, c WebSocketConnection) {
				dev := r.DeviceRepo.(*mocks.MockDeviceRepository)
				dev.EXPECT().UpdateDevice(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Return(nil)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			r := provideRepo(ctrl)
			c := mocks.NewMockWebSocketConnection(ctrl)
			tt.dbFn(r, c)

			client := provideClient(r, c)

			client.processMessage(tt.args.messageType, tt.args.message, tt.args.err, tt.args.unreg)
		})
	}
}

func TestParseTime(t *testing.T) {
	type Args struct {
		message  string
		err      error
		wantErr  bool
		wantTime string
	}

	tests := []struct {
		name string
		args Args
	}{
		{
			name: "should request for discarded events with duration",
			args: Args{
				message:  "since|duration|2m",
				wantTime: time.Now().Add(-time.Minute * 2).Format(time.Stamp),
			},
		},
		{
			name: "should error, when requesting for discarded events with duration",
			args: Args{
				message: "since|duration|2",
				wantErr: true,
				err:     errors.New(`"parsing time \"2022-08-01\" as \"2006-01-02T15:04:05Z07:00\": cannot parse \"\" as \"T\""`),
			},
		},
		{
			name: "should request for discarded events with timestamp",
			args: Args{
				message:  "since|timestamp|2022-08-01T00:00:00Z",
				wantTime: "Aug  1 00:00:00",
			},
		},
		{
			name: "should error, when requesting for discarded events with timestamp",
			args: Args{
				message: "since|timestamp|2022-08-01",
				wantErr: true,
				err:     errors.New(`parsing time "2022-08-01" as "2006-01-02T15:04:05Z07:00": cannot parse "" as "T"`),
			},
		},
		{
			name: "should error, malformatted since value passed",
			args: Args{
				message: "since|yay|2022-08-01",
				wantErr: true,
				err:     errors.New("will ignore 'since' value"),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			r := provideRepo(ctrl)
			c := mocks.NewMockWebSocketConnection(ctrl)

			client := provideClient(r, c)

			since, err := client.parseTime(tt.args.message)
			t.Log(since)

			if tt.args.wantErr {
				require.Error(t, tt.args.err, err)
				return
			}

			require.NoError(t, err)
			require.Equal(t, tt.args.wantTime, since.Format(time.Stamp))
		})
	}
}
