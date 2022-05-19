package services

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/mocks"
	"github.com/frain-dev/convoy/server/models"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

func provideAppService(ctrl *gomock.Controller) *AppService {
	appRepo := mocks.NewMockApplicationRepository(ctrl)
	eventRepo := mocks.NewMockEventRepository(ctrl)
	eventDeliveryRepo := mocks.NewMockEventDeliveryRepository(ctrl)
	eventQueue := mocks.NewMockQueuer(ctrl)
	cache := mocks.NewMockCache(ctrl)
	return NewAppService(appRepo, eventRepo, eventDeliveryRepo, eventQueue, cache)
}

func boolPtr(b bool) *bool {
	return &b
}
func stringPtr(s string) *string {
	return &s
}

func TestAppService_CreateApp(t *testing.T) {
	groupID := "1234567890"
	group := &datastore.Group{UID: groupID}

	type args struct {
		ctx    context.Context
		newApp *models.Application
		g      *datastore.Group
	}

	ctx := context.Background()
	tt := []struct {
		name        string
		args        args
		wantErr     bool
		wantErrMsg  string
		wantErrCode int
		wantApp     *datastore.Application
		dbFn        func(app *AppService)
	}{
		{
			name: "should_error_for_empty_name",
			args: args{
				ctx: ctx,
				newApp: &models.Application{
					AppName:         "",
					SupportEmail:    "app@test.com",
					IsDisabled:      false,
					SlackWebhookURL: "https:'//google.com",
				},
				g: group,
			},
			wantErr:     true,
			wantErrCode: http.StatusBadRequest,
			wantErrMsg:  "name:please provide your appName",
			dbFn:        func(app *AppService) {},
		},
		{
			name: "should_create_application",
			args: args{
				ctx: ctx,
				newApp: &models.Application{
					AppName:         "test_app",
					SupportEmail:    "app@test.com",
					IsDisabled:      false,
					SlackWebhookURL: "https://google.com",
				},
				g: group,
			},
			dbFn: func(app *AppService) {
				a, _ := app.appRepo.(*mocks.MockApplicationRepository)
				a.EXPECT().
					CreateApplication(gomock.Any(), gomock.Any(), gomock.Any()).Times(1).
					Return(nil)

				c, _ := app.cache.(*mocks.MockCache)
				c.EXPECT().Set(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any())
			},
			wantApp: &datastore.Application{
				GroupID:         groupID,
				Title:           "test_app",
				SupportEmail:    "app@test.com",
				SlackWebhookURL: "https://google.com",
				IsDisabled:      false,
				Endpoints:       []datastore.Endpoint{},
				Events:          0,
				DocumentStatus:  datastore.ActiveDocumentStatus,
			},
		},
		{
			name: "should_fail_to_create_application",
			args: args{
				ctx: ctx,
				newApp: &models.Application{
					AppName:         "test_app",
					SupportEmail:    "app@test.com",
					IsDisabled:      false,
					SlackWebhookURL: "https://google.com",
				},
				g: group,
			},
			dbFn: func(app *AppService) {
				a, _ := app.appRepo.(*mocks.MockApplicationRepository)
				a.EXPECT().
					CreateApplication(gomock.Any(), gomock.Any(), gomock.Any()).Times(1).
					Return(errors.New("failed"))
			},
			wantErr:     true,
			wantErrCode: http.StatusBadRequest,
			wantErrMsg:  "failed to create application",
		},
		{
			name: "should_error_for_app_name_not_unique",
			args: args{
				ctx: ctx,
				newApp: &models.Application{
					AppName:         "test_app",
					SupportEmail:    "app@test.com",
					IsDisabled:      false,
					SlackWebhookURL: "https://google.com",
				},
				g: group,
			},
			dbFn: func(app *AppService) {
				a, _ := app.appRepo.(*mocks.MockApplicationRepository)
				a.EXPECT().
					CreateApplication(gomock.Any(), gomock.Any(), gomock.Any()).Times(1).
					Return(datastore.ErrDuplicateAppName)
			},
			wantErr:     true,
			wantErrCode: http.StatusBadRequest,
			wantErrMsg:  "an application with this name exists: test_app",
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			as := provideAppService(ctrl)

			// Arrange Expectations
			if tc.dbFn != nil {
				tc.dbFn(as)
			}

			app, err := as.CreateApp(tc.args.ctx, tc.args.newApp, group)
			if tc.wantErr {
				require.NotNil(t, err)
				require.Equal(t, tc.wantErrCode, err.(*ServiceError).ErrCode())
				require.Equal(t, tc.wantErrMsg, err.(*ServiceError).Error())
				return
			}

			require.Nil(t, err)
			require.NotEmpty(t, app.UID)
			require.NotEmpty(t, app.ID)
			require.NotEmpty(t, app.CreatedAt)
			require.NotEmpty(t, app.UpdatedAt)
			require.Empty(t, app.DeletedAt)

			stripVariableFields(t, "application", app)
			require.Equal(t, tc.wantApp, app)
		})
	}
}

