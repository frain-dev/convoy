package services

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/mocks"
	"github.com/frain-dev/convoy/server/models"
	"github.com/frain-dev/convoy/util"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

func stripVariableFields(t *testing.T, obj string, v interface{}) {
	switch obj {
	case "project":
		g := v.(*datastore.Project)
		if g.Config != nil {
			for i := range g.Config.Signature.Versions {
				v := &g.Config.Signature.Versions[i]
				v.UID = ""
				v.CreatedAt = 0
			}
		}
		g.UID = ""
		g.CreatedAt, g.UpdatedAt, g.DeletedAt = 0, 0, nil
	case "endpoint":
		e := v.(*datastore.Endpoint)

		for i := range e.Secrets {
			s := &e.Secrets[i]
			s.UID = ""
			s.CreatedAt, s.UpdatedAt, s.DeletedAt = 0, 0, nil
		}

		e.UID, e.AppID = "", ""
		e.CreatedAt, e.UpdatedAt, e.DeletedAt = 0, 0, nil
	case "event":
		e := v.(*datastore.Event)
		e.UID = ""
		e.MatchedEndpoints = 0
		e.CreatedAt, e.UpdatedAt, e.DeletedAt = 0, 0, nil
	case "apiKey":
		a := v.(*datastore.APIKey)
		a.UID, a.MaskID, a.Salt, a.Hash = "", "", "", ""
		a.CreatedAt, a.UpdatedAt = 0, 0
	case "organisation":
		a := v.(*datastore.Organisation)
		a.UID = ""
		a.CreatedAt, a.UpdatedAt = 0, 0
	case "organisation_member":
		a := v.(*datastore.OrganisationMember)
		a.UID = ""
		a.CreatedAt, a.UpdatedAt = 0, 0
	case "organisation_invite":
		a := v.(*datastore.OrganisationInvite)
		a.UID = ""
		a.Token = ""
		a.CreatedAt, a.UpdatedAt, a.ExpiresAt, a.DeletedAt = 0, 0, 0, nil
	default:
		t.Errorf("invalid data body - %v of type %T", obj, obj)
		t.FailNow()
	}
}

func provideEndpointService(ctrl *gomock.Controller) *EndpointService {
	endpointRepo := mocks.NewMockEndpointRepository(ctrl)
	eventRepo := mocks.NewMockEventRepository(ctrl)
	eventDeliveryRepo := mocks.NewMockEventDeliveryRepository(ctrl)
	cache := mocks.NewMockCache(ctrl)
	queue := mocks.NewMockQueuer(ctrl)
	return NewEndpointService(endpointRepo, eventRepo, eventDeliveryRepo, cache, queue)
}

func boolPtr(b bool) *bool {
	return &b
}

func stringPtr(s string) *string {
	return &s
}

