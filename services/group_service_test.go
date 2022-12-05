package services

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/frain-dev/convoy/auth"
	"github.com/frain-dev/convoy/datastore"
	nooplimiter "github.com/frain-dev/convoy/limiter/noop"
	"github.com/frain-dev/convoy/mocks"
	"github.com/frain-dev/convoy/server/models"
	"github.com/frain-dev/convoy/util"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

func provideGroupService(ctrl *gomock.Controller) *ProjectService {
	projectRepo := mocks.NewMockProjectRepository(ctrl)
	eventRepo := mocks.NewMockEventRepository(ctrl)
	eventDeliveryRepo := mocks.NewMockEventDeliveryRepository(ctrl)
	apiKeyRepo := mocks.NewMockAPIKeyRepository(ctrl)
	cache := mocks.NewMockCache(ctrl)
	return NewProjectService(apiKeyRepo, projectRepo, eventRepo, eventDeliveryRepo, nooplimiter.NewNoopLimiter(), cache)
}

func TestGroupService_CreateProject(t *testing.T) {
	ctx := context.Background()
	type args struct {
		ctx      context.Context
		newGroup *models.Project
		org      *datastore.Organisation
		member   *datastore.OrganisationMember
	}
	tests := []struct {
		name        string
		args        args
		wantGroup   *datastore.Project
		dbFn        func(gs *ProjectService)
		wantErr     bool
		wantErrCode int
		wantErrMsg  string
	}{
		{
			name: "should_create_outgoing_group",
			args: args{
				ctx: ctx,
				newGroup: &models.Project{
					Name:              "test_group",
					Type:              "outgoing",
					LogoURL:           "https://google.com",
					RateLimit:         1000,
					RateLimitDuration: "1m",
					Config: &datastore.ProjectConfig{
						Signature: &datastore.SignatureConfiguration{
							Header: "X-Convoy-Signature",
						},
						Strategy: &datastore.StrategyConfiguration{
							Type:       "linear",
							Duration:   20,
							RetryCount: 4,
						},
						RateLimit: &datastore.RateLimitConfiguration{
							Count:    1000,
							Duration: 60,
						},
						DisableEndpoint: true,
						ReplayAttacks:   true,
					},
				},
				org: &datastore.Organisation{UID: "1234"},
				member: &datastore.OrganisationMember{
					UID:            "abc",
					OrganisationID: "1234",
					Role:           auth.Role{Type: auth.RoleSuperUser},
				},
			},
			dbFn: func(gs *ProjectService) {
				a, _ := gs.projectRepo.(*mocks.MockProjectRepository)
				a.EXPECT().CreateProject(gomock.Any(), gomock.Any()).
					Times(1).Return(nil)

				a.EXPECT().FetchProjectByID(gomock.Any(), gomock.Any()).Times(1).Return(&datastore.Project{UID: "abc", OrganisationID: "1234"}, nil)

				apiKeyRepo, _ := gs.apiKeyRepo.(*mocks.MockAPIKeyRepository)
				apiKeyRepo.EXPECT().CreateAPIKey(gomock.Any(), gomock.Any()).Times(1).Return(nil)
			},
			wantGroup: &datastore.Project{
				Name:              "test_group",
				Type:              "outgoing",
				LogoURL:           "https://google.com",
				RateLimit:         1000,
				OrganisationID:    "1234",
				RateLimitDuration: "1m",
				Config: &datastore.ProjectConfig{
					Signature: &datastore.SignatureConfiguration{
						Header: "X-Convoy-Signature",
					},
					Strategy: &datastore.StrategyConfiguration{
						Type:       "linear",
						Duration:   20,
						RetryCount: 4,
					},
					RateLimit: &datastore.RateLimitConfiguration{
						Count:    1000,
						Duration: 60,
					},
					RetentionPolicy: &datastore.DefaultRetentionPolicy,
					DisableEndpoint: true,
					ReplayAttacks:   true,
				},
			},
			wantErr: false,
		},
		{
			name: "should_create_incoming_group",
			args: args{
				ctx: ctx,
				newGroup: &models.Project{
					Name:              "test_group",
					Type:              "incoming",
					LogoURL:           "https://google.com",
					RateLimit:         1000,
					RateLimitDuration: "1m",
					Config: &datastore.ProjectConfig{
						Signature: &datastore.SignatureConfiguration{
							Header: "X-Convoy-Signature",
						},
						Strategy: &datastore.StrategyConfiguration{
							Type:       "linear",
							Duration:   20,
							RetryCount: 4,
						},
						RateLimit: &datastore.RateLimitConfiguration{
							Count:    1000,
							Duration: 60,
						},
						DisableEndpoint: true,
						ReplayAttacks:   true,
					},
				},
				org: &datastore.Organisation{UID: "1234"},
				member: &datastore.OrganisationMember{
					UID:            "abc",
					OrganisationID: "1234",
					Role:           auth.Role{Type: auth.RoleSuperUser},
				},
			},
			dbFn: func(gs *ProjectService) {
				a, _ := gs.projectRepo.(*mocks.MockProjectRepository)
				a.EXPECT().CreateProject(gomock.Any(), gomock.Any()).
					Times(1).Return(nil)

				a.EXPECT().FetchProjectByID(gomock.Any(), gomock.Any()).Times(1).Return(&datastore.Project{UID: "abc", OrganisationID: "1234"}, nil)

				apiKeyRepo, _ := gs.apiKeyRepo.(*mocks.MockAPIKeyRepository)
				apiKeyRepo.EXPECT().CreateAPIKey(gomock.Any(), gomock.Any()).Times(1).Return(nil)
			},
			wantGroup: &datastore.Project{
				Name:              "test_group",
				Type:              "incoming",
				LogoURL:           "https://google.com",
				OrganisationID:    "1234",
				RateLimit:         1000,
				RateLimitDuration: "1m",
				Config: &datastore.ProjectConfig{
					Signature: &datastore.SignatureConfiguration{
						Header: "X-Convoy-Signature",
					},
					Strategy: &datastore.StrategyConfiguration{
						Type:       "linear",
						Duration:   20,
						RetryCount: 4,
					},
					RateLimit: &datastore.RateLimitConfiguration{
						Count:    1000,
						Duration: 60,
					},
					RetentionPolicy: &datastore.DefaultRetentionPolicy,
					DisableEndpoint: true,
					ReplayAttacks:   true,
				},
			},
			wantErr: false,
		},
		{
			name: "should_create_incoming_group_with_defaults",
			args: args{
				ctx: ctx,
				newGroup: &models.Project{
					Name:    "test_group_1",
					Type:    "incoming",
					LogoURL: "https://google.com",
					Config:  &datastore.ProjectConfig{},
				},
				org: &datastore.Organisation{UID: "1234"},
				member: &datastore.OrganisationMember{
					UID:            "abc",
					OrganisationID: "1234",
					Role:           auth.Role{Type: auth.RoleSuperUser},
				},
			},
			dbFn: func(gs *ProjectService) {
				a, _ := gs.projectRepo.(*mocks.MockProjectRepository)
				a.EXPECT().CreateProject(gomock.Any(), gomock.Any()).
					Times(1).Return(nil)

				a.EXPECT().FetchProjectByID(gomock.Any(), gomock.Any()).Times(1).Return(&datastore.Project{UID: "abc", OrganisationID: "1234"}, nil)

				apiKeyRepo, _ := gs.apiKeyRepo.(*mocks.MockAPIKeyRepository)
				apiKeyRepo.EXPECT().CreateAPIKey(gomock.Any(), gomock.Any()).Times(1).Return(nil)
			},
			wantGroup: &datastore.Project{
				Name:              "test_group_1",
				Type:              "incoming",
				LogoURL:           "https://google.com",
				OrganisationID:    "1234",
				RateLimit:         5000,
				RateLimitDuration: "1m",
				Config: &datastore.ProjectConfig{
					Signature: &datastore.SignatureConfiguration{
						Header: "X-Convoy-Signature",
						Versions: []datastore.SignatureVersion{
							{
								Hash:     "SHA256",
								Encoding: datastore.HexEncoding,
							},
						},
					},
					Strategy:        &datastore.DefaultStrategyConfig,
					RateLimit:       &datastore.DefaultRateLimitConfig,
					RetentionPolicy: &datastore.DefaultRetentionPolicy,
					DisableEndpoint: false,
					ReplayAttacks:   false,
				},
			},
			wantErr: false,
		},
		{
			name: "should_create_outgoing_group_with_defaults",
			args: args{
				ctx: ctx,
				newGroup: &models.Project{
					Name:    "test_group",
					Type:    "outgoing",
					LogoURL: "https://google.com",
					Config: &datastore.ProjectConfig{
						Signature: &datastore.SignatureConfiguration{
							Header: "X-Convoy-Signature",
						},
					},
				},
				org: &datastore.Organisation{UID: "1234"},
				member: &datastore.OrganisationMember{
					UID:            "abc",
					OrganisationID: "1234",
					Role:           auth.Role{Type: auth.RoleSuperUser},
				},
			},
			dbFn: func(gs *ProjectService) {
				a, _ := gs.projectRepo.(*mocks.MockProjectRepository)
				a.EXPECT().CreateProject(gomock.Any(), gomock.Any()).
					Times(1).Return(nil)

				a.EXPECT().FetchProjectByID(gomock.Any(), gomock.Any()).Times(1).Return(&datastore.Project{UID: "abc", OrganisationID: "1234"}, nil)

				apiKeyRepo, _ := gs.apiKeyRepo.(*mocks.MockAPIKeyRepository)
				apiKeyRepo.EXPECT().CreateAPIKey(gomock.Any(), gomock.Any()).Times(1).Return(nil)
			},
			wantGroup: &datastore.Project{
				Name:              "test_group",
				Type:              "outgoing",
				LogoURL:           "https://google.com",
				RateLimit:         5000,
				OrganisationID:    "1234",
				RateLimitDuration: "1m",
				Config: &datastore.ProjectConfig{
					Signature: &datastore.SignatureConfiguration{
						Header: "X-Convoy-Signature",
					},
					Strategy:        &datastore.DefaultStrategyConfig,
					RateLimit:       &datastore.DefaultRateLimitConfig,
					RetentionPolicy: &datastore.DefaultRetentionPolicy,
					DisableEndpoint: false,
					ReplayAttacks:   false,
				},
			},
			wantErr: false,
		},
		{
			name: "should_fail_to_create_group",
			args: args{
				ctx: ctx,
				newGroup: &models.Project{
					Name:    "test_group",
					Type:    "incoming",
					LogoURL: "https://google.com",
					Config: &datastore.ProjectConfig{
						Signature: &datastore.SignatureConfiguration{
							Header: "X-Convoy-Signature",
						},
						Strategy: &datastore.StrategyConfiguration{
							Type:       "linear",
							Duration:   20,
							RetryCount: 4,
						},
						DisableEndpoint: true,
						ReplayAttacks:   true,
					},
				},
				org:    &datastore.Organisation{UID: "1234"},
				member: &datastore.OrganisationMember{},
			},
			dbFn: func(gs *ProjectService) {
				a, _ := gs.projectRepo.(*mocks.MockProjectRepository)
				a.EXPECT().CreateProject(gomock.Any(), gomock.Any()).
					Times(1).Return(errors.New("failed"))
			},
			wantErr:     true,
			wantErrCode: http.StatusBadRequest,
			wantErrMsg:  "failed to create group",
		},
		{
			name: "should_fail_to_create_default_api_key_for_group",
			args: args{
				ctx: ctx,
				newGroup: &models.Project{
					Name:    "test_group_1",
					Type:    "incoming",
					LogoURL: "https://google.com",
					Config:  &datastore.ProjectConfig{},
				},
				org: &datastore.Organisation{UID: "1234"},
				member: &datastore.OrganisationMember{
					UID:            "abc",
					OrganisationID: "1234",
					Role:           auth.Role{Type: auth.RoleSuperUser},
				},
			},
			dbFn: func(gs *ProjectService) {
				a, _ := gs.projectRepo.(*mocks.MockProjectRepository)
				a.EXPECT().CreateProject(gomock.Any(), gomock.Any()).
					Times(1).Return(nil)

				a.EXPECT().FetchProjectByID(gomock.Any(), gomock.Any()).Times(1).Return(&datastore.Project{UID: "abc", OrganisationID: "1234"}, nil)

				apiKeyRepo, _ := gs.apiKeyRepo.(*mocks.MockAPIKeyRepository)
				apiKeyRepo.EXPECT().CreateAPIKey(gomock.Any(), gomock.Any()).Times(1).Return(errors.New("failed"))
			},
			wantErr:     true,
			wantErrCode: http.StatusBadRequest,
			wantErrMsg:  "failed to create api key",
		},
		{
			name: "should_error_for_duplicate_group_name",
			args: args{
				ctx: ctx,
				newGroup: &models.Project{
					Name:    "test_group",
					Type:    "incoming",
					LogoURL: "https://google.com",
					Config: &datastore.ProjectConfig{
						Signature: &datastore.SignatureConfiguration{
							Header: "X-Convoy-Signature",
						},
						Strategy: &datastore.StrategyConfiguration{
							Type:       "linear",
							Duration:   20,
							RetryCount: 4,
						},
						DisableEndpoint: true,
						ReplayAttacks:   true,
					},
				},
				org:    &datastore.Organisation{UID: "1234"},
				member: &datastore.OrganisationMember{},
			},
			dbFn: func(gs *ProjectService) {
				a, _ := gs.projectRepo.(*mocks.MockProjectRepository)
				a.EXPECT().CreateProject(gomock.Any(), gomock.Any()).
					Times(1).Return(datastore.ErrDuplicateGroupName)
			},
			wantErr:     true,
			wantErrCode: http.StatusBadRequest,
			wantErrMsg:  "a group with this name already exists",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			gs := provideGroupService(ctrl)

			// Arrange Expectations
			if tc.dbFn != nil {
				tc.dbFn(gs)
			}

			group, apiKey, err := gs.CreateProject(tc.args.ctx, tc.args.newGroup, tc.args.org, tc.args.member)
			if tc.wantErr {
				require.NotNil(t, err)
				require.Equal(t, tc.wantErrCode, err.(*util.ServiceError).ErrCode())
				require.Equal(t, tc.wantErrMsg, err.(*util.ServiceError).Error())
				return
			}

			// fmt.Println("eee", err.Error())
			require.Nil(t, err)
			require.NotEmpty(t, group.UID)
			require.NotEmpty(t, group.ID)
			require.NotEmpty(t, group.CreatedAt)
			require.NotEmpty(t, group.UpdatedAt)
			require.Empty(t, group.DeletedAt)

			require.Equal(t, group.Name+"'s default key", apiKey.Name)
			require.Equal(t, group.UID, apiKey.Role.Group)
			require.Equal(t, auth.RoleAdmin, apiKey.Role.Type)
			require.NotEmpty(t, apiKey.ExpiresAt)
			require.NotEmpty(t, apiKey.UID)
			require.NotEmpty(t, apiKey.Key)
			require.NotEmpty(t, apiKey.CreatedAt)

			stripVariableFields(t, "group", group)
			require.Equal(t, tc.wantGroup, group)
		})
	}
}

