package server

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/server/models"
	pager "github.com/gobeam/mongo-go-pagination"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/mocks"
	"github.com/go-chi/chi/v5"
	"github.com/golang/mock/gomock"
	"github.com/sebdah/goldie/v2"
)

func verifyMatch(t *testing.T, w httptest.ResponseRecorder) {
	g := goldie.New(t, goldie.WithFixtureDir("./testdata"))
	g.Assert(t, t.Name(), w.Body.Bytes())
}

func TestApplicationHandler_GetApp(t *testing.T) {

	var app *applicationHandler

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	org := mocks.NewMockOrganisationRepository(ctrl)
	apprepo := mocks.NewMockApplicationRepository(ctrl)
	msgRepo := mocks.NewMockMessageRepository(ctrl)

	orgID := "1234567890"

	validID := "123456789"

	app = newApplicationHandler(msgRepo, apprepo, org)

	tt := []struct {
		name       string
		cfgPath    string
		method     string
		statusCode int
		id         string
		dbFn       func(appRepo *mocks.MockApplicationRepository, orgRepo *mocks.MockOrganisationRepository)
	}{
		{
			name:       "app not found",
			cfgPath:    "./testdata/Auth_Config/basic-convoy.json",
			method:     http.MethodGet,
			statusCode: http.StatusNotFound,
			id:         "12345",
			dbFn: func(appRepo *mocks.MockApplicationRepository, orgRepo *mocks.MockOrganisationRepository) {
				appRepo.EXPECT().
					FindApplicationByID(gomock.Any(), gomock.Any()).
					Return(nil, convoy.ErrApplicationNotFound).Times(1)
			},
		},
		{
			name:       "valid application",
			cfgPath:    "./testdata/Auth_Config/basic-convoy.json",
			method:     http.MethodGet,
			statusCode: http.StatusOK,
			id:         validID,
			dbFn: func(appRepo *mocks.MockApplicationRepository, orgRepo *mocks.MockOrganisationRepository) {
				appRepo.EXPECT().
					FindApplicationByID(gomock.Any(), gomock.Any()).Times(1).
					Return(&convoy.Application{
						UID:       validID,
						OrgID:     orgID,
						Title:     "Valid application",
						Endpoints: []convoy.Endpoint{},
					}, nil)

			},
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			// Arrange
			url := fmt.Sprintf("/v1/applications/%s", tc.id)
			req := httptest.NewRequest(tc.method, url, nil)
			req.SetBasicAuth("test", "test")
			w := httptest.NewRecorder()
			rctx := chi.NewRouteContext()
			rctx.URLParams.Add("id", tc.id)

			req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

			// Arrange Expectations
			if tc.dbFn != nil {
				tc.dbFn(apprepo, org)
			}

			err := config.LoadConfig(tc.cfgPath)
			if err != nil {
				t.Error("Failed to load config file")
			}

			router := buildRoutes(app)

			// Act
			router.ServeHTTP(w, req)

			// Assert
			if w.Code != tc.statusCode {
				t.Errorf("Want status '%d', got '%d'", tc.statusCode, w.Code)
			}

			verifyMatch(t, *w)
		})
	}

}

