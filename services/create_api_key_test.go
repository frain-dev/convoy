package services

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/frain-dev/convoy/auth"
	"github.com/frain-dev/convoy/mocks"
	"github.com/golang/mock/gomock"
	"github.com/guregu/null/v5"
	"github.com/stretchr/testify/require"

	"github.com/frain-dev/convoy/api/models"
	"github.com/frain-dev/convoy/datastore"
)

func provideCreateAPIKeyService(ctrl *gomock.Controller, member *datastore.OrganisationMember, newApiKey *models.APIKey) *CreateAPIKeyService {
	return &CreateAPIKeyService{
		ProjectRepo: mocks.NewMockProjectRepository(ctrl),
		APIKeyRepo:  mocks.NewMockAPIKeyRepository(ctrl),
		Member:      member,
		NewApiKey:   newApiKey,
	}
}

func TestCreateAPIKeyService_Run(t *testing.T) {
	ctx := context.Background()
	type args struct {
		ctx       context.Context
		newApiKey *models.APIKey
		member    *datastore.OrganisationMember
	}
	expires := null.NewTime(time.Now().Add(time.Hour), true)
	tests := []struct {
		name       string
		args       args
		wantAPIKey *datastore.APIKey
		dbFn       func(ss *CreateAPIKeyService)
		wantErr    bool
		wantErrMsg string
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
			dbFn: func(ss *CreateAPIKeyService) {
				g, _ := ss.ProjectRepo.(*mocks.MockProjectRepository)
				g.EXPECT().FetchProjectByID(gomock.Any(), gomock.Any()).Times(1).Return(&datastore.Project{UID: "abc", OrganisationID: "1234"}, nil)

				a, _ := ss.APIKeyRepo.(*mocks.MockAPIKeyRepository)
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
			wantErr:    true,
			wantErrMsg: "expiry date is invalid",
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
			wantErr:    true,
			wantErrMsg: "invalid api key role",
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
			dbFn: func(ss *CreateAPIKeyService) {
				g, _ := ss.ProjectRepo.(*mocks.MockProjectRepository)
				g.EXPECT().FetchProjectByID(gomock.Any(), "1234").
					Times(1).Return(nil, errors.New("failed"))
			},
			wantErr:    true,
			wantErrMsg: "failed to fetch project by id",
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
			dbFn: func(ss *CreateAPIKeyService) {
				g, _ := ss.ProjectRepo.(*mocks.MockProjectRepository)
				g.EXPECT().FetchProjectByID(gomock.Any(), "1234").
					Times(1).Return(&datastore.Project{UID: "1234", OrganisationID: "555"}, nil)
			},
			wantErr:    true,
			wantErrMsg: "unauthorized to access project",
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
			dbFn: func(ss *CreateAPIKeyService) {
				g, _ := ss.ProjectRepo.(*mocks.MockProjectRepository)
				g.EXPECT().FetchProjectByID(gomock.Any(), "1234").
					Times(1).Return(&datastore.Project{UID: "1234", OrganisationID: "555"}, nil)
			},
			wantErr:    true,
			wantErrMsg: "unauthorized to access project",
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
			dbFn: func(ss *CreateAPIKeyService) {
				g, _ := ss.ProjectRepo.(*mocks.MockProjectRepository)
				g.EXPECT().FetchProjectByID(gomock.Any(), "1234").
					Times(1).Return(&datastore.Project{UID: "1234", OrganisationID: "1234"}, nil)

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
			ss := provideCreateAPIKeyService(ctrl, tc.args.member, tc.args.newApiKey)

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

			stripVariableFields(t, "apiKey", apiKey)
			require.Equal(t, tc.wantAPIKey, apiKey)
		})
	}
}