func TestGroupService_UpdateProject(t *testing.T) {
	ctx := context.Background()

	type args struct {
		ctx    context.Context
		group  *datastore.Project
		update *models.UpdateProject
	}
	tests := []struct {
		name        string
		args        args
		wantErr     bool
		wantGroup   *datastore.Project
		dbFn        func(gs *ProjectService)
		wantErrCode int
		wantErrMsg  string
	}{
		{
			name: "should_update_group",
			args: args{
				ctx: ctx,
				group: &datastore.Project{
					UID:     "12345",
					Name:    "test_group",
					Type:    "incoming",
					LogoURL: "https://google.com",
					Config: &datastore.ProjectConfig{
						Signature: &datastore.SignatureConfiguration{
							Header: "X-Convoy-Signature",
						},
						Strategy: &datastore.StrategyConfiguration{
							Type:       "linear",
							Duration:   20,
							RetryCount: 4,
						},
						RateLimit:       &datastore.DefaultRateLimitConfig,
						DisableEndpoint: true,
						ReplayAttacks:   true,
					},
				},
				update: &models.UpdateProject{
					Name:    "test_group",
					LogoURL: "https://google.com",
					Config: &datastore.ProjectConfig{
						Signature: &datastore.SignatureConfiguration{
							Header: "X-Convoy-Signature",
						},
						Strategy: &datastore.StrategyConfiguration{
							Type:       "linear",
							Duration:   20,
							RetryCount: 4,
						},
						RateLimit:       &datastore.DefaultRateLimitConfig,
						DisableEndpoint: true,
						ReplayAttacks:   true,
					},
				},
			},
			wantGroup: &datastore.Project{
				UID:     "12345",
				Name:    "test_group",
				Type:    "incoming",
				LogoURL: "https://google.com",
				Config: &datastore.ProjectConfig{
					Signature: &datastore.SignatureConfiguration{
						Header: "X-Convoy-Signature",
					},
					Strategy: &datastore.StrategyConfiguration{
						Type:       "linear",
						Duration:   20,
						RetryCount: 4,
					},
					RateLimit:       &datastore.DefaultRateLimitConfig,
					DisableEndpoint: true,
					ReplayAttacks:   true,
				},
			},
			dbFn: func(gs *ProjectService) {
				a, _ := gs.projectRepo.(*mocks.MockProjectRepository)
				a.EXPECT().UpdateProject(gomock.Any(), gomock.Any()).Times(1).Return(nil)

				c, _ := gs.cache.(*mocks.MockCache)
				c.EXPECT().Set(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Times(1)
			},
		},
		{
			name: "should_error_for_empty_name",
			args: args{
				ctx:   ctx,
				group: &datastore.Project{UID: "12345"},
				update: &models.UpdateProject{
					Name:    "",
					LogoURL: "https://google.com",
					Config: &datastore.ProjectConfig{
						Signature: &datastore.SignatureConfiguration{
							Header: "X-Convoy-Signature",
						},
						Strategy: &datastore.StrategyConfiguration{
							Type:       "linear",
							Duration:   20,
							RetryCount: 4,
						},
						DisableEndpoint: true,
						ReplayAttacks:   true,
					},
				},
			},
			wantErr:     true,
			wantErrCode: http.StatusBadRequest,
			wantErrMsg:  "name:please provide a valid name",
		},
		{
			name: "should_fail_to_update_group",
			args: args{
				ctx:   ctx,
				group: &datastore.Project{UID: "12345"},
				update: &models.UpdateProject{
					Name:    "test_group",
					LogoURL: "https://google.com",
					Config: &datastore.ProjectConfig{
						Signature: &datastore.SignatureConfiguration{
							Header: "X-Convoy-Signature",
						},
						Strategy: &datastore.StrategyConfiguration{
							Type:       "linear",
							Duration:   20,
							RetryCount: 4,
						},
						DisableEndpoint: true,
						ReplayAttacks:   true,
					},
				},
			},
			dbFn: func(gs *ProjectService) {
				a, _ := gs.projectRepo.(*mocks.MockProjectRepository)
				a.EXPECT().UpdateProject(gomock.Any(), gomock.Any()).Times(1).Return(errors.New("failed"))
			},
			wantErr:     true,
			wantErrCode: http.StatusBadRequest,
			wantErrMsg:  "failed",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			gs := provideGroupService(ctrl)

			// Arrange Expectations
			if tc.dbFn != nil {
				tc.dbFn(gs)
			}

			group, err := gs.UpdateProject(tc.args.ctx, tc.args.group, tc.args.update)
			if tc.wantErr {
				require.NotNil(t, err)
				require.Equal(t, tc.wantErrCode, err.(*util.ServiceError).ErrCode())
				require.Equal(t, tc.wantErrMsg, err.(*util.ServiceError).Error())
				return
			}

			require.Nil(t, err)
			c1 := tc.wantGroup.Config
			c2 := group.Config

			tc.wantGroup.Config = nil
			group.Config = nil
			require.Equal(t, tc.wantGroup, group)
			require.Equal(t, c1, c2)
		})
	}
}