func TestApplicationHandler_GetApps(t *testing.T) {

	var app *applicationHandler

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	org := mocks.NewMockOrganisationRepository(ctrl)
	apprepo := mocks.NewMockApplicationRepository(ctrl)
	msgRepo := mocks.NewMockMessageRepository(ctrl)

	orgID := "1234567890"

	validID := "123456789"

	app = newApplicationHandler(msgRepo, apprepo, org)

	tt := []struct {
		name       string
		cfgPath    string
		method     string
		statusCode int
		dbFn       func(appRepo *mocks.MockApplicationRepository, orgRepo *mocks.MockOrganisationRepository)
	}{
		{
			name:       "valid applications",
			cfgPath:    "./testdata/Auth_Config/basic-convoy.json",
			method:     http.MethodGet,
			statusCode: http.StatusOK,
			dbFn: func(appRepo *mocks.MockApplicationRepository, orgRepo *mocks.MockOrganisationRepository) {
				appRepo.EXPECT().
					LoadApplicationsPaged(gomock.Any(), gomock.Any(), gomock.Any()).Times(1).
					Return([]convoy.Application{
						{
							UID:       validID,
							OrgID:     orgID,
							Title:     "Valid application - 0",
							Endpoints: []convoy.Endpoint{},
						},
					}, pager.PaginationData{}, nil)

			},
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			// Arrange
			req := httptest.NewRequest(tc.method, "/v1/applications", nil)
			req.SetBasicAuth("test", "test")
			w := httptest.NewRecorder()

			pageable := models.Pageable{
				Page:    1,
				PerPage: 10,
			}
			req = req.WithContext(context.WithValue(req.Context(), pageableCtx, pageable))

			// Arrange Expectations.
			if tc.dbFn != nil {
				tc.dbFn(apprepo, org)
			}

			err := config.LoadConfig(tc.cfgPath)
			if err != nil {
				t.Error("Failed to load config file")
			}

			router := buildRoutes(app)

			// Act
			router.ServeHTTP(w, req)

			if w.Code != tc.statusCode {
				t.Errorf("Want status '%d', got '%d'", tc.statusCode, w.Code)
			}

			verifyMatch(t, *w)
		})
	}

}

func TestApplicationHandler_CreateApp(t *testing.T) {

	var app *applicationHandler

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	org := mocks.NewMockOrganisationRepository(ctrl)
	apprepo := mocks.NewMockApplicationRepository(ctrl)
	msgRepo := mocks.NewMockMessageRepository(ctrl)

	orgID := "1234567890"

	organisation := &convoy.Organisation{
		UID: orgID,
	}

	bodyReader := strings.NewReader(`{ "org_id": "` + orgID + `", "name": "ABC_DEF_TEST"}`)

	app = newApplicationHandler(msgRepo, apprepo, org)

	tt := []struct {
		name       string
		cfgPath    string
		method     string
		statusCode int
		body       *strings.Reader
		dbFn       func(appRepo *mocks.MockApplicationRepository, orgRepo *mocks.MockOrganisationRepository)
	}{
		{
			name:       "valid application",
			cfgPath:    "./testdata/Auth_Config/basic-convoy.json",
			method:     http.MethodPost,
			statusCode: http.StatusCreated,
			body:       bodyReader,
			dbFn: func(appRepo *mocks.MockApplicationRepository, orgRepo *mocks.MockOrganisationRepository) {
				appRepo.EXPECT().
					CreateApplication(gomock.Any(), gomock.Any()).Times(1).
					Return(nil)

				orgRepo.EXPECT().
					FetchOrganisationByID(gomock.Any(), gomock.Any()).Times(1).
					Return(organisation, nil)

			},
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			// Arrange
			req := httptest.NewRequest(tc.method, "/v1/applications", tc.body)
			req.SetBasicAuth("test", "test")
			req.Header.Add("Content-Type", "application/json")
			w := httptest.NewRecorder()

			// Arrange Expectations
			if tc.dbFn != nil {
				tc.dbFn(apprepo, org)
			}

			err := config.LoadConfig(tc.cfgPath)
			if err != nil {
				t.Error("Failed to load config file")
			}

			router := buildRoutes(app)

			// Act.
			router.ServeHTTP(w, req)

			if w.Code != tc.statusCode {
				t.Errorf("Want status '%d', got '%d'", tc.statusCode, w.Code)
			}
		})
	}

}

