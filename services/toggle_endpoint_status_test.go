package services

import (
	"context"
	"testing"

	"github.com/pkg/errors"

	"github.com/frain-dev/convoy/mocks"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"

	"github.com/frain-dev/convoy/datastore"
)

func provideToggleEndpointStatusService(ctrl *gomock.Controller, EndpointID, projectID string) *ToggleEndpointStatusService {
	return &ToggleEndpointStatusService{
		EndpointRepo: mocks.NewMockEndpointRepository(ctrl),
		EndpointId:   EndpointID,
		ProjectID:    projectID,
	}
}

func TestToggleEndpointStatusService_Run(t *testing.T) {
	ctx := context.Background()
	type args struct {
		ctx        context.Context
		endpointID string
		projectID  string
	}
	tests := []struct {
		name         string
		args         args
		dbFn         func(es *ToggleEndpointStatusService)
		wantEndpoint *datastore.Endpoint
		wantErr      bool
		wantErrMsg   string
	}{
		{
			name: "should_toggle_endpoint_active_status",
			args: args{
				ctx:        ctx,
				endpointID: "123",
				projectID:  "abc",
			},
			dbFn: func(es *ToggleEndpointStatusService) {
				e, _ := es.EndpointRepo.(*mocks.MockEndpointRepository)
				e.EXPECT().FindEndpointByID(gomock.Any(), "123", "abc").Times(1).Return(
					&datastore.Endpoint{UID: "123", Status: datastore.ActiveEndpointStatus}, nil,
				)

				e.EXPECT().UpdateEndpointStatus(gomock.Any(), "abc", "123", datastore.InactiveEndpointStatus).Times(1).Return(nil)
			},
			wantEndpoint: &datastore.Endpoint{UID: "123", Status: datastore.InactiveEndpointStatus},
		},
		{
			name: "should_toggle_endpoint_inactive_status",
			args: args{
				ctx:        ctx,
				endpointID: "123",
				projectID:  "abc",
			},
			dbFn: func(es *ToggleEndpointStatusService) {
				e, _ := es.EndpointRepo.(*mocks.MockEndpointRepository)
				e.EXPECT().FindEndpointByID(gomock.Any(), "123", "abc").Times(1).Return(
					&datastore.Endpoint{UID: "123", Status: datastore.InactiveEndpointStatus}, nil,
				)

				e.EXPECT().UpdateEndpointStatus(gomock.Any(), "abc", "123", datastore.ActiveEndpointStatus).Times(1).Return(nil)
			},
			wantEndpoint: &datastore.Endpoint{UID: "123", Status: datastore.ActiveEndpointStatus},
		},
		{
			name: "should_fail_to_find_endpoint",
			args: args{
				ctx:        ctx,
				endpointID: "123",
				projectID:  "abc",
			},
			dbFn: func(es *ToggleEndpointStatusService) {
				e, _ := es.EndpointRepo.(*mocks.MockEndpointRepository)
				e.EXPECT().FindEndpointByID(gomock.Any(), "123", "abc").Times(1).Return(
					nil, errors.New("failed"),
				)
			},
			wantErr:    true,
			wantErrMsg: "failed to find endpoint",
		},
		{
			name: "should_fail_to_udpate_endpoint_status",
			args: args{
				ctx:        ctx,
				endpointID: "123",
				projectID:  "abc",
			},
			dbFn: func(es *ToggleEndpointStatusService) {
				e, _ := es.EndpointRepo.(*mocks.MockEndpointRepository)
				e.EXPECT().FindEndpointByID(gomock.Any(), "123", "abc").Times(1).Return(
					&datastore.Endpoint{UID: "123", Status: datastore.InactiveEndpointStatus}, nil,
				)

				e.EXPECT().UpdateEndpointStatus(gomock.Any(), "abc", "123", datastore.ActiveEndpointStatus).Times(1).Return(errors.New("failed"))
			},
			wantErr:    true,
			wantErrMsg: "failed to update endpoint status",
		},
		{
			name: "should_fail_to_pending_endpoint",
			args: args{
				ctx:        ctx,
				endpointID: "123",
				projectID:  "abc",
			},
			dbFn: func(es *ToggleEndpointStatusService) {
				e, _ := es.EndpointRepo.(*mocks.MockEndpointRepository)
				e.EXPECT().FindEndpointByID(gomock.Any(), "123", "abc").Times(1).Return(
					&datastore.Endpoint{UID: "123", Status: datastore.PendingEndpointStatus}, nil,
				)
			},
			wantErr:    true,
			wantErrMsg: "endpoint is in pending status",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			s := provideToggleEndpointStatusService(ctrl, tt.args.endpointID, tt.args.projectID)

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
