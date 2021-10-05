package server

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/mocks"
	"github.com/go-chi/chi/v5"
	"github.com/golang/mock/gomock"
)

func TestApplicationHandler_GetOrganisation(t *testing.T) {

	var app *applicationHandler

	realOrgID := "1234567890"
	fakeOrgID := "12345"

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	orgRepo := mocks.NewMockOrganisationRepository(ctrl)
	appRepo := mocks.NewMockApplicationRepository(ctrl)
	msgRepo := mocks.NewMockMessageRepository(ctrl)

	app = newApplicationHandler(msgRepo, appRepo, orgRepo)

	tt := []struct {
		name       string
		cfgPath    string
		method     string
		statusCode int
		id         string
		dbFn       func(app *applicationHandler)
	}{
		{
			name:       "Organisation not found",
			cfgPath:    "./testdata/Auth_Config/basic-convoy.json",
			method:     http.MethodGet,
			statusCode: http.StatusNotFound,
			id:         fakeOrgID,
			dbFn: func(app *applicationHandler) {
				o, _ := app.orgRepo.(*mocks.MockOrganisationRepository)
				o.EXPECT().
					FetchOrganisationByID(gomock.Any(), fakeOrgID).
					Return(nil, convoy.ErrOrganisationNotFound).Times(1)
			},
		},
		{
			name:       "Valid Organisation",
			cfgPath:    "./testdata/Auth_Config/basic-convoy.json",
			method:     http.MethodGet,
			statusCode: http.StatusOK,
			id:         realOrgID,
			dbFn: func(app *applicationHandler) {
				orgRepo.EXPECT().
					FetchOrganisationByID(gomock.Any(), realOrgID).Times(1).
					Return(&convoy.Organisation{
						UID:     realOrgID,
						OrgName: "Valid organisation",
					}, nil)
			},
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			// Arrange
			url := fmt.Sprintf("/v1/organisations/%s", tc.id)
			req := httptest.NewRequest(tc.method, url, nil)
			req.SetBasicAuth("test", "test")
			req.Header.Add("Content-Type", "application/json")
			w := httptest.NewRecorder()

			rctx := chi.NewRouteContext()
			rctx.URLParams.Add("orgID", tc.id)

			req = req.Clone(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

			// Arrange Expectations
			if tc.dbFn != nil {
				tc.dbFn(app)
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

func TestApplicationHandler_CreateOrganisation(t *testing.T) {
	var app *applicationHandler

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	orgRepo := mocks.NewMockOrganisationRepository(ctrl)
	appRepo := mocks.NewMockApplicationRepository(ctrl)
	msgRepo := mocks.NewMockMessageRepository(ctrl)

	app = newApplicationHandler(msgRepo, appRepo, orgRepo)

	bodyReader := strings.NewReader(`{"name": "ABC_DEF_TEST_UPDATE"}`)

	tt := []struct {
		name       string
		cfgPath    string
		method     string
		statusCode int
		body       *strings.Reader
		dbFn       func(*applicationHandler)
	}{
		{
			name:       "Valid organisation",
			cfgPath:    "./testdata/Auth_Config/basic-convoy.json",
			method:     http.MethodPost,
			statusCode: http.StatusCreated,
			body:       bodyReader,
			dbFn: func(app *applicationHandler) {
				o, _ := app.orgRepo.(*mocks.MockOrganisationRepository)
				o.EXPECT().
					CreateOrganisation(gomock.Any(), gomock.Any()).Times(1).
					Return(nil)
			},
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			// Arrange
			req := httptest.NewRequest(tc.method, "/v1/organisations", tc.body)
			req.SetBasicAuth("test", "test")
			req.Header.Add("Content-Type", "application/json")
			w := httptest.NewRecorder()

			// Arrange Expectations
			if tc.dbFn != nil {
				tc.dbFn(app)
			}

			err := config.LoadConfig(tc.cfgPath)
			if err != nil {
				t.Error("Failed to load config file")
			}

			router := buildRoutes(app)

			// Act.
			router.ServeHTTP(w, req)

			// Assert.
			if w.Code != tc.statusCode {
				t.Errorf("want status '%d', got '%d'", tc.statusCode, w.Code)
			}
		})
	}
}

func TestApplicationHandler_UpdateOrganisation(t *testing.T) {
	var app *applicationHandler

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	orgRepo := mocks.NewMockOrganisationRepository(ctrl)
	appRepo := mocks.NewMockApplicationRepository(ctrl)
	msgRepo := mocks.NewMockMessageRepository(ctrl)

	app = newApplicationHandler(msgRepo, appRepo, orgRepo)

	realOrgID := "1234567890"

	bodyReader := strings.NewReader(`{"name": "ABC_DEF_TEST_UPDATE"}`)
	tt := []struct {
		name       string
		cfgPath    string
		method     string
		statusCode int
		orgID      string
		body       *strings.Reader
		dbFn       func(app *applicationHandler)
	}{
		{
			name:       "Valid organisation update",
			cfgPath:    "./testdata/Auth_Config/basic-convoy.json",
			method:     http.MethodPut,
			statusCode: http.StatusAccepted,
			orgID:      realOrgID,
			body:       bodyReader,
			dbFn: func(app *applicationHandler) {
				o, _ := app.orgRepo.(*mocks.MockOrganisationRepository)
				o.EXPECT().
					UpdateOrganisation(gomock.Any(), gomock.Any()).Times(1).
					Return(nil)

				o.EXPECT().
					FetchOrganisationByID(gomock.Any(), gomock.Any()).Times(2).
					Return(&convoy.Organisation{
						UID:     realOrgID,
						OrgName: "Valid organisation update",
					}, nil)
			},
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			// Arrange
			url := fmt.Sprintf("/v1/organisations/%s", tc.orgID)
			req := httptest.NewRequest(tc.method, url, tc.body)
			req.SetBasicAuth("test", "test")
			req.Header.Add("Content-Type", "application/json")
			w := httptest.NewRecorder()

			rctx := chi.NewRouteContext()
			rctx.URLParams.Add("orgID", tc.orgID)

			req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

			// Arrange Expectations
			if tc.dbFn != nil {
				tc.dbFn(app)
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

func TestApplicationHandler_GetOrganisations(t *testing.T) {
	var app *applicationHandler

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	orgRepo := mocks.NewMockOrganisationRepository(ctrl)
	appRepo := mocks.NewMockApplicationRepository(ctrl)
	msgRepo := mocks.NewMockMessageRepository(ctrl)

	app = newApplicationHandler(msgRepo, appRepo, orgRepo)

	realOrgID := "1234567890"

	tt := []struct {
		name       string
		cfgPath    string
		method     string
		statusCode int
		dbFn       func(app *applicationHandler)
	}{
		{
			name:       "valid organisations",
			cfgPath:    "./testdata/Auth_Config/basic-convoy.json",
			method:     http.MethodGet,
			statusCode: http.StatusOK,
			dbFn: func(app *applicationHandler) {
				o, _ := app.orgRepo.(*mocks.MockOrganisationRepository)
				o.EXPECT().
					LoadOrganisations(gomock.Any()).Times(1).
					Return([]*convoy.Organisation{
						{
							UID:     realOrgID,
							OrgName: "Valid organisations - 0",
						},
					}, nil)
			},
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, "/v1/organisations", nil)
			req.SetBasicAuth("test", "test")
			w := httptest.NewRecorder()

			// Arrange Expectations
			if tc.dbFn != nil {
				tc.dbFn(app)
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