func TestApplicationHandler_UpdateApp(t *testing.T) {

	var app *applicationHandler

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	org := mocks.NewMockOrganisationRepository(ctrl)
	apprepo := mocks.NewMockApplicationRepository(ctrl)
	msgRepo := mocks.NewMockMessageRepository(ctrl)

	orgID := "1234567890"

	appId := "12345"
	bodyReader := strings.NewReader(`{"name": "ABC_DEF_TEST_UPDATE"}`)

	app = newApplicationHandler(msgRepo, apprepo, org)

	tt := []struct {
		name       string
		cfgPath    string
		method     string
		statusCode int
		appId      string
		body       *strings.Reader
		dbFn       func(appRepo *mocks.MockApplicationRepository, orgRepo *mocks.MockOrganisationRepository)
	}{
		{
			name:       "valid application update",
			cfgPath:    "./testdata/Auth_Config/basic-convoy.json",
			method:     http.MethodPut,
			statusCode: http.StatusAccepted,
			appId:      appId,
			body:       bodyReader,
			dbFn: func(appRepo *mocks.MockApplicationRepository, orgRepo *mocks.MockOrganisationRepository) {
				appRepo.EXPECT().
					UpdateApplication(gomock.Any(), gomock.Any()).Times(1).
					Return(nil)

				appRepo.EXPECT().
					FindApplicationByID(gomock.Any(), gomock.Any()).Times(1).
					Return(&convoy.Application{
						UID:       appId,
						OrgID:     orgID,
						Title:     "Valid application update",
						Endpoints: []convoy.Endpoint{},
					}, nil)

			},
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			// Arrange
			url := fmt.Sprintf("/v1/applications/%s", tc.appId)
			req := httptest.NewRequest(tc.method, url, tc.body)
			req.SetBasicAuth("test", "test")
			req.Header.Add("Content-Type", "application/json")

			w := httptest.NewRecorder()
			rctx := chi.NewRouteContext()
			rctx.URLParams.Add("appID", tc.appId)

			req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
			req = req.WithContext(context.WithValue(req.Context(), appCtx, &convoy.Application{
				UID:       appId,
				OrgID:     orgID,
				Title:     "Valid application update",
				Endpoints: []convoy.Endpoint{},
			}))

			if tc.dbFn != nil {
				tc.dbFn(apprepo, org)
			}

			err := config.LoadConfig(tc.cfgPath)
			if err != nil {
				t.Error("Failed to load config file")
			}

			router := buildRoutes(app)

			// Act.
			router.ServeHTTP(w, req)

			if w.Code != tc.statusCode {
				t.Errorf("Want status '%d', got '%d'", tc.statusCode, w.Code)
			}

			verifyMatch(t, *w)
		})
	}

}

func TestApplicationHandler_CreateAppEndpoint(t *testing.T) {

	var app *applicationHandler

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	org := mocks.NewMockOrganisationRepository(ctrl)
	apprepo := mocks.NewMockApplicationRepository(ctrl)
	msgRepo := mocks.NewMockMessageRepository(ctrl)

	orgID := "1234567890"

	bodyReader := strings.NewReader(`{"url": "https://google.com", "description": "Test"}`)

	app = newApplicationHandler(msgRepo, apprepo, org)

	appId := "123456789"

	tt := []struct {
		name       string
		cfgPath    string
		method     string
		statusCode int
		appId      string
		body       *strings.Reader
		dbFn       func(appRepo *mocks.MockApplicationRepository, orgRepo *mocks.MockOrganisationRepository)
	}{
		{
			name:       "valid application endpoint",
			cfgPath:    "./testdata/Auth_Config/basic-convoy.json",
			method:     http.MethodPost,
			statusCode: http.StatusCreated,
			appId:      appId,
			body:       bodyReader,
			dbFn: func(appRepo *mocks.MockApplicationRepository, orgRepo *mocks.MockOrganisationRepository) {
				appRepo.EXPECT().
					UpdateApplication(gomock.Any(), gomock.Any()).Times(1).
					Return(nil)

				appRepo.EXPECT().
					FindApplicationByID(gomock.Any(), gomock.Any()).Times(2).
					Return(&convoy.Application{
						UID:       appId,
						OrgID:     orgID,
						Title:     "Valid application endpoint",
						Endpoints: []convoy.Endpoint{},
					}, nil)

			},
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			url := fmt.Sprintf("/v1/applications/%s/endpoints", tc.appId)
			req := httptest.NewRequest(tc.method, url, tc.body)
			req.SetBasicAuth("test", "test")
			req.Header.Add("Content-Type", "application/json")
			w := httptest.NewRecorder()

			if tc.dbFn != nil {
				tc.dbFn(apprepo, org)
			}

			err := config.LoadConfig(tc.cfgPath)
			if err != nil {
				t.Error("Failed to load config file")
			}

			router := buildRoutes(app)

			// Act
			router.ServeHTTP(w, req)

			if w.Code != tc.statusCode {
				t.Errorf("Want status '%d', got '%d'", tc.statusCode, w.Code)
			}
		})
	}

}

