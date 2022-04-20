package server

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/mocks"

	"github.com/go-chi/chi/v5"
	"github.com/golang/mock/gomock"
)

func TestApplicationHandler_CreateAPIKey(t *testing.T) {
	var app *applicationHandler

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	app = provideApplication(ctrl)

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

func TestApplicationHandler_CreateAppPortalAPIKey(t *testing.T) {
	var app *applicationHandler

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	app = provideApplication(ctrl)

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

	appID := "123456"
	application := &datastore.Application{
		UID:     appID,
		GroupID: groupId,
	}

	tt := []struct {
		name           string
		cfgPath        string
		statusCode     int
		stripTimestamp bool
		appID          string
		dbFn           func(app *applicationHandler)
	}{
		{
			name:           "create api key",
			stripTimestamp: true,
			cfgPath:        "./testdata/Auth_Config/no-auth-convoy.json",
			statusCode:     http.StatusCreated,
			appID:          appID,
			dbFn: func(app *applicationHandler) {
				a, _ := app.apiKeyRepo.(*mocks.MockAPIKeyRepository)
				g, _ := app.groupRepo.(*mocks.MockGroupRepository)
				ap, _ := app.appRepo.(*mocks.MockApplicationRepository)
				c, _ := app.cache.(*mocks.MockCache)
				c.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any()).Times(2)
				g.EXPECT().
					FetchGroupByID(gomock.Any(), gomock.Any()).
					Times(1).Return(group, nil)
				c.EXPECT().Set(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Times(2)
				a.EXPECT().CreateAPIKey(gomock.Any(), gomock.Any()).Times(1).Return(nil)
				ap.EXPECT().FindApplicationByID(gomock.Any(), gomock.Any()).Times(1).Return(application, nil)
			},
		},

		{
			name:           "app id does not belong to group",
			stripTimestamp: false,
			cfgPath:        "./testdata/Auth_Config/no-auth-convoy.json",
			statusCode:     http.StatusBadRequest,
			appID:          "123",
			dbFn: func(app *applicationHandler) {
				g, _ := app.groupRepo.(*mocks.MockGroupRepository)
				ap, _ := app.appRepo.(*mocks.MockApplicationRepository)
				c, _ := app.cache.(*mocks.MockCache)
				c.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any()).Times(2)
				g.EXPECT().
					FetchGroupByID(gomock.Any(), gomock.Any()).
					Times(1).Return(group, nil)
				c.EXPECT().Set(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Times(2)
				ap.EXPECT().FindApplicationByID(gomock.Any(), gomock.Any()).Times(1).Return(&datastore.Application{UID: "123", GroupID: "123"}, nil)
			},
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			// Arrange
			url := fmt.Sprintf("/api/v1/security/applications/%s/keys?groupId=%s", appID, groupId)
			req := httptest.NewRequest(http.MethodPost, url, nil)
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
	app := provideApplication(ctrl)

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
			statusCode: http.StatusBadRequest,
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
	app := provideApplication(ctrl)

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
			statusCode:     http.StatusBadRequest,
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
	app := provideApplication(ctrl)

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
			statusCode: http.StatusBadRequest,
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

func TestApplicationHandler_UpdateAPIKey(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	app := provideApplication(ctrl)

	groupID := "1234567890"

	group := &datastore.Group{
		UID: groupID,
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

	keyID := "12345"
	apiKey := &datastore.APIKey{UID: keyID}

	tt := []struct {
		name       string
		cfgPath    string
		statusCode int
		keyID      string
		body       *strings.Reader
		dbFn       func(app *applicationHandler)
	}{
		{
			name:       "update api key",
			cfgPath:    "./testdata/Auth_Config/no-auth-convoy.json",
			statusCode: http.StatusOK,
			keyID:      keyID,
			body: strings.NewReader(`{
				"role": {
					"type": "admin",
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
					Times(1).Return([]datastore.Group{*group}, nil)
				a.EXPECT().FindAPIKeyByID(gomock.Any(), gomock.Any()).Times(1).Return(apiKey, nil)
				a.EXPECT().UpdateAPIKey(gomock.Any(), gomock.Any()).Times(1).Return(nil)
			},
		},
		{
			name:       "invalid role",
			cfgPath:    "./testdata/Auth_Config/no-auth-convoy.json",
			statusCode: http.StatusBadRequest,
			keyID:      keyID,
			body: strings.NewReader(`{
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
			url := fmt.Sprintf("/api/v1/security/keys/%s", tc.keyID)
			req := httptest.NewRequest(http.MethodPut, url, tc.body)
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
