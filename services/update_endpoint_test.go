package services

import (
	"context"
	"errors"
	"testing"

	"github.com/frain-dev/convoy/mocks"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"

	"github.com/frain-dev/convoy/api/models"
	"github.com/frain-dev/convoy/datastore"
)

func provideUpdateEndpointService(ctrl *gomock.Controller, e models.UpdateEndpoint, Endpoint *datastore.Endpoint, Project *datastore.Project) *UpdateEndpointService {
	return &UpdateEndpointService{
		Cache:        mocks.NewMockCache(ctrl),
		EndpointRepo: mocks.NewMockEndpointRepository(ctrl),
		ProjectRepo:  mocks.NewMockProjectRepository(ctrl),
		E:            e,
		Endpoint:     Endpoint,
		Project:      Project,
	}
}

func TestUpdateEndpointService_Run(t *testing.T) {
	ctx := context.Background()
	project := &datastore.Project{UID: "1234567890"}
	type args struct {
		ctx      context.Context
		e        models.UpdateEndpoint
		endpoint *datastore.Endpoint
		project  *datastore.Project
	}
	tests := []struct {
		name         string
		args         args
		wantEndpoint *datastore.Endpoint
		dbFn         func(as *UpdateEndpointService)
		wantErr      bool
		wantErrMsg   string
	}{
		{
			name: "should_update_endpoint",
			args: args{
				ctx: ctx,
				e: models.UpdateEndpoint{
					Name:              stringPtr("Endpoint2"),
					Description:       "test_endpoint",
					URL:               "https://fb.com",
					RateLimit:         10000,
					RateLimitDuration: "1m",
					HttpTimeout:       "20s",
				},
				endpoint: &datastore.Endpoint{UID: "endpoint2"},
				project:  project,
			},
			wantEndpoint: &datastore.Endpoint{
				Title:             "Endpoint2",
				Description:       "test_endpoint",
				TargetURL:         "https://fb.com",
				RateLimit:         10000,
				RateLimitDuration: "1m0s",
				HttpTimeout:       "20s",
			},
			dbFn: func(as *UpdateEndpointService) {
				a, _ := as.EndpointRepo.(*mocks.MockEndpointRepository)
				a.EXPECT().FindEndpointByID(gomock.Any(), gomock.Any(), "1234567890").
					Times(1).Return(&datastore.Endpoint{UID: "endpoint2"}, nil)

				a.EXPECT().UpdateEndpoint(gomock.Any(), gomock.Any(), gomock.Any()).
					Times(1).Return(nil)

				c, _ := as.Cache.(*mocks.MockCache)
				c.EXPECT().Set(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any())
			},
			wantErr: false,
		},
		{
			name: "should_error_for_invalid_rate_limit_duration",
			args: args{
				ctx: ctx,
				e: models.UpdateEndpoint{
					Name:              stringPtr("Endpoint 1"),
					Description:       "test_endpoint",
					URL:               "https://fb.com",
					RateLimit:         10000,
					RateLimitDuration: "m",
					HttpTimeout:       "20s",
				},
				endpoint: &datastore.Endpoint{UID: "endpoint1"},
				project:  project,
			},
			dbFn: func(as *UpdateEndpointService) {
				a, _ := as.EndpointRepo.(*mocks.MockEndpointRepository)
				a.EXPECT().FindEndpointByID(gomock.Any(), gomock.Any(), "1234567890").
					Times(1).Return(&datastore.Endpoint{UID: "endpoint1"}, nil)
			},
			wantErr:    true,
			wantErrMsg: `time: invalid duration "m"`,
		},
		{
			name: "should_fail_to_update_endpoint",
			args: args{
				ctx: ctx,
				e: models.UpdateEndpoint{
					Name:              stringPtr("Endpoint 1"),
					Description:       "test_endpoint",
					URL:               "https://fb.com",
					RateLimit:         10000,
					RateLimitDuration: "1m",
					HttpTimeout:       "20s",
				},
				endpoint: &datastore.Endpoint{UID: "endpoint1"},
				project:  project,
			},
			dbFn: func(as *UpdateEndpointService) {
				a, _ := as.EndpointRepo.(*mocks.MockEndpointRepository)
				a.EXPECT().FindEndpointByID(gomock.Any(), gomock.Any(), "1234567890").
					Times(1).Return(&datastore.Endpoint{UID: "endpoint1"}, nil)

				a.EXPECT().UpdateEndpoint(gomock.Any(), gomock.Any(), gomock.Any()).
					Times(1).Return(errors.New("failed"))
			},
			wantErr:    true,
			wantErrMsg: "an error occurred while updating endpoints",
		},
		{
			name: "should_error_for_endpoint_not_found",
			args: args{
				ctx: ctx,
				e: models.UpdateEndpoint{
					Name:              stringPtr("endpoint1"),
					Description:       "test_endpoint",
					URL:               "https://fb.com",
					RateLimit:         10000,
					RateLimitDuration: "1m",
					HttpTimeout:       "20s",
				},
				endpoint: &datastore.Endpoint{UID: "endpoint1"},
				project:  project,
			},
			dbFn: func(as *UpdateEndpointService) {
				a, _ := as.EndpointRepo.(*mocks.MockEndpointRepository)
				a.EXPECT().FindEndpointByID(gomock.Any(), gomock.Any(), "1234567890").
					Times(1).Return(nil, datastore.ErrEndpointNotFound)
			},
			wantErr:    true,
			wantErrMsg: "endpoint not found",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			as := provideUpdateEndpointService(ctrl, tc.args.e, tc.args.endpoint, tc.args.project)

			// Arrange Expectations
			if tc.dbFn != nil {
				tc.dbFn(as)
			}

			endpoint, err := as.Run(tc.args.ctx)
			if tc.wantErr {
				require.NotNil(t, err)
				require.Equal(t, tc.wantErrMsg, err.(*ServiceError).Error())
				return
			}

			require.Nil(t, err)
			require.NotEmpty(t, endpoint.UpdatedAt)

			stripVariableFields(t, "endpoint", endpoint)
			require.Equal(t, tc.wantEndpoint, endpoint)
		})
	}
}