func TestApplicationHandler_UpdateAppEndpoint_InvalidRequest(t *testing.T) {

	var app *applicationHandler

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	org := mocks.NewMockOrganisationRepository(ctrl)
	apprepo := mocks.NewMockApplicationRepository(ctrl)
	msgRepo := mocks.NewMockMessageRepository(ctrl)

	orgID := "1234567890"

	appId := "12345"
	endpointId := "9999900000-8888"
	bodyReader := strings.NewReader(`{"url": "http://localhost", "description": "Test"}`)

	app = newApplicationHandler(msgRepo, apprepo, org)

	tt := []struct {
		name       string
		cfgPath    string
		method     string
		statusCode int
		appId      string
		endpointId string
		body       *strings.Reader
		dbFn       func(appRepo *mocks.MockApplicationRepository, orgRepo *mocks.MockOrganisationRepository)
	}{
		{
			name:       "invalid application endpoint update",
			cfgPath:    "./testdata/Auth_Config/basic-convoy.json",
			method:     http.MethodPut,
			statusCode: http.StatusBadRequest,
			appId:      appId,
			endpointId: endpointId,
			body:       bodyReader,
			dbFn: func(appRepo *mocks.MockApplicationRepository, orgRepo *mocks.MockOrganisationRepository) {
				appRepo.EXPECT().
					UpdateApplication(gomock.Any(), gomock.Any()).Times(0).
					Return(nil)

				appRepo.EXPECT().
					FindApplicationByID(gomock.Any(), gomock.Any()).Times(1).
					Return(&convoy.Application{
						UID:       appId,
						OrgID:     orgID,
						Title:     "invalid application update",
						Endpoints: []convoy.Endpoint{},
					}, nil)

			},
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			// Arrange
			url := fmt.Sprintf("/v1/applications/%s/endpoints/%s", tc.appId, tc.endpointId)
			req := httptest.NewRequest(tc.method, url, tc.body)
			req.SetBasicAuth("test", "test")
			req.Header.Add("Content-Type", "application/json")

			w := httptest.NewRecorder()
			rctx := chi.NewRouteContext()
			rctx.URLParams.Add("appID", tc.appId)
			rctx.URLParams.Add("endpointID", tc.endpointId)

			req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
			req = req.WithContext(context.WithValue(req.Context(), appCtx, &convoy.Application{
				UID:       appId,
				OrgID:     orgID,
				Title:     "Valid application update",
				Endpoints: []convoy.Endpoint{},
			}))

			// Arrange Expectations
			if tc.dbFn != nil {
				tc.dbFn(apprepo, org)
			}

			err := config.LoadConfig(tc.cfgPath)
			if err != nil {
				t.Error("Failed to load config file")
			}

			router := buildRoutes(app)

			// Act
			router.ServeHTTP(w, req)

			if w.Code != tc.statusCode {
				t.Errorf("Want status '%d', got '%d'", tc.statusCode, w.Code)
			}

			verifyMatch(t, *w)
		})
	}

}

