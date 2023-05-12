package services

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"testing"
	"time"

	"gopkg.in/guregu/null.v4"

	"github.com/frain-dev/convoy/api/models"
	"github.com/frain-dev/convoy/auth"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/mocks"
	"github.com/frain-dev/convoy/util"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

func provideSecurityService(ctrl *gomock.Controller) *SecurityService {
	projectRepo := mocks.NewMockProjectRepository(ctrl)
	apiKeyRepo := mocks.NewMockAPIKeyRepository(ctrl)
	return NewSecurityService(projectRepo, apiKeyRepo)
}

func sameMinute(date1, date2 time.Time) bool {
	s1 := date1.Format(time.Stamp)
	s2 := date2.Format(time.Stamp)

	return s1 == s2
}

func TestSecurityService_CreateAPIKey(t *testing.T) {
	ctx := context.Background()
	type args struct {
		ctx       context.Context
		newApiKey *models.APIKey
		member    *datastore.OrganisationMember
	}
	expires := null.NewTime(time.Now().Add(time.Hour), true)
	tests := []struct {
		name        string
		args        args
		wantAPIKey  *datastore.APIKey
		dbFn        func(ss *SecurityService)
		wantErr     bool
		wantErrCode int
		wantErrMsg  string
	}{
		{
			name: "should_create_api_key",
			args: args{
				ctx: ctx,
				newApiKey: &models.APIKey{
					Name: "test_api_key",
					Type: "api",
					Role: models.Role{
						Type:    auth.RoleAdmin,
						Project: "1234",
					},
					ExpiresAt: expires,
				},
				member: &datastore.OrganisationMember{
					UID:            "abc",
					OrganisationID: "1234",
					Role:           auth.Role{Type: auth.RoleSuperUser},
				},
			},
			wantAPIKey: &datastore.APIKey{
				Name: "test_api_key",
				Type: "api",
				Role: auth.Role{
					Type:    auth.RoleAdmin,
					Project: "1234",
				},
				ExpiresAt: expires,
			},
			dbFn: func(ss *SecurityService) {
				g, _ := ss.projectRepo.(*mocks.MockProjectRepository)
				g.EXPECT().FetchProjectByID(gomock.Any(), gomock.Any()).Times(1).Return(&datastore.Project{UID: "abc", OrganisationID: "1234"}, nil)

				a, _ := ss.apiKeyRepo.(*mocks.MockAPIKeyRepository)
				a.EXPECT().CreateAPIKey(gomock.Any(), gomock.Any()).
					Times(1).Return(nil)
			},
		},
		{
			name: "should_error_for_invalid_expiry",
			args: args{
				ctx: ctx,
				newApiKey: &models.APIKey{
					Name: "test_api_key",
					Type: "api",
					Role: models.Role{
						Type:    auth.RoleAdmin,
						Project: "1234",
						App:     "1234",
					},
					ExpiresAt: null.NewTime(expires.ValueOrZero().Add(-2*time.Hour), true),
				},
				member: &datastore.OrganisationMember{
					UID:            "abc",
					OrganisationID: "1234",
					Role:           auth.Role{Type: auth.RoleSuperUser},
				},
			},
			wantErr:     true,
			wantErrCode: http.StatusBadRequest,
			wantErrMsg:  "expiry date is invalid",
		},
		{
			name: "should_error_for_invalid_api_key_role",
			args: args{
				ctx: ctx,
				newApiKey: &models.APIKey{
					Name: "test_api_key",
					Type: "api",
					Role: models.Role{
						Type:    "abc",
						Project: "1234",
						App:     "1234",
					},
					ExpiresAt: expires,
				},
				member: nil,
			},
			wantErr:     true,
			wantErrCode: http.StatusBadRequest,
			wantErrMsg:  "invalid api key role",
		},
		{
			name: "should_fail_to_fetch_project",
			args: args{
				ctx: ctx,
				newApiKey: &models.APIKey{
					Name: "test_api_key",
					Type: "api",
					Role: models.Role{
						Type:    auth.RoleAdmin,
						Project: "1234",
						App:     "1234",
					},
					ExpiresAt: expires,
				},
				member: &datastore.OrganisationMember{
					UID:            "abc",
					OrganisationID: "1234",
					Role:           auth.Role{Type: auth.RoleSuperUser},
				},
			},
			dbFn: func(ss *SecurityService) {
				g, _ := ss.projectRepo.(*mocks.MockProjectRepository)
				g.EXPECT().FetchProjectByID(gomock.Any(), "1234").
					Times(1).Return(nil, errors.New("failed"))
			},
			wantErr:     true,
			wantErrCode: http.StatusBadRequest,
			wantErrMsg:  "failed to fetch project by id",
		},
		{
			name: "should_error_for_organisation_id_mismatch",
			args: args{
				ctx: ctx,
				newApiKey: &models.APIKey{
					Name: "test_api_key",
					Type: "api",
					Role: models.Role{
						Type:    auth.RoleAdmin,
						Project: "1234",
						App:     "1234",
					},
					ExpiresAt: expires,
				},
				member: &datastore.OrganisationMember{
					UID:            "abc",
					OrganisationID: "1234",
					Role:           auth.Role{Type: auth.RoleSuperUser},
				},
			},
			dbFn: func(ss *SecurityService) {
				g, _ := ss.projectRepo.(*mocks.MockProjectRepository)
				g.EXPECT().FetchProjectByID(gomock.Any(), "1234").
					Times(1).Return(&datastore.Project{UID: "1234", OrganisationID: "555"}, nil)
			},
			wantErr:     true,
			wantErrCode: http.StatusUnauthorized,
			wantErrMsg:  "unauthorized to access project",
		},
		{
			name: "should_error_for_member_not_authorized_to_access_project",
			args: args{
				ctx: ctx,
				newApiKey: &models.APIKey{
					Name: "test_api_key",
					Type: "api",
					Role: models.Role{
						Type:    auth.RoleAdmin,
						Project: "1234",
						App:     "1234",
					},
					ExpiresAt: expires,
				},
				member: &datastore.OrganisationMember{
					UID:            "abc",
					OrganisationID: "555",
					Role:           auth.Role{Type: auth.RoleAdmin},
				},
			},
			dbFn: func(ss *SecurityService) {
				g, _ := ss.projectRepo.(*mocks.MockProjectRepository)
				g.EXPECT().FetchProjectByID(gomock.Any(), "1234").
					Times(1).Return(&datastore.Project{UID: "1234", OrganisationID: "555"}, nil)
			},
			wantErr:     true,
			wantErrCode: http.StatusUnauthorized,
			wantErrMsg:  "unauthorized to access project",
		},
		{
			name: "should_fail_to_create_api_key",
			args: args{
				ctx: ctx,
				newApiKey: &models.APIKey{
					Name: "test_api_key",
					Type: "api",
					Role: models.Role{
						Type:    auth.RoleAdmin,
						Project: "1234",
						App:     "1234",
					},
					ExpiresAt: expires,
				},
				member: &datastore.OrganisationMember{
					UID:            "abc",
					OrganisationID: "1234",
					Role:           auth.Role{Type: auth.RoleSuperUser},
				},
			},
			dbFn: func(ss *SecurityService) {
				g, _ := ss.projectRepo.(*mocks.MockProjectRepository)
				g.EXPECT().FetchProjectByID(gomock.Any(), "1234").
					Times(1).Return(&datastore.Project{UID: "1234", OrganisationID: "1234"}, nil)

				a, _ := ss.apiKeyRepo.(*mocks.MockAPIKeyRepository)
				a.EXPECT().CreateAPIKey(gomock.Any(), gomock.Any()).
					Times(1).Return(errors.New("failed"))
			},
			wantErr:     true,
			wantErrCode: http.StatusBadRequest,
			wantErrMsg:  "failed to create api key",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			ss := provideSecurityService(ctrl)

			if tc.dbFn != nil {
				tc.dbFn(ss)
			}

			apiKey, keyString, err := ss.CreateAPIKey(tc.args.ctx, tc.args.member, tc.args.newApiKey)
			if tc.wantErr {
				require.NotNil(t, err)
				require.Equal(t, tc.wantErrCode, err.(*util.ServiceError).ErrCode())
				require.Equal(t, tc.wantErrMsg, err.(*util.ServiceError).Error())
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

			stripVariableFields(t, "apiKey", apiKey)
			require.Equal(t, tc.wantAPIKey, apiKey)
		})
	}
}

