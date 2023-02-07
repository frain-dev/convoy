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

func provideProjectService(ctrl *gomock.Controller) *ProjectService {
	projectRepo := mocks.NewMockProjectRepository(ctrl)
	eventRepo := mocks.NewMockEventRepository(ctrl)
	eventDeliveryRepo := mocks.NewMockEventDeliveryRepository(ctrl)
	apiKeyRepo := mocks.NewMockAPIKeyRepository(ctrl)
	cache := mocks.NewMockCache(ctrl)
	return NewProjectService(apiKeyRepo, projectRepo, eventRepo, eventDeliveryRepo, nooplimiter.NewNoopLimiter(), cache)
}

func TestProjectService_CreateProject(t *testing.T) {
	ctx := context.Background()
	type args struct {
		ctx        context.Context
		newProject *models.Project
		org        *datastore.Organisation
		member     *datastore.OrganisationMember
	}
	tests := []struct {
		name        string
		args        args
		wantProject *datastore.Project
		dbFn        func(gs *ProjectService)
		wantErr     bool
		wantErrCode int
		wantErrMsg  string
	}{
		{
			name: "should_create_outgoing_project",
			args: args{
				ctx: ctx,
				newProject: &models.Project{
					Name:              "test_project",
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
						ReplayAttacks: true,
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
			wantProject: &datastore.Project{
				Name:              "test_project",
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
					ReplayAttacks:   true,
				},
			},
			wantErr: false,
		},
		{
			name: "should_create_incoming_project",
			args: args{
				ctx: ctx,
				newProject: &models.Project{
					Name:              "test_project",
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
						ReplayAttacks: true,
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
			wantProject: &datastore.Project{
				Name:              "test_project",
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
					ReplayAttacks:   true,
				},
			},
			wantErr: false,
		},
		{
			name: "should_create_incoming_project_with_defaults",
			args: args{
				ctx: ctx,
				newProject: &models.Project{
					Name:    "test_project_1",
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
			wantProject: &datastore.Project{
				Name:              "test_project_1",
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
					ReplayAttacks:   false,
				},
			},
			wantErr: false,
		},
		{
			name: "should_create_outgoing_project_with_defaults",
			args: args{
				ctx: ctx,
				newProject: &models.Project{
					Name:    "test_project",
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
			wantProject: &datastore.Project{
				Name:              "test_project",
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
					ReplayAttacks:   false,
				},
			},
			wantErr: false,
		},
		{
			name: "should_fail_to_create_project",
			args: args{
				ctx: ctx,
				newProject: &models.Project{
					Name:    "test_project",
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
						ReplayAttacks: true,
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
			wantErrMsg:  "failed to create project",
		},
		{
			name: "should_fail_to_create_default_api_key_for_project",
			args: args{
				ctx: ctx,
				newProject: &models.Project{
					Name:    "test_project_1",
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
			name: "should_error_for_duplicate_project_name",
			args: args{
				ctx: ctx,
				newProject: &models.Project{
					Name:    "test_project",
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
						ReplayAttacks: true,
					},
				},
				org:    &datastore.Organisation{UID: "1234"},
				member: &datastore.OrganisationMember{},
			},
			dbFn: func(gs *ProjectService) {
				a, _ := gs.projectRepo.(*mocks.MockProjectRepository)
				a.EXPECT().CreateProject(gomock.Any(), gomock.Any()).
					Times(1).Return(datastore.ErrDuplicateProjectName)
			},
			wantErr:     true,
			wantErrCode: http.StatusBadRequest,
			wantErrMsg:  "a project with this name already exists",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			gs := provideProjectService(ctrl)

			// Arrange Expectations
			if tc.dbFn != nil {
				tc.dbFn(gs)
			}

			project, apiKey, err := gs.CreateProject(tc.args.ctx, tc.args.newProject, tc.args.org, tc.args.member)
			if tc.wantErr {
				require.NotNil(t, err)
				require.Equal(t, tc.wantErrCode, err.(*util.ServiceError).ErrCode())
				require.Equal(t, tc.wantErrMsg, err.(*util.ServiceError).Error())
				return
			}

			// fmt.Println("eee", err.Error())
			require.Nil(t, err)
			require.NotEmpty(t, project.UID)
			require.NotEmpty(t, project.ID)
			require.NotEmpty(t, project.CreatedAt)
			require.NotEmpty(t, project.UpdatedAt)
			require.Empty(t, project.DeletedAt)

			require.Equal(t, project.Name+"'s default key", apiKey.Name)
			require.Equal(t, project.UID, apiKey.Role.Project)
			require.Equal(t, auth.RoleAdmin, apiKey.Role.Type)
			require.NotEmpty(t, apiKey.ExpiresAt)
			require.NotEmpty(t, apiKey.UID)
			require.NotEmpty(t, apiKey.Key)
			require.NotEmpty(t, apiKey.CreatedAt)

			stripVariableFields(t, "project", project)
			require.Equal(t, tc.wantProject, project)
		})
	}
}

func TestProjectService_UpdateProject(t *testing.T) {
	ctx := context.Background()

	type args struct {
		ctx     context.Context
		project *datastore.Project
		update  *models.UpdateProject
	}
	tests := []struct {
		name        string
		args        args
		wantErr     bool
		wantProject *datastore.Project
		dbFn        func(gs *ProjectService)
		wantErrCode int
		wantErrMsg  string
	}{
		{
			name: "should_update_project",
			args: args{
				ctx: ctx,
				project: &datastore.Project{
					UID:     "12345",
					Name:    "test_project",
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
						RateLimit:     &datastore.DefaultRateLimitConfig,
						ReplayAttacks: true,
					},
				},
				update: &models.UpdateProject{
					Name:    "test_project",
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
						RateLimit:     &datastore.DefaultRateLimitConfig,
						ReplayAttacks: true,
					},
				},
			},
			wantProject: &datastore.Project{
				UID:     "12345",
				Name:    "test_project",
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
					RateLimit:     &datastore.DefaultRateLimitConfig,
					ReplayAttacks: true,
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
				ctx:     ctx,
				project: &datastore.Project{UID: "12345"},
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
						ReplayAttacks: true,
					},
				},
			},
			wantErr:     true,
			wantErrCode: http.StatusBadRequest,
			wantErrMsg:  "name:please provide a valid name",
		},
		{
			name: "should_fail_to_update_project",
			args: args{
				ctx:     ctx,
				project: &datastore.Project{UID: "12345"},
				update: &models.UpdateProject{
					Name:    "test_project",
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
						ReplayAttacks: true,
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
			gs := provideProjectService(ctrl)

			// Arrange Expectations
			if tc.dbFn != nil {
				tc.dbFn(gs)
			}

			project, err := gs.UpdateProject(tc.args.ctx, tc.args.project, tc.args.update)
			if tc.wantErr {
				require.NotNil(t, err)
				require.Equal(t, tc.wantErrCode, err.(*util.ServiceError).ErrCode())
				require.Equal(t, tc.wantErrMsg, err.(*util.ServiceError).Error())
				return
			}

			require.Nil(t, err)
			c1 := tc.wantProject.Config
			c2 := project.Config

			tc.wantProject.Config = nil
			project.Config = nil
			require.Equal(t, tc.wantProject, project)
			require.Equal(t, c1, c2)
		})
	}
}

func TestProjectService_GetProjects(t *testing.T) {
	ctx := context.Background()
	type args struct {
		ctx    context.Context
		filter *datastore.ProjectFilter
	}
	tests := []struct {
		name         string
		args         args
		wantErr      bool
		wantProjects []*datastore.Project
		dbFn         func(gs *ProjectService)
		wantErrCode  int
		wantErrMsg   string
	}{
		{
			name: "should_get_projects",
			args: args{
				ctx:    ctx,
				filter: &datastore.ProjectFilter{Names: []string{"default_project"}},
			},
			dbFn: func(gs *ProjectService) {
				g, _ := gs.projectRepo.(*mocks.MockProjectRepository)
				g.EXPECT().LoadProjects(gomock.Any(), &datastore.ProjectFilter{Names: []string{"default_project"}}).
					Times(1).Return([]*datastore.Project{
					{UID: "123"},
					{UID: "abc"},
				}, nil)

				g.EXPECT().FillProjectsStatistics(gomock.Any(), gomock.Any()).Times(2).DoAndReturn(func(ctx context.Context, project *datastore.Project) error {
					project.Statistics = &datastore.ProjectStatistics{
						MessagesSent: 1,
						TotalApps:    1,
					}

					return nil
				})
			},
			wantProjects: []*datastore.Project{
				{
					UID: "123",
					Statistics: &datastore.ProjectStatistics{
						MessagesSent: 1,
						TotalApps:    1,
					},
				},
				{
					UID: "abc",
					Statistics: &datastore.ProjectStatistics{
						MessagesSent: 1,
						TotalApps:    1,
					},
				},
			},
			wantErr: false,
		},
		{
			name: "should_get_projects_trims-whitespaces-from-query",
			args: args{
				ctx:    ctx,
				filter: &datastore.ProjectFilter{Names: []string{" default_project "}},
			},
			dbFn: func(gs *ProjectService) {
				g, _ := gs.projectRepo.(*mocks.MockProjectRepository)
				g.EXPECT().LoadProjects(gomock.Any(), &datastore.ProjectFilter{Names: []string{"default_project"}}).
					Times(1).Return([]*datastore.Project{
					{UID: "123"},
					{UID: "abc"},
				}, nil)

				g.EXPECT().FillProjectsStatistics(gomock.Any(), gomock.Any()).Times(2).DoAndReturn(func(ctx context.Context, project *datastore.Project) error {
					project.Statistics = &datastore.ProjectStatistics{
						MessagesSent: 1,
						TotalApps:    1,
					}

					return nil
				})
			},
			wantProjects: []*datastore.Project{
				{
					UID: "123",
					Statistics: &datastore.ProjectStatistics{
						MessagesSent: 1,
						TotalApps:    1,
					},
				},
				{
					UID: "abc",
					Statistics: &datastore.ProjectStatistics{
						MessagesSent: 1,
						TotalApps:    1,
					},
				},
			},
			wantErr: false,
		},
		{
			name: "should_get_projects_trims-whitespaces-from-query-retains-case",
			args: args{
				ctx:    ctx,
				filter: &datastore.ProjectFilter{Names: []string{"  deFault_Project"}},
			},
			dbFn: func(gs *ProjectService) {
				g, _ := gs.projectRepo.(*mocks.MockProjectRepository)
				g.EXPECT().LoadProjects(gomock.Any(), &datastore.ProjectFilter{Names: []string{"deFault_Project"}}).
					Times(1).Return([]*datastore.Project{
					{UID: "123"},
					{UID: "abc"},
				}, nil)

				g.EXPECT().FillProjectsStatistics(gomock.Any(), gomock.Any()).Times(2).DoAndReturn(func(ctx context.Context, project *datastore.Project) error {
					project.Statistics = &datastore.ProjectStatistics{
						MessagesSent: 1,
						TotalApps:    1,
					}

					return nil
				})
			},
			wantProjects: []*datastore.Project{
				{
					UID: "123",
					Statistics: &datastore.ProjectStatistics{
						MessagesSent: 1,
						TotalApps:    1,
					},
				},
				{
					UID: "abc",
					Statistics: &datastore.ProjectStatistics{
						MessagesSent: 1,
						TotalApps:    1,
					},
				},
			},
			wantErr: false,
		},
		{
			name: "should_fail_to_get_projects",
			args: args{
				ctx:    ctx,
				filter: &datastore.ProjectFilter{Names: []string{"default_project"}},
			},
			dbFn: func(gs *ProjectService) {
				g, _ := gs.projectRepo.(*mocks.MockProjectRepository)
				g.EXPECT().LoadProjects(gomock.Any(), &datastore.ProjectFilter{Names: []string{"default_project"}}).
					Times(1).Return(nil, errors.New("failed"))
			},
			wantErr:     true,
			wantErrCode: http.StatusBadRequest,
			wantErrMsg:  "an error occurred while fetching projects",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			gs := provideProjectService(ctrl)

			// Arrange Expectations
			if tc.dbFn != nil {
				tc.dbFn(gs)
			}

			projects, err := gs.GetProjects(tc.args.ctx, tc.args.filter)
			if tc.wantErr {
				require.NotNil(t, err)
				require.Equal(t, tc.wantErrCode, err.(*util.ServiceError).ErrCode())
				require.Equal(t, tc.wantErrMsg, err.(*util.ServiceError).Error())
				return
			}

			require.Nil(t, err)
			require.Equal(t, tc.wantProjects, projects)
		})
	}
}

func TestProjectService_FillProjectStatistics(t *testing.T) {
	ctx := context.Background()

	type args struct {
		ctx context.Context
		g   *datastore.Project
	}
	tests := []struct {
		name        string
		args        args
		dbFn        func(gs *ProjectService)
		wantProject *datastore.Project
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
				g.EXPECT().FillProjectsStatistics(gomock.Any(), gomock.Any()).Times(1).DoAndReturn(func(ctx context.Context, project *datastore.Project) error {
					project.Statistics = &datastore.ProjectStatistics{
						MessagesSent: 1,
						TotalApps:    1,
					}
					return nil
				})
			},
			wantProject: &datastore.Project{
				UID: "1234",
				Statistics: &datastore.ProjectStatistics{
					MessagesSent: 1,
					TotalApps:    1,
				},
			},
			wantErr: false,
		},
		{
			name: "should_fail_to_fill_project_statistics",
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
			wantErrMsg:  "failed to count project statistics",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			gs := provideProjectService(ctrl)

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
			require.Equal(t, tc.wantProject, tc.args.g)
		})
	}
}

func TestProjectService_DeleteProject(t *testing.T) {
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
			name: "should_delete_project",
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
			name: "should_fail_to_delete_project",
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
			wantErrMsg:  "failed to delete project",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			gs := provideProjectService(ctrl)

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
