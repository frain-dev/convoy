package server

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/golang/mock/gomock"
	"github.com/hookcamp/hookcamp"
	"github.com/hookcamp/hookcamp/mocks"
	goldie "github.com/sebdah/goldie/v2"
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

	orgID := "1234567890"

	organisation := &hookcamp.Organisation{
		UID: orgID,
	}

	validID := "123456789"

	// apprepo.EXPECT().

	app = newApplicationHandler(apprepo, org)

	tt := []struct {
		name       string
		method     string
		statusCode int
		input      string
		dbFn       func(appRepo *mocks.MockApplicationRepository, orgRepo *mocks.MockOrganisationRepository)
	}{
		{
			name:       "app not found",
			method:     http.MethodGet,
			statusCode: http.StatusNotFound,
			input:      "12345",
			dbFn: func(appRepo *mocks.MockApplicationRepository, orgRepo *mocks.MockOrganisationRepository) {
				appRepo.EXPECT().
					FindApplicationByID(gomock.Any(), gomock.Any()).
					Return(nil, hookcamp.ErrApplicationNotFound).Times(1)
			},
		},
		{
			name:       "valid application",
			method:     http.MethodGet,
			statusCode: http.StatusOK,
			input:      validID,
			dbFn: func(appRepo *mocks.MockApplicationRepository, orgRepo *mocks.MockOrganisationRepository) {
				appRepo.EXPECT().
					FindApplicationByID(gomock.Any(), gomock.Any()).Times(1).
					Return(&hookcamp.Application{
						UID:       validID,
						OrgID:     orgID,
						Title:     "Valid application",
						Endpoints: []hookcamp.Endpoint{},
					}, nil)

			},
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			request := httptest.NewRequest(tc.method, fmt.Sprintf("/v1/apps/%s", tc.input), nil)
			responseRecorder := httptest.NewRecorder()
			rctx := chi.NewRouteContext()
			rctx.URLParams.Add("id", tc.input)

			request = request.WithContext(context.WithValue(request.Context(), chi.RouteCtxKey, rctx))

			request = request.WithContext(setOrgInContext(request.Context(), organisation))

			if tc.dbFn != nil {
				tc.dbFn(apprepo, org)
			}

			requireAppOwnership(apprepo)(http.HandlerFunc(app.GetApp)).
				ServeHTTP(responseRecorder, request)

			if responseRecorder.Code != tc.statusCode {
				t.Errorf("Want status '%d', got '%d'", tc.statusCode, responseRecorder.Code)
			}

			verifyMatch(t, *responseRecorder)
		})
	}

}

func TestApplicationHandler_GetApps(t *testing.T) {

	var app *applicationHandler

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	org := mocks.NewMockOrganisationRepository(ctrl)
	apprepo := mocks.NewMockApplicationRepository(ctrl)

	orgID := "1234567890"

	organisation := &hookcamp.Organisation{
		UID: orgID,
	}

	validID := "123456789"

	app = newApplicationHandler(apprepo, org)

	tt := []struct {
		name       string
		method     string
		statusCode int
		dbFn       func(appRepo *mocks.MockApplicationRepository, orgRepo *mocks.MockOrganisationRepository)
	}{
		{
			name:       "valid application",
			method:     http.MethodGet,
			statusCode: http.StatusOK,
			dbFn: func(appRepo *mocks.MockApplicationRepository, orgRepo *mocks.MockOrganisationRepository) {
				appRepo.EXPECT().
					LoadApplications(gomock.Any()).Times(1).
					Return([]hookcamp.Application{
						{
							UID:       validID,
							OrgID:     orgID,
							Title:     "Valid application - 0",
							Endpoints: []hookcamp.Endpoint{},
						},
					}, nil)

			},
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			request := httptest.NewRequest(tc.method, "/v1/apps", nil)
			responseRecorder := httptest.NewRecorder()

			request = request.WithContext(setOrgInContext(request.Context(), organisation))

			if tc.dbFn != nil {
				tc.dbFn(apprepo, org)
			}

			fetchAllApps(apprepo)(http.HandlerFunc(app.GetApps)).
				ServeHTTP(responseRecorder, request)

			if responseRecorder.Code != tc.statusCode {
				t.Errorf("Want status '%d', got '%d'", tc.statusCode, responseRecorder.Code)
			}
		})
	}

}