func TestAppService_LoadApplicationsPaged(t *testing.T) {

	ctx := context.Background()

	type args struct {
		ctx      context.Context
		uid      string
		q        string
		pageable datastore.Pageable
	}
	tests := []struct {
		name               string
		args               args
		dbFn               func(app *AppService)
		wantApps           []datastore.Application
		wantPaginationData datastore.PaginationData
		wantErr            bool
		wantErrObj         error
	}{
		{
			name: "should_load_apps",
			args: args{
				ctx: ctx,
				uid: "1234",
				q:   "test_app",
				pageable: datastore.Pageable{
					Page:    1,
					PerPage: 10,
					Sort:    1,
				},
			},
			wantApps: []datastore.Application{
				{UID: "123"},
				{UID: "abc"},
			},
			wantPaginationData: datastore.PaginationData{
				Total:     2,
				Page:      1,
				PerPage:   10,
				Prev:      0,
				Next:      2,
				TotalPage: 3,
			},
			dbFn: func(app *AppService) {
				a, _ := app.appRepo.(*mocks.MockApplicationRepository)
				a.EXPECT().
					LoadApplicationsPaged(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Times(1).
					Return([]datastore.Application{
						{UID: "123"},
						{UID: "abc"},
					}, datastore.PaginationData{
						Total:     2,
						Page:      1,
						PerPage:   10,
						Prev:      0,
						Next:      2,
						TotalPage: 3,
					}, nil)
			},
			wantErr: false,
		},
		{
			name: "should_fail_load_apps",
			args: args{
				ctx: ctx,
				uid: "1234",
				q:   "test_app",
				pageable: datastore.Pageable{
					Page:    1,
					PerPage: 10,
					Sort:    1,
				},
			},
			dbFn: func(app *AppService) {
				a, _ := app.appRepo.(*mocks.MockApplicationRepository)
				a.EXPECT().
					LoadApplicationsPaged(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Times(1).
					Return(nil, datastore.PaginationData{}, errors.New("failed"))
			},
			wantErr:    true,
			wantErrObj: NewServiceError(http.StatusInternalServerError, errors.New("an error occurred while fetching apps")),
		},
		{
			name: "should_load_apps_trims-whitespaces-from-query",
			args: args{
				ctx: ctx,
				uid: "uid",
				q:   " falsetto ",
				pageable: datastore.Pageable{
					Page:    1,
					PerPage: 10,
					Sort:    1,
				},
			},
			wantApps: []datastore.Application{
				{UID: "123"},
				{UID: "abc"},
			},
			wantPaginationData: datastore.PaginationData{
				Total:     2,
				Page:      1,
				PerPage:   10,
				Prev:      0,
				Next:      2,
				TotalPage: 3,
			},
			dbFn: func(app *AppService) {
				a, _ := app.appRepo.(*mocks.MockApplicationRepository)
				a.EXPECT().
					LoadApplicationsPaged(gomock.Any(), gomock.Any(), "falsetto", gomock.Any()).Times(1).
					Return([]datastore.Application{
						{UID: "123"},
						{UID: "abc"},
					}, datastore.PaginationData{
						Total:     2,
						Page:      1,
						PerPage:   10,
						Prev:      0,
						Next:      2,
						TotalPage: 3,
					}, nil)
			},
			wantErr: false,
		},
		{
			name: "should_load_apps_trims-whitespaces-from-query-retains-case",
			args: args{
				ctx: ctx,
				uid: "uid",
				q:   "   FalSetto  ",
				pageable: datastore.Pageable{
					Page:    1,
					PerPage: 10,
					Sort:    1,
				},
			},
			wantApps: []datastore.Application{
				{UID: "123"},
				{UID: "abc"},
			},
			wantPaginationData: datastore.PaginationData{
				Total:     2,
				Page:      1,
				PerPage:   10,
				Prev:      0,
				Next:      2,
				TotalPage: 3,
			},
			dbFn: func(app *AppService) {
				a, _ := app.appRepo.(*mocks.MockApplicationRepository)
				a.EXPECT().
					LoadApplicationsPaged(gomock.Any(), gomock.Any(), "FalSetto", gomock.Any()).Times(1).
					Return([]datastore.Application{
						{UID: "123"},
						{UID: "abc"},
					}, datastore.PaginationData{
						Total:     2,
						Page:      1,
						PerPage:   10,
						Prev:      0,
						Next:      2,
						TotalPage: 3,
					}, nil)
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			as := provideAppService(ctrl)

			// Arrange Expectations
			if tt.dbFn != nil {
				tt.dbFn(as)
			}

			apps, paginationData, err := as.LoadApplicationsPaged(tt.args.ctx, tt.args.uid, tt.args.q, tt.args.pageable)
			if tt.wantErr {
				require.NotNil(t, err)
				require.Equal(t, tt.wantErrObj, err)
				return
			}

			require.Nil(t, err)
			require.Equal(t, tt.wantApps, apps)
			require.Equal(t, tt.wantPaginationData, paginationData)
		})
	}
}

func TestAppService_UpdateApplication(t *testing.T) {
	ctx := context.Background()

	type args struct {
		ctx       context.Context
		appUpdate *models.UpdateApplication
		app       *datastore.Application
	}
	tests := []struct {
		name        string
		args        args
		wantApp     *datastore.Application
		dbFn        func(app *AppService)
		wantErr     bool
		wantErrMsg  string
		wantErrCode int
	}{
		{
			name: "should_update_app",
			args: args{
				ctx: ctx,
				appUpdate: &models.UpdateApplication{
					AppName:         stringPtr("app_testing"),
					SupportEmail:    stringPtr("ab@test.com"),
					IsDisabled:      boolPtr(false),
					SlackWebhookURL: stringPtr("https://netflix.com"),
				},
				app: &datastore.Application{
					Title:           "test_app",
					SupportEmail:    "123@test.com",
					IsDisabled:      true,
					SlackWebhookURL: "https://google.com",
				},
			},
			wantApp: &datastore.Application{
				Title:           "app_testing",
				SupportEmail:    "ab@test.com",
				IsDisabled:      false,
				SlackWebhookURL: "https://netflix.com",
			},
			dbFn: func(app *AppService) {
				a, _ := app.appRepo.(*mocks.MockApplicationRepository)
				a.EXPECT().UpdateApplication(gomock.Any(), gomock.Any(), gomock.Any()).Times(1).Return(nil)

				c, _ := app.cache.(*mocks.MockCache)
				c.EXPECT().Set(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any())
			},
			wantErr: false,
		},
		{
			name: "should_error_for_empty_app_name",
			args: args{
				ctx: ctx,
				appUpdate: &models.UpdateApplication{
					SupportEmail:    stringPtr("ab@test.com"),
					IsDisabled:      boolPtr(false),
					SlackWebhookURL: stringPtr("https://netflix.com"),
				},
				app: &datastore.Application{
					Title:           "test_app",
					SupportEmail:    "123@test.com",
					IsDisabled:      true,
					SlackWebhookURL: "https://google.com",
				},
			},
			wantErr:     true,
			wantErrCode: http.StatusBadRequest,
			wantErrMsg:  "name:please provide your appName",
		},
		{
			name: "should_fail_to_update_app",
			args: args{
				ctx: ctx,
				appUpdate: &models.UpdateApplication{
					AppName:         stringPtr("app_testing"),
					SupportEmail:    stringPtr("ab@test.com"),
					IsDisabled:      boolPtr(false),
					SlackWebhookURL: stringPtr("https://netflix.com"),
				},
				app: &datastore.Application{
					Title:           "test_app",
					SupportEmail:    "123@test.com",
					IsDisabled:      true,
					SlackWebhookURL: "https://google.com",
				},
			},
			dbFn: func(app *AppService) {
				a, _ := app.appRepo.(*mocks.MockApplicationRepository)
				a.EXPECT().UpdateApplication(gomock.Any(), gomock.Any(), gomock.Any()).Times(1).Return(errors.New("failed"))
			},
			wantErr:     true,
			wantErrCode: http.StatusBadRequest,
			wantErrMsg:  "an error occurred while updating app",
		},

		{
			name: "should_error_for_app_name_not_unique",
			args: args{
				ctx: ctx,
				appUpdate: &models.UpdateApplication{
					AppName:         stringPtr("app_testing"),
					SupportEmail:    stringPtr("ab@test.com"),
					IsDisabled:      boolPtr(false),
					SlackWebhookURL: stringPtr("https://netflix.com"),
				},
				app: &datastore.Application{
					Title:           "test_app",
					SupportEmail:    "123@test.com",
					IsDisabled:      true,
					SlackWebhookURL: "https://google.com",
				},
			},
			dbFn: func(app *AppService) {
				a, _ := app.appRepo.(*mocks.MockApplicationRepository)
				a.EXPECT().
					UpdateApplication(gomock.Any(), gomock.Any(), gomock.Any()).Times(1).
					Return(datastore.ErrDuplicateAppName)
			},
			wantErr:     true,
			wantErrCode: http.StatusBadRequest,
			wantErrMsg:  "an application with this name exists: app_testing",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			as := provideAppService(ctrl)

			// Arrange Expectations
			if tt.dbFn != nil {
				tt.dbFn(as)
			}

			err := as.UpdateApplication(tt.args.ctx, tt.args.appUpdate, tt.args.app)
			if tt.wantErr {
				require.NotNil(t, err)
				require.Equal(t, tt.wantErrCode, err.(*ServiceError).ErrCode())
				require.Equal(t, tt.wantErrMsg, err.(*ServiceError).Error())
				return
			}

			require.Nil(t, err)
			require.Equal(t, tt.wantApp, tt.args.app)
		})
	}
}

func TestAppService_DeleteApplication(t *testing.T) {
	ctx := context.Background()

	type args struct {
		ctx context.Context
		app *datastore.Application
	}
	tests := []struct {
		name       string
		args       args
		dbFn       func(app *AppService)
		wantErr    bool
		wantErrObj error
	}{
		{
			name: "should_delete_application",
			args: args{
				ctx: ctx,
				app: &datastore.Application{
					UID: "12345",
				},
			},
			dbFn: func(app *AppService) {
				a, _ := app.appRepo.(*mocks.MockApplicationRepository)
				a.EXPECT().DeleteApplication(gomock.Any(), &datastore.Application{UID: "12345"}).Times(1).Return(nil)

				c, _ := app.cache.(*mocks.MockCache)
				c.EXPECT().Delete(gomock.Any(), gomock.Any())
			},
			wantErr:    false,
			wantErrObj: nil,
		},
		{
			name: "should_fail_to_delete_application",
			args: args{
				ctx: ctx,
				app: &datastore.Application{UID: "abc"},
			},
			dbFn: func(app *AppService) {
				a, _ := app.appRepo.(*mocks.MockApplicationRepository)
				a.EXPECT().DeleteApplication(gomock.Any(), &datastore.Application{UID: "abc"}).Times(1).Return(errors.New("failed"))
			},
			wantErr:    true,
			wantErrObj: NewServiceError(http.StatusBadRequest, errors.New("an error occurred while deleting app")),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			as := provideAppService(ctrl)

			// Arrange Expectations
			if tt.dbFn != nil {
				tt.dbFn(as)
			}

			err := as.DeleteApplication(tt.args.ctx, tt.args.app)
			if tt.wantErr {
				require.NotNil(t, err)
				require.Equal(t, tt.wantErrObj, err)
				return
			}

			require.Nil(t, err)
		})
	}
}

