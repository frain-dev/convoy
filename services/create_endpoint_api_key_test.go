package services

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/frain-dev/convoy/auth"
	"github.com/frain-dev/convoy/mocks"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"gopkg.in/guregu/null.v4"

	"github.com/frain-dev/convoy/api/models"
	"github.com/frain-dev/convoy/datastore"
)

func provideCreateEndpointAPIKeyService(ctrl *gomock.Controller, d *models.CreateEndpointApiKey) *CreateEndpointAPIKeyService {
	return &CreateEndpointAPIKeyService{
		APIKeyRepo: mocks.NewMockAPIKeyRepository(ctrl),
		D:          d,
	}
}

func sameMinute(date1, date2 time.Time) bool {
	s1 := date1.Format(time.Stamp)
	s2 := date2.Format(time.Stamp)

	return s1 == s2
}

func TestCreateEndpointAPIKeyService_Run(t *testing.T) {
	ctx := context.Background()
	type args struct {
		ctx       context.Context
		newApiKey *models.CreateEndpointApiKey
	}
	tests := []struct {
		name          string
		args          args
		wantAPIKey    *datastore.APIKey
		dbFn          func(ss *CreateEndpointAPIKeyService)
		verifyBaseUrl bool
		wantBaseUrl   string
		wantErr       bool
		wantErrMsg    string
	}{
		{
			name: "should_create_portal_api_key",
			args: args{
				ctx: ctx,
				newApiKey: &models.CreateEndpointApiKey{
					Project:  &datastore.Project{UID: "1234"},
					Endpoint: &datastore.Endpoint{UID: "abc", ProjectID: "1234", Name: "test_endpoint"},
					KeyType:  datastore.AppPortalKey,
					BaseUrl:  "https://getconvoy.io",
					Name:     "api-key-1",
				},
			},
			wantAPIKey: &datastore.APIKey{
				Name: "api-key-1",
				Type: datastore.AppPortalKey,
				Role: auth.Role{
					Type:     auth.RoleAdmin,
					Project:  "1234",
					Endpoint: "abc",
				},
				ExpiresAt: null.NewTime(time.Now().Add(time.Minute*30), true),
			},
			dbFn: func(ss *CreateEndpointAPIKeyService) {
				a, _ := ss.APIKeyRepo.(*mocks.MockAPIKeyRepository)
				a.EXPECT().CreateAPIKey(gomock.Any(), gomock.Any()).
					Times(1).Return(nil)
			},
			verifyBaseUrl: true,
			wantBaseUrl:   "?projectID=1234&appId=abc",
		},
		{
			name: "should_create_cli_api_key",
			args: args{
				ctx: ctx,
				newApiKey: &models.CreateEndpointApiKey{
					Project:    &datastore.Project{UID: "1234"},
					Endpoint:   &datastore.Endpoint{UID: "abc", ProjectID: "1234", Name: "test_endpoint"},
					KeyType:    datastore.CLIKey,
					BaseUrl:    "https://getconvoy.io",
					Name:       "api-key-1",
					Expiration: 7,
				},
			},
			wantAPIKey: &datastore.APIKey{
				Name: "api-key-1",
				Type: datastore.CLIKey,
				Role: auth.Role{
					Type:     auth.RoleAdmin,
					Project:  "1234",
					Endpoint: "abc",
				},
				ExpiresAt: null.NewTime(time.Now().Add(time.Hour*24*7), true),
			},
			dbFn: func(ss *CreateEndpointAPIKeyService) {
				a, _ := ss.APIKeyRepo.(*mocks.MockAPIKeyRepository)
				a.EXPECT().CreateAPIKey(gomock.Any(), gomock.Any()).
					Times(1).Return(nil)
			},
		},
		{
			name: "should_error_for_endpoint_not_belong_to_project_api_key",
			args: args{
				ctx: ctx,
				newApiKey: &models.CreateEndpointApiKey{
					Project:  &datastore.Project{UID: "1234"},
					Endpoint: &datastore.Endpoint{ProjectID: "12345"},
				},
			},
			wantErr:    true,
			wantErrMsg: "endpoint does not belong to project",
		},
		{
			name: "should_fail_to_create_app_portal_api_key",
			args: args{
				ctx: ctx,
				newApiKey: &models.CreateEndpointApiKey{
					Project:  &datastore.Project{UID: "1234"},
					Endpoint: &datastore.Endpoint{UID: "abc", ProjectID: "1234", Name: "test_app"},
					BaseUrl:  "https://getconvoy.io",
				},
			},
			dbFn: func(ss *CreateEndpointAPIKeyService) {
				a, _ := ss.APIKeyRepo.(*mocks.MockAPIKeyRepository)
				a.EXPECT().CreateAPIKey(gomock.Any(), gomock.Any()).
					Times(1).Return(errors.New("failed"))
			},
			wantErr:    true,
			wantErrMsg: "failed to create api key",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			ss := provideCreateEndpointAPIKeyService(ctrl, tc.args.newApiKey)

			if tc.dbFn != nil {
				tc.dbFn(ss)
			}

			apiKey, keyString, err := ss.Run(tc.args.ctx)
			if tc.wantErr {
				require.NotNil(t, err)
				require.Equal(t, tc.wantErrMsg, err.(*ServiceError).Error())
				return
			}

			require.Nil(t, err)
			require.NotEmpty(t, apiKey.UID)
			require.NotEmpty(t, apiKey.MaskID)
			require.NotEmpty(t, apiKey.Hash)
			require.NotEmpty(t, apiKey.Salt)
			require.NotEmpty(t, apiKey.CreatedAt)
			require.NotEmpty(t, apiKey.UpdatedAt)
			require.NotEmpty(t, keyString)
			require.Empty(t, apiKey.DeletedAt)

			if tc.verifyBaseUrl {
				require.Equal(t, tc.wantBaseUrl, fmt.Sprintf("?projectID=%s&appId=%s", tc.args.newApiKey.Project.UID, tc.args.newApiKey.Endpoint.UID))
			}

			require.True(t, sameMinute(apiKey.ExpiresAt.ValueOrZero(), tc.wantAPIKey.ExpiresAt.ValueOrZero()))

			stripVariableFields(t, "apiKey", apiKey)
			stripVariableFields(t, "apiKey", tc.wantAPIKey)
			apiKey.ExpiresAt = null.Time{}
			tc.wantAPIKey.ExpiresAt = null.Time{}
			require.Equal(t, tc.wantAPIKey, apiKey)
		})
	}
}