func TestApplicationHandler_UpdateAppEndpoint_ValidRequest(t *testing.T) {

	var app *applicationHandler

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	org := mocks.NewMockOrganisationRepository(ctrl)
	apprepo := mocks.NewMockApplicationRepository(ctrl)
	msgRepo := mocks.NewMockMessageRepository(ctrl)

	orgID := "1234567890"

	appId := "12345"
	endpointId := "9999900000-8888"
	bodyReader := strings.NewReader(`{"url": "https://google.com", "description": "Test"}`)

	app = newApplicationHandler(msgRepo, apprepo, org)

	tt := []struct {
		name       string
		cfgPath    string
		method     string
		statusCode int
		appId      string
		endpointId string
		body       *strings.Reader
		dbFn       func(appRepo *mocks.MockApplicationRepository, orgRepo *mocks.MockOrganisationRepository)
	}{
		{
			name:       "valid application endpoint update",
			cfgPath:    "./testdata/Auth_Config/basic-convoy.json",
			method:     http.MethodPut,
			statusCode: http.StatusAccepted,
			appId:      appId,
			endpointId: endpointId,
			body:       bodyReader,
			dbFn: func(appRepo *mocks.MockApplicationRepository, orgRepo *mocks.MockOrganisationRepository) {
				appRepo.EXPECT().
					UpdateApplication(gomock.Any(), gomock.Any()).Times(1).
					Return(nil)

				appRepo.EXPECT().
					FindApplicationByID(gomock.Any(), gomock.Any()).Times(1).
					Return(&convoy.Application{
						UID:   appId,
						OrgID: orgID,
						Title: "Valid application update",
						Endpoints: []convoy.Endpoint{
							{
								UID:         endpointId,
								TargetURL:   "http://",
								Description: "desc",
							},
						},
					}, nil)

			},
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			url := fmt.Sprintf("/v1/applications/%s/endpoints/%s", tc.appId, tc.endpointId)
			req := httptest.NewRequest(tc.method, url, tc.body)
			req.SetBasicAuth("test", "test")
			req.Header.Add("Content-Type", "application/json")

			w := httptest.NewRecorder()
			rctx := chi.NewRouteContext()
			rctx.URLParams.Add("appID", tc.appId)
			rctx.URLParams.Add("endpointID", tc.endpointId)

			req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
			req = req.WithContext(context.WithValue(req.Context(), appCtx, &convoy.Application{
				UID:   appId,
				OrgID: orgID,
				Title: "Valid application update",
				Endpoints: []convoy.Endpoint{
					{
						UID:         endpointId,
						TargetURL:   "http://",
						Description: "desc",
					},
				},
			}))

			if tc.dbFn != nil {
				tc.dbFn(apprepo, org)
			}

			err := config.LoadConfig(tc.cfgPath)
			if err != nil {
				t.Error("Failed to load config file")
			}

			router := buildRoutes(app)

			// Act
			router.ServeHTTP(w, req)

			if w.Code != tc.statusCode {
				t.Errorf("Want status '%d', got '%d'", tc.statusCode, w.Code)
			}
		})
	}

}