func TestAppService_CreateAppEndpoint(t *testing.T) {

	ctx := context.Background()
	type args struct {
		ctx context.Context
		e   models.Endpoint
		app *datastore.Application
	}
	tests := []struct {
		name         string
		args         args
		wantApp      *datastore.Application
		wantEndpoint *datastore.Endpoint
		dbFn         func(app *AppService)
		wantErr      bool
		wantErrCode  int
		wantErrMsg   string
	}{
		{
			name: "should_create_app_endpoint",
			args: args{
				ctx: ctx,
				e:   models.Endpoint{Secret: "1234", URL: "https://google.com", Description: "test_endpoint", Events: []string{"payment.created"}},
				app: &datastore.Application{UID: "abc"},
			},
			dbFn: func(app *AppService) {
				a, _ := app.appRepo.(*mocks.MockApplicationRepository)
				a.EXPECT().UpdateApplication(gomock.Any(), gomock.Any(), gomock.Any()).Times(1).Return(nil)

				c, _ := app.cache.(*mocks.MockCache)
				c.EXPECT().Set(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any())
			},
			wantApp: &datastore.Application{
				UID: "abc",
				Endpoints: []datastore.Endpoint{
					{
						Secret:            "1234",
						TargetURL:         "https://google.com",
						Description:       "test_endpoint",
						Status:            datastore.ActiveEndpointStatus,
						RateLimit:         5000,
						RateLimitDuration: "1m0s",
						DocumentStatus:    datastore.ActiveDocumentStatus,
						Events:            []string{"payment.created"},
					},
				},
			},
			wantEndpoint: &datastore.Endpoint{
				Secret:            "1234",
				TargetURL:         "https://google.com",
				Description:       "test_endpoint",
				Status:            datastore.ActiveEndpointStatus,
				RateLimit:         5000,
				RateLimitDuration: "1m0s",
				DocumentStatus:    datastore.ActiveDocumentStatus,
				Events:            []string{"payment.created"},
			},
			wantErr: false,
		},
		{
			name: "should_create_app_endpoint_with_no_events",
			args: args{
				ctx: ctx,
				e: models.Endpoint{
					Secret:            "1234",
					RateLimit:         100,
					RateLimitDuration: "1m",
					URL:               "https://google.com",
					Description:       "test_endpoint",
				},
				app: &datastore.Application{UID: "abc"},
			},
			dbFn: func(app *AppService) {
				a, _ := app.appRepo.(*mocks.MockApplicationRepository)
				a.EXPECT().UpdateApplication(gomock.Any(), gomock.Any(), gomock.Any()).Times(1).Return(nil)

				c, _ := app.cache.(*mocks.MockCache)
				c.EXPECT().Set(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any())
			},
			wantApp: &datastore.Application{
				UID: "abc",
				Endpoints: []datastore.Endpoint{
					{
						Secret:            "1234",
						TargetURL:         "https://google.com",
						Description:       "test_endpoint",
						Status:            datastore.ActiveEndpointStatus,
						RateLimit:         100,
						RateLimitDuration: "1m0s",
						DocumentStatus:    datastore.ActiveDocumentStatus,
						Events:            []string{"*"},
					},
				},
			},
			wantEndpoint: &datastore.Endpoint{
				Secret:            "1234",
				TargetURL:         "https://google.com",
				Description:       "test_endpoint",
				Status:            datastore.ActiveEndpointStatus,
				RateLimit:         100,
				RateLimitDuration: "1m0s",
				DocumentStatus:    datastore.ActiveDocumentStatus,
				Events:            []string{"*"},
			},
			wantErr: false,
		},
		{
			name: "should_error_for_invalid_rate_limit_duration",
			args: args{
				ctx: ctx,
				e: models.Endpoint{
					Secret:            "1234",
					RateLimit:         100,
					RateLimitDuration: "m",
					URL:               "https://google.com",
					Description:       "test_endpoint",
				},
				app: &datastore.Application{UID: "abc"},
			},
			wantErr:     true,
			wantErrCode: http.StatusBadRequest,
			wantErrMsg:  `an error occurred parsing the rate limit duration: time: invalid duration "m"`,
		},
		{
			name: "should_fail_to_create_app_endpoint",
			args: args{
				ctx: ctx,
				e: models.Endpoint{
					Secret:            "1234",
					RateLimit:         100,
					RateLimitDuration: "1m",
					URL:               "https://google.com",
					Description:       "test_endpoint",
				},
				app: &datastore.Application{UID: "abc"},
			},
			dbFn: func(app *AppService) {
				a, _ := app.appRepo.(*mocks.MockApplicationRepository)
				a.EXPECT().UpdateApplication(gomock.Any(), gomock.Any(), gomock.Any()).Times(1).Return(errors.New("failed"))
			},
			wantErr:     true,
			wantErrCode: http.StatusBadRequest,
			wantErrMsg:  "an error occurred while adding app endpoint",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			as := provideAppService(ctrl)

			// Arrange Expectations
			if tc.dbFn != nil {
				tc.dbFn(as)
			}

			appEndpoint, err := as.CreateAppEndpoint(tc.args.ctx, tc.args.e, tc.args.app)
			if tc.wantErr {
				require.NotNil(t, err)
				require.Equal(t, tc.wantErrCode, err.(*ServiceError).ErrCode())
				require.Equal(t, tc.wantErrMsg, err.(*ServiceError).Error())
				return
			}

			require.Nil(t, err)
			require.NotEmpty(t, appEndpoint.UID)
			require.NotEmpty(t, appEndpoint.CreatedAt)
			require.NotEmpty(t, appEndpoint.UpdatedAt)
			require.Empty(t, appEndpoint.DeletedAt)

			stripVariableFields(t, "endpoint", appEndpoint)
			require.Equal(t, tc.wantEndpoint, appEndpoint)

			for i := range tc.args.app.Endpoints {
				stripVariableFields(t, "endpoint", &tc.args.app.Endpoints[i])
			}

			require.Equal(t, tc.wantApp, tc.args.app)
		})
	}
}

