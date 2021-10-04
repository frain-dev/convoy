package server

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/frain-dev/convoy"
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
		method     string
		statusCode int
		id         string
		dbFn       func(appRepo *mocks.MockApplicationRepository, orgRepo *mocks.MockOrganisationRepository)
	}{
		{
			name:       "Organisation not found",
			method:     http.MethodGet,
			statusCode: http.StatusNotFound,
			id:         fakeOrgID,
			dbFn: func(appRepo *mocks.MockApplicationRepository, orgRepo *mocks.MockOrganisationRepository) {
				orgRepo.EXPECT().
					FetchOrganisationByID(gomock.Any(), fakeOrgID).
					Return(nil, convoy.ErrOrganisationNotFound).Times(1)
			},
		},
		{
			name:       "Valid Organisation",
			method:     http.MethodGet,
			statusCode: http.StatusOK,
			id:         realOrgID,
			dbFn: func(appRepo *mocks.MockApplicationRepository, orgRepo *mocks.MockOrganisationRepository) {
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
			url := fmt.Sprintf("/v1/organisations/%s", tc.id)
			req := httptest.NewRequest(tc.method, url, nil)
			rctx := chi.NewRouteContext()
			rctx.URLParams.Add("orgID", tc.id)

			req = req.Clone(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

			w := httptest.NewRecorder()

			if tc.dbFn != nil {
				tc.dbFn(appRepo, orgRepo)
			}

			requireOrganisation(orgRepo)(http.HandlerFunc(app.GetOrganisation)).
				ServeHTTP(w, req)

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
		method     string
		statusCode int
		body       *strings.Reader
		dbFn       func(appRepo *mocks.MockApplicationRepository, orgRepo *mocks.MockOrganisationRepository)
	}{
		{
			name:       "Valid organisation",
			method:     http.MethodPost,
			statusCode: http.StatusCreated,
			body:       bodyReader,
			dbFn: func(appRepo *mocks.MockApplicationRepository, orgRepo *mocks.MockOrganisationRepository) {
				orgRepo.EXPECT().
					CreateOrganisation(gomock.Any(), gomock.Any()).Times(1).
					Return(nil)
			},
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, "/v1/organisations", tc.body)
			w := httptest.NewRecorder()

			if tc.dbFn != nil {
				tc.dbFn(appRepo, orgRepo)
			}

			http.HandlerFunc(app.CreateOrganisation).
				ServeHTTP(w, req)

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
		method     string
		statusCode int
		orgID      string
		body       *strings.Reader
		dbFn       func(appRepo *mocks.MockApplicationRepository, orgRepo *mocks.MockOrganisationRepository)
	}{
		{
			name:       "Valid organisation update",
			method:     http.MethodPut,
			statusCode: http.StatusAccepted,
			orgID:      realOrgID,
			body:       bodyReader,
			dbFn: func(appRepo *mocks.MockApplicationRepository, orgRepo *mocks.MockOrganisationRepository) {
				orgRepo.EXPECT().
					UpdateOrganisation(gomock.Any(), gomock.Any()).Times(1).
					Return(nil)

				orgRepo.EXPECT().
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
			req := httptest.NewRequest(tc.method, fmt.Sprintf("/v1/organisations/%s", tc.orgID), tc.body)
			w := httptest.NewRecorder()
			rctx := chi.NewRouteContext()
			rctx.URLParams.Add("orgID", tc.orgID)

			req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

			if tc.dbFn != nil {
				tc.dbFn(appRepo, orgRepo)
			}

			requireOrganisation(orgRepo)(http.HandlerFunc(app.UpdateOrganisation)).
				ServeHTTP(w, req)

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
		method     string
		statusCode int
		dbFn       func(appRepo *mocks.MockApplicationRepository, orgRepo *mocks.MockOrganisationRepository)
	}{
		{
			name:       "valid organisations",
			method:     http.MethodGet,
			statusCode: http.StatusOK,
			dbFn: func(appRepo *mocks.MockApplicationRepository, orgRepo *mocks.MockOrganisationRepository) {
				orgRepo.EXPECT().
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
			w := httptest.NewRecorder()

			if tc.dbFn != nil {
				tc.dbFn(appRepo, orgRepo)
			}

			http.HandlerFunc(app.GetOrganisations).
				ServeHTTP(w, req)

			if w.Code != tc.statusCode {
				t.Errorf("Want status '%d', got '%d'", tc.statusCode, w.Code)
			}

			verifyMatch(t, *w)
		})
	}
}
