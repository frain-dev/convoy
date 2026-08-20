package services

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/internal/notifications"
	"github.com/frain-dev/convoy/mocks"
	"github.com/frain-dev/convoy/pkg/msgpack"
	"github.com/frain-dev/convoy/queue"
)

func provideActivateEndpointService(ctrl *gomock.Controller, endpointID, projectID string) *ActivateEndpointService {
	licenser := mocks.NewMockLicenser(ctrl)
	licenser.EXPECT().AdvancedEndpointMgmt().AnyTimes().Return(true)

	return &ActivateEndpointService{
		EndpointRepo: mocks.NewMockEndpointRepository(ctrl),
		Logger:       mocks.NewMockLogger(ctrl),
		Licenser:     licenser,
		EndpointId:   endpointID,
		ProjectID:    projectID,
	}
}

func TestActivateEndpointService_Run(t *testing.T) {
	ctx := context.Background()
	type args struct {
		ctx        context.Context
		endpointID string
		projectID  string
	}
	tests := []struct {
		name         string
		args         args
		dbFn         func(es *ActivateEndpointService)
		wantEndpoint *datastore.Endpoint
		wantErr      bool
		wantErrMsg   string
	}{
		{
			// Regression: the response must carry the persisted (active) status, not
			// the pre-update snapshot, or clients render a stale "inactive".
			name: "should_activate_inactive_endpoint_and_return_active_status",
			args: args{
				ctx:        ctx,
				endpointID: "123",
				projectID:  "abc",
			},
			dbFn: func(es *ActivateEndpointService) {
				e, _ := es.EndpointRepo.(*mocks.MockEndpointRepository)
				e.EXPECT().FindEndpointByID(gomock.Any(), "123", "abc").Times(1).Return(
					&datastore.Endpoint{UID: "123", Status: datastore.InactiveEndpointStatus}, nil,
				)

				e.EXPECT().UpdateEndpointStatus(gomock.Any(), "abc", "123", datastore.ActiveEndpointStatus).Times(1).Return(false, nil)
			},
			wantEndpoint: &datastore.Endpoint{UID: "123", Status: datastore.ActiveEndpointStatus},
		},
		{
			name: "should_activate_paused_endpoint_and_return_active_status",
			args: args{
				ctx:        ctx,
				endpointID: "123",
				projectID:  "abc",
			},
			dbFn: func(es *ActivateEndpointService) {
				e, _ := es.EndpointRepo.(*mocks.MockEndpointRepository)
				e.EXPECT().FindEndpointByID(gomock.Any(), "123", "abc").Times(1).Return(
					&datastore.Endpoint{UID: "123", Status: datastore.PausedEndpointStatus}, nil,
				)

				e.EXPECT().UpdateEndpointStatus(gomock.Any(), "abc", "123", datastore.ActiveEndpointStatus).Times(1).Return(false, nil)
			},
			wantEndpoint: &datastore.Endpoint{UID: "123", Status: datastore.ActiveEndpointStatus},
		},
		{
			name: "should_reject_already_active_endpoint",
			args: args{
				ctx:        ctx,
				endpointID: "123",
				projectID:  "abc",
			},
			dbFn: func(es *ActivateEndpointService) {
				e, _ := es.EndpointRepo.(*mocks.MockEndpointRepository)
				e.EXPECT().FindEndpointByID(gomock.Any(), "123", "abc").Times(1).Return(
					&datastore.Endpoint{UID: "123", Status: datastore.ActiveEndpointStatus}, nil,
				)
			},
			wantErr:    true,
			wantErrMsg: "current endpoint status - active, does not support activation",
		},
		{
			name: "should_fail_to_find_endpoint",
			args: args{
				ctx:        ctx,
				endpointID: "123",
				projectID:  "abc",
			},
			dbFn: func(es *ActivateEndpointService) {
				e, _ := es.EndpointRepo.(*mocks.MockEndpointRepository)
				e.EXPECT().FindEndpointByID(gomock.Any(), "123", "abc").Times(1).Return(
					nil, errors.New("failed"),
				)

				ml, _ := es.Logger.(*mocks.MockLogger)
				ml.EXPECT().ErrorContext(gomock.Any(), "failed to find endpoint", "error", gomock.Any()).Times(1)
			},
			wantErr:    true,
			wantErrMsg: "failed to find endpoint",
		},
		{
			name: "should_fail_to_update_endpoint_status",
			args: args{
				ctx:        ctx,
				endpointID: "123",
				projectID:  "abc",
			},
			dbFn: func(es *ActivateEndpointService) {
				e, _ := es.EndpointRepo.(*mocks.MockEndpointRepository)
				e.EXPECT().FindEndpointByID(gomock.Any(), "123", "abc").Times(1).Return(
					&datastore.Endpoint{UID: "123", Status: datastore.InactiveEndpointStatus}, nil,
				)

				e.EXPECT().UpdateEndpointStatus(gomock.Any(), "abc", "123", datastore.ActiveEndpointStatus).Times(1).Return(false, errors.New("failed"))

				ml, _ := es.Logger.(*mocks.MockLogger)
				ml.EXPECT().ErrorContext(gomock.Any(), "failed to activate endpoint", "error", gomock.Any()).Times(1)
			},
			wantErr:    true,
			wantErrMsg: "failed to activate endpoint",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			s := provideActivateEndpointService(ctrl, tt.args.endpointID, tt.args.projectID)

			// Arrange Expectations
			if tt.dbFn != nil {
				tt.dbFn(s)
			}

			endpoint, err := s.Run(tt.args.ctx)
			if tt.wantErr {
				require.NotNil(t, err)
				require.Equal(t, tt.wantErrMsg, err.(*ServiceError).Error())
				return
			}

			require.Nil(t, err)
			require.Equal(t, tt.wantEndpoint, endpoint)
		})
	}
}

