package server

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	mongopagination "github.com/gobeam/mongo-go-pagination"

	"github.com/frain-dev/convoy"

	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/mocks"
	"github.com/go-chi/chi/v5"
	"github.com/golang/mock/gomock"
)

func TestApplicationHandler_CreateAPIKey(t *testing.T) {
	var app *applicationHandler

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	groupRepo := mocks.NewMockGroupRepository(ctrl)
	appRepo := mocks.NewMockApplicationRepository(ctrl)
	eventRepo := mocks.NewMockEventRepository(ctrl)
	eventDeliveryRepo := mocks.NewMockEventDeliveryRepository(ctrl)
	eventQueue := mocks.NewMockQueuer(ctrl)
	apiKeyRepo := mocks.NewMockAPIKeyRepo(ctrl)

	app = newApplicationHandler(eventRepo, eventDeliveryRepo, appRepo, groupRepo, apiKeyRepo, eventQueue)

	tt := []struct {
		name           string
		cfgPath        string
		statusCode     int
		stripTimestamp bool
		body           *strings.Reader
		dbFn           func(app *applicationHandler)
	}{
		{
			name:           "create api key",
			stripTimestamp: true,
			cfgPath:        "./testdata/Auth_Config/no-auth-convoy.json",
			statusCode:     http.StatusCreated,
			body: strings.NewReader(`{
					"expires_at": "2029-01-02T15:04:05+01:00", 
					"role": {
                        "type": "ui_admin",
                        "groups": [
                            "sendcash-pay"
                        ]
                    }
                }`),
			dbFn: func(app *applicationHandler) {
				a, _ := app.apiKeyRepo.(*mocks.MockAPIKeyRepo)
				a.EXPECT().CreateAPIKey(gomock.Any(), gomock.Any()).Times(1).Return(nil)
			},
		},
		{
			name:           "invalid expiry date",
			stripTimestamp: false,
			cfgPath:        "./testdata/Auth_Config/no-auth-convoy.json",
			statusCode:     http.StatusBadRequest,
			body: strings.NewReader(`{
					"expires_at": "2020-01-02T15:04:05+01:00", 
					"role": {
                        "type": "ui_admin",
                        "groups": [
                            "sendcash-pay"
                        ]
                    }
                }`),
			dbFn: nil,
		},
		{
			name:           "create api key without expires_at field",
			stripTimestamp: true,
			cfgPath:        "./testdata/Auth_Config/no-auth-convoy.json",
			statusCode:     http.StatusCreated,
			body: strings.NewReader(`{
					"role": {
                        "type": "ui_admin",
                        "groups": [
                            "sendcash-pay"
                        ]
                    }
                }`),
			dbFn: func(app *applicationHandler) {
				a, _ := app.apiKeyRepo.(*mocks.MockAPIKeyRepo)
				a.EXPECT().CreateAPIKey(gomock.Any(), gomock.Any()).Times(1).Return(nil)
			},
		},
		{
			name:           "invalid role",
			stripTimestamp: false,
			cfgPath:        "./testdata/Auth_Config/no-auth-convoy.json",
			statusCode:     http.StatusBadRequest,
			body: strings.NewReader(`{
					"key": "12344",
					"expires_at": "2029-01-02T15:04:05+01:00",
                    "role": {
                        "type": "admin",
                        "groups": []
                    }
                }`),
			dbFn: nil,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			// Arrange
			url := "/api/v1/security/keys"
			req := httptest.NewRequest(http.MethodPost, url, tc.body)
			req.SetBasicAuth("test", "test")
			w := httptest.NewRecorder()
			rctx := chi.NewRouteContext()
			req.Header.Add("Content-Type", "application/json")

			req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

			// Arrange Expectations
			if tc.dbFn != nil {
				tc.dbFn(app)
			}

			err := config.LoadConfig(tc.cfgPath)
			if err != nil {
				t.Errorf("Failed to load config file: %v", err)
			}
			initRealmChain(t, app.apiKeyRepo)

			router := buildRoutes(app)

			// Act
			router.ServeHTTP(w, req)

			// Assert
			if w.Code != tc.statusCode {
				t.Errorf("Want status '%d', got '%d'", tc.statusCode, w.Code)
			}

			fmt.Println("d", w.Body.String())

			if tc.stripTimestamp {
				d := stripTimestamp(t, "apiKey", w.Body)
				w.Body = d
			}

			verifyMatch(t, *w)
		})
	}
}

