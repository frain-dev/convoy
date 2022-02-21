package server

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	mcache "github.com/frain-dev/convoy/cache/memory"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/logger"
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
	apiKeyRepo := mocks.NewMockAPIKeyRepository(ctrl)
	logger := logger.NewNoopLogger()
	tracer := mocks.NewMockTracer(ctrl)
	cache := mcache.NewMemoryCache()

	app = newApplicationHandler(eventRepo, eventDeliveryRepo, appRepo, groupRepo, apiKeyRepo, eventQueue, logger, tracer, cache)

	groupId := "1234567890"
	group := &datastore.Group{
		UID: groupId,
		Config: &datastore.GroupConfig{
			Signature: datastore.SignatureConfiguration{
				Header: config.SignatureHeaderProvider("X-datastore.Signature"),
				Hash:   "SHA256",
			},
			Strategy: datastore.StrategyConfiguration{
				Type: config.StrategyProvider("default"),
				Default: datastore.DefaultStrategyConfiguration{
					IntervalSeconds: 60,
					RetryLimit:      1,
				},
			},
			DisableEndpoint: true,
		},
	}

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
				a, _ := app.apiKeyRepo.(*mocks.MockAPIKeyRepository)
				g, _ := app.groupRepo.(*mocks.MockGroupRepository)
				g.EXPECT().
					FetchGroupsByIDs(gomock.Any(), gomock.Any()).
					Times(2).Return([]datastore.Group{*group}, nil)
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
				a, _ := app.apiKeyRepo.(*mocks.MockAPIKeyRepository)
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
                        "type": "invalid-role",
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

			err := config.LoadConfig(tc.cfgPath, new(config.Configuration))
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
	apiKeyRepo := mocks.NewMockAPIKeyRepository(ctrl)
	logger := logger.NewNoopLogger()
	tracer := mocks.NewMockTracer(ctrl)
	cache := mcache.NewMemoryCache()

	app := newApplicationHandler(eventRepo, eventDeliveryRepo, appRepo, groupRepo, apiKeyRepo, eventQueue, logger, tracer, cache)

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
				a, _ := app.apiKeyRepo.(*mocks.MockAPIKeyRepository)
				a.EXPECT().RevokeAPIKeys(gomock.Any(), gomock.Any()).Times(1).Return(nil)
			},
		},
		{
			name:       "should error for revoke api key",
			cfgPath:    "./testdata/Auth_Config/no-auth-convoy.json",
			statusCode: http.StatusInternalServerError,
			keyID:      "123",
			dbFn: func(app *applicationHandler) {
				a, _ := app.apiKeyRepo.(*mocks.MockAPIKeyRepository)
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

			err := config.LoadConfig(tc.cfgPath, new(config.Configuration))
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
	apiKeyRepo := mocks.NewMockAPIKeyRepository(ctrl)
	logger := logger.NewNoopLogger()
	tracer := mocks.NewMockTracer(ctrl)
	cache := mcache.NewMemoryCache()

	app := newApplicationHandler(eventRepo, eventDeliveryRepo, appRepo, groupRepo, apiKeyRepo, eventQueue, logger, tracer, cache)

	keyID := "12345"
	apiKey := &datastore.APIKey{UID: keyID}

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
				a, _ := app.apiKeyRepo.(*mocks.MockAPIKeyRepository)
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
				a, _ := app.apiKeyRepo.(*mocks.MockAPIKeyRepository)
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

			err := config.LoadConfig(tc.cfgPath, new(config.Configuration))
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
	apiKeyRepo := mocks.NewMockAPIKeyRepository(ctrl)
	logger := logger.NewNoopLogger()
	tracer := mocks.NewMockTracer(ctrl)
	cache := mcache.NewMemoryCache()

	app := newApplicationHandler(eventRepo, eventDeliveryRepo, appRepo, groupRepo, apiKeyRepo, eventQueue, logger, tracer, cache)

	keyID := "12345"
	apiKey := &datastore.APIKey{UID: keyID}

	page := &datastore.Pageable{
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
				a, _ := app.apiKeyRepo.(*mocks.MockAPIKeyRepository)
				a.EXPECT().
					LoadAPIKeysPaged(gomock.Any(), gomock.Any()).
					Times(1).
					Return(
						[]datastore.APIKey{*apiKey},
						datastore.PaginationData{PerPage: int64(page.PerPage)}, nil)
			},
		},
		{
			name:       "should_fail_to_load_api_keys",
			cfgPath:    "./testdata/Auth_Config/no-auth-convoy.json",
			statusCode: http.StatusInternalServerError,
			dbFn: func(app *applicationHandler) {
				a, _ := app.apiKeyRepo.(*mocks.MockAPIKeyRepository)
				a.EXPECT().
					LoadAPIKeysPaged(gomock.Any(), gomock.Any()).
					Times(1).Return(nil, datastore.PaginationData{}, errors.New("abc"))
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

			err := config.LoadConfig(tc.cfgPath, new(config.Configuration))
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
