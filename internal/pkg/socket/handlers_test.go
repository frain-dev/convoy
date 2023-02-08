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
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func provideRepo(ctrl *gomock.Controller) *Repo {
	endpointRepo := mocks.NewMockEndpointRepository(ctrl)
	subRepo := mocks.NewMockSubscriptionRepository(ctrl)
	sourceRepo := mocks.NewMockSourceRepository(ctrl)
	deviceRepo := mocks.NewMockDeviceRepository(ctrl)
	projectRepo := mocks.NewMockProjectRepository(ctrl)
	orgMemberRepo := mocks.NewMockOrganisationMemberRepository(ctrl)
	eventDeliveryRepo := mocks.NewMockEventDeliveryRepository(ctrl)

	return &Repo{
		ProjectRepo:         projectRepo,
		OrgMemberRepository: orgMemberRepo,
		EndpointRepo:        endpointRepo,
		DeviceRepo:          deviceRepo,
		SubscriptionRepo:    subRepo,
		SourceRepo:          sourceRepo,
		EventDeliveryRepo:   eventDeliveryRepo,
	}
}

func TestHub_listen(t *testing.T) {
	ctx := context.Background()
	lastSeen := primitive.NewDateTimeFromTime(time.Now().Add(-time.Minute))
	type args struct {
		ctx           context.Context
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
				ctx: ctx,
				listenRequest: &ListenRequest{
					ProjectID: "1234",
					HostName:  "hostname_1",
					DeviceID:  "device-id",
					SourceID:  "source-id",
				},
			},
			dbFn: func(h *Repo) {
				p := h.ProjectRepo.(*mocks.MockProjectRepository)
				p.EXPECT().FetchProjectByID(gomock.Any(), "1234").Times(1).Return(
					&datastore.Project{
						UID:            "1234",
						Name:           "test",
						OrganisationID: "abc",
						Type:           datastore.IncomingProject,
					},
					nil,
				)

				d := h.DeviceRepo.(*mocks.MockDeviceRepository)
				d.EXPECT().FetchDeviceByID(gomock.Any(), "device-id", "", "1234").Times(1).Return(
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

				sub.EXPECT().FindSubscriptionByDeviceID(gomock.Any(), "1234", "device-id", datastore.SubscriptionTypeCLI).
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
			name: "should_fail_to_find_device",
			args: args{
				ctx: ctx,
				listenRequest: &ListenRequest{
					ProjectID: "1234",
					HostName:  "hostname_1",
					DeviceID:  "device-id",
					SourceID:  "source-id",
				},
			},
			dbFn: func(h *Repo) {
				p := h.ProjectRepo.(*mocks.MockProjectRepository)
				p.EXPECT().FetchProjectByID(gomock.Any(), "1234").Times(1).Return(
					&datastore.Project{
						UID:            "1234",
						Name:           "test",
						OrganisationID: "abc",
						Type:           datastore.IncomingProject,
					},
					nil,
				)

				d := h.DeviceRepo.(*mocks.MockDeviceRepository)
				d.EXPECT().FetchDeviceByID(gomock.Any(), "device-id", "", "1234").Times(1).Return(nil, errors.New("device not found"))
			},
			wantErr:     true,
			wantErrCode: http.StatusBadRequest,
			wantErrMsg:  "device not found",
		},
		{
			name: "should_fail_to_find_project",
			args: args{
				ctx: ctx,
				listenRequest: &ListenRequest{
					ProjectID: "1234",
					HostName:  "hostname_1",
					DeviceID:  "device-id",
					SourceID:  "source-id",
				},
			},
			dbFn: func(h *Repo) {
				p := h.ProjectRepo.(*mocks.MockProjectRepository)
				p.EXPECT().FetchProjectByID(gomock.Any(), "1234").Times(1).Return(nil, errors.New("failed"))
			},
			wantErr:     true,
			wantErrCode: http.StatusBadRequest,
			wantErrMsg:  "failed to find project",
		},
		{
			name: "should_fail_to_find_source",
			args: args{
				ctx: ctx,
				listenRequest: &ListenRequest{
					ProjectID: "1234",
					HostName:  "hostname_1",
					DeviceID:  "device-id",
					SourceID:  "source-id",
				},
			},
			dbFn: func(h *Repo) {
				p := h.ProjectRepo.(*mocks.MockProjectRepository)
				p.EXPECT().FetchProjectByID(gomock.Any(), "1234").Times(1).Return(
					&datastore.Project{
						UID:            "1234",
						Name:           "test",
						OrganisationID: "abc",
						Type:           datastore.IncomingProject,
					},
					nil,
				)

				d := h.DeviceRepo.(*mocks.MockDeviceRepository)
				d.EXPECT().FetchDeviceByID(gomock.Any(), "device-id", "", "1234").Times(1).Return(
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
			name: "should_fail_to_find_subscription",
			args: args{
				ctx: ctx,
				listenRequest: &ListenRequest{
					ProjectID: "1234",
					HostName:  "hostname_1",
					DeviceID:  "device-id",
					SourceID:  "source-id",
				},
			},
			dbFn: func(h *Repo) {
				p := h.ProjectRepo.(*mocks.MockProjectRepository)
				p.EXPECT().FetchProjectByID(gomock.Any(), "1234").Times(1).Return(
					&datastore.Project{
						UID:            "1234",
						Name:           "test",
						OrganisationID: "abc",
						Type:           datastore.IncomingProject,
					},
					nil,
				)

				d := h.DeviceRepo.(*mocks.MockDeviceRepository)
				d.EXPECT().FetchDeviceByID(gomock.Any(), "device-id", "", "1234").Times(1).Return(
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
				sub.EXPECT().FindSubscriptionByDeviceID(gomock.Any(), "1234", "device-id", datastore.SubscriptionTypeCLI).
					Times(1).Return(nil, errors.New("failed to find subscription by id"))
			},
			wantErr:     true,
			wantErrCode: http.StatusBadRequest,
			wantErrMsg:  "failed to find subscription by id",
		},
		{
			name: "should_create_new_subscription_and_listen_successfully",
			args: args{
				ctx: ctx,
				listenRequest: &ListenRequest{
					ProjectID: "1234",
					HostName:  "hostname_1",
					DeviceID:  "device-id",
					SourceID:  "source-id",
				},
			},
			dbFn: func(h *Repo) {
				p := h.ProjectRepo.(*mocks.MockProjectRepository)
				p.EXPECT().FetchProjectByID(gomock.Any(), "1234").Times(1).Return(
					&datastore.Project{
						UID:            "1234",
						Name:           "test",
						OrganisationID: "abc",
						Type:           datastore.IncomingProject,
					},
					nil,
				)
				d := h.DeviceRepo.(*mocks.MockDeviceRepository)
				d.EXPECT().FetchDeviceByID(gomock.Any(), "device-id", "", "1234").Times(1).Return(
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

				sub.EXPECT().FindSubscriptionByDeviceID(gomock.Any(), "1234", "device-id", datastore.SubscriptionTypeCLI).
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
				ctx: ctx,
				listenRequest: &ListenRequest{
					ProjectID: "1234",
					HostName:  "hostname_1",
					DeviceID:  "device-id",
					SourceID:  "source-id",
				},
			},
			dbFn: func(h *Repo) {
				p := h.ProjectRepo.(*mocks.MockProjectRepository)
				p.EXPECT().FetchProjectByID(gomock.Any(), "1234").Times(1).Return(
					&datastore.Project{
						UID:            "1234",
						Name:           "test",
						OrganisationID: "abc",
						Type:           datastore.IncomingProject,
					},
					nil,
				)

				d := h.DeviceRepo.(*mocks.MockDeviceRepository)
				d.EXPECT().FetchDeviceByID(gomock.Any(), "device-id", "", "1234").Times(1).Return(
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
				sub.EXPECT().FindSubscriptionByDeviceID(gomock.Any(), "1234", "device-id", datastore.SubscriptionTypeCLI).
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

			device, err := listen(tt.args.ctx, tt.args.listenRequest, r)
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
		user         *datastore.User
		loginRequest *LoginRequest
	}
	tests := []struct {
		name        string
		args        args
		dbFn        func(h *Repo)
		want        *LoginResponse
		stripData   bool
		wantErr     bool
		wantErrCode int
		wantErrMsg  string
	}{
		{
			name: "should_fail_to_find_user_projects",
			args: args{
				ctx:  ctx,
				user: &datastore.User{UID: "abc", FirstName: "Daniel", LastName: "O.J"},
				loginRequest: &LoginRequest{
					HostName: "hostname_1",
				},
			},
			dbFn: func(h *Repo) {
				o := h.OrgMemberRepository.(*mocks.MockOrganisationMemberRepository)
				o.EXPECT().FindUserProjects(gomock.Any(), "abc").Times(1).Return(nil, errors.New("failed"))
			},
			wantErr:     true,
			wantErrCode: http.StatusBadRequest,
			wantErrMsg:  "failed to find user projects",
		},
		{
			name: "should_create_new_device_and_login_successfully",
			args: args{
				ctx:  ctx,
				user: &datastore.User{UID: "abc", FirstName: "Daniel", LastName: "O.J"},
				loginRequest: &LoginRequest{
					HostName: "hostname_1",
				},
			},
			dbFn: func(h *Repo) {
				o := h.OrgMemberRepository.(*mocks.MockOrganisationMemberRepository)
				o.EXPECT().FindUserProjects(gomock.Any(), "abc").Times(1).Return(
					[]datastore.Project{
						{UID: "project_1", Name: "test_project_1", Type: datastore.IncomingProject},
						{UID: "project_2", Name: "test_project_2", Type: datastore.IncomingProject},
					},
					nil,
				)

				d := h.DeviceRepo.(*mocks.MockDeviceRepository)
				d.EXPECT().FetchDeviceByHostName(gomock.Any(), "hostname_1", "", "project_1").Times(1).Return(nil, datastore.ErrDeviceNotFound)

				d.EXPECT().FetchDeviceByHostName(gomock.Any(), "hostname_1", "", "project_2").Times(1).Return(
					&datastore.Device{
						UID:       "222",
						ProjectID: "project_2",
						HostName:  "hostname_1",
						Status:    datastore.DeviceStatusOffline,
					}, nil,
				)

				d.EXPECT().CreateDevice(gomock.Any(), gomock.Any()).Times(1).Return(nil)
			},
			want: &LoginResponse{
				UserName: "Daniel O.J",
				Projects: []ProjectDevice{
					{
						Project: &datastore.Project{UID: "project_1", Name: "test_project_1", Type: datastore.IncomingProject},
						Device: &datastore.Device{
							ProjectID: "project_1",
							HostName:  "hostname_1",
							Status:    datastore.DeviceStatusOffline,
						},
					},
					{
						Project: &datastore.Project{UID: "project_2", Name: "test_project_2", Type: datastore.IncomingProject},
						Device: &datastore.Device{
							ProjectID: "project_2",
							HostName:  "hostname_1",
							Status:    datastore.DeviceStatusOffline,
						},
					},
				},
			},
			stripData: true,
			wantErr:   false,
		},
		{
			name: "should_fail_to_create_device",
			args: args{
				ctx:  ctx,
				user: &datastore.User{UID: "abc", FirstName: "Daniel", LastName: "O.J"},
				loginRequest: &LoginRequest{
					HostName: "hostname_1",
				},
			},
			dbFn: func(h *Repo) {
				o := h.OrgMemberRepository.(*mocks.MockOrganisationMemberRepository)
				o.EXPECT().FindUserProjects(gomock.Any(), "abc").Times(1).Return(
					[]datastore.Project{
						{UID: "project_1", Name: "test_project_1", Type: datastore.IncomingProject},
						{UID: "project_2", Name: "test_project_2", Type: datastore.IncomingProject},
					},
					nil,
				)

				d := h.DeviceRepo.(*mocks.MockDeviceRepository)
				d.EXPECT().FetchDeviceByHostName(gomock.Any(), "hostname_1", "", "project_1").Times(1).Return(nil, datastore.ErrDeviceNotFound)

				d.EXPECT().CreateDevice(gomock.Any(), gomock.Any()).Times(1).Return(errors.New("failed"))
			},
			wantErr:     true,
			wantErrCode: http.StatusBadRequest,
			wantErrMsg:  "failed to create new device",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			r := provideRepo(ctrl)

			if tt.dbFn != nil {
				tt.dbFn(r)
			}

			loginResponse, err := login(tt.args.ctx, tt.args.loginRequest, r, tt.args.user)
			if tt.wantErr {
				require.NotNil(t, err)
				require.Equal(t, tt.wantErrCode, err.(*util.ServiceError).ErrCode())
				require.Equal(t, tt.wantErrMsg, err.(*util.ServiceError).Error())
				return
			}

			require.Nil(t, err)
			if tt.stripData {
				for i := range loginResponse.Projects {
					stripVariableFields(t, "device", loginResponse.Projects[i].Device)
				}
			}

			require.Equal(t, tt.want, loginResponse)
		})
	}
}

func stripVariableFields(t *testing.T, obj string, v interface{}) {
	switch obj {
	case "project":
		g := v.(*datastore.Project)
		if g.Config != nil {
			for i := range g.Config.Signature.Versions {
				v := &g.Config.Signature.Versions[i]
				v.UID = ""
				v.CreatedAt = 0
			}
		}
		g.UID = ""
		g.CreatedAt, g.UpdatedAt, g.DeletedAt = 0, 0, nil
	case "device":
		d := v.(*datastore.Device)
		d.UID = ""
		d.CreatedAt, d.UpdatedAt, d.DeletedAt = 0, 0, nil
	default:
		t.Errorf("invalid data body - %v of type %T", obj, obj)
		t.FailNow()
	}
}