// The all-clear leg. It answers the disable alert, so it may only fire for an
// endpoint that was actually reported down, and only on the tick whose write
// flipped the status.
func TestActivateEndpointService_ReactivationAlert(t *testing.T) {
	tests := []struct {
		name    string
		status  datastore.EndpointStatus
		changed bool
		// wantTypes is the notification type of every job the activation
		// enqueued, one per configured channel.
		wantTypes []notifications.NotificationType
	}{
		{
			name:    "disabled_endpoint_announces_once_on_every_channel",
			status:  datastore.InactiveEndpointStatus,
			changed: true,
			wantTypes: []notifications.NotificationType{
				notifications.EmailNotificationType,
				notifications.SlackNotificationType,
				notifications.TeamsNotificationType,
			},
		},
		{
			// A concurrent activation already flipped it, so this call has no
			// transition to announce.
			name:    "disabled_endpoint_stays_silent_when_the_write_changed_nothing",
			status:  datastore.InactiveEndpointStatus,
			changed: false,
		},
		{
			// A paused endpoint was never reported down, so there is nothing to
			// clear.
			name:    "paused_endpoint_never_announces",
			status:  datastore.PausedEndpointStatus,
			changed: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			s := provideActivateEndpointService(ctrl, "123", "abc")
			q := mocks.NewMockQueuer(ctrl)
			s.Queue = q
			s.Project = &datastore.Project{UID: "abc", LogoURL: "https://logo"}

			e, _ := s.EndpointRepo.(*mocks.MockEndpointRepository)
			e.EXPECT().FindEndpointByID(gomock.Any(), "123", "abc").Times(1).Return(
				&datastore.Endpoint{
					UID:             "123",
					Url:             "https://endpoint",
					Status:          tt.status,
					SupportEmail:    "support@convoy.test",
					SlackWebhookURL: "https://hooks.slack.test/services/x",
					TeamsWebhookURL: "https://outlook.office.com/webhook/x",
				}, nil,
			)
			e.EXPECT().UpdateEndpointStatus(gomock.Any(), "abc", "123", datastore.ActiveEndpointStatus).
				Times(1).Return(tt.changed, nil)

			var gotTypes []notifications.NotificationType
			q.EXPECT().Write(gomock.Any(), convoy.NotificationProcessor, convoy.DefaultQueue, gomock.Any()).
				Times(len(tt.wantTypes)).
				DoAndReturn(func(_ context.Context, _ convoy.TaskName, _ convoy.QueueName, job *queue.Job) error {
					var n notifications.Notification
					require.NoError(t, msgpack.DecodeMsgPack(job.Payload, &n))
					gotTypes = append(gotTypes, n.NotificationType)
					return nil
				})

			endpoint, err := s.Run(context.Background())
			require.Nil(t, err)
			require.Equal(t, datastore.ActiveEndpointStatus, endpoint.Status)
			require.ElementsMatch(t, tt.wantTypes, gotTypes)
		})
	}
}

func TestActivateEndpointService_ReactivationAlert_SkipsWithoutLicense(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	licenser := mocks.NewMockLicenser(ctrl)
	licenser.EXPECT().AdvancedEndpointMgmt().Times(1).Return(false)

	s := &ActivateEndpointService{
		EndpointRepo: mocks.NewMockEndpointRepository(ctrl),
		Logger:       mocks.NewMockLogger(ctrl),
		Licenser:     licenser,
		EndpointId:   "123",
		ProjectID:    "abc",
		Project:      &datastore.Project{UID: "abc", LogoURL: "https://logo"},
		Queue:        mocks.NewMockQueuer(ctrl),
	}

	e, _ := s.EndpointRepo.(*mocks.MockEndpointRepository)
	e.EXPECT().FindEndpointByID(gomock.Any(), "123", "abc").Times(1).Return(
		&datastore.Endpoint{
			UID:             "123",
			Url:             "https://endpoint",
			Status:          datastore.InactiveEndpointStatus,
			SupportEmail:    "support@convoy.test",
			SlackWebhookURL: "https://hooks.slack.test/services/x",
			TeamsWebhookURL: "https://outlook.office.com/webhook/x",
		}, nil,
	)
	e.EXPECT().UpdateEndpointStatus(gomock.Any(), "abc", "123", datastore.ActiveEndpointStatus).
		Times(1).Return(true, nil)

	endpoint, err := s.Run(context.Background())
	require.Nil(t, err)
	require.Equal(t, datastore.ActiveEndpointStatus, endpoint.Status)
}