func stripVariableFields(t *testing.T, obj string, v interface{}) {
	switch obj {
	case "application":
		a := v.(*datastore.Application)
		a.UID = ""
		a.CreatedAt, a.UpdatedAt, a.DeletedAt = 0, 0, 0
	case "group":
		g := v.(*datastore.Group)
		g.UID = ""
		g.CreatedAt, g.UpdatedAt, g.DeletedAt = 0, 0, 0
	case "endpoint":
		e := v.(*datastore.Endpoint)
		e.UID = ""
		e.CreatedAt, e.UpdatedAt, e.DeletedAt = 0, 0, 0
	case "event":
		e := v.(*datastore.Event)
		e.UID = ""
		e.MatchedEndpoints = 0
		e.CreatedAt, e.UpdatedAt, e.DeletedAt = 0, 0, 0
	case "apiKey":
		a := v.(*datastore.APIKey)
		a.UID, a.MaskID, a.Salt, a.Hash = "", "", "", ""
		a.CreatedAt, a.UpdatedAt = 0, 0
	default:
		t.Errorf("invalid data body - %v of type %T", obj, obj)
		t.FailNow()
	}
}

func TestAppService_UpdateAppEndpoint(t *testing.T) {
	ctx := context.Background()
	type args struct {
		ctx        context.Context
		e          models.Endpoint
		endPointId string
		app        *datastore.Application
	}
	tests := []struct {
		name         string
		args         args
		wantApp      *datastore.Application
		wantEndpoint *datastore.Endpoint
		dbFn         func(as *AppService)
		wantErr      bool
		wantErrCode  int
		wantErrMsg   string
	}{
		{
			name: "should_update_app_endpoint",
			args: args{
				ctx: ctx,
				e: models.Endpoint{
					Secret:            "newly-generated-secret",
					Events:            []string{"payment.created", "payment.success"},
					URL:               "https://fb.com",
					RateLimit:         10000,
					RateLimitDuration: "1m",
					HttpTimeout:       "20s",
				},
				endPointId: "endpoint2",
				app: &datastore.Application{
					UID: "1234",
					Endpoints: []datastore.Endpoint{
						{
							UID:       "endpoint1",
							TargetURL: "https://google.com",
						},
						{
							UID:       "endpoint2",
							TargetURL: "https://netflix.com",
							Secret:    "old-assigned-secret",
						},
					},
				},
			},
			wantApp: &datastore.Application{
				UID: "1234",
				Endpoints: []datastore.Endpoint{
					{
						UID:       "endpoint1",
						TargetURL: "https://google.com",
						Secret:    "",
					},
					{
						UID:               "endpoint2",
						Events:            []string{"payment.created", "payment.success"},
						TargetURL:         "https://fb.com",
						Secret:            "newly-generated-secret",
						RateLimit:         10000,
						RateLimitDuration: "1m0s",
						Status:            datastore.ActiveEndpointStatus,
						HttpTimeout:       "20s",
					},
				},
			},
			wantEndpoint: &datastore.Endpoint{
				Secret:            "newly-generated-secret",
				UID:               "endpoint2",
				Events:            []string{"payment.created", "payment.success"},
				TargetURL:         "https://fb.com",
				RateLimit:         10000,
				Status:            datastore.ActiveEndpointStatus,
				RateLimitDuration: "1m0s",
				HttpTimeout:       "20s",
			},
			dbFn: func(as *AppService) {
				a, _ := as.appRepo.(*mocks.MockApplicationRepository)
				a.EXPECT().UpdateApplication(gomock.Any(), gomock.Any(), gomock.Any()).
					Times(1).Return(nil)

				c, _ := as.cache.(*mocks.MockCache)
				c.EXPECT().Set(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any())
			},
			wantErr: false,
		},
		{
			name: "should_error_for_invalid_rate_limit_duration",
			args: args{
				ctx: ctx,
				e: models.Endpoint{
					URL:               "https://fb.com",
					RateLimit:         10000,
					RateLimitDuration: "m",
					HttpTimeout:       "20s",
				},
				endPointId: "endpoint1",
				app: &datastore.Application{
					UID: "1234",
					Endpoints: []datastore.Endpoint{
						{
							UID:       "endpoint1",
							TargetURL: "https://google.com",
						},
					},
				},
			},
			wantErr:     true,
			wantErrCode: http.StatusBadRequest,
			wantErrMsg:  `time: invalid duration "m"`,
		},
		{
			name: "should_fail_to_update_app_endpoint",
			args: args{
				ctx: ctx,
				e: models.Endpoint{
					URL:               "https://fb.com",
					RateLimit:         10000,
					RateLimitDuration: "1m",
					HttpTimeout:       "20s",
				},
				endPointId: "endpoint1",
				app: &datastore.Application{
					UID: "1234",
					Endpoints: []datastore.Endpoint{
						{
							UID:       "endpoint1",
							TargetURL: "https://google.com",
						},
					},
				},
			},
			dbFn: func(as *AppService) {
				a, _ := as.appRepo.(*mocks.MockApplicationRepository)
				a.EXPECT().UpdateApplication(gomock.Any(), gomock.Any(), gomock.Any()).
					Times(1).Return(errors.New("failed"))
			},
			wantErr:     true,
			wantErrCode: http.StatusBadRequest,
			wantErrMsg:  "an error occurred while updating app endpoints",
		},
		{
			name: "should_error_for_endpoint_not_found",
			args: args{
				ctx: ctx,
				e: models.Endpoint{
					URL:               "https://fb.com",
					RateLimit:         10000,
					RateLimitDuration: "1m",
					HttpTimeout:       "20s",
				},
				endPointId: "endpoint1",
				app: &datastore.Application{
					UID: "1234",
					Endpoints: []datastore.Endpoint{
						{
							UID:       "123",
							TargetURL: "https://google.com",
						},
					},
				},
			},
			wantErr:     true,
			wantErrCode: http.StatusBadRequest,
			wantErrMsg:  "endpoint not found",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			as := provideAppService(ctrl)

			// Arrange Expectations
			if tc.dbFn != nil {
				tc.dbFn(as)
			}

			appEndpoint, err := as.UpdateAppEndpoint(tc.args.ctx, tc.args.e, tc.args.endPointId, tc.args.app)
			if tc.wantErr {
				require.NotNil(t, err)
				require.Equal(t, tc.wantErrCode, err.(*ServiceError).ErrCode())
				require.Equal(t, tc.wantErrMsg, err.(*ServiceError).Error())
				return
			}

			require.Nil(t, err)
			require.NotEmpty(t, appEndpoint.UpdatedAt)

			appEndpoint.UpdatedAt = 0
			require.Equal(t, tc.wantEndpoint, appEndpoint)

			for i := range tc.args.app.Endpoints {
				uid := tc.args.app.Endpoints[i].UID
				stripVariableFields(t, "endpoint", &tc.args.app.Endpoints[i])
				tc.args.app.Endpoints[i].UID = uid
			}
			require.Equal(t, tc.wantApp, tc.args.app)
		})
	}
}

