package services

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/frain-dev/convoy/mocks"
	"github.com/frain-dev/convoy/util"
	"github.com/golang/mock/gomock"
	"go.mongodb.org/mongo-driver/mongo"

	"github.com/frain-dev/convoy/datastore"
)

func provideHub(ctrl *gomock.Controller) *Hub {
	appRepo := mocks.NewMockApplicationRepository(ctrl)
	subRepo := mocks.NewMockSubscriptionRepository(ctrl)
	sourceRepo := mocks.NewMockSourceRepository(ctrl)
	deviceRepo := mocks.NewMockDeviceRepository(ctrl)
	fn := func(fn func(doc map[string]interface{}) error, pipeline mongo.Pipeline, collection string, stop chan struct{}) error {
		return nil
	}

	return NewHub(deviceRepo, subRepo, sourceRepo, appRepo, fn)
}

func TestHub_listen(t *testing.T) {
	ctx := context.Background()
	lastSeen := primitive.NewDateTimeFromTime(time.Now().Add(-time.Minute))
	type args struct {
		ctx           context.Context
		group         *datastore.Group
		app           *datastore.Application
		listenRequest *ListenRequest
	}
	tests := []struct {
		name        string
		args        args
		dbFn        func(h *Hub)
		want        *datastore.Device
		wantErr     bool
		wantErrCode int
		wantErrMsg  string
	}{
		{
			name: "should_listen_successfully",
			args: args{
				ctx:   ctx,
				group: &datastore.Group{UID: "1234"},
				app:   &datastore.Application{UID: "abc"},
				listenRequest: &ListenRequest{
					HostName:   "",
					DeviceID:   "device-id",
					SourceID:   "source-id",
					EventTypes: []string{"charge.success"},
				},
			},
			dbFn: func(h *Hub) {
				d := h.deviceRepo.(*mocks.MockDeviceRepository)
				d.EXPECT().FetchDeviceByID(gomock.Any(), "device-id", "abc", "1234").Times(1).Return(
					&datastore.Device{
						UID:            "device-id",
						GroupID:        "1234",
						AppID:          "abc",
						HostName:       "",
						Status:         datastore.DeviceStatusOnline,
						DocumentStatus: datastore.ActiveDocumentStatus,
						LastSeenAt:     lastSeen,
					},
					nil,
				)

				s, _ := h.sourceRepo.(*mocks.MockSourceRepository)
				s.EXPECT().FindSourceByID(gomock.Any(), gomock.Any(), "source-id").Times(1).Return(
					&datastore.Source{UID: "1234", GroupID: "1234"},
					nil,
				)

				sub, _ := h.subscriptionRepo.(*mocks.MockSubscriptionRepository)
				sub.EXPECT().FindSubscriptionByDeviceID(gomock.Any(), "1234", "device-id", "source-id").
					Times(1).Return(&datastore.Subscription{}, nil)

			},
			want: &datastore.Device{
				UID:            "device-id",
				GroupID:        "1234",
				AppID:          "abc",
				HostName:       "",
				Status:         datastore.DeviceStatusOnline,
				DocumentStatus: datastore.ActiveDocumentStatus,
				LastSeenAt:     lastSeen,
			},
			wantErr: false,
		},
		{
			name: "should_error_for_wrong_device_group_id",
			args: args{
				ctx:   ctx,
				group: &datastore.Group{UID: "1234"},
				app:   &datastore.Application{UID: "abc"},
				listenRequest: &ListenRequest{
					HostName:   "",
					DeviceID:   "device-id",
					SourceID:   "source-id",
					EventTypes: []string{"charge.success"},
				},
			},
			dbFn: func(h *Hub) {
				d := h.deviceRepo.(*mocks.MockDeviceRepository)
				d.EXPECT().FetchDeviceByID(gomock.Any(), "device-id", "abc", "1234").Times(1).Return(
					&datastore.Device{
						UID:            "device-id",
						GroupID:        "2",
						AppID:          "abc",
						HostName:       "",
						Status:         datastore.DeviceStatusOnline,
						DocumentStatus: datastore.ActiveDocumentStatus,
						LastSeenAt:     lastSeen,
					},
					nil,
				)
			},
			wantErr:     true,
			wantErrCode: http.StatusUnauthorized,
			wantErrMsg:  "unauthorized to access device",
		},
		{
			name: "should_error_for_wrong_device_app_id",
			args: args{
				ctx:   ctx,
				group: &datastore.Group{UID: "1234"},
				app:   &datastore.Application{UID: "abc"},
				listenRequest: &ListenRequest{
					HostName:   "",
					DeviceID:   "device-id",
					SourceID:   "source-id",
					EventTypes: []string{"charge.success"},
				},
			},
			dbFn: func(h *Hub) {
				d := h.deviceRepo.(*mocks.MockDeviceRepository)
				d.EXPECT().FetchDeviceByID(gomock.Any(), "device-id", "abc", "1234").Times(1).Return(
					&datastore.Device{
						UID:            "device-id",
						GroupID:        "1234",
						AppID:          "abcd",
						HostName:       "",
						Status:         datastore.DeviceStatusOnline,
						DocumentStatus: datastore.ActiveDocumentStatus,
						LastSeenAt:     lastSeen,
					},
					nil,
				)
			},
			wantErr:     true,
			wantErrCode: http.StatusUnauthorized,
			wantErrMsg:  "unauthorized to access device",
		},
		{
			name: "should_fail_to_find_device",
			args: args{
				ctx:   ctx,
				group: &datastore.Group{UID: "1234"},
				app:   &datastore.Application{UID: "abc"},
				listenRequest: &ListenRequest{
					HostName:   "",
					DeviceID:   "device-id",
					SourceID:   "source-id",
					EventTypes: []string{"charge.success"},
				},
			},
			dbFn: func(h *Hub) {
				d := h.deviceRepo.(*mocks.MockDeviceRepository)
				d.EXPECT().FetchDeviceByID(gomock.Any(), "device-id", "abc", "1234").Times(1).Return(nil, errors.New("failed"))
			},
			wantErr:     true,
			wantErrCode: http.StatusBadRequest,
			wantErrMsg:  "device not found",
		},
		{
			name: "should_listen_successfully",
			args: args{
				ctx:   ctx,
				group: &datastore.Group{UID: "1234"},
				app:   &datastore.Application{UID: "abc"},
				listenRequest: &ListenRequest{
					HostName:   "",
					DeviceID:   "device-id",
					SourceID:   "source-id",
					EventTypes: []string{"charge.success"},
				},
			},
			dbFn: func(h *Hub) {
				d := h.deviceRepo.(*mocks.MockDeviceRepository)
				d.EXPECT().FetchDeviceByID(gomock.Any(), "device-id", "abc", "1234").Times(1).Return(
					&datastore.Device{
						UID:            "device-id",
						GroupID:        "1234",
						AppID:          "abc",
						HostName:       "",
						Status:         datastore.DeviceStatusOnline,
						DocumentStatus: datastore.ActiveDocumentStatus,
						LastSeenAt:     lastSeen,
					},
					nil,
				)

				s, _ := h.sourceRepo.(*mocks.MockSourceRepository)
				s.EXPECT().FindSourceByID(gomock.Any(), gomock.Any(), "source-id").Times(1).Return(nil, errors.New("failed"))
			},
			wantErr:     true,
			wantErrCode: http.StatusBadRequest,
			wantErrMsg:  "failed to find source",
		},
		{
			name: "should_error_for_wrong_source_group_id",
			args: args{
				ctx:   ctx,
				group: &datastore.Group{UID: "1234"},
				app:   &datastore.Application{UID: "abc"},
				listenRequest: &ListenRequest{
					HostName:   "",
					DeviceID:   "device-id",
					SourceID:   "source-id",
					EventTypes: []string{"charge.success"},
				},
			},
			dbFn: func(h *Hub) {
				d := h.deviceRepo.(*mocks.MockDeviceRepository)
				d.EXPECT().FetchDeviceByID(gomock.Any(), "device-id", "abc", "1234").Times(1).Return(
					&datastore.Device{
						UID:            "device-id",
						GroupID:        "1234",
						AppID:          "abc",
						HostName:       "",
						Status:         datastore.DeviceStatusOnline,
						DocumentStatus: datastore.ActiveDocumentStatus,
						LastSeenAt:     lastSeen,
					},
					nil,
				)

				s, _ := h.sourceRepo.(*mocks.MockSourceRepository)
				s.EXPECT().FindSourceByID(gomock.Any(), gomock.Any(), "source-id").Times(1).Return(
					&datastore.Source{UID: "1234", GroupID: "ref"},
					nil,
				)
			},
			wantErr:     true,
			wantErrCode: http.StatusUnauthorized,
			wantErrMsg:  "unauthorized to access source",
		},

		{
			name: "should_fail_to_find_subscription",
			args: args{
				ctx:   ctx,
				group: &datastore.Group{UID: "1234"},
				app:   &datastore.Application{UID: "abc"},
				listenRequest: &ListenRequest{
					HostName:   "",
					DeviceID:   "device-id",
					SourceID:   "source-id",
					EventTypes: []string{"charge.success"},
				},
			},
			dbFn: func(h *Hub) {
				d := h.deviceRepo.(*mocks.MockDeviceRepository)
				d.EXPECT().FetchDeviceByID(gomock.Any(), "device-id", "abc", "1234").Times(1).Return(
					&datastore.Device{
						UID:            "device-id",
						GroupID:        "1234",
						AppID:          "abc",
						HostName:       "",
						Status:         datastore.DeviceStatusOnline,
						DocumentStatus: datastore.ActiveDocumentStatus,
						LastSeenAt:     lastSeen,
					},
					nil,
				)

				s, _ := h.sourceRepo.(*mocks.MockSourceRepository)
				s.EXPECT().FindSourceByID(gomock.Any(), gomock.Any(), "source-id").Times(1).Return(
					&datastore.Source{UID: "1234", GroupID: "1234"},
					nil,
				)

				sub, _ := h.subscriptionRepo.(*mocks.MockSubscriptionRepository)
				sub.EXPECT().FindSubscriptionByDeviceID(gomock.Any(), "1234", "device-id", "source-id").
					Times(1).Return(nil, errors.New("failed"))
			},
			wantErr:     true,
			wantErrCode: http.StatusBadRequest,
			wantErrMsg:  "failed to find subscription by id",
		},
		{
			name: "should_create_new_subscription_and_listen_successfully",
			args: args{
				ctx:   ctx,
				group: &datastore.Group{UID: "1234"},
				app:   &datastore.Application{UID: "abc"},
				listenRequest: &ListenRequest{
					HostName:   "",
					DeviceID:   "device-id",
					SourceID:   "source-id",
					EventTypes: []string{"charge.success"},
				},
			},
			dbFn: func(h *Hub) {
				d := h.deviceRepo.(*mocks.MockDeviceRepository)
				d.EXPECT().FetchDeviceByID(gomock.Any(), "device-id", "abc", "1234").Times(1).Return(
					&datastore.Device{
						UID:            "device-id",
						GroupID:        "1234",
						AppID:          "abc",
						HostName:       "",
						Status:         datastore.DeviceStatusOnline,
						DocumentStatus: datastore.ActiveDocumentStatus,
					},
					nil,
				)

				s, _ := h.sourceRepo.(*mocks.MockSourceRepository)
				s.EXPECT().FindSourceByID(gomock.Any(), gomock.Any(), "source-id").Times(1).Return(
					&datastore.Source{UID: "1234", GroupID: "1234"},
					nil,
				)

				sub, _ := h.subscriptionRepo.(*mocks.MockSubscriptionRepository)
				sub.EXPECT().FindSubscriptionByDeviceID(gomock.Any(), "1234", "device-id", "source-id").
					Times(1).Return(nil, datastore.ErrSubscriptionNotFound)

				sub.EXPECT().CreateSubscription(gomock.Any(), "1234", gomock.Any()).Times(1).Return(nil)
			},
			want: &datastore.Device{
				UID:            "device-id",
				GroupID:        "1234",
				AppID:          "abc",
				HostName:       "",
				Status:         datastore.DeviceStatusOnline,
				DocumentStatus: datastore.ActiveDocumentStatus,
			},
			wantErr: false,
		},
		{
			name: "should_fail_to_create_new_subscription",
			args: args{
				ctx:   ctx,
				group: &datastore.Group{UID: "1234"},
				app:   &datastore.Application{UID: "abc"},
				listenRequest: &ListenRequest{
					HostName:   "",
					DeviceID:   "device-id",
					SourceID:   "source-id",
					EventTypes: []string{"charge.success"},
				},
			},
			dbFn: func(h *Hub) {
				d := h.deviceRepo.(*mocks.MockDeviceRepository)
				d.EXPECT().FetchDeviceByID(gomock.Any(), "device-id", "abc", "1234").Times(1).Return(
					&datastore.Device{
						UID:            "device-id",
						GroupID:        "1234",
						AppID:          "abc",
						HostName:       "",
						Status:         datastore.DeviceStatusOnline,
						DocumentStatus: datastore.ActiveDocumentStatus,
						LastSeenAt:     lastSeen,
					},
					nil,
				)

				s, _ := h.sourceRepo.(*mocks.MockSourceRepository)
				s.EXPECT().FindSourceByID(gomock.Any(), gomock.Any(), "source-id").Times(1).Return(
					&datastore.Source{UID: "1234", GroupID: "1234"},
					nil,
				)

				sub, _ := h.subscriptionRepo.(*mocks.MockSubscriptionRepository)
				sub.EXPECT().FindSubscriptionByDeviceID(gomock.Any(), "1234", "device-id", "source-id").
					Times(1).Return(nil, datastore.ErrSubscriptionNotFound)

				sub.EXPECT().CreateSubscription(gomock.Any(), "1234", gomock.Any()).Times(1).Return(errors.New("failed"))
			},
			want: &datastore.Device{
				UID:            "device-id",
				GroupID:        "1234",
				AppID:          "abc",
				HostName:       "",
				Status:         datastore.DeviceStatusOnline,
				DocumentStatus: datastore.ActiveDocumentStatus,
				LastSeenAt:     lastSeen,
			},
			wantErr:     true,
			wantErrCode: http.StatusBadRequest,
			wantErrMsg:  "failed to create new subscription",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			h := provideHub(ctrl)

			if tt.dbFn != nil {
				tt.dbFn(h)
			}

			device, err := h.listen(tt.args.ctx, tt.args.group, tt.args.app, tt.args.listenRequest)
			if tt.wantErr {
				require.NotNil(t, err)
				require.Equal(t, tt.wantErrCode, err.(*util.ServiceError).ErrCode())
				require.Equal(t, tt.wantErrMsg, err.(*util.ServiceError).Error())
				return
			}

			require.Nil(t, err)
			require.Equal(t, tt.want, device)
		})
	}
}