func TestApplicationHandler_CreateApp(t *testing.T) {

	var app *applicationHandler

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	org := mocks.NewMockOrganisationRepository(ctrl)
	apprepo := mocks.NewMockApplicationRepository(ctrl)

	orgID := "1234567890"

	organisation := &hookcamp.Organisation{
		UID: orgID,
	}

	bodyReader := strings.NewReader(`{"name": "ABC_DEF_TEST"}`)

	app = newApplicationHandler(apprepo, org)

	tt := []struct {
		name       string
		method     string
		statusCode int
		body       *strings.Reader
		dbFn       func(appRepo *mocks.MockApplicationRepository, orgRepo *mocks.MockOrganisationRepository)
	}{
		{
			name:       "valid application",
			method:     http.MethodPost,
			statusCode: http.StatusCreated,
			body:       bodyReader,
			dbFn: func(appRepo *mocks.MockApplicationRepository, orgRepo *mocks.MockOrganisationRepository) {
				appRepo.EXPECT().
					CreateApplication(gomock.Any(), gomock.Any()).Times(1).
					Return(nil)

			},
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			request := httptest.NewRequest(tc.method, "/v1/apps", tc.body)
			responseRecorder := httptest.NewRecorder()

			request = request.WithContext(setOrgInContext(request.Context(), organisation))

			if tc.dbFn != nil {
				tc.dbFn(apprepo, org)
			}

			validateNewApp(apprepo)(http.HandlerFunc(app.CreateApp)).
				ServeHTTP(responseRecorder, request)

			if responseRecorder.Code != tc.statusCode {
				t.Errorf("Want status '%d', got '%d'", tc.statusCode, responseRecorder.Code)
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

	orgID := "1234567890"

	organisation := &hookcamp.Organisation{
		UID: orgID,
	}

	appId := "12345"
	bodyReader := strings.NewReader(`{"name": "ABC_DEF_TEST_UPDATE"}`)

	app = newApplicationHandler(apprepo, org)

	tt := []struct {
		name       string
		method     string
		statusCode int
		appId      string
		body       *strings.Reader
		dbFn       func(appRepo *mocks.MockApplicationRepository, orgRepo *mocks.MockOrganisationRepository)
	}{
		{
			name:       "valid application update",
			method:     http.MethodPost,
			statusCode: http.StatusAccepted,
			appId:      appId,
			body:       bodyReader,
			dbFn: func(appRepo *mocks.MockApplicationRepository, orgRepo *mocks.MockOrganisationRepository) {
				appRepo.EXPECT().
					UpdateApplication(gomock.Any(), gomock.Any()).Times(1).
					Return(nil)

				appRepo.EXPECT().
					FindApplicationByID(gomock.Any(), gomock.Any()).Times(1).
					Return(&hookcamp.Application{
						UID:       appId,
						OrgID:     orgID,
						Title:     "Valid application update",
						Endpoints: []hookcamp.Endpoint{},
					}, nil)

			},
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			request := httptest.NewRequest(tc.method, fmt.Sprintf("/v1/apps/%s", tc.appId), tc.body)
			responseRecorder := httptest.NewRecorder()
			rctx := chi.NewRouteContext()
			rctx.URLParams.Add("id", tc.appId)

			request = request.WithContext(context.WithValue(request.Context(), chi.RouteCtxKey, rctx))

			request = request.WithContext(setOrgInContext(request.Context(), organisation))

			if tc.dbFn != nil {
				tc.dbFn(apprepo, org)
			}

			validateAppUpdate(apprepo)(http.HandlerFunc(app.UpdateApp)).
				ServeHTTP(responseRecorder, request)

			if responseRecorder.Code != tc.statusCode {
				t.Errorf("Want status '%d', got '%d'", tc.statusCode, responseRecorder.Code)
			}
		})
	}

}

func TestApplicationHandler_CreateAppEndpoint(t *testing.T) {

	var app *applicationHandler

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	org := mocks.NewMockOrganisationRepository(ctrl)
	apprepo := mocks.NewMockApplicationRepository(ctrl)

	orgID := "1234567890"

	organisation := &hookcamp.Organisation{
		UID: orgID,
	}

	bodyReader := strings.NewReader(`{"url": "http://localhost", "description": "Test"}`)

	app = newApplicationHandler(apprepo, org)

	appId := "123456789"

	tt := []struct {
		name       string
		method     string
		statusCode int
		appId      string
		body       *strings.Reader
		dbFn       func(appRepo *mocks.MockApplicationRepository, orgRepo *mocks.MockOrganisationRepository)
	}{
		{
			name:       "valid application",
			method:     http.MethodPost,
			statusCode: http.StatusCreated,
			appId:      appId,
			body:       bodyReader,
			dbFn: func(appRepo *mocks.MockApplicationRepository, orgRepo *mocks.MockOrganisationRepository) {
				appRepo.EXPECT().
					UpdateApplication(gomock.Any(), gomock.Any()).Times(1).
					Return(nil)

				appRepo.EXPECT().
					FindApplicationByID(gomock.Any(), gomock.Any()).Times(1).
					Return(&hookcamp.Application{
						UID:       appId,
						OrgID:     orgID,
						Title:     "Valid application update",
						Endpoints: []hookcamp.Endpoint{},
					}, nil)

			},
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			request := httptest.NewRequest(tc.method, fmt.Sprintf("/v1/apps/%s/endpoint", tc.appId), tc.body)
			responseRecorder := httptest.NewRecorder()

			request = request.WithContext(setOrgInContext(request.Context(), organisation))

			if tc.dbFn != nil {
				tc.dbFn(apprepo, org)
			}

			validateNewAppEndpoint(apprepo)(http.HandlerFunc(app.CreateAppEndpoint)).
				ServeHTTP(responseRecorder, request)

			if responseRecorder.Code != tc.statusCode {
				t.Errorf("Want status '%d', got '%d'", tc.statusCode, responseRecorder.Code)
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

	orgID := "1234567890"

	organisation := &hookcamp.Organisation{
		UID: orgID,
	}

	appId := "12345"
	endpointId := "9999900000-8888"
	bodyReader := strings.NewReader(`{"url": "http://localhost", "description": "Test"}`)

	app = newApplicationHandler(apprepo, org)

	tt := []struct {
		name       string
		method     string
		statusCode int
		appId      string
		endpointId string
		body       *strings.Reader
		dbFn       func(appRepo *mocks.MockApplicationRepository, orgRepo *mocks.MockOrganisationRepository)
	}{
		{
			name:       "invalid application endpoint update",
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
					Return(&hookcamp.Application{
						UID:       appId,
						OrgID:     orgID,
						Title:     "invalid application update",
						Endpoints: []hookcamp.Endpoint{},
					}, nil)

			},
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			request := httptest.NewRequest(tc.method, fmt.Sprintf("/v1/apps/%s/endpoint/%s", tc.appId, tc.endpointId), tc.body)
			responseRecorder := httptest.NewRecorder()
			rctx := chi.NewRouteContext()
			rctx.URLParams.Add("appID", tc.appId)
			rctx.URLParams.Add("endpointID", tc.endpointId)

			request = request.WithContext(context.WithValue(request.Context(), chi.RouteCtxKey, rctx))

			request = request.WithContext(setOrgInContext(request.Context(), organisation))

			if tc.dbFn != nil {
				tc.dbFn(apprepo, org)
			}

			validateAppEndpointUpdate(apprepo)(http.HandlerFunc(app.UpdateAppEndpoint)).
				ServeHTTP(responseRecorder, request)

			if responseRecorder.Code != tc.statusCode {
				t.Errorf("Want status '%d', got '%d'", tc.statusCode, responseRecorder.Code)
			}
		})
	}

}

func TestApplicationHandler_UpdateAppEndpoint_ValidRequest(t *testing.T) {

	var app *applicationHandler

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	org := mocks.NewMockOrganisationRepository(ctrl)
	apprepo := mocks.NewMockApplicationRepository(ctrl)

	orgID := "1234567890"

	organisation := &hookcamp.Organisation{
		UID: orgID,
	}

	appId := "12345"
	endpointId := "9999900000-8888"
	bodyReader := strings.NewReader(`{"url": "http://localhost", "description": "Test"}`)

	app = newApplicationHandler(apprepo, org)

	tt := []struct {
		name       string
		method     string
		statusCode int
		appId      string
		endpointId string
		body       *strings.Reader
		dbFn       func(appRepo *mocks.MockApplicationRepository, orgRepo *mocks.MockOrganisationRepository)
	}{
		{
			name:       "valid application endpoint update",
			method:     http.MethodPost,
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
					Return(&hookcamp.Application{
						UID:   appId,
						OrgID: orgID,
						Title: "Valid application update",
						Endpoints: []hookcamp.Endpoint{
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
			request := httptest.NewRequest(tc.method, fmt.Sprintf("/v1/apps/%s/endpoint/%s", tc.appId, tc.endpointId), tc.body)
			responseRecorder := httptest.NewRecorder()
			rctx := chi.NewRouteContext()
			rctx.URLParams.Add("appID", tc.appId)
			rctx.URLParams.Add("endpointID", tc.endpointId)

			request = request.WithContext(context.WithValue(request.Context(), chi.RouteCtxKey, rctx))

			request = request.WithContext(setOrgInContext(request.Context(), organisation))

			if tc.dbFn != nil {
				tc.dbFn(apprepo, org)
			}

			validateAppEndpointUpdate(apprepo)(http.HandlerFunc(app.UpdateAppEndpoint)).
				ServeHTTP(responseRecorder, request)

			if responseRecorder.Code != tc.statusCode {
				t.Errorf("Want status '%d', got '%d'", tc.statusCode, responseRecorder.Code)
			}
		})
	}

}
