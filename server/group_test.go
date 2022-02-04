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

func TestApplicationHandler_GetGroup(t *testing.T) {

	realOrgID := "1234567890"
	fakeOrgID := "12345"

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	tt := []struct {
		name       string
		cfgPath    string
		method     string
		statusCode int
		id         string
		dbFn       func(app *applicationHandler)
	}{
		{
			name:       "group not found",
			cfgPath:    "./testdata/Auth_Config/no-auth-convoy.json",
			method:     http.MethodGet,
			statusCode: http.StatusInternalServerError,
			id:         fakeOrgID,
			dbFn: func(app *applicationHandler) {
				o, _ := app.groupRepo.(*mocks.MockGroupRepository)

				o.EXPECT().
					FetchGroupByID(gomock.Any(), gomock.Any()).Times(1).
					Return(nil, datastore.ErrGroupNotFound)
			},
		},
		{
			name:       "valid group",
			cfgPath:    "./testdata/Auth_Config/no-auth-convoy.json",
			method:     http.MethodGet,
			statusCode: http.StatusOK,
			id:         realOrgID,
			dbFn: func(app *applicationHandler) {
				o, _ := app.groupRepo.(*mocks.MockGroupRepository)

				o.EXPECT().
					FetchGroupByID(gomock.Any(), gomock.Any()).Times(1).
					Return(&datastore.Group{
						UID:  realOrgID,
						Name: "sendcash-pay",
					}, nil)

				a, _ := app.appRepo.(*mocks.MockApplicationRepository)
				a.EXPECT().
					CountGroupApplications(gomock.Any(), gomock.AssignableToTypeOf("")).Times(1).
					Return(int64(1), nil)

				e, _ := app.eventRepo.(*mocks.MockEventRepository)
				e.EXPECT().
					CountGroupMessages(gomock.Any(), gomock.AssignableToTypeOf("")).Times(1).
					Return(int64(1), nil)
			},
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			var app *applicationHandler

			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			groupRepo := mocks.NewMockGroupRepository(ctrl)
			appRepo := mocks.NewMockApplicationRepository(ctrl)
			eventRepo := mocks.NewMockEventRepository(ctrl)
			eventDeliveryRepo := mocks.NewMockEventDeliveryRepository(ctrl)
			eventQueue := mocks.NewMockQueuer(ctrl)
			logger := logger.NewNoopLogger()
			tracer := mocks.NewMockTracer(ctrl)
			apiKeyRepo := mocks.NewMockAPIKeyRepository(ctrl)
			cache := mcache.NewMemoryCache()

			app = newApplicationHandler(eventRepo, eventDeliveryRepo, appRepo, groupRepo, apiKeyRepo, eventQueue, logger, tracer, cache)

			// Arrange
			url := fmt.Sprintf("/api/v1/groups/%s", tc.id)
			req := httptest.NewRequest(tc.method, url, nil)
			req.SetBasicAuth("test", "test")
			req.Header.Add("Content-Type", "application/json")
			w := httptest.NewRecorder()

			rctx := chi.NewRouteContext()
			rctx.URLParams.Add("groupID", tc.id)

			req = req.Clone(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

			// Arrange Expectations
			if tc.dbFn != nil {
				tc.dbFn(app)
			}

			err := config.LoadConfig(tc.cfgPath, provideFakeOverride())
			if err != nil {
				t.Errorf("Failed to load config file: %v", err)
			}
			initRealmChain(t, app.apiKeyRepo)

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

func TestApplicationHandler_CreateGroup(t *testing.T) {

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	bodyReader := strings.NewReader(`{"name": "ABC_DEF_TEST_UPDATE", "config": {"strategy": {"type": "default", "default": {"intervalSeconds": 10, "retryLimit": 3 }}, "signature": { "header": "X-Company-Signature", "hash": "SHA1" }}}`)

	tt := []struct {
		name       string
		cfgPath    string
		method     string
		statusCode int
		body       *strings.Reader
		dbFn       func(*applicationHandler)
	}{
		{
			name:       "valid group",
			cfgPath:    "./testdata/Auth_Config/basic-convoy.json",
			method:     http.MethodPost,
			statusCode: http.StatusCreated,
			body:       bodyReader,
			dbFn: func(app *applicationHandler) {
				o, _ := app.groupRepo.(*mocks.MockGroupRepository)
				o.EXPECT().
					CreateGroup(gomock.Any(), gomock.Any()).Times(1).
					Return(nil)

			},
		},

		{
			name:       "invalid request - no group name",
			cfgPath:    "./testdata/Auth_Config/basic-convoy.json",
			method:     http.MethodPost,
			statusCode: http.StatusBadRequest,
			body:       strings.NewReader(`{"config": {"strategy": {"type": "default", "default": {"intervalSeconds": 10, "retryLimit": 3 }}, "signature": { "header": "X-Company-Signature", "hash": "SHA1" }}}`),
		},

		{
			name:       "invalid request - no group strategy type field",
			cfgPath:    "./testdata/Auth_Config/basic-convoy.json",
			method:     http.MethodPost,
			statusCode: http.StatusBadRequest,
			body:       strings.NewReader(`{"name": "ABC_DEF_TEST_UPDATE", "config": {"strategy": {"default": {"intervalSeconds": 10, "retryLimit": 3 }}, "signature": { "header": "X-Company-Signature", "hash": "SHA1" }}}`),
		},

		{
			name:       "invalid request - unsupported group strategy type",
			cfgPath:    "./testdata/Auth_Config/basic-convoy.json",
			method:     http.MethodPost,
			statusCode: http.StatusBadRequest,
			body:       strings.NewReader(`{"name": "ABC_DEF_TEST_UPDATE", "config": {"strategy": {"type": "unsupported", "default": {"intervalSeconds": 10, "retryLimit": 3 }}, "signature": { "header": "X-Company-Signature", "hash": "SHA1" }}}`),
		},

		{
			name:       "invalid request - no group interval seconds field",
			cfgPath:    "./testdata/Auth_Config/basic-convoy.json",
			method:     http.MethodPost,
			statusCode: http.StatusBadRequest,
			body:       strings.NewReader(`{"name": "ABC_DEF_TEST_UPDATE", "config": {"strategy": {"type": "default", "default": {"retryLimit": 3 }}, "signature": { "header": "X-Company-Signature", "hash": "SHA1" }}}`),
		},

		{
			name:       "invalid request - no group retry limit field",
			cfgPath:    "./testdata/Auth_Config/basic-convoy.json",
			method:     http.MethodPost,
			statusCode: http.StatusBadRequest,
			body:       strings.NewReader(`{"name": "ABC_DEF_TEST_UPDATE", "config": {"strategy": {"type": "default", "default": {"intervalSeconds": 10 }}, "signature": { "header": "X-Company-Signature", "hash": "SHA1" }}}`),
		},

		{
			name:       "invalid request - no group header field",
			cfgPath:    "./testdata/Auth_Config/basic-convoy.json",
			method:     http.MethodPost,
			statusCode: http.StatusBadRequest,
			body:       strings.NewReader(`{"name": "ABC_DEF_TEST_UPDATE", "config": {"strategy": {"type": "default", "default": {"intervalSeconds": 10, "retryLimit": 3 }}, "signature": {"hash": "SHA1" }}}`),
		},

		{
			name:       "invalid request - no group hash field",
			cfgPath:    "./testdata/Auth_Config/basic-convoy.json",
			method:     http.MethodPost,
			statusCode: http.StatusBadRequest,
			body:       strings.NewReader(`{"name": "ABC_DEF_TEST_UPDATE", "config": {"strategy": {"type": "default", "default": {"intervalSeconds": 10, "retryLimit": 3 }}, "signature": { "header": "X-Company-Signature" }}}`),
		},

		{
			name:       "invalid request - unsupported group hash field",
			cfgPath:    "./testdata/Auth_Config/basic-convoy.json",
			method:     http.MethodPost,
			statusCode: http.StatusBadRequest,
			body:       strings.NewReader(`{"name": "ABC_DEF_TEST_UPDATE", "config": {"strategy": {"type": "default", "default": {"intervalSeconds": 10, "retryLimit": 3 }}, "signature": { "header": "X-Company-Signature", "hash": "unsupported" }}}`),
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			var app *applicationHandler

			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			groupRepo := mocks.NewMockGroupRepository(ctrl)
			appRepo := mocks.NewMockApplicationRepository(ctrl)
			eventRepo := mocks.NewMockEventRepository(ctrl)
			eventDeliveryRepo := mocks.NewMockEventDeliveryRepository(ctrl)
			eventQueue := mocks.NewMockQueuer(ctrl)
			logger := logger.NewNoopLogger()
			tracer := mocks.NewMockTracer(ctrl)
			apiKeyRepo := mocks.NewMockAPIKeyRepository(ctrl)
			cache := mcache.NewMemoryCache()

			app = newApplicationHandler(eventRepo, eventDeliveryRepo, appRepo, groupRepo, apiKeyRepo, eventQueue, logger, tracer, cache)

			// Arrange
			req := httptest.NewRequest(tc.method, "/api/v1/groups", tc.body)
			req.SetBasicAuth("test", "test")
			req.Header.Add("Content-Type", "application/json")
			w := httptest.NewRecorder()

			// Arrange Expectations
			if tc.dbFn != nil {
				tc.dbFn(app)
			}

			err := config.LoadConfig(tc.cfgPath, provideFakeOverride())
			if err != nil {
				t.Errorf("Failed to load config file: %v", err)
			}
			initRealmChain(t, app.apiKeyRepo)

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

func TestApplicationHandler_UpdateGroup(t *testing.T) {

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	realOrgID := "1234567890"

	bodyReader := strings.NewReader(`{"name": "ABC_DEF_TEST_UPDATE", "config": {"strategy": {"type": "default", "default": {"intervalSeconds": 10, "retryLimit": 3 }}, "signature": { "header": "X-Company-Signature", "hash": "SHA1" }}}`)

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
			name:       "valid group update",
			cfgPath:    "./testdata/Auth_Config/basic-convoy.json",
			method:     http.MethodPut,
			statusCode: http.StatusAccepted,
			orgID:      realOrgID,
			body:       bodyReader,
			dbFn: func(app *applicationHandler) {
				g, _ := app.groupRepo.(*mocks.MockGroupRepository)
				g.EXPECT().
					UpdateGroup(gomock.Any(), gomock.Any()).Times(1).
					Return(nil)

				g.EXPECT().
					FetchGroupByID(gomock.Any(), gomock.Any()).Times(1).
					Return(&datastore.Group{
						UID:  realOrgID,
						Name: "sendcash-pay",
					}, nil)
			},
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			var app *applicationHandler

			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			groupRepo := mocks.NewMockGroupRepository(ctrl)
			appRepo := mocks.NewMockApplicationRepository(ctrl)
			eventRepo := mocks.NewMockEventRepository(ctrl)
			eventDeliveryRepo := mocks.NewMockEventDeliveryRepository(ctrl)
			eventQueue := mocks.NewMockQueuer(ctrl)
			logger := logger.NewNoopLogger()
			tracer := mocks.NewMockTracer(ctrl)
			apiKeyRepo := mocks.NewMockAPIKeyRepository(ctrl)
			cache := mcache.NewMemoryCache()

			app = newApplicationHandler(eventRepo, eventDeliveryRepo, appRepo, groupRepo, apiKeyRepo, eventQueue, logger, tracer, cache)

			// Arrange
			url := fmt.Sprintf("/api/v1/groups/%s", tc.orgID)
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

			err := config.LoadConfig(tc.cfgPath, provideFakeOverride())
			if err != nil {
				t.Errorf("Failed to load config file: %v", err)
			}
			initRealmChain(t, app.apiKeyRepo)

			router := buildRoutes(app)

			// Act.
			router.ServeHTTP(w, req)

			if w.Code != tc.statusCode {
				t.Errorf("Want status '%d', got '%d'", tc.statusCode, w.Code)
			}
			fmt.Println("s", w.Body.String())
			verifyMatch(t, *w)
		})
	}

}

func TestApplicationHandler_GetGroups(t *testing.T) {

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	realOrgID := "1234567890"

	tt := []struct {
		name       string
		cfgPath    string
		method     string
		statusCode int
		dbFn       func(app *applicationHandler)
	}{
		{
			name:       "valid groups",
			cfgPath:    "./testdata/Auth_Config/basic-convoy.json",
			method:     http.MethodGet,
			statusCode: http.StatusOK,
			dbFn: func(app *applicationHandler) {
				o, _ := app.groupRepo.(*mocks.MockGroupRepository)
				o.EXPECT().
					LoadGroups(gomock.Any(), gomock.Any()).Times(1).
					Return([]*datastore.Group{
						{
							UID:  realOrgID,
							Name: "sendcash-pay",
						},
					}, nil)

				a, _ := app.appRepo.(*mocks.MockApplicationRepository)
				a.EXPECT().
					CountGroupApplications(gomock.Any(), gomock.AssignableToTypeOf("")).Times(1).
					Return(int64(1), nil)

				e, _ := app.eventRepo.(*mocks.MockEventRepository)
				e.EXPECT().
					CountGroupMessages(gomock.Any(), gomock.AssignableToTypeOf("")).Times(1).
					Return(int64(1), nil)
			},
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			var app *applicationHandler

			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			groupRepo := mocks.NewMockGroupRepository(ctrl)
			appRepo := mocks.NewMockApplicationRepository(ctrl)
			eventRepo := mocks.NewMockEventRepository(ctrl)
			eventDeliveryRepo := mocks.NewMockEventDeliveryRepository(ctrl)
			eventQueue := mocks.NewMockQueuer(ctrl)
			logger := logger.NewNoopLogger()
			tracer := mocks.NewMockTracer(ctrl)
			apiKeyRepo := mocks.NewMockAPIKeyRepository(ctrl)
			cache := mcache.NewMemoryCache()

			app = newApplicationHandler(eventRepo, eventDeliveryRepo, appRepo, groupRepo, apiKeyRepo, eventQueue, logger, tracer, cache)

			req := httptest.NewRequest(tc.method, "/api/v1/groups", nil)
			req.SetBasicAuth("test", "test")
			w := httptest.NewRecorder()

			// Arrange Expectations
			if tc.dbFn != nil {
				tc.dbFn(app)
			}

			err := config.LoadConfig(tc.cfgPath, provideFakeOverride())
			if err != nil {
				t.Errorf("Failed to load config file: %v", err)
			}
			initRealmChain(t, app.apiKeyRepo)

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

func TestApplicationHandler_DeleteGroup(t *testing.T) {

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

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
			name:       "valid group delete",
			cfgPath:    "./testdata/Auth_Config/no-auth-convoy.json",
			method:     http.MethodDelete,
			statusCode: http.StatusOK,
			orgID:      realOrgID,
			body:       bodyReader,
			dbFn: func(app *applicationHandler) {
				g, _ := app.groupRepo.(*mocks.MockGroupRepository)
				g.EXPECT().
					FetchGroupByID(gomock.Any(), gomock.Any()).Times(1).
					Return(&datastore.Group{
						UID:  realOrgID,
						Name: "sendcash-pay",
					}, nil)

				g.EXPECT().
					DeleteGroup(gomock.Any(), gomock.Any()).Times(1).
					Return(nil)

				a, _ := app.appRepo.(*mocks.MockApplicationRepository)
				a.EXPECT().DeleteGroupApps(gomock.Any(), gomock.AssignableToTypeOf("")).Times(1).
					Return(nil)

				e, _ := app.eventRepo.(*mocks.MockEventRepository)
				e.EXPECT().DeleteGroupEvents(gomock.Any(), gomock.AssignableToTypeOf("")).Times(1).
					Return(nil)

			},
		},
		{
			name:       "failed group delete",
			cfgPath:    "./testdata/Auth_Config/no-auth-convoy.json",
			method:     http.MethodDelete,
			statusCode: http.StatusInternalServerError,
			orgID:      realOrgID,
			body:       bodyReader,
			dbFn: func(app *applicationHandler) {
				g, _ := app.groupRepo.(*mocks.MockGroupRepository)
				g.EXPECT().
					FetchGroupByID(gomock.Any(), gomock.Any()).Times(1).
					Return(&datastore.Group{
						UID:  realOrgID,
						Name: "sendcash-pay",
					}, nil)

				g.EXPECT().
					DeleteGroup(gomock.Any(), gomock.Any()).Times(1).
					Return(errors.New("abc"))
			},
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			var app *applicationHandler

			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			groupRepo := mocks.NewMockGroupRepository(ctrl)
			apiKeyRepo := mocks.NewMockAPIKeyRepository(ctrl)
			appRepo := mocks.NewMockApplicationRepository(ctrl)
			eventRepo := mocks.NewMockEventRepository(ctrl)
			eventDeliveryRepo := mocks.NewMockEventDeliveryRepository(ctrl)
			eventQueue := mocks.NewMockQueuer(ctrl)
			logger := logger.NewNoopLogger()
			tracer := mocks.NewMockTracer(ctrl)
			cache := mcache.NewMemoryCache()

			app = newApplicationHandler(eventRepo, eventDeliveryRepo, appRepo, groupRepo, apiKeyRepo, eventQueue, logger, tracer, cache)

			// Arrange
			url := fmt.Sprintf("/api/v1/groups/%s", tc.orgID)
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

			err := config.LoadConfig(tc.cfgPath, new(config.Configuration))
			if err != nil {
				t.Errorf("Failed to load config file: %v", err)
			}
			initRealmChain(t, app.apiKeyRepo)

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
