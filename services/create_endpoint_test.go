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

func provideCreateEndpointService(ctrl *gomock.Controller, e models.CreateEndpoint, projectID string) *CreateEndpointService {
	return &CreateEndpointService{
		Cache:        mocks.NewMockCache(ctrl),
		EndpointRepo: mocks.NewMockEndpointRepository(ctrl),
		ProjectRepo:  mocks.NewMockProjectRepository(ctrl),
		E:            e,
		ProjectID:    projectID,
	}
}

func TestCreateEndpointService_Run(t *testing.T) {
	projectID := "1234567890"
	project := &datastore.Project{UID: projectID, Type: datastore.OutgoingProject}

	ctx := context.Background()
	type args struct {
		ctx context.Context
		e   models.CreateEndpoint
		g   *datastore.Project
	}
	tests := []struct {
		name         string
		args         args
		wantEndpoint *datastore.Endpoint
		dbFn         func(endpoint *CreateEndpointService)
		wantErr      bool
		wantErrMsg   string
	}{
		{
			name: "should_create_endpoint",
			args: args{
				ctx: ctx,
				e: models.CreateEndpoint{
					Name:            "endpoint",
					SupportEmail:    "endpoint@test.com",
					IsDisabled:      false,
					SlackWebhookURL: "https://google.com",
					Secret:          "1234",
					URL:             "https://google.com",
					Description:     "test_endpoint",
				},
				g: project,
			},
			dbFn: func(app *CreateEndpointService) {
				a, _ := app.EndpointRepo.(*mocks.MockEndpointRepository)
				a.EXPECT().CreateEndpoint(gomock.Any(), gomock.Any(), gomock.Any()).Times(1).Return(nil)
			},
			wantEndpoint: &datastore.Endpoint{
				Title:           "endpoint",
				SupportEmail:    "endpoint@test.com",
				SlackWebhookURL: "https://google.com",
				ProjectID:       project.UID,
				Secrets: []datastore.Secret{
					{Value: "1234"},
				},
				AdvancedSignatures: true,
				TargetURL:          "https://google.com",
				Description:        "test_endpoint",
				RateLimit:          5000,
				Status:             datastore.ActiveEndpointStatus,
				RateLimitDuration:  "1m0s",
			},
			wantErr: false,
		},
		{
			name: "should_create_endpoint_with_custom_authentication",
			args: args{
				ctx: ctx,
				e: models.CreateEndpoint{
					Name:              "endpoint",
					Secret:            "1234",
					RateLimit:         100,
					RateLimitDuration: "1m",
					URL:               "https://google.com",
					Description:       "test_endpoint",
					Authentication: &models.EndpointAuthentication{
						Type: datastore.APIKeyAuthentication,
						ApiKey: &models.ApiKey{
							HeaderName:  "x-api-key",
							HeaderValue: "x-api-key",
						},
					},
				},
				g: project,
			},
			dbFn: func(app *CreateEndpointService) {
				a, _ := app.EndpointRepo.(*mocks.MockEndpointRepository)
				a.EXPECT().CreateEndpoint(gomock.Any(), gomock.Any(), gomock.Any()).Times(1).Return(nil)
			},
			wantEndpoint: &datastore.Endpoint{
				ProjectID: project.UID,
				Title:     "endpoint",
				Secrets: []datastore.Secret{
					{Value: "1234"},
				},
				TargetURL:          "https://google.com",
				AdvancedSignatures: true,
				Description:        "test_endpoint",
				RateLimit:          100,
				Status:             datastore.ActiveEndpointStatus,
				RateLimitDuration:  "1m0s",
				Authentication: &datastore.EndpointAuthentication{
					Type: datastore.APIKeyAuthentication,
					ApiKey: &datastore.ApiKey{
						HeaderName:  "x-api-key",
						HeaderValue: "x-api-key",
					},
				},
			},
			wantErr: false,
		},
		{
			name: "should_error_for_invalid_rate_limit_duration",
			args: args{
				ctx: ctx,
				e: models.CreateEndpoint{
					Name:              "test_endpoint",
					Secret:            "1234",
					RateLimit:         100,
					RateLimitDuration: "m",
					URL:               "https://google.com",
					Description:       "test_endpoint",
				},
				g: project,
			},
			wantErr:    true,
			wantErrMsg: `an error occurred parsing the rate limit duration: time: invalid duration "m"`,
		},
		{
			name: "should_fail_to_create_endpoint",
			args: args{
				ctx: ctx,
				e: models.CreateEndpoint{
					Name:              "test_endpoint",
					Secret:            "1234",
					RateLimit:         100,
					RateLimitDuration: "1m",
					URL:               "https://google.com",
					Description:       "test_endpoint",
				},
				g: project,
			},
			dbFn: func(app *CreateEndpointService) {
				a, _ := app.EndpointRepo.(*mocks.MockEndpointRepository)
				a.EXPECT().CreateEndpoint(gomock.Any(), gomock.Any(), gomock.Any()).Times(1).Return(errors.New("failed"))
			},
			wantErr:    true,
			wantErrMsg: "an error occurred while adding endpoint",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			as := provideCreateEndpointService(ctrl, tc.args.e, tc.args.g.UID)

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
			require.NotEmpty(t, endpoint.UID)
			require.NotEmpty(t, endpoint.CreatedAt)
			require.NotEmpty(t, endpoint.UpdatedAt)
			require.Empty(t, endpoint.DeletedAt)

			stripVariableFields(t, "endpoint", endpoint)
			require.Equal(t, tc.wantEndpoint, endpoint)
		})
	}
}
