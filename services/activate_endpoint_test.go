package services

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/mocks"
)

func provideActivateEndpointService(ctrl *gomock.Controller, endpointID, projectID string) *ActivateEndpointService {
	return &ActivateEndpointService{
		EndpointRepo: mocks.NewMockEndpointRepository(ctrl),
		Logger:       mocks.NewMockLogger(ctrl),
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

				e.EXPECT().UpdateEndpointStatus(gomock.Any(), "abc", "123", datastore.ActiveEndpointStatus).Times(1).Return(nil)
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

				e.EXPECT().UpdateEndpointStatus(gomock.Any(), "abc", "123", datastore.ActiveEndpointStatus).Times(1).Return(nil)
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

				e.EXPECT().UpdateEndpointStatus(gomock.Any(), "abc", "123", datastore.ActiveEndpointStatus).Times(1).Return(errors.New("failed"))

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