func TestEndpointService_LoadEndpointsPaged(t *testing.T) {
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
		dbFn               func(app *EndpointService)
		wantApps           []datastore.Endpoint
		wantPaginationData datastore.PaginationData
		wantErr            bool
		wantErrObj         error
	}{
		{
			name: "should_load_endpoints",
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
			wantApps: []datastore.Endpoint{
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
			dbFn: func(app *EndpointService) {
				a, _ := app.endpointRepo.(*mocks.MockEndpointRepository)
				a.EXPECT().
					LoadEndpointsPaged(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Times(1).
					Return([]datastore.Endpoint{
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
			name: "should_fail_load_endpoints",
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
			dbFn: func(app *EndpointService) {
				a, _ := app.endpointRepo.(*mocks.MockEndpointRepository)
				a.EXPECT().
					LoadEndpointsPaged(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Times(1).
					Return(nil, datastore.PaginationData{}, errors.New("failed"))
			},
			wantErr:    true,
			wantErrObj: util.NewServiceError(http.StatusInternalServerError, errors.New("an error occurred while fetching endpoints")),
		},
		{
			name: "should_load_endpoints_trims-whitespaces-from-query",
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
			wantApps: []datastore.Endpoint{
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
			dbFn: func(app *EndpointService) {
				a, _ := app.endpointRepo.(*mocks.MockEndpointRepository)
				a.EXPECT().
					LoadEndpointsPaged(gomock.Any(), gomock.Any(), "falsetto", gomock.Any()).Times(1).
					Return([]datastore.Endpoint{
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
			name: "should_load_endpoints_trims-whitespaces-from-query-retains-case",
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
			wantApps: []datastore.Endpoint{
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
			dbFn: func(app *EndpointService) {
				a, _ := app.endpointRepo.(*mocks.MockEndpointRepository)
				a.EXPECT().
					LoadEndpointsPaged(gomock.Any(), gomock.Any(), "FalSetto", gomock.Any()).Times(1).
					Return([]datastore.Endpoint{
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
			as := provideEndpointService(ctrl)

			// Arrange Expectations
			if tt.dbFn != nil {
				tt.dbFn(as)
			}

			apps, paginationData, err := as.LoadEndpointsPaged(tt.args.ctx, tt.args.uid, tt.args.q, tt.args.pageable)
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

func TestEndpointService_CreateEndpoint(t *testing.T) {
	projectID := "1234567890"
	project := &datastore.Project{UID: projectID}

	ctx := context.Background()
	type args struct {
		ctx context.Context
		e   models.Endpoint
		g   *datastore.Project
	}
	tests := []struct {
		name         string
		args         args
		wantEndpoint *datastore.Endpoint
		dbFn         func(endpoint *EndpointService)
		wantErr      bool
		wantErrCode  int
		wantErrMsg   string
	}{
		{
			name: "should_create_endpoint",
			args: args{
				ctx: ctx,
				e: models.Endpoint{
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
			dbFn: func(app *EndpointService) {
				a, _ := app.endpointRepo.(*mocks.MockEndpointRepository)
				a.EXPECT().CreateEndpoint(gomock.Any(), gomock.Any(), gomock.Any()).Times(1).Return(nil)

				c, _ := app.cache.(*mocks.MockCache)
				c.EXPECT().Set(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any())
			},
			wantEndpoint: &datastore.Endpoint{
				Title:           "endpoint",
				SupportEmail:    "endpoint@test.com",
				SlackWebhookURL: "https://google.com",
				ProjectID:       project.UID,
				Secrets: []datastore.Secret{
					{Value: "1234"},
				},
				TargetURL:         "https://google.com",
				Description:       "test_endpoint",
				RateLimit:         5000,
				Status:            datastore.ActiveEndpointStatus,
				RateLimitDuration: "1m0s",
			},
			wantErr: false,
		},
		{
			name: "should_create_endpoint_with_custom_authentication",
			args: args{
				ctx: ctx,
				e: models.Endpoint{
					Name:              "endpoint",
					Secret:            "1234",
					RateLimit:         100,
					RateLimitDuration: "1m",
					URL:               "https://google.com",
					Description:       "test_endpoint",
					Authentication: &datastore.EndpointAuthentication{
						Type: datastore.APIKeyAuthentication,
						ApiKey: &datastore.ApiKey{
							HeaderName:  "x-api-key",
							HeaderValue: "x-api-key",
						},
					},
				},
				g: project,
			},
			dbFn: func(app *EndpointService) {
				a, _ := app.endpointRepo.(*mocks.MockEndpointRepository)
				a.EXPECT().CreateEndpoint(gomock.Any(), gomock.Any(), gomock.Any()).Times(1).Return(nil)

				c, _ := app.cache.(*mocks.MockCache)
				c.EXPECT().Set(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any())
			},
			wantEndpoint: &datastore.Endpoint{
				ProjectID: project.UID,
				Title:     "endpoint",
				Secrets: []datastore.Secret{
					{Value: "1234"},
				},
				TargetURL:         "https://google.com",
				Description:       "test_endpoint",
				RateLimit:         100,
				Status:            datastore.ActiveEndpointStatus,
				RateLimitDuration: "1m0s",
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
				e: models.Endpoint{
					Name:              "test_endpoint",
					Secret:            "1234",
					RateLimit:         100,
					RateLimitDuration: "m",
					URL:               "https://google.com",
					Description:       "test_endpoint",
				},
				g: project,
			},
			wantErr:     true,
			wantErrCode: http.StatusBadRequest,
			wantErrMsg:  `an error occurred parsing the rate limit duration: time: invalid duration "m"`,
		},
		{
			name: "should_fail_to_create_endpoint",
			args: args{
				ctx: ctx,
				e: models.Endpoint{
					Name:              "test_endpoint",
					Secret:            "1234",
					RateLimit:         100,
					RateLimitDuration: "1m",
					URL:               "https://google.com",
					Description:       "test_endpoint",
				},
				g: project,
			},
			dbFn: func(app *EndpointService) {
				a, _ := app.endpointRepo.(*mocks.MockEndpointRepository)
				a.EXPECT().CreateEndpoint(gomock.Any(), gomock.Any(), gomock.Any()).Times(1).Return(errors.New("failed"))
			},
			wantErr:     true,
			wantErrCode: http.StatusBadRequest,
			wantErrMsg:  "an error occurred while adding endpoint",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			as := provideEndpointService(ctrl)

			// Arrange Expectations
			if tc.dbFn != nil {
				tc.dbFn(as)
			}

			endpoint, err := as.CreateEndpoint(tc.args.ctx, tc.args.e, tc.args.g.UID)
			if tc.wantErr {
				require.NotNil(t, err)
				require.Equal(t, tc.wantErrCode, err.(*util.ServiceError).ErrCode())
				require.Equal(t, tc.wantErrMsg, err.(*util.ServiceError).Error())
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

func TestEndpointService_UpdateEndpoint(t *testing.T) {
	ctx := context.Background()
	type args struct {
		ctx      context.Context
		e        models.UpdateEndpoint
		endpoint *datastore.Endpoint
	}
	tests := []struct {
		name         string
		args         args
		wantEndpoint *datastore.Endpoint
		dbFn         func(as *EndpointService)
		wantErr      bool
		wantErrCode  int
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
			},
			wantEndpoint: &datastore.Endpoint{
				Title:             "Endpoint2",
				Description:       "test_endpoint",
				TargetURL:         "https://fb.com",
				RateLimit:         10000,
				RateLimitDuration: "1m0s",
				HttpTimeout:       "20s",
			},
			dbFn: func(as *EndpointService) {
				a, _ := as.endpointRepo.(*mocks.MockEndpointRepository)
				a.EXPECT().FindEndpointByID(gomock.Any(), gomock.Any()).
					Times(1).Return(&datastore.Endpoint{UID: "endpoint2"}, nil)

				a.EXPECT().UpdateEndpoint(gomock.Any(), gomock.Any(), gomock.Any()).
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
				e: models.UpdateEndpoint{
					Name:              stringPtr("Endpoint 1"),
					Description:       "test_endpoint",
					URL:               "https://fb.com",
					RateLimit:         10000,
					RateLimitDuration: "m",
					HttpTimeout:       "20s",
				},
				endpoint: &datastore.Endpoint{UID: "endpoint1"},
			},
			dbFn: func(as *EndpointService) {
				a, _ := as.endpointRepo.(*mocks.MockEndpointRepository)
				a.EXPECT().FindEndpointByID(gomock.Any(), gomock.Any()).
					Times(1).Return(&datastore.Endpoint{UID: "endpoint1"}, nil)
			},
			wantErr:     true,
			wantErrCode: http.StatusBadRequest,
			wantErrMsg:  `time: invalid duration "m"`,
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
			},
			dbFn: func(as *EndpointService) {
				a, _ := as.endpointRepo.(*mocks.MockEndpointRepository)
				a.EXPECT().FindEndpointByID(gomock.Any(), gomock.Any()).
					Times(1).Return(&datastore.Endpoint{UID: "endpoint1"}, nil)

				a.EXPECT().UpdateEndpoint(gomock.Any(), gomock.Any(), gomock.Any()).
					Times(1).Return(errors.New("failed"))
			},
			wantErr:     true,
			wantErrCode: http.StatusBadRequest,
			wantErrMsg:  "an error occurred while updating endpoints",
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
			},
			dbFn: func(as *EndpointService) {
				a, _ := as.endpointRepo.(*mocks.MockEndpointRepository)
				a.EXPECT().FindEndpointByID(gomock.Any(), gomock.Any()).
					Times(1).Return(nil, datastore.ErrEndpointNotFound)
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
			as := provideEndpointService(ctrl)

			// Arrange Expectations
			if tc.dbFn != nil {
				tc.dbFn(as)
			}

			endpoint, err := as.UpdateEndpoint(tc.args.ctx, tc.args.e, tc.args.endpoint)
			if tc.wantErr {
				require.NotNil(t, err)
				require.Equal(t, tc.wantErrCode, err.(*util.ServiceError).ErrCode())
				require.Equal(t, tc.wantErrMsg, err.(*util.ServiceError).Error())
				return
			}

			require.Nil(t, err)
			require.NotEmpty(t, endpoint.UpdatedAt)

			stripVariableFields(t, "endpoint", endpoint)
			require.Equal(t, tc.wantEndpoint, endpoint)
		})
	}
}

func TestEndpointService_DeleteEndpoint(t *testing.T) {
	projectID := "1234567890"
	project := &datastore.Project{UID: projectID}

	ctx := context.Background()
	type args struct {
		ctx context.Context
		e   *datastore.Endpoint
		g   *datastore.Project
	}
	tests := []struct {
		name        string
		args        args
		dbFn        func(as *EndpointService)
		wantApp     *datastore.Endpoint
		wantErr     bool
		wantErrCode int
		wantErrMsg  string
	}{
		{
			name: "should_delete_endpoint",
			args: args{
				ctx: ctx,
				e:   &datastore.Endpoint{UID: "endpoint2"},
				g:   project,
			},
			dbFn: func(as *EndpointService) {
				endpointRepo := as.endpointRepo.(*mocks.MockEndpointRepository)
				endpointRepo.EXPECT().DeleteEndpoint(gomock.Any(), gomock.Any()).Times(1).Return(nil)

				c, _ := as.cache.(*mocks.MockCache)
				c.EXPECT().Delete(gomock.Any(), gomock.Any())
			},
			wantErr: false,
		},
		{
			name: "should_fail_to_delete_endpoint",
			args: args{
				ctx: ctx,
				e:   &datastore.Endpoint{UID: "endpoint2"},
				g:   project,
			},
			dbFn: func(as *EndpointService) {
				endpointRepo := as.endpointRepo.(*mocks.MockEndpointRepository)
				endpointRepo.EXPECT().DeleteEndpoint(gomock.Any(), gomock.Any()).Times(1).Return(errors.New("failed"))
			},
			wantErr:     true,
			wantErrCode: http.StatusBadRequest,
			wantErrMsg:  "an error occurred while deleting endpoint",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			as := provideEndpointService(ctrl)

			// Arrange Expectations
			if tc.dbFn != nil {
				tc.dbFn(as)
			}

			err := as.DeleteEndpoint(tc.args.ctx, tc.args.e)
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

func TestEndpointService_ExpireEndpointSecret(t *testing.T) {
	ctx := context.Background()

	type args struct {
		ctx      context.Context
		secret   *models.ExpireSecret
		endpoint *datastore.Endpoint
	}
	tests := []struct {
		name        string
		args        args
		dbFn        func(es *EndpointService)
		wantErr     bool
		wantErrCode int
		wantErrMsg  string
	}{
		{
			name: "should_expire_endpoint_secret",
			args: args{
				ctx: ctx,
				secret: &models.ExpireSecret{
					Secret:     "abce",
					Expiration: 10,
				},
				endpoint: &datastore.Endpoint{
					UID:       "abc",
					ProjectID: "1234",
					Secrets: []datastore.Secret{
						{
							UID:   "1234",
							Value: "test_secret",
						},
					},
					AdvancedSignatures: false,
				},
			},
			dbFn: func(es *EndpointService) {
				endpointRepo := es.endpointRepo.(*mocks.MockEndpointRepository)

				endpointRepo.EXPECT().ExpireSecret(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Times(1).Return(nil)

				eq, _ := es.queue.(*mocks.MockQueuer)
				eq.EXPECT().Write(convoy.ExpireSecretsProcessor, convoy.DefaultQueue, gomock.Any()).
					Times(1).Return(nil)

				c, _ := es.cache.(*mocks.MockCache)
				c.EXPECT().Set(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Times(1).Return(nil)
			},
			wantErr:     false,
			wantErrCode: 0,
			wantErrMsg:  "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			as := provideEndpointService(ctrl)

			// Arrange Expectations
			if tt.dbFn != nil {
				tt.dbFn(as)
			}

			_, err := as.ExpireSecret(tt.args.ctx, tt.args.secret, tt.args.endpoint)
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