func TestApplicationHandler_RevokeAPIKey(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	groupRepo := mocks.NewMockGroupRepository(ctrl)
	appRepo := mocks.NewMockApplicationRepository(ctrl)
	eventRepo := mocks.NewMockEventRepository(ctrl)
	eventDeliveryRepo := mocks.NewMockEventDeliveryRepository(ctrl)
	eventQueue := mocks.NewMockQueuer(ctrl)
	apiKeyRepo := mocks.NewMockAPIKeyRepo(ctrl)

	app := newApplicationHandler(eventRepo, eventDeliveryRepo, appRepo, groupRepo, apiKeyRepo, eventQueue)

	tt := []struct {
		name       string
		cfgPath    string
		statusCode int
		keyID      string
		dbFn       func(app *applicationHandler)
	}{
		{
			name:       "revoke api key",
			cfgPath:    "./testdata/Auth_Config/no-auth-convoy.json",
			statusCode: http.StatusOK,
			keyID:      "123",
			dbFn: func(app *applicationHandler) {
				a, _ := app.apiKeyRepo.(*mocks.MockAPIKeyRepo)
				a.EXPECT().RevokeAPIKeys(gomock.Any(), gomock.Any()).Times(1).Return(nil)
			},
		},
		{
			name:       "should error for revoke api key",
			cfgPath:    "./testdata/Auth_Config/no-auth-convoy.json",
			statusCode: http.StatusInternalServerError,
			keyID:      "123",
			dbFn: func(app *applicationHandler) {
				a, _ := app.apiKeyRepo.(*mocks.MockAPIKeyRepo)
				a.EXPECT().RevokeAPIKeys(gomock.Any(), gomock.Any()).Times(1).Return(errors.New("abc"))
			},
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			// Arrange
			url := fmt.Sprintf("/api/v1/security/keys/%s/revoke", tc.keyID)
			req := httptest.NewRequest(http.MethodPut, url, nil)
			req.SetBasicAuth("test", "test")
			w := httptest.NewRecorder()
			rctx := chi.NewRouteContext()
			req.Header.Add("Content-Type", "application/json")

			req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

			// Arrange Expectations
			if tc.dbFn != nil {
				tc.dbFn(app)
			}

			err := config.LoadConfig(tc.cfgPath)
			if err != nil {
				t.Errorf("Failed to load config file: %v", err)
			}
			initRealmChain(t, app.apiKeyRepo)

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

func TestApplicationHandler_GetAPIKeyByID(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	groupRepo := mocks.NewMockGroupRepository(ctrl)
	appRepo := mocks.NewMockApplicationRepository(ctrl)
	eventRepo := mocks.NewMockEventRepository(ctrl)
	eventDeliveryRepo := mocks.NewMockEventDeliveryRepository(ctrl)
	eventQueue := mocks.NewMockQueuer(ctrl)
	apiKeyRepo := mocks.NewMockAPIKeyRepo(ctrl)

	app := newApplicationHandler(eventRepo, eventDeliveryRepo, appRepo, groupRepo, apiKeyRepo, eventQueue)

	keyID := "12345"
	apiKey := &convoy.APIKey{UID: keyID}

	tt := []struct {
		name           string
		cfgPath        string
		stripTimestamp bool
		statusCode     int
		keyID          string
		dbFn           func(app *applicationHandler)
	}{
		{
			name:           "should_find_api_key",
			stripTimestamp: true,
			cfgPath:        "./testdata/Auth_Config/no-auth-convoy.json",
			statusCode:     http.StatusOK,
			keyID:          keyID,
			dbFn: func(app *applicationHandler) {
				a, _ := app.apiKeyRepo.(*mocks.MockAPIKeyRepo)
				a.EXPECT().FindAPIKeyByID(gomock.Any(), gomock.Any()).Times(1).Return(apiKey, nil)
			},
		},
		{
			name:           "should_fail_to_find_api_key",
			stripTimestamp: false,
			cfgPath:        "./testdata/Auth_Config/no-auth-convoy.json",
			statusCode:     http.StatusInternalServerError,
			keyID:          keyID,
			dbFn: func(app *applicationHandler) {
				a, _ := app.apiKeyRepo.(*mocks.MockAPIKeyRepo)
				a.EXPECT().FindAPIKeyByID(gomock.Any(), gomock.Any()).Times(1).Return(nil, errors.New("abc"))
			},
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			// Arrange
			url := fmt.Sprintf("/api/v1/security/keys/%s", tc.keyID)
			req := httptest.NewRequest(http.MethodGet, url, nil)
			req.SetBasicAuth("test", "test")
			w := httptest.NewRecorder()
			rctx := chi.NewRouteContext()
			req.Header.Add("Content-Type", "application/json")

			req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

			// Arrange Expectations
			if tc.dbFn != nil {
				tc.dbFn(app)
			}

			err := config.LoadConfig(tc.cfgPath)
			if err != nil {
				t.Errorf("Failed to load config file: %v", err)
			}
			initRealmChain(t, app.apiKeyRepo)

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

func TestApplicationHandler_GetAPIKeys(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	groupRepo := mocks.NewMockGroupRepository(ctrl)
	appRepo := mocks.NewMockApplicationRepository(ctrl)
	eventRepo := mocks.NewMockEventRepository(ctrl)
	eventDeliveryRepo := mocks.NewMockEventDeliveryRepository(ctrl)
	eventQueue := mocks.NewMockQueuer(ctrl)
	apiKeyRepo := mocks.NewMockAPIKeyRepo(ctrl)

	app := newApplicationHandler(eventRepo, eventDeliveryRepo, appRepo, groupRepo, apiKeyRepo, eventQueue)

	keyID := "12345"
	apiKey := &convoy.APIKey{UID: keyID}

	page := &convoy.Pageable{
		Page:    1,
		PerPage: 100,
		Sort:    1,
	}

	tt := []struct {
		name       string
		cfgPath    string
		statusCode int
		dbFn       func(app *applicationHandler)
	}{
		{
			name:       "should_load_api_keys",
			cfgPath:    "./testdata/Auth_Config/no-auth-convoy.json",
			statusCode: http.StatusOK,
			dbFn: func(app *applicationHandler) {
				a, _ := app.apiKeyRepo.(*mocks.MockAPIKeyRepo)
				a.EXPECT().LoadAPIKeysPaged(gomock.Any(), gomock.Any()).Times(1).Return([]convoy.APIKey{*apiKey}, &mongopagination.PaginationData{PerPage: int64(page.PerPage)}, nil)
			},
		},
		{
			name:       "should_fail_to_load_api_keys",
			cfgPath:    "./testdata/Auth_Config/no-auth-convoy.json",
			statusCode: http.StatusInternalServerError,
			dbFn: func(app *applicationHandler) {
				a, _ := app.apiKeyRepo.(*mocks.MockAPIKeyRepo)
				a.EXPECT().LoadAPIKeysPaged(gomock.Any(), gomock.Any()).Times(1).Return(nil, nil, errors.New("abc"))
			},
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			// Arrange
			url := fmt.Sprintf("/api/v1/security/keys?perPage=%d&page=%d&sort=%d", page.PerPage, page.Page, page.Sort)
			req := httptest.NewRequest(http.MethodGet, url, nil)
			req.SetBasicAuth("test", "test")
			w := httptest.NewRecorder()
			rctx := chi.NewRouteContext()
			req.Header.Add("Content-Type", "application/json")

			req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

			// Arrange Expectations
			if tc.dbFn != nil {
				tc.dbFn(app)
			}

			err := config.LoadConfig(tc.cfgPath)
			if err != nil {
				t.Errorf("Failed to load config file: %v", err)
			}
			initRealmChain(t, app.apiKeyRepo)

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