func TestGroupService_GetGroups(t *testing.T) {
	ctx := context.Background()
	type args struct {
		ctx    context.Context
		filter *datastore.GroupFilter
	}
	tests := []struct {
		name        string
		args        args
		wantErr     bool
		wantGroups  []*datastore.Project
		dbFn        func(gs *ProjectService)
		wantErrCode int
		wantErrMsg  string
	}{
		{
			name: "should_get_groups",
			args: args{
				ctx:    ctx,
				filter: &datastore.GroupFilter{Names: []string{"default_group"}},
			},
			dbFn: func(gs *ProjectService) {
				g, _ := gs.projectRepo.(*mocks.MockProjectRepository)
				g.EXPECT().LoadProjects(gomock.Any(), &datastore.GroupFilter{Names: []string{"default_group"}}).
					Times(1).Return([]*datastore.Project{
					{UID: "123"},
					{UID: "abc"},
				}, nil)

				g.EXPECT().FillProjectsStatistics(gomock.Any(), gomock.Any()).Times(1).DoAndReturn(func(ctx context.Context, groups []*datastore.Project) error {
					groups[0].Statistics = &datastore.GroupStatistics{
						MessagesSent: 1,
						TotalApps:    1,
					}

					groups[1].Statistics = &datastore.GroupStatistics{
						MessagesSent: 1,
						TotalApps:    1,
					}
					return nil
				})
			},
			wantGroups: []*datastore.Project{
				{
					UID: "123",
					Statistics: &datastore.GroupStatistics{
						MessagesSent: 1,
						TotalApps:    1,
					},
				},
				{
					UID: "abc",
					Statistics: &datastore.GroupStatistics{
						MessagesSent: 1,
						TotalApps:    1,
					},
				},
			},
			wantErr: false,
		},
		{
			name: "should_get_groups_trims-whitespaces-from-query",
			args: args{
				ctx:    ctx,
				filter: &datastore.GroupFilter{Names: []string{" default_group "}},
			},
			dbFn: func(gs *ProjectService) {
				g, _ := gs.projectRepo.(*mocks.MockProjectRepository)
				g.EXPECT().LoadProjects(gomock.Any(), &datastore.GroupFilter{Names: []string{"default_group"}}).
					Times(1).Return([]*datastore.Project{
					{UID: "123"},
					{UID: "abc"},
				}, nil)

				g.EXPECT().FillProjectsStatistics(gomock.Any(), gomock.Any()).Times(1).DoAndReturn(func(ctx context.Context, groups []*datastore.Project) error {
					groups[0].Statistics = &datastore.GroupStatistics{
						MessagesSent: 1,
						TotalApps:    1,
					}

					groups[1].Statistics = &datastore.GroupStatistics{
						MessagesSent: 1,
						TotalApps:    1,
					}
					return nil
				})
			},
			wantGroups: []*datastore.Project{
				{
					UID: "123",
					Statistics: &datastore.GroupStatistics{
						MessagesSent: 1,
						TotalApps:    1,
					},
				},
				{
					UID: "abc",
					Statistics: &datastore.GroupStatistics{
						MessagesSent: 1,
						TotalApps:    1,
					},
				},
			},
			wantErr: false,
		},
		{
			name: "should_get_groups_trims-whitespaces-from-query-retains-case",
			args: args{
				ctx:    ctx,
				filter: &datastore.GroupFilter{Names: []string{"  deFault_Group"}},
			},
			dbFn: func(gs *ProjectService) {
				g, _ := gs.projectRepo.(*mocks.MockProjectRepository)
				g.EXPECT().LoadProjects(gomock.Any(), &datastore.GroupFilter{Names: []string{"deFault_Group"}}).
					Times(1).Return([]*datastore.Project{
					{UID: "123"},
					{UID: "abc"},
				}, nil)

				g.EXPECT().FillProjectsStatistics(gomock.Any(), gomock.Any()).Times(1).DoAndReturn(func(ctx context.Context, groups []*datastore.Project) error {
					groups[0].Statistics = &datastore.GroupStatistics{
						MessagesSent: 1,
						TotalApps:    1,
					}

					groups[1].Statistics = &datastore.GroupStatistics{
						MessagesSent: 1,
						TotalApps:    1,
					}
					return nil
				})
			},
			wantGroups: []*datastore.Project{
				{
					UID: "123",
					Statistics: &datastore.GroupStatistics{
						MessagesSent: 1,
						TotalApps:    1,
					},
				},
				{
					UID: "abc",
					Statistics: &datastore.GroupStatistics{
						MessagesSent: 1,
						TotalApps:    1,
					},
				},
			},
			wantErr: false,
		},
		{
			name: "should_fail_to_get_groups",
			args: args{
				ctx:    ctx,
				filter: &datastore.GroupFilter{Names: []string{"default_group"}},
			},
			dbFn: func(gs *ProjectService) {
				g, _ := gs.projectRepo.(*mocks.MockProjectRepository)
				g.EXPECT().LoadProjects(gomock.Any(), &datastore.GroupFilter{Names: []string{"default_group"}}).
					Times(1).Return(nil, errors.New("failed"))
			},
			wantErr:     true,
			wantErrCode: http.StatusBadRequest,
			wantErrMsg:  "an error occurred while fetching Groups",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			gs := provideGroupService(ctrl)

			// Arrange Expectations
			if tc.dbFn != nil {
				tc.dbFn(gs)
			}

			group, err := gs.GetProjects(tc.args.ctx, tc.args.filter)
			if tc.wantErr {
				require.NotNil(t, err)
				require.Equal(t, tc.wantErrCode, err.(*util.ServiceError).ErrCode())
				require.Equal(t, tc.wantErrMsg, err.(*util.ServiceError).Error())
				return
			}

			require.Nil(t, err)
			require.Equal(t, tc.wantGroups, group)
		})
	}
}

