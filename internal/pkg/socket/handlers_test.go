package socket

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/mocks"
	"github.com/frain-dev/convoy/util"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

func provideRepo(ctrl *gomock.Controller) *Repo {
	endpointRepo := mocks.NewMockEndpointRepository(ctrl)
	subRepo := mocks.NewMockSubscriptionRepository(ctrl)
	sourceRepo := mocks.NewMockSourceRepository(ctrl)
	deviceRepo := mocks.NewMockDeviceRepository(ctrl)
	eventDeliveryRepo := mocks.NewMockEventDeliveryRepository(ctrl)

	return &Repo{
		EndpointRepo:      endpointRepo,
		DeviceRepo:        deviceRepo,
		SubscriptionRepo:  subRepo,
		SourceRepo:        sourceRepo,
		EventDeliveryRepo: eventDeliveryRepo,
	}
}

func TestHub_listen(t *testing.T) {
	ctx := context.Background()
	lastSeen := time.Now().Add(-time.Minute)
	type args struct {
		ctx           context.Context
		project       *datastore.Project
		endpoint      *datastore.Endpoint
		listenRequest *ListenRequest
	}
	tests := []struct {
		name        string
		args        args
		dbFn        func(h *Repo)
		want        *datastore.Device
		wantErr     bool
		wantErrCode int
		wantErrMsg  string
	}{
		{
			name: "should_listen_successfully",
			args: args{
				ctx:      ctx,
				project:  &datastore.Project{UID: "1234", Type: datastore.IncomingProject},
				endpoint: &datastore.Endpoint{UID: "abc"},
				listenRequest: &ListenRequest{
					HostName:   "",
					DeviceID:   "device-id",
					SourceID:   "source-id",
					EventTypes: []string{"charge.success"},
				},
			},
			dbFn: func(h *Repo) {
				d := h.DeviceRepo.(*mocks.MockDeviceRepository)
				d.EXPECT().FetchDeviceByID(gomock.Any(), "device-id", "abc", "1234").Times(1).Return(
					&datastore.Device{
						UID:        "device-id",
						ProjectID:  "1234",
						EndpointID: "abc",
						HostName:   "",
						Status:     datastore.DeviceStatusOnline,
						LastSeenAt: lastSeen,
					},
					nil,
				)

				s, _ := h.SourceRepo.(*mocks.MockSourceRepository)
				s.EXPECT().FindSourceByID(gomock.Any(), gomock.Any(), "source-id").Times(1).Return(
					&datastore.Source{UID: "1234", ProjectID: "1234"},
					nil,
				)

				sub, _ := h.SubscriptionRepo.(*mocks.MockSubscriptionRepository)
				sub.EXPECT().UpdateSubscription(gomock.Any(), gomock.Any(), gomock.Any()).Times(1).Return(nil)

				sub.EXPECT().FindSubscriptionByDeviceID(gomock.Any(), "1234", "device-id").
					Times(1).Return(&datastore.Subscription{}, nil)
			},
			want: &datastore.Device{
				UID:        "device-id",
				ProjectID:  "1234",
				EndpointID: "abc",
				HostName:   "",
				Status:     datastore.DeviceStatusOnline,
				LastSeenAt: lastSeen,
			},
			wantErr: false,
		},
		{
			name: "should_error_for_wrong_device_project_id",
			args: args{
				ctx:      ctx,
				project:  &datastore.Project{UID: "1234"},
				endpoint: &datastore.Endpoint{UID: "abc"},
				listenRequest: &ListenRequest{
					HostName:   "",
					DeviceID:   "device-id",
					SourceID:   "source-id",
					EventTypes: []string{"charge.success"},
				},
			},
			dbFn: func(h *Repo) {
				d := h.DeviceRepo.(*mocks.MockDeviceRepository)
				d.EXPECT().FetchDeviceByID(gomock.Any(), "device-id", "abc", "1234").Times(1).Return(
					&datastore.Device{
						UID:        "device-id",
						ProjectID:  "2",
						EndpointID: "abc",
						HostName:   "",
						Status:     datastore.DeviceStatusOnline,
						LastSeenAt: lastSeen,
					},
					nil,
				)
			},
			wantErr:     true,
			wantErrCode: http.StatusUnauthorized,
			wantErrMsg:  "this device cannot access this project",
		},
		{
			name: "should_error_for_wrong_device_app_id",
			args: args{
				ctx:      ctx,
				project:  &datastore.Project{UID: "1234"},
				endpoint: &datastore.Endpoint{UID: "abc"},
				listenRequest: &ListenRequest{
					HostName:   "",
					DeviceID:   "device-id",
					SourceID:   "source-id",
					EventTypes: []string{"charge.success"},
				},
			},
			dbFn: func(h *Repo) {
				d := h.DeviceRepo.(*mocks.MockDeviceRepository)
				d.EXPECT().FetchDeviceByID(gomock.Any(), "device-id", "abc", "1234").Times(1).Return(
					&datastore.Device{
						UID:        "device-id",
						ProjectID:  "1234",
						EndpointID: "abcd",
						HostName:   "",
						Status:     datastore.DeviceStatusOnline,
						LastSeenAt: lastSeen,
					},
					nil,
				)
			},
			wantErr:     true,
			wantErrCode: http.StatusUnauthorized,
			wantErrMsg:  "this device cannot access this application",
		},
		{
			name: "should_fail_to_find_device",
			args: args{
				ctx:      ctx,
				project:  &datastore.Project{UID: "1234"},
				endpoint: &datastore.Endpoint{UID: "abc"},
				listenRequest: &ListenRequest{
					HostName:   "",
					DeviceID:   "device-id",
					SourceID:   "source-id",
					EventTypes: []string{"charge.success"},
				},
			},
			dbFn: func(h *Repo) {
				d := h.DeviceRepo.(*mocks.MockDeviceRepository)
				d.EXPECT().FetchDeviceByID(gomock.Any(), "device-id", "abc", "1234").Times(1).Return(nil, errors.New("device not found"))
			},
			wantErr:     true,
			wantErrCode: http.StatusBadRequest,
			wantErrMsg:  "device not found",
		},
		{
			name: "should_fail_to_find_source",
			args: args{
				ctx:      ctx,
				project:  &datastore.Project{UID: "1234", Type: datastore.IncomingProject},
				endpoint: &datastore.Endpoint{UID: "abc"},
				listenRequest: &ListenRequest{
					HostName:   "",
					DeviceID:   "device-id",
					SourceID:   "source-id",
					EventTypes: []string{"charge.success"},
				},
			},
			dbFn: func(h *Repo) {
				d := h.DeviceRepo.(*mocks.MockDeviceRepository)
				d.EXPECT().FetchDeviceByID(gomock.Any(), "device-id", "abc", "1234").Times(1).Return(
					&datastore.Device{
						UID:        "device-id",
						ProjectID:  "1234",
						EndpointID: "abc",
						HostName:   "",
						Status:     datastore.DeviceStatusOnline,
						LastSeenAt: lastSeen,
					},
					nil,
				)

				s, _ := h.SourceRepo.(*mocks.MockSourceRepository)
				s.EXPECT().FindSourceByID(gomock.Any(), gomock.Any(), "source-id").Times(1).Return(nil, errors.New("failed to find source"))
			},
			wantErr:     true,
			wantErrCode: http.StatusBadRequest,
			wantErrMsg:  "failed to find source",
		},
		{
			name: "should_error_for_wrong_source_project_id",
			args: args{
				ctx:      ctx,
				project:  &datastore.Project{UID: "1234", Type: datastore.IncomingProject},
				endpoint: &datastore.Endpoint{UID: "abc"},
				listenRequest: &ListenRequest{
					HostName:   "",
					DeviceID:   "device-id",
					SourceID:   "source-id",
					EventTypes: []string{"charge.success"},
				},
			},
			dbFn: func(h *Repo) {
				d := h.DeviceRepo.(*mocks.MockDeviceRepository)
				d.EXPECT().FetchDeviceByID(gomock.Any(), "device-id", "abc", "1234").Times(1).Return(
					&datastore.Device{
						UID:        "device-id",
						ProjectID:  "1234",
						EndpointID: "abc",
						HostName:   "",
						Status:     datastore.DeviceStatusOnline,
						LastSeenAt: lastSeen,
					},
					nil,
				)

				s, _ := h.SourceRepo.(*mocks.MockSourceRepository)
				s.EXPECT().FindSourceByID(gomock.Any(), gomock.Any(), "source-id").Times(1).Return(
					&datastore.Source{UID: "1234", ProjectID: "ref"},
					nil,
				)
			},
			wantErr:     true,
			wantErrCode: http.StatusUnauthorized,
			wantErrMsg:  "this device cannot access this source",
		},

		{
			name: "should_fail_to_find_subscription",
			args: args{
				ctx:      ctx,
				project:  &datastore.Project{UID: "1234", Type: datastore.IncomingProject},
				endpoint: &datastore.Endpoint{UID: "abc"},
				listenRequest: &ListenRequest{
					HostName:   "",
					DeviceID:   "device-id",
					SourceID:   "source-id",
					EventTypes: []string{"charge.success"},
				},
			},
			dbFn: func(h *Repo) {
				d := h.DeviceRepo.(*mocks.MockDeviceRepository)
				d.EXPECT().FetchDeviceByID(gomock.Any(), "device-id", "abc", "1234").Times(1).Return(
					&datastore.Device{
						UID:        "device-id",
						ProjectID:  "1234",
						EndpointID: "abc",
						HostName:   "",
						Status:     datastore.DeviceStatusOnline,
						LastSeenAt: lastSeen,
					},
					nil,
				)

				s, _ := h.SourceRepo.(*mocks.MockSourceRepository)
				s.EXPECT().FindSourceByID(gomock.Any(), gomock.Any(), "source-id").Times(1).Return(
					&datastore.Source{UID: "1234", ProjectID: "1234"},
					nil,
				)

				sub, _ := h.SubscriptionRepo.(*mocks.MockSubscriptionRepository)
				sub.EXPECT().FindSubscriptionByDeviceID(gomock.Any(), "1234", "device-id").
					Times(1).Return(nil, errors.New("failed to find subscription by id"))
			},
			wantErr:     true,
			wantErrCode: http.StatusBadRequest,
			wantErrMsg:  "failed to find subscription by id",
		},
		{
			name: "should_create_new_subscription_and_listen_successfully",
			args: args{
				ctx:      ctx,
				project:  &datastore.Project{UID: "1234", Type: datastore.IncomingProject},
				endpoint: &datastore.Endpoint{UID: "abc"},
				listenRequest: &ListenRequest{
					HostName:   "",
					DeviceID:   "device-id",
					SourceID:   "source-id",
					EventTypes: []string{"charge.success"},
				},
			},
			dbFn: func(h *Repo) {
				d := h.DeviceRepo.(*mocks.MockDeviceRepository)
				d.EXPECT().FetchDeviceByID(gomock.Any(), "device-id", "abc", "1234").Times(1).Return(
					&datastore.Device{
						UID:        "device-id",
						ProjectID:  "1234",
						EndpointID: "abc",
						HostName:   "",
						Status:     datastore.DeviceStatusOnline,
					},
					nil,
				)

				s, _ := h.SourceRepo.(*mocks.MockSourceRepository)
				s.EXPECT().FindSourceByID(gomock.Any(), gomock.Any(), "source-id").Times(1).Return(
					&datastore.Source{UID: "1234", ProjectID: "1234"},
					nil,
				)

				sub, _ := h.SubscriptionRepo.(*mocks.MockSubscriptionRepository)

				sub.EXPECT().FindSubscriptionByDeviceID(gomock.Any(), "1234", "device-id").
					Times(1).Return(nil, datastore.ErrSubscriptionNotFound)

				sub.EXPECT().CreateSubscription(gomock.Any(), "1234", gomock.Any()).Times(1).Return(nil)
			},
			want: &datastore.Device{
				UID:        "device-id",
				ProjectID:  "1234",
				EndpointID: "abc",
				HostName:   "",
				Status:     datastore.DeviceStatusOnline,
			},
			wantErr: false,
		},
		{
			name: "should_fail_to_create_new_subscription",
			args: args{
				ctx:      ctx,
				project:  &datastore.Project{UID: "1234", Type: datastore.IncomingProject},
				endpoint: &datastore.Endpoint{UID: "abc"},
				listenRequest: &ListenRequest{
					HostName:   "",
					DeviceID:   "device-id",
					SourceID:   "source-id",
					EventTypes: []string{"charge.success"},
				},
			},
			dbFn: func(h *Repo) {
				d := h.DeviceRepo.(*mocks.MockDeviceRepository)
				d.EXPECT().FetchDeviceByID(gomock.Any(), "device-id", "abc", "1234").Times(1).Return(
					&datastore.Device{
						UID:        "device-id",
						ProjectID:  "1234",
						EndpointID: "abc",
						HostName:   "",
						Status:     datastore.DeviceStatusOnline,
						LastSeenAt: lastSeen,
					},
					nil,
				)

				s, _ := h.SourceRepo.(*mocks.MockSourceRepository)
				s.EXPECT().FindSourceByID(gomock.Any(), gomock.Any(), "source-id").Times(1).Return(
					&datastore.Source{UID: "1234", ProjectID: "1234"},
					nil,
				)

				sub, _ := h.SubscriptionRepo.(*mocks.MockSubscriptionRepository)
				sub.EXPECT().FindSubscriptionByDeviceID(gomock.Any(), "1234", "device-id").
					Times(1).Return(nil, datastore.ErrSubscriptionNotFound)

				sub.EXPECT().CreateSubscription(gomock.Any(), "1234", gomock.Any()).Times(1).Return(errors.New("failed to create new subscription"))
			},
			want: &datastore.Device{
				UID:        "device-id",
				ProjectID:  "1234",
				EndpointID: "abc",
				HostName:   "",
				Status:     datastore.DeviceStatusOnline,
				LastSeenAt: lastSeen,
			},
			wantErr:     true,
			wantErrCode: http.StatusBadRequest,
			wantErrMsg:  "failed to create new subscription",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			r := provideRepo(ctrl)

			if tt.dbFn != nil {
				tt.dbFn(r)
			}

			h := NewHub()

			device, err := listen(tt.args.ctx, tt.args.project, tt.args.endpoint, tt.args.listenRequest, h, r)
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
		project      *datastore.Project
		endpoint     *datastore.Endpoint
		loginRequest *LoginRequest
	}
	tests := []struct {
		name        string
		args        args
		dbFn        func(h *Repo)
		want        *datastore.Device
		checkData   bool
		wantErr     bool
		wantErrCode int
		wantErrMsg  string
	}{
		{
			name: "should_create_new_device_and_login_successfully",
			args: args{
				ctx:      ctx,
				project:  &datastore.Project{UID: "1234"},
				endpoint: &datastore.Endpoint{UID: "abc"},
				loginRequest: &LoginRequest{
					HostName: "hostname_1",
					DeviceID: "",
				},
			},
			dbFn: func(h *Repo) {
				d := h.DeviceRepo.(*mocks.MockDeviceRepository)
				d.EXPECT().FetchDeviceByHostName(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any())

				d.EXPECT().CreateDevice(gomock.Any(), gomock.Any()).Times(1).Return(nil)
			},
			want: &datastore.Device{
				ProjectID:  "1234",
				EndpointID: "abc",
				HostName:   "hostname_1",
				Status:     datastore.DeviceStatusOffline,
			},
			wantErr: false,
		},
		{
			name: "should_fail_to_create_new_device",
			args: args{
				ctx:      ctx,
				project:  &datastore.Project{UID: "1234"},
				endpoint: &datastore.Endpoint{UID: "abc"},
				loginRequest: &LoginRequest{
					HostName: "hostname_1",
					DeviceID: "",
				},
			},
			dbFn: func(h *Repo) {
				d := h.DeviceRepo.(*mocks.MockDeviceRepository)
				d.EXPECT().FetchDeviceByHostName(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any())

				d.EXPECT().CreateDevice(gomock.Any(), gomock.Any()).Times(1).Return(errors.New("failed to create new device"))
			},
			wantErr:     true,
			wantErrCode: http.StatusBadRequest,
			wantErrMsg:  "failed to create new device",
		},
		{
			name: "should_login_with_existing_device_successfully",
			args: args{
				ctx:      ctx,
				project:  &datastore.Project{UID: "1234"},
				endpoint: &datastore.Endpoint{UID: "abc"},
				loginRequest: &LoginRequest{
					HostName: "hostname_1",
					DeviceID: "device-id",
				},
			},
			dbFn: func(h *Repo) {
				d := h.DeviceRepo.(*mocks.MockDeviceRepository)
				d.EXPECT().FetchDeviceByID(gomock.Any(), "device-id", "abc", "1234").Times(1).Return(
					&datastore.Device{
						UID:        "device-id",
						ProjectID:  "1234",
						EndpointID: "abc",
						HostName:   "hostname_1",
						Status:     datastore.DeviceStatusOffline,
						LastSeenAt: time.Now(),
					},
					nil,
				)
			},
			want: &datastore.Device{
				ProjectID:  "1234",
				EndpointID: "abc",
				HostName:   "hostname_1",
				Status:     datastore.DeviceStatusOffline,
			},
			wantErr: false,
		},
		{
			name: "should_fail_to_find_device",
			args: args{
				ctx:      ctx,
				project:  &datastore.Project{UID: "1234"},
				endpoint: &datastore.Endpoint{UID: "abc"},
				loginRequest: &LoginRequest{
					HostName: "hostname_1",
					DeviceID: "device-id",
				},
			},
			dbFn: func(h *Repo) {
				d := h.DeviceRepo.(*mocks.MockDeviceRepository)
				d.EXPECT().FetchDeviceByID(gomock.Any(), "device-id", "abc", "1234").Times(1).
					Return(nil, errors.New("failed to find device by id"))
			},
			want: &datastore.Device{
				ProjectID:  "1234",
				EndpointID: "abc",
				HostName:   "hostname_1",
			},
			wantErr:     true,
			wantErrCode: http.StatusBadRequest,
			wantErrMsg:  "failed to find device by id",
		},
		{
			name: "should_error_for_wrong_device_project_id",
			args: args{
				ctx:      ctx,
				project:  &datastore.Project{UID: "1234"},
				endpoint: &datastore.Endpoint{UID: "abc"},
				loginRequest: &LoginRequest{
					HostName: "hostname_1",
					DeviceID: "device-id",
				},
			},
			dbFn: func(h *Repo) {
				d := h.DeviceRepo.(*mocks.MockDeviceRepository)
				d.EXPECT().FetchDeviceByID(gomock.Any(), "device-id", "abc", "1234").Times(1).Return(
					&datastore.Device{
						UID:        "device-id",
						ProjectID:  "123",
						EndpointID: "abc",
						HostName:   "hostname_1",
						Status:     datastore.DeviceStatusOnline,
						LastSeenAt: time.Now(),
					},
					nil,
				)
			},
			wantErr:     true,
			wantErrCode: http.StatusUnauthorized,
			wantErrMsg:  "this device cannot access this project",
		},
		{
			name: "should_error_for_wrong_device_app_id",
			args: args{
				ctx:      ctx,
				project:  &datastore.Project{UID: "1234"},
				endpoint: &datastore.Endpoint{UID: "abc"},
				loginRequest: &LoginRequest{
					HostName: "hostname_1",
					DeviceID: "device-id",
				},
			},
			dbFn: func(h *Repo) {
				d := h.DeviceRepo.(*mocks.MockDeviceRepository)
				d.EXPECT().FetchDeviceByID(gomock.Any(), "device-id", "abc", "1234").Times(1).Return(
					&datastore.Device{
						UID:        "device-id",
						ProjectID:  "1234",
						EndpointID: "abcd",
						HostName:   "hostname_1",
						Status:     datastore.DeviceStatusOnline,
						LastSeenAt: time.Now(),
					},
					nil,
				)
			},
			wantErr:     true,
			wantErrCode: http.StatusUnauthorized,
			wantErrMsg:  "this device cannot access this application",
		},
		{
			name: "should_login_with_existing_device_and_update_device_status_successfully",
			args: args{
				ctx:      ctx,
				project:  &datastore.Project{UID: "1234"},
				endpoint: &datastore.Endpoint{UID: "abc"},
				loginRequest: &LoginRequest{
					HostName: "hostname_1",
					DeviceID: "device-id",
				},
			},
			dbFn: func(h *Repo) {
				d := h.DeviceRepo.(*mocks.MockDeviceRepository)
				d.EXPECT().FetchDeviceByID(gomock.Any(), "device-id", "abc", "1234").Times(1).Return(
					&datastore.Device{
						UID:        "device-id",
						ProjectID:  "1234",
						EndpointID: "abc",
						HostName:   "hostname_1",
						Status:     datastore.DeviceStatusOnline,
						LastSeenAt: time.Now(),
					},
					nil,
				)
				d.EXPECT().UpdateDevice(gomock.Any(), &datastore.Device{
					UID:        "device-id",
					ProjectID:  "1234",
					EndpointID: "abc",
					HostName:   "hostname_1",
					Status:     datastore.DeviceStatusOffline,
					LastSeenAt: time.Now(),
				}, "abc", "1234").Times(1).Return(nil)
			},
			want: &datastore.Device{
				ProjectID:  "1234",
				EndpointID: "abc",
				HostName:   "hostname_1",
			},
			wantErr: false,
		},
		{
			name: "should_fail_to_update_device_status",
			args: args{
				ctx:      ctx,
				project:  &datastore.Project{UID: "1234"},
				endpoint: &datastore.Endpoint{UID: "abc"},
				loginRequest: &LoginRequest{
					HostName: "hostname_1",
					DeviceID: "device-id",
				},
			},
			dbFn: func(h *Repo) {
				d := h.DeviceRepo.(*mocks.MockDeviceRepository)
				d.EXPECT().FetchDeviceByID(gomock.Any(), "device-id", "abc", "1234").Times(1).Return(
					&datastore.Device{
						UID:        "device-id",
						ProjectID:  "1234",
						EndpointID: "abc",
						HostName:   "hostname_1",
						Status:     datastore.DeviceStatusOnline,
						LastSeenAt: time.Now(),
					},
					nil,
				)
				d.EXPECT().UpdateDevice(gomock.Any(), &datastore.Device{
					UID:        "device-id",
					ProjectID:  "1234",
					EndpointID: "abc",
					HostName:   "hostname_1",
					Status:     datastore.DeviceStatusOffline,
					LastSeenAt: time.Now(),
				}, "abc", "1234").Times(1).Return(errors.New("failed to update device to online"))
			},
			wantErr:     true,
			wantErrCode: http.StatusBadRequest,
			wantErrMsg:  "failed to update device to online",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			r := provideRepo(ctrl)

			if tt.dbFn != nil {
				tt.dbFn(r)
			}

			h := NewHub()

			device, err := login(tt.args.ctx, tt.args.project, tt.args.endpoint, tt.args.loginRequest, h, r)
			if tt.wantErr {
				require.NotNil(t, err)
				require.Equal(t, tt.wantErrCode, err.(*util.ServiceError).ErrCode())
				require.Equal(t, tt.wantErrMsg, err.(*util.ServiceError).Error())
				return
			}

			require.Nil(t, err)
			require.NotEmpty(t, device.UID)
			require.Equal(t, tt.want.EndpointID, device.EndpointID)
			require.Equal(t, tt.want.ProjectID, device.ProjectID)
			require.Equal(t, datastore.DeviceStatusOffline, device.Status)
			require.Equal(t, tt.want.HostName, device.HostName)
		})
	}
}