func TestSecurityService_CreateEndpointAPIKey(t *testing.T) {
	ctx := context.Background()
	type args struct {
		ctx       context.Context
		newApiKey *models.CreateEndpointApiKey
	}
	tests := []struct {
		name          string
		args          args
		wantAPIKey    *datastore.APIKey
		dbFn          func(ss *SecurityService)
		verifyBaseUrl bool
		wantBaseUrl   string
		wantErr       bool
		wantErrCode   int
		wantErrMsg    string
	}{
		{
			name: "should_create_portal_api_key",
			args: args{
				ctx: ctx,
				newApiKey: &models.CreateEndpointApiKey{
					Project:  &datastore.Project{UID: "1234"},
					Endpoint: &datastore.Endpoint{UID: "abc", ProjectID: "1234", Title: "test_endpoint"},
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
			dbFn: func(ss *SecurityService) {
				a, _ := ss.apiKeyRepo.(*mocks.MockAPIKeyRepository)
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
					Endpoint:   &datastore.Endpoint{UID: "abc", ProjectID: "1234", Title: "test_endpoint"},
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
			dbFn: func(ss *SecurityService) {
				a, _ := ss.apiKeyRepo.(*mocks.MockAPIKeyRepository)
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
			wantErr:     true,
			wantErrCode: http.StatusBadRequest,
			wantErrMsg:  "endpoint does not belong to project",
		},
		{
			name: "should_fail_to_create_app_portal_api_key",
			args: args{
				ctx: ctx,
				newApiKey: &models.CreateEndpointApiKey{
					Project:  &datastore.Project{UID: "1234"},
					Endpoint: &datastore.Endpoint{UID: "abc", ProjectID: "1234", Title: "test_app"},
					BaseUrl:  "https://getconvoy.io",
				},
			},
			dbFn: func(ss *SecurityService) {
				a, _ := ss.apiKeyRepo.(*mocks.MockAPIKeyRepository)
				a.EXPECT().CreateAPIKey(gomock.Any(), gomock.Any()).
					Times(1).Return(errors.New("failed"))
			},
			wantErr:     true,
			wantErrCode: http.StatusBadRequest,
			wantErrMsg:  "failed to create api key",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			ss := provideSecurityService(ctrl)

			if tc.dbFn != nil {
				tc.dbFn(ss)
			}

			apiKey, keyString, err := ss.CreateEndpointAPIKey(tc.args.ctx, tc.args.newApiKey)
			if tc.wantErr {
				require.NotNil(t, err)
				require.Equal(t, tc.wantErrCode, err.(*util.ServiceError).ErrCode())
				require.Equal(t, tc.wantErrMsg, err.(*util.ServiceError).Error())
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

func TestSecurityService_GetAPIKeyByID(t *testing.T) {
	ctx := context.Background()
	type args struct {
		ctx context.Context
		uid string
	}
	tests := []struct {
		name        string
		args        args
		dbFn        func(ss *SecurityService)
		wantAPIKey  *datastore.APIKey
		wantErr     bool
		wantErrCode int
		wantErrMsg  string
	}{
		{
			name: "should_get_api_key_by_id",
			args: args{
				ctx: ctx,
				uid: "1234",
			},
			dbFn: func(ss *SecurityService) {
				a, _ := ss.apiKeyRepo.(*mocks.MockAPIKeyRepository)
				a.EXPECT().FindAPIKeyByID(gomock.Any(), "1234").
					Times(1).Return(&datastore.APIKey{UID: "1234"}, nil)
			},
			wantAPIKey: &datastore.APIKey{UID: "1234"},
		},
		{
			name: "should_error_for_empty_uid",
			args: args{
				ctx: ctx,
				uid: "",
			},
			wantErr:     true,
			wantErrCode: http.StatusBadRequest,
			wantErrMsg:  "key id is empty",
		},
		{
			name: "should_fail_to_get_api_key_by_id",
			args: args{
				ctx: ctx,
				uid: "1234",
			},
			dbFn: func(ss *SecurityService) {
				a, _ := ss.apiKeyRepo.(*mocks.MockAPIKeyRepository)
				a.EXPECT().FindAPIKeyByID(gomock.Any(), "1234").
					Times(1).Return(nil, errors.New("failed"))
			},
			wantErr:     true,
			wantErrCode: http.StatusBadRequest,
			wantErrMsg:  "failed to fetch api key",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			ss := provideSecurityService(ctrl)

			if tc.dbFn != nil {
				tc.dbFn(ss)
			}

			apiKey, err := ss.GetAPIKeyByID(tc.args.ctx, tc.args.uid)
			if tc.wantErr {
				require.NotNil(t, err)
				require.Equal(t, tc.wantErrCode, err.(*util.ServiceError).ErrCode())
				require.Equal(t, tc.wantErrMsg, err.(*util.ServiceError).Error())
				return
			}

			require.Nil(t, err)
			require.Equal(t, tc.wantAPIKey, apiKey)
		})
	}
}

func TestSecurityService_UpdateAPIKey(t *testing.T) {
	ctx := context.Background()
	type args struct {
		ctx  context.Context
		uid  string
		role *auth.Role
	}
	tests := []struct {
		name        string
		args        args
		dbFn        func(ss *SecurityService)
		wantAPIKey  *datastore.APIKey
		wantErr     bool
		wantErrCode int
		wantErrMsg  string
	}{
		{
			name: "should_update_api_key",
			args: args{
				ctx: ctx,
				uid: "1234",
				role: &auth.Role{
					Type:    auth.RoleAdmin,
					Project: "1234",
				},
			},
			dbFn: func(ss *SecurityService) {
				g, _ := ss.projectRepo.(*mocks.MockProjectRepository)
				g.EXPECT().FetchProjectByID(gomock.Any(), "1234").
					Times(1).Return(&datastore.Project{UID: "1234"},
					nil)

				a, _ := ss.apiKeyRepo.(*mocks.MockAPIKeyRepository)
				a.EXPECT().FindAPIKeyByID(gomock.Any(), "1234").
					Times(1).Return(
					&datastore.APIKey{
						UID: "ref",
						Role: auth.Role{
							Type:     auth.RoleAPI,
							Project:  "avs",
							Endpoint: "",
						},
					}, nil)

				a.EXPECT().UpdateAPIKey(gomock.Any(), gomock.Any()).
					Times(1).Return(nil)
			},
			wantAPIKey: &datastore.APIKey{
				UID: "ref",
				Role: auth.Role{
					Type:    auth.RoleAdmin,
					Project: "1234",
				},
			},
		},
		{
			name: "should_error_for_empty_uid",
			args: args{
				ctx: ctx,
				uid: "",
				role: &auth.Role{
					Type:    auth.RoleAdmin,
					Project: "1234",
				},
			},
			wantErr:     true,
			wantErrCode: http.StatusBadRequest,
			wantErrMsg:  "key id is empty",
		},
		{
			name: "should_update_api_key",
			args: args{
				ctx: ctx,
				uid: "1234",
				role: &auth.Role{
					Type: "abc",
				},
			},
			wantErr:     true,
			wantErrCode: http.StatusBadRequest,
			wantErrMsg:  "invalid api key role",
		},
		{
			name: "should_fail_to_fetch_project",
			args: args{
				ctx: ctx,
				uid: "1234",
				role: &auth.Role{
					Type:    auth.RoleAdmin,
					Project: "1234",
				},
			},
			dbFn: func(ss *SecurityService) {
				g, _ := ss.projectRepo.(*mocks.MockProjectRepository)
				g.EXPECT().FetchProjectByID(gomock.Any(), "1234").
					Times(1).Return(nil, errors.New("failed"))
			},
			wantErr:     true,
			wantErrCode: http.StatusBadRequest,
			wantErrMsg:  "invalid project",
		},
		{
			name: "should_fail_find_api_key_by_id",
			args: args{
				ctx: ctx,
				uid: "1234",
				role: &auth.Role{
					Type:    auth.RoleAdmin,
					Project: "1234",
				},
			},
			dbFn: func(ss *SecurityService) {
				g, _ := ss.projectRepo.(*mocks.MockProjectRepository)
				g.EXPECT().FetchProjectByID(gomock.Any(), "1234").
					Times(1).Return(&datastore.Project{UID: "1234"},
					nil)

				a, _ := ss.apiKeyRepo.(*mocks.MockAPIKeyRepository)
				a.EXPECT().FindAPIKeyByID(gomock.Any(), "1234").
					Times(1).Return(nil, errors.New("failed"))
			},
			wantErr:     true,
			wantErrCode: http.StatusBadRequest,
			wantErrMsg:  "failed to fetch api key",
		},
		{
			name: "should_update_api_key",
			args: args{
				ctx: ctx,
				uid: "1234",
				role: &auth.Role{
					Type:    auth.RoleAdmin,
					Project: "1234",
				},
			},
			dbFn: func(ss *SecurityService) {
				g, _ := ss.projectRepo.(*mocks.MockProjectRepository)
				g.EXPECT().FetchProjectByID(gomock.Any(), "1234").
					Times(1).Return(&datastore.Project{UID: "1234"},
					nil)

				a, _ := ss.apiKeyRepo.(*mocks.MockAPIKeyRepository)
				a.EXPECT().FindAPIKeyByID(gomock.Any(), "1234").
					Times(1).Return(
					&datastore.APIKey{
						UID: "ref",
						Role: auth.Role{
							Type:     auth.RoleAPI,
							Project:  "avs",
							Endpoint: "",
						},
					}, nil)

				a.EXPECT().UpdateAPIKey(gomock.Any(), gomock.Any()).
					Times(1).Return(errors.New("failed"))
			},
			wantErr:     true,
			wantErrCode: http.StatusBadRequest,
			wantErrMsg:  "failed to update api key",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			ss := provideSecurityService(ctrl)

			if tc.dbFn != nil {
				tc.dbFn(ss)
			}

			apiKey, err := ss.UpdateAPIKey(tc.args.ctx, tc.args.uid, tc.args.role)
			if tc.wantErr {
				require.NotNil(t, err)
				require.Equal(t, tc.wantErrCode, err.(*util.ServiceError).ErrCode())
				require.Equal(t, tc.wantErrMsg, err.(*util.ServiceError).Error())
				return
			}

			require.Nil(t, err)
			require.Equal(t, tc.wantAPIKey, apiKey)
		})
	}
}

func TestSecurityService_CreatePersonalAPIKey(t *testing.T) {
	ctx := context.Background()
	expires := null.NewTime(time.Now().Add(time.Hour), true)

	type args struct {
		ctx       context.Context
		user      *datastore.User
		newApiKey *models.PersonalAPIKey
	}
	tests := []struct {
		name        string
		args        args
		dbFn        func(ss *SecurityService)
		wantAPIKey  *datastore.APIKey
		wantErr     bool
		wantErrCode int
		wantErrMsg  string
	}{
		{
			name: "should_create_personal_apiKey",
			args: args{
				ctx:       ctx,
				user:      &datastore.User{UID: "1234"},
				newApiKey: &models.PersonalAPIKey{Name: "test_personal_key", Expiration: 1},
			},
			dbFn: func(ss *SecurityService) {
				a, _ := ss.apiKeyRepo.(*mocks.MockAPIKeyRepository)
				a.EXPECT().CreateAPIKey(gomock.Any(), gomock.Any()).
					Times(1).Return(nil)
			},
			wantAPIKey: &datastore.APIKey{
				UserID:    "1234",
				Name:      "test_personal_key",
				ExpiresAt: expires,
				Type:      datastore.PersonalKey,
			},
			wantErr: false,
		},
		{
			name: "should_fail_to_create_personal_apiKey",
			args: args{
				ctx:       ctx,
				user:      &datastore.User{UID: "1234"},
				newApiKey: &models.PersonalAPIKey{Name: "test_personal_key", Expiration: 1},
			},
			dbFn: func(ss *SecurityService) {
				a, _ := ss.apiKeyRepo.(*mocks.MockAPIKeyRepository)
				a.EXPECT().CreateAPIKey(gomock.Any(), gomock.Any()).
					Times(1).Return(errors.New("failed"))
			},
			wantErr:     true,
			wantErrCode: http.StatusBadRequest,
			wantErrMsg:  "failed to create api key",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			ss := provideSecurityService(ctrl)

			if tt.dbFn != nil {
				tt.dbFn(ss)
			}

			apiKey, keyString, err := ss.CreatePersonalAPIKey(tt.args.ctx, tt.args.user, tt.args.newApiKey)
			if tt.wantErr {
				require.NotNil(t, err)
				require.Equal(t, tt.wantErrCode, err.(*util.ServiceError).ErrCode())
				require.Equal(t, tt.wantErrMsg, err.(*util.ServiceError).Error())
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

			stripVariableFields(t, "apiKey", apiKey)
			require.InDelta(t, tt.wantAPIKey.ExpiresAt.ValueOrZero().Unix(), apiKey.ExpiresAt.ValueOrZero().Unix(), float64(time.Second))
			tt.wantAPIKey.ExpiresAt = null.Time{}
			apiKey.ExpiresAt = null.Time{}
			require.Equal(t, tt.wantAPIKey, apiKey)
		})
	}
}

func TestSecurityService_RevokePersonalAPIKey(t *testing.T) {
	ctx := context.Background()

	type args struct {
		ctx  context.Context
		uid  string
		user *datastore.User
	}
	tests := []struct {
		name        string
		dbFn        func(ss *SecurityService)
		args        args
		wantErr     bool
		wantErrCode int
		wantErrMsg  string
	}{
		{
			name: "should_revoke_api_key",
			args: args{
				ctx:  ctx,
				uid:  "1234",
				user: &datastore.User{UID: "abc"},
			},
			dbFn: func(ss *SecurityService) {
				a, _ := ss.apiKeyRepo.(*mocks.MockAPIKeyRepository)
				a.EXPECT().FindAPIKeyByID(gomock.Any(), "1234").Times(1).Return(&datastore.APIKey{UserID: "abc", Type: datastore.PersonalKey}, nil)

				a.EXPECT().RevokeAPIKeys(gomock.Any(), []string{"1234"}).
					Times(1).Return(nil)
			},
		},
		{
			name: "should_error_for_empty_uid",
			args: args{
				ctx:  ctx,
				uid:  "",
				user: nil,
			},
			wantErr:     true,
			wantErrCode: http.StatusBadRequest,
			wantErrMsg:  "key id is empty",
		},
		{
			name: "should_fail_to_find_api_key",
			args: args{
				ctx: ctx,
				uid: "1234",
			},
			dbFn: func(ss *SecurityService) {
				a, _ := ss.apiKeyRepo.(*mocks.MockAPIKeyRepository)
				a.EXPECT().FindAPIKeyByID(gomock.Any(), "1234").Times(1).Return(nil, errors.New("failed"))
			},
			wantErr:     true,
			wantErrCode: http.StatusBadRequest,
			wantErrMsg:  "failed to fetch api key",
		},
		{
			name: "should_error_for_wrong_user_id",
			args: args{
				ctx:  ctx,
				uid:  "1234",
				user: &datastore.User{UID: "abcd"},
			},
			dbFn: func(ss *SecurityService) {
				a, _ := ss.apiKeyRepo.(*mocks.MockAPIKeyRepository)
				a.EXPECT().FindAPIKeyByID(gomock.Any(), "1234").Times(1).Return(&datastore.APIKey{UserID: "abc", Type: datastore.PersonalKey}, nil)
			},
			wantErr:     true,
			wantErrCode: http.StatusUnauthorized,
			wantErrMsg:  "unauthorized",
		},
		{
			name: "should_error_for_wrong_key_type",
			args: args{
				ctx:  ctx,
				uid:  "1234",
				user: &datastore.User{UID: "abc"},
			},
			dbFn: func(ss *SecurityService) {
				a, _ := ss.apiKeyRepo.(*mocks.MockAPIKeyRepository)
				a.EXPECT().FindAPIKeyByID(gomock.Any(), "1234").Times(1).Return(&datastore.APIKey{UserID: "abc", Type: datastore.AppPortalKey}, nil)
			},
			wantErr:     true,
			wantErrCode: http.StatusUnauthorized,
			wantErrMsg:  "unauthorized",
		},
		{
			name: "should_fail_to_revoke_api_key",
			args: args{
				ctx:  ctx,
				uid:  "abc",
				user: &datastore.User{UID: "abc"},
			},
			dbFn: func(ss *SecurityService) {
				a, _ := ss.apiKeyRepo.(*mocks.MockAPIKeyRepository)
				a.EXPECT().FindAPIKeyByID(gomock.Any(), "abc").Times(1).Return(&datastore.APIKey{UserID: "abc", Type: datastore.PersonalKey}, nil)

				a.EXPECT().RevokeAPIKeys(gomock.Any(), []string{"abc"}).
					Times(1).Return(errors.New("failed"))
			},
			wantErr:     true,
			wantErrCode: http.StatusBadRequest,
			wantErrMsg:  "failed to revoke api key",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			ss := provideSecurityService(ctrl)

			if tt.dbFn != nil {
				tt.dbFn(ss)
			}

			err := ss.RevokePersonalAPIKey(tt.args.ctx, tt.args.uid, tt.args.user)
			if tt.wantErr {
				require.NotNil(t, err)
				require.Equal(t, tt.wantErrCode, err.(*util.ServiceError).ErrCode())
				require.Equal(t, tt.wantErrMsg, err.(*util.ServiceError).Error())
				return
			}

			require.Nil(t, err)
		})
	}
}

func TestSecurityService_RegenerateProjectAPIKey(t *testing.T) {
	ctx := context.Background()
	type args struct {
		ctx     context.Context
		project *datastore.Project
		member  *datastore.OrganisationMember
	}
	tests := []struct {
		name        string
		dbFn        func(ss *SecurityService)
		args        args
		wantAPIKey  *datastore.APIKey
		wantErr     bool
		wantErrCode int
		wantErrMsg  string
	}{
		{
			name: "should_regenerate_project_api_key",
			dbFn: func(ss *SecurityService) {
				a, _ := ss.apiKeyRepo.(*mocks.MockAPIKeyRepository)
				a.EXPECT().FindAPIKeyByProjectID(gomock.Any(), "1234").Times(1).Return(&datastore.APIKey{
					UID: "45678",
					Role: auth.Role{
						Type:     auth.RoleAdmin,
						Project:  "1234",
						Endpoint: "",
					},
				}, nil)

				a.EXPECT().RevokeAPIKeys(gomock.Any(), []string{"45678"}).Times(1).Return(nil)

				g, _ := ss.projectRepo.(*mocks.MockProjectRepository)
				g.EXPECT().FetchProjectByID(gomock.Any(), gomock.Any()).Times(1).Return(&datastore.Project{
					UID:            "1234",
					Name:           "test_project",
					OrganisationID: "org1",
				}, nil)

				a.EXPECT().CreateAPIKey(gomock.Any(), gomock.Any()).
					Times(1).Return(nil)
			},
			args: args{
				ctx: ctx,
				project: &datastore.Project{
					UID:            "1234",
					Name:           "test_project",
					OrganisationID: "org1",
				},
				member: &datastore.OrganisationMember{
					UID:            "abc",
					OrganisationID: "org1",
					Role: auth.Role{
						Type:     auth.RoleSuperUser,
						Project:  "1234",
						Endpoint: "",
					},
				},
			},
			wantAPIKey: &datastore.APIKey{
				Name: "test_project's key",
				Role: auth.Role{
					Type:     "admin",
					Project:  "1234",
					Endpoint: "",
				},
			},
			wantErr:     false,
			wantErrCode: 0,
			wantErrMsg:  "",
		},
		{
			name: "should_fail_to_find_api_key",
			dbFn: func(ss *SecurityService) {
				a, _ := ss.apiKeyRepo.(*mocks.MockAPIKeyRepository)
				a.EXPECT().FindAPIKeyByProjectID(gomock.Any(), "1234").Times(1).Return(nil, errors.New("failed"))
			},
			args: args{
				ctx: ctx,
				project: &datastore.Project{
					UID:            "1234",
					Name:           "test_project",
					OrganisationID: "org1",
				},
				member: &datastore.OrganisationMember{
					UID:            "abc",
					OrganisationID: "org1",
					Role: auth.Role{
						Type:     auth.RoleSuperUser,
						Project:  "1234",
						Endpoint: "",
					},
				},
			},
			wantErr:     true,
			wantErrCode: http.StatusBadRequest,
			wantErrMsg:  "failed to fetch project api key",
		},
		{
			name: "should_fail_to_revoke_api_key",
			dbFn: func(ss *SecurityService) {
				a, _ := ss.apiKeyRepo.(*mocks.MockAPIKeyRepository)
				a.EXPECT().FindAPIKeyByProjectID(gomock.Any(), "1234").Times(1).Return(&datastore.APIKey{
					UID: "45678",
					Role: auth.Role{
						Type:     auth.RoleAdmin,
						Project:  "1234",
						Endpoint: "",
					},
				}, nil)

				a.EXPECT().RevokeAPIKeys(gomock.Any(), []string{"45678"}).Times(1).Return(errors.New("failed"))
			},
			args: args{
				ctx: ctx,
				project: &datastore.Project{
					UID:            "1234",
					Name:           "test_project",
					OrganisationID: "org1",
				},
				member: &datastore.OrganisationMember{
					UID:            "abc",
					OrganisationID: "org1",
					Role: auth.Role{
						Type:     auth.RoleSuperUser,
						Project:  "1234",
						Endpoint: "",
					},
				},
			},
			wantErr:     true,
			wantErrCode: http.StatusBadRequest,
			wantErrMsg:  "failed to revoke api key",
		},
		{
			name: "should_fail_to_create_new_api_key",
			dbFn: func(ss *SecurityService) {
				a, _ := ss.apiKeyRepo.(*mocks.MockAPIKeyRepository)
				a.EXPECT().FindAPIKeyByProjectID(gomock.Any(), "1234").Times(1).Return(&datastore.APIKey{
					UID: "45678",
					Role: auth.Role{
						Type:     auth.RoleAdmin,
						Project:  "1234",
						Endpoint: "",
					},
				}, nil)

				a.EXPECT().RevokeAPIKeys(gomock.Any(), []string{"45678"}).Times(1).Return(nil)

				g, _ := ss.projectRepo.(*mocks.MockProjectRepository)
				g.EXPECT().FetchProjectByID(gomock.Any(), gomock.Any()).Times(1).Return(&datastore.Project{
					UID:            "1234",
					Name:           "test_project",
					OrganisationID: "org1",
				}, nil)

				a.EXPECT().CreateAPIKey(gomock.Any(), gomock.Any()).
					Times(1).Return(errors.New("failed"))
			},
			args: args{
				ctx: ctx,
				project: &datastore.Project{
					UID:            "1234",
					Name:           "test_project",
					OrganisationID: "org1",
				},
				member: &datastore.OrganisationMember{
					UID:            "abc",
					OrganisationID: "org1",
					Role: auth.Role{
						Type:     auth.RoleSuperUser,
						Project:  "1234",
						Endpoint: "",
					},
				},
			},
			wantErr:     true,
			wantErrCode: http.StatusBadRequest,
			wantErrMsg:  "failed to create api key",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			ss := provideSecurityService(ctrl)

			if tt.dbFn != nil {
				tt.dbFn(ss)
			}

			apiKey, keyString, err := ss.RegenerateProjectAPIKey(tt.args.ctx, tt.args.project, tt.args.member)
			if tt.wantErr {
				require.NotNil(t, err)
				require.Equal(t, tt.wantErrCode, err.(*util.ServiceError).ErrCode())
				require.Equal(t, tt.wantErrMsg, err.(*util.ServiceError).Error())
				return
			}

			require.Nil(t, err)
			require.NotEmpty(t, keyString)

			stripVariableFields(t, "apiKey", apiKey)
			require.Equal(t, tt.wantAPIKey, apiKey)
		})
	}
}
