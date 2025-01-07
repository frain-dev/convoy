package services

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/frain-dev/convoy/auth"
	"github.com/frain-dev/convoy/mocks"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/frain-dev/convoy/datastore"
)

func provideRegenerateProjectAPIKeyService(ctrl *gomock.Controller, project *datastore.Project, member *datastore.OrganisationMember) *RegenerateProjectAPIKeyService {
	return &RegenerateProjectAPIKeyService{
		ProjectRepo: mocks.NewMockProjectRepository(ctrl),
		UserRepo:    mocks.NewMockUserRepository(ctrl),
		APIKeyRepo:  mocks.NewMockAPIKeyRepository(ctrl),
		Project:     project,
		Member:      member,
	}
}

func TestRegenerateProjectAPIKeyService_Run(t *testing.T) {
	ctx := context.Background()
	type args struct {
		ctx     context.Context
		project *datastore.Project
		member  *datastore.OrganisationMember
	}
	tests := []struct {
		name        string
		dbFn        func(ss *RegenerateProjectAPIKeyService)
		args        args
		wantAPIKey  *datastore.APIKey
		wantErr     bool
		wantErrCode int
		wantErrMsg  string
	}{
		{
			name: "should_regenerate_project_api_key",
			dbFn: func(ss *RegenerateProjectAPIKeyService) {
				a, _ := ss.APIKeyRepo.(*mocks.MockAPIKeyRepository)
				a.EXPECT().FindAPIKeyByProjectID(gomock.Any(), "1234").Times(1).Return(&datastore.APIKey{
					UID: "45678",
					Role: auth.Role{
						Type:     auth.RoleAdmin,
						Project:  "1234",
						Endpoint: "",
					},
				}, nil)

				a.EXPECT().RevokeAPIKeys(gomock.Any(), []string{"45678"}).Times(1).Return(nil)

				g, _ := ss.ProjectRepo.(*mocks.MockProjectRepository)
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
						Type:     auth.RoleOrganisationAdmin,
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
			dbFn: func(ss *RegenerateProjectAPIKeyService) {
				a, _ := ss.APIKeyRepo.(*mocks.MockAPIKeyRepository)
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
						Type:     auth.RoleOrganisationAdmin,
						Project:  "1234",
						Endpoint: "",
					},
				},
			},
			wantErr:     true,
			wantErrCode: http.StatusBadRequest,
			wantErrMsg:  "failed to fetch api project key",
		},
		{
			name: "should_error_for_non_superuser",
			dbFn: func(ss *RegenerateProjectAPIKeyService) {},
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
						Type:     auth.RoleAdmin,
						Project:  "1234",
						Endpoint: "",
					},
				},
			},
			wantErr:     true,
			wantErrCode: http.StatusBadRequest,
			wantErrMsg:  "unauthorized to access project",
		},
		{
			name: "should_fail_to_revoke_api_key",
			dbFn: func(ss *RegenerateProjectAPIKeyService) {
				a, _ := ss.APIKeyRepo.(*mocks.MockAPIKeyRepository)
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
						Type:     auth.RoleOrganisationAdmin,
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
			dbFn: func(ss *RegenerateProjectAPIKeyService) {
				a, _ := ss.APIKeyRepo.(*mocks.MockAPIKeyRepository)
				a.EXPECT().FindAPIKeyByProjectID(gomock.Any(), "1234").Times(1).Return(&datastore.APIKey{
					UID: "45678",
					Role: auth.Role{
						Type:     auth.RoleAdmin,
						Project:  "1234",
						Endpoint: "",
					},
				}, nil)

				a.EXPECT().RevokeAPIKeys(gomock.Any(), []string{"45678"}).Times(1).Return(nil)

				g, _ := ss.ProjectRepo.(*mocks.MockProjectRepository)
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
						Type:     auth.RoleOrganisationAdmin,
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
			ss := provideRegenerateProjectAPIKeyService(ctrl, tt.args.project, tt.args.member)

			if tt.dbFn != nil {
				tt.dbFn(ss)
			}

			apiKey, keyString, err := ss.Run(tt.args.ctx)
			if tt.wantErr {
				require.NotNil(t, err)
				require.Equal(t, tt.wantErrMsg, err.(*ServiceError).Error())
				return
			}

			require.Nil(t, err)
			require.NotEmpty(t, keyString)

			stripVariableFields(t, "apiKey", apiKey)
			require.Equal(t, tt.wantAPIKey, apiKey)
		})
	}
}
