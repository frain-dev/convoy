package services

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/frain-dev/convoy/mocks"
	"github.com/golang/mock/gomock"

	"github.com/frain-dev/convoy/auth"
	"github.com/frain-dev/convoy/datastore"
)

func provideUpdateAPIKeyService(ctrl *gomock.Controller, uid string, role *auth.Role) *UpdateAPIKeyService {
	return &UpdateAPIKeyService{
		ProjectRepo: mocks.NewMockProjectRepository(ctrl),
		UserRepo:    mocks.NewMockUserRepository(ctrl),
		APIKeyRepo:  mocks.NewMockAPIKeyRepository(ctrl),
		UID:         uid,
		Role:        role,
	}
}

func TestUpdateAPIKeyService_Run(t *testing.T) {
	ctx := context.Background()
	type args struct {
		uid  string
		role *auth.Role
		ctx  context.Context
	}

	tests := []struct {
		name       string
		args       args
		dbFn       func(co *UpdateAPIKeyService)
		wantAPIKey *datastore.APIKey
		wantErr    bool
		wantErrMsg string
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
			dbFn: func(ss *UpdateAPIKeyService) {
				g, _ := ss.ProjectRepo.(*mocks.MockProjectRepository)
				g.EXPECT().FetchProjectByID(gomock.Any(), "1234").
					Times(1).Return(&datastore.Project{UID: "1234"},
					nil)

				a, _ := ss.APIKeyRepo.(*mocks.MockAPIKeyRepository)
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
			wantErr:    true,
			wantErrMsg: "key id is empty",
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
			wantErr:    true,
			wantErrMsg: "invalid api key role",
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
			dbFn: func(ss *UpdateAPIKeyService) {
				g, _ := ss.ProjectRepo.(*mocks.MockProjectRepository)
				g.EXPECT().FetchProjectByID(gomock.Any(), "1234").
					Times(1).Return(nil, errors.New("failed"))
			},
			wantErr:    true,
			wantErrMsg: "invalid project",
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
			dbFn: func(ss *UpdateAPIKeyService) {
				g, _ := ss.ProjectRepo.(*mocks.MockProjectRepository)
				g.EXPECT().FetchProjectByID(gomock.Any(), "1234").
					Times(1).Return(&datastore.Project{UID: "1234"},
					nil)

				a, _ := ss.APIKeyRepo.(*mocks.MockAPIKeyRepository)
				a.EXPECT().FindAPIKeyByID(gomock.Any(), "1234").
					Times(1).Return(nil, errors.New("failed"))
			},
			wantErr:    true,
			wantErrMsg: "failed to fetch api key",
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
			dbFn: func(ss *UpdateAPIKeyService) {
				g, _ := ss.ProjectRepo.(*mocks.MockProjectRepository)
				g.EXPECT().FetchProjectByID(gomock.Any(), "1234").
					Times(1).Return(&datastore.Project{UID: "1234"},
					nil)

				a, _ := ss.APIKeyRepo.(*mocks.MockAPIKeyRepository)
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
			wantErr:    true,
			wantErrMsg: "failed to update api key",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			ua := provideUpdateAPIKeyService(ctrl, tt.args.uid, tt.args.role)

			// Arrange Expectations
			if tt.dbFn != nil {
				tt.dbFn(ua)
			}

			apiKey, err := ua.Run(tt.args.ctx)
			if tt.wantErr {
				require.NotNil(t, err)
				require.Equal(t, tt.wantErrMsg, err.(*ServiceError).Error())
				return
			}

			require.Nil(t, err)
			require.Equal(t, tt.wantAPIKey, apiKey)
		})
	}
}