func Test_applicationHandler_GetDashboardSummary(t *testing.T) {

	var app *applicationHandler

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	org := mocks.NewMockOrganisationRepository(ctrl)
	apprepo := mocks.NewMockApplicationRepository(ctrl)
	msgRepo := mocks.NewMockMessageRepository(ctrl)

	orgID := "1234567890"

	organisation := &convoy.Organisation{
		UID:     orgID,
		OrgName: "Valid organisation",
	}

	app = newApplicationHandler(msgRepo, apprepo, org)

	tt := []struct {
		name       string
		method     string
		statusCode int
		dbFn       func(msgRepo *mocks.MockMessageRepository, appRepo *mocks.MockApplicationRepository, orgRepo *mocks.MockOrganisationRepository)
	}{
		{
			name:       "valid organisations",
			method:     http.MethodGet,
			statusCode: http.StatusOK,
			dbFn: func(msgRepo *mocks.MockMessageRepository, appRepo *mocks.MockApplicationRepository, orgRepo *mocks.MockOrganisationRepository) {
				appRepo.EXPECT().
					SearchApplicationsByOrgId(gomock.Any(), gomock.Any(), gomock.Any()).Times(1).
					Return([]convoy.Application{
						{
							UID:       "validID",
							OrgID:     orgID,
							Title:     "Valid application - 0",
							Endpoints: []convoy.Endpoint{},
						},
					}, nil)
				msgRepo.EXPECT().
					LoadMessageIntervals(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Times(1).
					Return([]models.MessageInterval{
						{
							Data: models.MessageIntervalData{
								Interval: 12,
								Time:     "2020-10",
							},
							Count: 10,
						},
					}, nil)

			},
		},
	}

	format := "2006-01-02T15:04:05"

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			request := httptest.NewRequest(tc.method, fmt.Sprintf("/v1/dashboard/%s/summary?startDate=%s&type=daily", orgID, time.Now().Format(format)), nil)
			responseRecorder := httptest.NewRecorder()
			rctx := chi.NewRouteContext()
			rctx.URLParams.Add("orgID", orgID)

			request = request.WithContext(context.WithValue(request.Context(), chi.RouteCtxKey, rctx))
			request = request.WithContext(context.WithValue(request.Context(), orgCtx, organisation))

			if tc.dbFn != nil {
				tc.dbFn(msgRepo, apprepo, org)
			}

			fetchDashboardSummary(apprepo, msgRepo)(http.HandlerFunc(app.GetDashboardSummary)).
				ServeHTTP(responseRecorder, request)

			if responseRecorder.Code != tc.statusCode {
				t.Errorf("Want status '%d', got '%d'", tc.statusCode, responseRecorder.Code)
			}
		})
	}

}

func Test_applicationHandler_GetPaginatedApps(t *testing.T) {

	var app *applicationHandler

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	org := mocks.NewMockOrganisationRepository(ctrl)
	apprepo := mocks.NewMockApplicationRepository(ctrl)
	msgRepo := mocks.NewMockMessageRepository(ctrl)

	orgID := "1234567890"

	organisation := &convoy.Organisation{
		UID:     orgID,
		OrgName: "Valid organisation",
	}

	app = newApplicationHandler(msgRepo, apprepo, org)

	tt := []struct {
		name       string
		method     string
		statusCode int
		dbFn       func(appRepo *mocks.MockApplicationRepository, orgRepo *mocks.MockOrganisationRepository)
	}{
		{
			name:       "valid organisations",
			method:     http.MethodGet,
			statusCode: http.StatusOK,
			dbFn: func(appRepo *mocks.MockApplicationRepository, orgRepo *mocks.MockOrganisationRepository) {
				appRepo.EXPECT().
					LoadApplicationsPagedByOrgId(gomock.Any(), gomock.Any(), gomock.Any()).Times(1).
					Return([]convoy.Application{
						{
							UID:       "validID",
							OrgID:     orgID,
							Title:     "Valid application - 0",
							Endpoints: []convoy.Endpoint{},
						},
					},
						pager.PaginationData{},
						nil)

			},
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			request := httptest.NewRequest(tc.method, fmt.Sprintf("/v1/dashboard/%s/apps?page=1", orgID), nil)
			responseRecorder := httptest.NewRecorder()
			rctx := chi.NewRouteContext()
			rctx.URLParams.Add("orgID", orgID)

			request = request.WithContext(context.WithValue(request.Context(), chi.RouteCtxKey, rctx))
			request = request.WithContext(context.WithValue(request.Context(), orgCtx, organisation))

			pageable := models.Pageable{
				Page:    1,
				PerPage: 10,
			}
			request = request.WithContext(context.WithValue(request.Context(), pageableCtx, pageable))

			if tc.dbFn != nil {
				tc.dbFn(apprepo, org)
			}

			fetchOrganisationApps(apprepo)(http.HandlerFunc(app.GetPaginatedApps)).
				ServeHTTP(responseRecorder, request)

			if responseRecorder.Code != tc.statusCode {
				t.Errorf("Want status '%d', got '%d'", tc.statusCode, responseRecorder.Code)
			}

			verifyMatch(t, *responseRecorder)
		})
	}

}