func TestAppService_DeleteAppEndpoint(t *testing.T) {
	ctx := context.Background()
	type args struct {
		ctx context.Context
		e   *datastore.Endpoint
		app *datastore.Application
	}
	tests := []struct {
		name        string
		args        args
		dbFn        func(as *AppService)
		wantApp     *datastore.Application
		wantErr     bool
		wantErrCode int
		wantErrMsg  string
	}{
		{
			name: "should_delete_app_endpoint",
			args: args{
				ctx: ctx,
				e:   &datastore.Endpoint{UID: "endpoint2"},
				app: &datastore.Application{
					UID: "abc",
					Endpoints: []datastore.Endpoint{
						{
							UID:       "endpoint1",
							TargetURL: "https:?/netflix.com",
						},
						{
							UID:       "endpoint2",
							TargetURL: "https:?/netflix.com",
						},
						{
							UID:       "endpoint3",
							TargetURL: "https:?/netflix.com",
						},
					},
				},
			},
			wantApp: &datastore.Application{
				UID: "abc",
				Endpoints: []datastore.Endpoint{
					{
						UID:       "endpoint1",
						TargetURL: "https:?/netflix.com",
					},
					{
						UID:       "endpoint3",
						TargetURL: "https:?/netflix.com",
					},
				},
			},
			dbFn: func(as *AppService) {
				appRepo := as.appRepo.(*mocks.MockApplicationRepository)
				appRepo.EXPECT().UpdateApplication(gomock.Any(), gomock.Any(), gomock.Any()).Times(1).Return(nil)

				c, _ := as.cache.(*mocks.MockCache)
				c.EXPECT().Set(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any())
			},
			wantErr: false,
		},
		{
			name: "should_fail_to_delete_app_endpoint",
			args: args{
				ctx: ctx,
				e:   &datastore.Endpoint{UID: "endpoint2"},
				app: &datastore.Application{
					UID: "abc",
					Endpoints: []datastore.Endpoint{
						{
							UID:       "endpoint1",
							TargetURL: "https:?/netflix.com",
						},
						{
							UID:       "endpoint2",
							TargetURL: "https:?/netflix.com",
						},
						{
							UID:       "endpoint3",
							TargetURL: "https:?/netflix.com",
						},
					},
				},
			},
			dbFn: func(as *AppService) {
				appRepo := as.appRepo.(*mocks.MockApplicationRepository)
				appRepo.EXPECT().UpdateApplication(gomock.Any(), gomock.Any(), gomock.Any()).Times(1).Return(errors.New("failed"))
			},
			wantErr:     true,
			wantErrCode: http.StatusBadRequest,
			wantErrMsg:  "an error occurred while deleting app endpoint",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			as := provideAppService(ctrl)

			// Arrange Expectations
			if tc.dbFn != nil {
				tc.dbFn(as)
			}

			err := as.DeleteAppEndpoint(tc.args.ctx, tc.args.e, tc.args.app)
			if tc.wantErr {
				require.NotNil(t, err)
				require.Equal(t, tc.wantErrCode, err.(*ServiceError).ErrCode())
				require.Equal(t, tc.wantErrMsg, err.(*ServiceError).Error())
				return
			}

			require.Nil(t, err)
			require.Equal(t, tc.wantApp, tc.args.app)
		})
	}
}