func TestGroupService_FillProjectStatistics(t *testing.T) {
	ctx := context.Background()

	type args struct {
		ctx context.Context
		g   *datastore.Project
	}
	tests := []struct {
		name        string
		args        args
		dbFn        func(gs *ProjectService)
		wantGroup   *datastore.Project
		wantErr     bool
		wantErrCode int
		wantErrMsg  string
	}{
		{
			name: "should_fill_statistics",
			args: args{
				ctx: ctx,
				g:   &datastore.Project{UID: "1234"},
			},
			dbFn: func(gs *ProjectService) {
				g, _ := gs.projectRepo.(*mocks.MockProjectRepository)
				g.EXPECT().FillProjectsStatistics(gomock.Any(), gomock.Any()).Times(1).DoAndReturn(func(ctx context.Context, groups []*datastore.Project) error {
					groups[0].Statistics = &datastore.GroupStatistics{
						MessagesSent: 1,
						TotalApps:    1,
					}
					return nil
				})
			},
			wantGroup: &datastore.Project{
				UID: "1234",
				Statistics: &datastore.GroupStatistics{
					MessagesSent: 1,
					TotalApps:    1,
				},
			},
			wantErr: false,
		},
		{
			name: "should_fail_to_fill_group_statistics",
			args: args{
				ctx: ctx,
				g:   &datastore.Project{UID: "1234"},
			},
			dbFn: func(gs *ProjectService) {
				g, _ := gs.projectRepo.(*mocks.MockProjectRepository)
				g.EXPECT().FillProjectsStatistics(gomock.Any(), gomock.Any()).Times(1).Return(errors.New("failed"))
			},
			wantErr:     true,
			wantErrCode: http.StatusBadRequest,
			wantErrMsg:  "failed to count group statistics",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			gs := provideGroupService(ctrl)

			// Arrange Expectations
			if tc.dbFn != nil {
				tc.dbFn(gs)
			}

			err := gs.FillProjectStatistics(tc.args.ctx, []*datastore.Project{tc.args.g})
			if tc.wantErr {
				require.NotNil(t, err)
				require.Equal(t, tc.wantErrCode, err.(*util.ServiceError).ErrCode())
				require.Equal(t, tc.wantErrMsg, err.(*util.ServiceError).Error())
				return
			}

			require.Nil(t, err)
			require.Equal(t, tc.wantGroup, tc.args.g)
		})
	}
}