func TestHub_login(t *testing.T) {
	ctx := context.Background()

	type args struct {
		ctx          context.Context
		group        *datastore.Group
		app          *datastore.Application
		loginRequest *LoginRequest
	}
	var tests = []struct {
		name        string
		args        args
		dbFn        func(h *Hub)
		want        *datastore.Device
		checkData   bool
		wantErr     bool
		wantErrCode int
		wantErrMsg  string
	}{
		{
			name: "should_create_new_device_and_login_successfully",
			args: args{
				ctx:   ctx,
				group: &datastore.Group{UID: "1234"},
				app:   &datastore.Application{UID: "abc"},
				loginRequest: &LoginRequest{
					HostName: "hostname_1",
					DeviceID: "",
				},
			},
			dbFn: func(h *Hub) {
				d := h.deviceRepo.(*mocks.MockDeviceRepository)
				d.EXPECT().CreateDevice(gomock.Any(), gomock.Any()).Times(1).Return(nil)
			},
			want: &datastore.Device{
				GroupID:  "1234",
				AppID:    "abc",
				HostName: "hostname_1",
			},
			wantErr: false,
		},
		{
			name: "should_fail_to_create_new_device",
			args: args{
				ctx:   ctx,
				group: &datastore.Group{UID: "1234"},
				app:   &datastore.Application{UID: "abc"},
				loginRequest: &LoginRequest{
					HostName: "hostname_1",
					DeviceID: "",
				},
			},
			dbFn: func(h *Hub) {
				d := h.deviceRepo.(*mocks.MockDeviceRepository)
				d.EXPECT().CreateDevice(gomock.Any(), gomock.Any()).Times(1).Return(errors.New("failed"))
			},
			wantErr:     true,
			wantErrCode: http.StatusBadRequest,
			wantErrMsg:  "failed to create new device",
		},
		{
			name: "should_login_with_existing_device_successfully",
			args: args{
				ctx:   ctx,
				group: &datastore.Group{UID: "1234"},
				app:   &datastore.Application{UID: "abc"},
				loginRequest: &LoginRequest{
					HostName: "hostname_1",
					DeviceID: "device-id",
				},
			},
			dbFn: func(h *Hub) {
				d := h.deviceRepo.(*mocks.MockDeviceRepository)
				d.EXPECT().FetchDeviceByID(gomock.Any(), "device-id", "abc", "1234").Times(1).Return(
					&datastore.Device{
						UID:            "device-id",
						GroupID:        "1234",
						AppID:          "abc",
						HostName:       "hostname_1",
						Status:         datastore.DeviceStatusOnline,
						DocumentStatus: datastore.ActiveDocumentStatus,
						LastSeenAt:     primitive.NewDateTimeFromTime(time.Now()),
					},
					nil,
				)
			},
			want: &datastore.Device{
				GroupID:  "1234",
				AppID:    "abc",
				HostName: "hostname_1",
			},
			wantErr: false,
		},
		{
			name: "should_fail_to_find_device",
			args: args{
				ctx:   ctx,
				group: &datastore.Group{UID: "1234"},
				app:   &datastore.Application{UID: "abc"},
				loginRequest: &LoginRequest{
					HostName: "hostname_1",
					DeviceID: "device-id",
				},
			},
			dbFn: func(h *Hub) {
				d := h.deviceRepo.(*mocks.MockDeviceRepository)
				d.EXPECT().FetchDeviceByID(gomock.Any(), "device-id", "abc", "1234").Times(1).Return(nil, errors.New("failed"))
			},
			want: &datastore.Device{
				GroupID:  "1234",
				AppID:    "abc",
				HostName: "hostname_1",
			},
			wantErr:     true,
			wantErrCode: http.StatusBadRequest,
			wantErrMsg:  "failed to find device by id",
		},
		{
			name: "should_error_for_wrong_device_group_id",
			args: args{
				ctx:   ctx,
				group: &datastore.Group{UID: "1234"},
				app:   &datastore.Application{UID: "abc"},
				loginRequest: &LoginRequest{
					HostName: "hostname_1",
					DeviceID: "device-id",
				},
			},
			dbFn: func(h *Hub) {
				d := h.deviceRepo.(*mocks.MockDeviceRepository)
				d.EXPECT().FetchDeviceByID(gomock.Any(), "device-id", "abc", "1234").Times(1).Return(
					&datastore.Device{
						UID:            "device-id",
						GroupID:        "123",
						AppID:          "abc",
						HostName:       "hostname_1",
						Status:         datastore.DeviceStatusOnline,
						DocumentStatus: datastore.ActiveDocumentStatus,
						LastSeenAt:     primitive.NewDateTimeFromTime(time.Now()),
					},
					nil,
				)
			},
			wantErr:     true,
			wantErrCode: http.StatusUnauthorized,
			wantErrMsg:  "unauthorized to access device",
		},
		{
			name: "should_error_for_wrong_device_app_id",
			args: args{
				ctx:   ctx,
				group: &datastore.Group{UID: "1234"},
				app:   &datastore.Application{UID: "abc"},
				loginRequest: &LoginRequest{
					HostName: "hostname_1",
					DeviceID: "device-id",
				},
			},
			dbFn: func(h *Hub) {
				d := h.deviceRepo.(*mocks.MockDeviceRepository)
				d.EXPECT().FetchDeviceByID(gomock.Any(), "device-id", "abc", "1234").Times(1).Return(
					&datastore.Device{
						UID:            "device-id",
						GroupID:        "1234",
						AppID:          "abcd",
						HostName:       "hostname_1",
						Status:         datastore.DeviceStatusOnline,
						DocumentStatus: datastore.ActiveDocumentStatus,
						LastSeenAt:     primitive.NewDateTimeFromTime(time.Now()),
					},
					nil,
				)
			},
			wantErr:     true,
			wantErrCode: http.StatusUnauthorized,
			wantErrMsg:  "unauthorized to access device",
		},
		{
			name: "should_login_with_existing_device_and_update_device_status_successfully",
			args: args{
				ctx:   ctx,
				group: &datastore.Group{UID: "1234"},
				app:   &datastore.Application{UID: "abc"},
				loginRequest: &LoginRequest{
					HostName: "hostname_1",
					DeviceID: "device-id",
				},
			},
			dbFn: func(h *Hub) {
				d := h.deviceRepo.(*mocks.MockDeviceRepository)
				d.EXPECT().FetchDeviceByID(gomock.Any(), "device-id", "abc", "1234").Times(1).Return(
					&datastore.Device{
						UID:            "device-id",
						GroupID:        "1234",
						AppID:          "abc",
						HostName:       "hostname_1",
						Status:         datastore.DeviceStatusOffline,
						DocumentStatus: datastore.ActiveDocumentStatus,
						LastSeenAt:     primitive.NewDateTimeFromTime(time.Now()),
					},
					nil,
				)
				d.EXPECT().UpdateDevice(gomock.Any(), &datastore.Device{
					UID:            "device-id",
					GroupID:        "1234",
					AppID:          "abc",
					HostName:       "hostname_1",
					Status:         datastore.DeviceStatusOnline,
					DocumentStatus: datastore.ActiveDocumentStatus,
					LastSeenAt:     primitive.NewDateTimeFromTime(time.Now()),
				}, "abc", "1234").Times(1).Return(nil)
			},
			want: &datastore.Device{
				GroupID:  "1234",
				AppID:    "abc",
				HostName: "hostname_1",
			},
			wantErr: false,
		},
		{
			name: "should_fail_to_update_device_status",
			args: args{
				ctx:   ctx,
				group: &datastore.Group{UID: "1234"},
				app:   &datastore.Application{UID: "abc"},
				loginRequest: &LoginRequest{
					HostName: "hostname_1",
					DeviceID: "device-id",
				},
			},
			dbFn: func(h *Hub) {
				d := h.deviceRepo.(*mocks.MockDeviceRepository)
				d.EXPECT().FetchDeviceByID(gomock.Any(), "device-id", "abc", "1234").Times(1).Return(
					&datastore.Device{
						UID:            "device-id",
						GroupID:        "1234",
						AppID:          "abc",
						HostName:       "hostname_1",
						Status:         datastore.DeviceStatusOffline,
						DocumentStatus: datastore.ActiveDocumentStatus,
						LastSeenAt:     primitive.NewDateTimeFromTime(time.Now()),
					},
					nil,
				)
				d.EXPECT().UpdateDevice(gomock.Any(), &datastore.Device{
					UID:            "device-id",
					GroupID:        "1234",
					AppID:          "abc",
					HostName:       "hostname_1",
					Status:         datastore.DeviceStatusOnline,
					DocumentStatus: datastore.ActiveDocumentStatus,
					LastSeenAt:     primitive.NewDateTimeFromTime(time.Now()),
				}, "abc", "1234").Times(1).Return(errors.New("failed"))
			},
			wantErr:     true,
			wantErrCode: http.StatusBadRequest,
			wantErrMsg:  "failed to update device to online",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			h := provideHub(ctrl)

			if tt.dbFn != nil {
				tt.dbFn(h)
			}

			device, err := h.login(tt.args.ctx, tt.args.group, tt.args.app, tt.args.loginRequest)
			if tt.wantErr {
				require.NotNil(t, err)
				require.Equal(t, tt.wantErrCode, err.(*util.ServiceError).ErrCode())
				require.Equal(t, tt.wantErrMsg, err.(*util.ServiceError).Error())
				return
			}

			require.Nil(t, err)
			require.NotEmpty(t, device.UID)
			require.Equal(t, tt.want.AppID, device.AppID)
			require.Equal(t, tt.want.GroupID, device.GroupID)
			require.Equal(t, datastore.DeviceStatusOnline, device.Status)
			require.Equal(t, tt.want.HostName, device.HostName)

		})
	}
}