func TestGroupService_DeleteProject(t *testing.T) {
	ctx := context.Background()
	type args struct {
		ctx context.Context
		id  string
	}
	tests := []struct {
		name        string
		args        args
		wantErr     bool
		dbFn        func(gs *ProjectService)
		wantErrCode int
		wantErrMsg  string
	}{
		{
			name: "should_delete_group",
			args: args{
				ctx: ctx,
				id:  "12345",
			},
			dbFn: func(gs *ProjectService) {
				g, _ := gs.projectRepo.(*mocks.MockProjectRepository)
				g.EXPECT().DeleteProject(gomock.Any(), "12345").Times(1).Return(nil)
			},
			wantErr: false,
		},
		{
			name: "should_fail_to_delete_group",
			args: args{
				ctx: ctx,
				id:  "12345",
			},
			dbFn: func(gs *ProjectService) {
				g, _ := gs.projectRepo.(*mocks.MockProjectRepository)
				g.EXPECT().DeleteProject(gomock.Any(), "12345").Times(1).Return(errors.New("failed"))
			},
			wantErr:     true,
			wantErrCode: http.StatusBadRequest,
			wantErrMsg:  "failed to delete group",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			gs := provideGroupService(ctrl)

			// Arrange Expectations
			if tc.dbFn != nil {
				tc.dbFn(gs)
			}

			err := gs.DeleteProject(tc.args.ctx, tc.args.id)
			if tc.wantErr {
				require.NotNil(t, err)
				require.Equal(t, tc.wantErrCode, err.(*util.ServiceError).ErrCode())
				require.Equal(t, tc.wantErrMsg, err.(*util.ServiceError).Error())
				return
			}

			require.Nil(t, err)
		})
	}
}
