package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/frain-dev/convoy/auth/realm_chain"
	mcache "github.com/frain-dev/convoy/cache/memory"
	"github.com/frain-dev/convoy/logger"

	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/datastore"

	"github.com/frain-dev/convoy/mocks"
	"github.com/go-chi/chi/v5"
	"github.com/golang/mock/gomock"
	"github.com/sebdah/goldie/v2"
)

func verifyMatch(t *testing.T, w httptest.ResponseRecorder) {
	g := goldie.New(
		t,
		goldie.WithDiffEngine(goldie.ColoredDiff),
	)
	g.Assert(t, t.Name(), w.Body.Bytes())
}

func initRealmChain(t *testing.T, apiKeyRepo datastore.APIKeyRepository) {
	cfg, err := config.Get()
	if err != nil {
		t.Errorf("failed to get config: %v", err)
	}

	err = realm_chain.Init(&cfg.Auth, apiKeyRepo)
	if err != nil {
		t.Errorf("failed to initialize realm chain : %v", err)
	}
}

func stripTimestamp(t *testing.T, obj string, b *bytes.Buffer) *bytes.Buffer {
	var res serverResponse

	err := json.NewDecoder(b).Decode(&res)
	if err != nil {
		t.Errorf("could not stripTimestamp: %s", err)
		t.FailNow()
	}

	if res.Data == nil {
		return bytes.NewBuffer([]byte(``))
	}

	switch obj {
	case "application":
		var a datastore.Application
		err := json.Unmarshal(res.Data, &a)
		if err != nil {
			t.Errorf("could not stripTimestamp: %s", err)
			t.FailNow()
		}

		a.UID = ""
		a.CreatedAt, a.UpdatedAt, a.DeletedAt = 0, 0, 0

		jsonData, err := json.Marshal(a)
		if err != nil {
			t.Error(err)
		}

		return bytes.NewBuffer(jsonData)
	case "endpoint":
		var e datastore.Endpoint
		err := json.Unmarshal(res.Data, &e)
		if err != nil {
			t.Errorf("could not stripTimestamp: %s", err)
			t.FailNow()
		}

		e.UID = ""
		e.CreatedAt, e.UpdatedAt, e.DeletedAt = 0, 0, 0

		jsonData, err := json.Marshal(e)
		if err != nil {
			t.Error(err)
		}

		return bytes.NewBuffer(jsonData)
	case "apiKey":
		var e datastore.APIKey
		err := json.Unmarshal(res.Data, &e)
		if err != nil {
			t.Errorf("could not stripTimestamp: %s", err)
			t.FailNow()
		}

		e.UID = ""
		e.CreatedAt = 0
		e.ExpiresAt = 0

		jsonData, err := json.Marshal(e)
		if err != nil {
			t.Error(err)
		}

		return bytes.NewBuffer(jsonData)
	default:
		t.Errorf("invalid data body - %v of type %T", obj, obj)
		t.FailNow()
	}

	return nil
}

func provideFakeOverride() *config.Configuration {
	return new(config.Configuration)
}

func TestApplicationHandler_GetApp(t *testing.T) {

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

	groupID := "1234567890"
	group := &datastore.Group{UID: groupID}

	validID := "123456789"

	app = newApplicationHandler(eventRepo, eventDeliveryRepo, appRepo, groupRepo, apiKeyRepo, eventQueue, logger, tracer, cache)

	tt := []struct {
		name       string
		cfgPath    string
		method     string
		statusCode int
		id         string
		dbFn       func(app *applicationHandler)
	}{
		{
			name:       "app not found",
			cfgPath:    "./testdata/Auth_Config/no-auth-convoy.json",
			method:     http.MethodGet,
			statusCode: http.StatusNotFound,
			id:         "12345",
			dbFn: func(app *applicationHandler) {
				a, _ := app.appRepo.(*mocks.MockApplicationRepository)
				a.EXPECT().
					FindApplicationByID(gomock.Any(), gomock.Any()).Times(1).
					Return(nil, datastore.ErrApplicationNotFound)

				o, _ := app.groupRepo.(*mocks.MockGroupRepository)

				o.EXPECT().
					LoadGroups(gomock.Any(), gomock.Any()).Times(1).
					Return([]*datastore.Group{group}, nil)
			},
		},
		{
			name:       "valid application",
			cfgPath:    "./testdata/Auth_Config/no-auth-convoy.json",
			method:     http.MethodGet,
			statusCode: http.StatusOK,
			id:         validID,
			dbFn: func(app *applicationHandler) {
				a, _ := app.appRepo.(*mocks.MockApplicationRepository)
				a.EXPECT().
					FindApplicationByID(gomock.Any(), gomock.Any()).Times(1).
					Return(&datastore.Application{
						UID:       validID,
						GroupID:   groupID,
						Title:     "Valid application",
						Endpoints: []datastore.Endpoint{},
					}, nil)

				o, _ := app.groupRepo.(*mocks.MockGroupRepository)
				o.EXPECT().
					LoadGroups(gomock.Any(), gomock.Any()).Times(1).
					Return([]*datastore.Group{group}, nil)
			},
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			// Arrange
			url := fmt.Sprintf("/api/v1/applications/%s", tc.id)
			req := httptest.NewRequest(tc.method, url, nil)
			req.SetBasicAuth("test", "test")
			w := httptest.NewRecorder()
			rctx := chi.NewRouteContext()
			rctx.URLParams.Add("id", tc.id)

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

func TestApplicationHandler_GetApps(t *testing.T) {

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

	groupID := "1234567890"
	group := &datastore.Group{UID: groupID}

	validID := "123456789"

	app = newApplicationHandler(eventRepo, eventDeliveryRepo, appRepo, groupRepo, apiKeyRepo, eventQueue, logger, tracer, cache)

	tt := []struct {
		name       string
		cfgPath    string
		method     string
		statusCode int
		dbFn       func(app *applicationHandler)
	}{
		{
			name:       "valid applications",
			cfgPath:    "./testdata/Auth_Config/no-auth-convoy.json",
			method:     http.MethodGet,
			statusCode: http.StatusOK,
			dbFn: func(app *applicationHandler) {
				a, _ := app.appRepo.(*mocks.MockApplicationRepository)
				a.EXPECT().
					LoadApplicationsPaged(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Times(1).
					Return([]datastore.Application{
						{
							UID:       validID,
							GroupID:   groupID,
							Title:     "Valid application - 0",
							Endpoints: []datastore.Endpoint{},
						},
					}, datastore.PaginationData{}, nil)

				o, _ := app.groupRepo.(*mocks.MockGroupRepository)
				o.EXPECT().
					LoadGroups(gomock.Any(), gomock.Any()).Times(1).
					Return([]*datastore.Group{group}, nil)
			},
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			// Arrange
			req := httptest.NewRequest(tc.method, "/api/v1/applications", nil)
			req.SetBasicAuth("test", "test")
			w := httptest.NewRecorder()

			pageable := datastore.Pageable{
				Page:    1,
				PerPage: 10,
			}
			req = req.WithContext(context.WithValue(req.Context(), pageableCtx, pageable))

			// Arrange Expectations.
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

			if w.Code != tc.statusCode {
				t.Errorf("Want status '%d', got '%d'", tc.statusCode, w.Code)
			}

			verifyMatch(t, *w)
		})
	}

}

func TestApplicationHandler_CreateApp(t *testing.T) {

	groupID := "1234567890"
	group := &datastore.Group{
		UID: groupID,
	}

	bodyReader := strings.NewReader(`{ "group_id": "` + groupID + `", "name": "ABC_DEF_TEST", "secret": "12345" }`)

	tt := []struct {
		name       string
		cfgPath    string
		method     string
		statusCode int
		body       *strings.Reader
		dbFn       func(app *applicationHandler)
	}{
		{
			name:       "invalid request",
			cfgPath:    "./testdata/Auth_Config/no-auth-convoy.json",
			method:     http.MethodPost,
			statusCode: http.StatusBadRequest,
			body:       strings.NewReader(``),
			dbFn: func(app *applicationHandler) {
				o, _ := app.groupRepo.(*mocks.MockGroupRepository)
				o.EXPECT().
					LoadGroups(gomock.Any(), gomock.Any()).Times(1).
					Return([]*datastore.Group{group}, nil)
			},
		},
		{
			name:       "invalid request - no app name",
			cfgPath:    "./testdata/Auth_Config/no-auth-convoy.json",
			method:     http.MethodPost,
			statusCode: http.StatusBadRequest,
			body:       strings.NewReader(`{}`),
			dbFn: func(app *applicationHandler) {
				o, _ := app.groupRepo.(*mocks.MockGroupRepository)
				o.EXPECT().
					LoadGroups(gomock.Any(), gomock.Any()).Times(1).
					Return([]*datastore.Group{group}, nil)
			},
		},
		{
			name:       "valid application",
			cfgPath:    "./testdata/Auth_Config/no-auth-convoy.json",
			method:     http.MethodPost,
			statusCode: http.StatusCreated,
			body:       bodyReader,
			dbFn: func(app *applicationHandler) {
				a, _ := app.appRepo.(*mocks.MockApplicationRepository)
				a.EXPECT().
					CreateApplication(gomock.Any(), gomock.Any()).Times(1).
					Return(nil)

				o, _ := app.groupRepo.(*mocks.MockGroupRepository)

				o.EXPECT().
					LoadGroups(gomock.Any(), gomock.Any()).Times(1).
					Return([]*datastore.Group{group}, nil)
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
			req := httptest.NewRequest(tc.method, "/api/v1/applications", tc.body)
			req.SetBasicAuth("test", "test")
			req.Header.Add("Content-Type", "application/json")
			w := httptest.NewRecorder()

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

			fmt.Printf("bodyyy: '%s'\n", w.Body.String())
			d := stripTimestamp(t, "application", w.Body)

			w.Body = d
			verifyMatch(t, *w)
		})
	}

}

func TestApplicationHandler_UpdateApp(t *testing.T) {

	groupID := "1234567890"
	group := &datastore.Group{UID: groupID}

	appId := "12345"
	bodyReader := strings.NewReader(`{"name": "ABC_DEF_TEST_UPDATE"}`)

	tt := []struct {
		name       string
		cfgPath    string
		method     string
		statusCode int
		appId      string
		body       *strings.Reader
		dbFn       func(app *applicationHandler)
	}{
		{
			name:       "invalid request",
			cfgPath:    "./testdata/Auth_Config/no-auth-convoy.json",
			method:     http.MethodPut,
			statusCode: http.StatusBadRequest,
			appId:      appId,
			body:       strings.NewReader(``),
			dbFn: func(app *applicationHandler) {
				a, _ := app.appRepo.(*mocks.MockApplicationRepository)
				a.EXPECT().
					FindApplicationByID(gomock.Any(), gomock.Any()).Times(1).
					Return(&datastore.Application{
						UID:       appId,
						GroupID:   groupID,
						Title:     "Valid application update",
						Endpoints: []datastore.Endpoint{},
					}, nil)

				o, _ := app.groupRepo.(*mocks.MockGroupRepository)

				o.EXPECT().
					LoadGroups(gomock.Any(), gomock.Any()).Times(1).
					Return([]*datastore.Group{group}, nil)
			},
		},
		{
			name:       "invalid request - no app name",
			cfgPath:    "./testdata/Auth_Config/no-auth-convoy.json",
			method:     http.MethodPut,
			statusCode: http.StatusBadRequest,
			appId:      appId,
			body:       strings.NewReader(`{}`),
			dbFn: func(app *applicationHandler) {
				a, _ := app.appRepo.(*mocks.MockApplicationRepository)
				a.EXPECT().
					FindApplicationByID(gomock.Any(), gomock.Any()).Times(1).
					Return(&datastore.Application{
						UID:       appId,
						GroupID:   groupID,
						Title:     "Valid application update",
						Endpoints: []datastore.Endpoint{},
					}, nil)

				o, _ := app.groupRepo.(*mocks.MockGroupRepository)

				o.EXPECT().
					LoadGroups(gomock.Any(), gomock.Any()).Times(1).
					Return([]*datastore.Group{group}, nil)
			},
		},
		{
			name:       "valid request - update secret",
			cfgPath:    "./testdata/Auth_Config/no-auth-convoy.json",
			method:     http.MethodPut,
			statusCode: http.StatusAccepted,
			appId:      appId,
			body:       strings.NewReader(`{ "name": "ABC", "secret": "xyz" }`),
			dbFn: func(app *applicationHandler) {
				a, _ := app.appRepo.(*mocks.MockApplicationRepository)
				a.EXPECT().
					FindApplicationByID(gomock.Any(), gomock.Any()).Times(1).
					Return(&datastore.Application{
						UID:       appId,
						GroupID:   groupID,
						Title:     "Valid application update",
						Endpoints: []datastore.Endpoint{},
					}, nil)

				a.EXPECT().
					UpdateApplication(gomock.Any(), gomock.Any()).Times(1).
					Return(nil)

				o, _ := app.groupRepo.(*mocks.MockGroupRepository)

				o.EXPECT().
					LoadGroups(gomock.Any(), gomock.Any()).Times(1).
					Return([]*datastore.Group{group}, nil)
			},
		},
		{
			name:       "valid request - update support email",
			cfgPath:    "./testdata/Auth_Config/no-auth-convoy.json",
			method:     http.MethodPut,
			statusCode: http.StatusAccepted,
			appId:      appId,
			body:       strings.NewReader(`{ "name": "ABC", "support_email": "engineering@frain.dev" }`),
			dbFn: func(app *applicationHandler) {
				a, _ := app.appRepo.(*mocks.MockApplicationRepository)
				a.EXPECT().
					UpdateApplication(gomock.Any(), gomock.Any()).Times(1).
					Return(nil)

				a.EXPECT().
					FindApplicationByID(gomock.Any(), gomock.Any()).Times(1).
					Return(&datastore.Application{
						UID:       appId,
						GroupID:   groupID,
						Title:     "Valid application update",
						Endpoints: []datastore.Endpoint{},
					}, nil)

				o, _ := app.groupRepo.(*mocks.MockGroupRepository)

				o.EXPECT().
					LoadGroups(gomock.Any(), gomock.Any()).Times(1).
					Return([]*datastore.Group{group}, nil)
			},
		},
		{
			name:       "valid application update",
			cfgPath:    "./testdata/Auth_Config/no-auth-convoy.json",
			method:     http.MethodPut,
			statusCode: http.StatusAccepted,
			appId:      appId,
			body:       bodyReader,
			dbFn: func(app *applicationHandler) {
				a, _ := app.appRepo.(*mocks.MockApplicationRepository)
				a.EXPECT().
					UpdateApplication(gomock.Any(), gomock.Any()).Times(1).
					Return(nil)

				a.EXPECT().
					FindApplicationByID(gomock.Any(), gomock.Any()).Times(1).
					Return(&datastore.Application{
						UID:       appId,
						GroupID:   groupID,
						Title:     "Valid application update",
						Endpoints: []datastore.Endpoint{},
					}, nil)

				o, _ := app.groupRepo.(*mocks.MockGroupRepository)

				o.EXPECT().
					LoadGroups(gomock.Any(), gomock.Any()).Times(1).
					Return([]*datastore.Group{group}, nil)
			},
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			// Arrange
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

			url := fmt.Sprintf("/api/v1/applications/%s", tc.appId)
			req := httptest.NewRequest(tc.method, url, tc.body)
			req.SetBasicAuth("test", "test")
			req.Header.Add("Content-Type", "application/json")

			w := httptest.NewRecorder()
			rctx := chi.NewRouteContext()
			rctx.URLParams.Add("appID", tc.appId)

			req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
			req = req.WithContext(context.WithValue(req.Context(), appCtx, &datastore.Application{
				UID:       appId,
				GroupID:   groupID,
				Title:     "Valid application update",
				Endpoints: []datastore.Endpoint{},
			}))

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

func TestApplicationHandler_CreateAppEndpoint(t *testing.T) {

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

	groupID := "1234567890"
	group := &datastore.Group{UID: groupID}

	bodyReader := strings.NewReader(`{"url": "https://google.com", "description": "Test"}`)

	app = newApplicationHandler(eventRepo, eventDeliveryRepo, appRepo, groupRepo, apiKeyRepo, eventQueue, logger, tracer, cache)

	appId := "123456789"

	tt := []struct {
		name       string
		cfgPath    string
		method     string
		statusCode int
		appId      string
		body       *strings.Reader
		dbFn       func(app *applicationHandler)
	}{
		{
			name:       "valid endpoint",
			cfgPath:    "./testdata/Auth_Config/no-auth-convoy.json",
			method:     http.MethodPost,
			statusCode: http.StatusCreated,
			appId:      appId,
			body:       bodyReader,
			dbFn: func(app *applicationHandler) {
				a, _ := app.appRepo.(*mocks.MockApplicationRepository)
				a.EXPECT().
					UpdateApplication(gomock.Any(), gomock.Any()).Times(1).
					Return(nil)

				a.EXPECT().
					FindApplicationByID(gomock.Any(), gomock.Any()).Times(2).
					Return(&datastore.Application{
						UID:       appId,
						GroupID:   groupID,
						Title:     "Valid application endpoint",
						Endpoints: []datastore.Endpoint{},
					}, nil)

				o, _ := app.groupRepo.(*mocks.MockGroupRepository)

				o.EXPECT().
					LoadGroups(gomock.Any(), gomock.Any()).Times(1).
					Return([]*datastore.Group{group}, nil)
			},
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			url := fmt.Sprintf("/api/v1/applications/%s/endpoints", tc.appId)
			req := httptest.NewRequest(tc.method, url, tc.body)
			req.SetBasicAuth("test", "test")
			req.Header.Add("Content-Type", "application/json")
			w := httptest.NewRecorder()

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

			if w.Code != tc.statusCode {
				t.Errorf("Want status '%d', got '%d'", tc.statusCode, w.Code)
			}
		})
	}

}

func TestApplicationHandler_UpdateAppEndpoint(t *testing.T) {

	groupID := "1234567890"
	group := &datastore.Group{UID: groupID}

	appId := "12345"
	endpointId := "9999900000-8888"

	tt := []struct {
		name       string
		cfgPath    string
		method     string
		statusCode int
		appId      string
		endpointId string
		body       *strings.Reader
		dbFn       func(app *applicationHandler)
	}{
		{
			name:       "invalid request",
			cfgPath:    "./testdata/Auth_Config/no-auth-convoy.json",
			method:     http.MethodPut,
			statusCode: http.StatusBadRequest,
			appId:      appId,
			endpointId: endpointId,
			body:       strings.NewReader(``),
			dbFn: func(app *applicationHandler) {
				a, _ := app.appRepo.(*mocks.MockApplicationRepository)
				a.EXPECT().
					UpdateApplication(gomock.Any(), gomock.Any()).Times(0).
					Return(nil)

				a.EXPECT().
					FindApplicationByID(gomock.Any(), gomock.Any()).Times(1).
					Return(&datastore.Application{
						UID:       appId,
						GroupID:   groupID,
						Title:     "invalid application update",
						Endpoints: []datastore.Endpoint{},
					}, nil)

				o, _ := app.groupRepo.(*mocks.MockGroupRepository)

				o.EXPECT().
					LoadGroups(gomock.Any(), gomock.Any()).Times(1).
					Return([]*datastore.Group{group}, nil)
			},
		},
		{
			name:       "valid application",
			cfgPath:    "./testdata/Auth_Config/no-auth-convoy.json",
			method:     http.MethodPut,
			statusCode: http.StatusAccepted,
			appId:      appId,
			endpointId: endpointId,
			body:       strings.NewReader(`{"url": "https://google.com", "description": "Correct endpoint"}`),
			dbFn: func(app *applicationHandler) {
				a, _ := app.appRepo.(*mocks.MockApplicationRepository)
				a.EXPECT().
					UpdateApplication(gomock.Any(), gomock.Any()).Times(1).
					Return(nil)

				a.EXPECT().
					FindApplicationByID(gomock.Any(), gomock.Any()).Times(1).
					Return(&datastore.Application{
						UID:     appId,
						GroupID: groupID,
						Title:   "Valid application update",
						Endpoints: []datastore.Endpoint{
							{
								UID:         endpointId,
								TargetURL:   "http://",
								Description: "desc",
							},
						},
					}, nil)

				o, _ := app.groupRepo.(*mocks.MockGroupRepository)

				o.EXPECT().
					LoadGroups(gomock.Any(), gomock.Any()).Times(1).
					Return([]*datastore.Group{group}, nil)
			},
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			// Arrange
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

			url := fmt.Sprintf("/api/v1/applications/%s/endpoints/%s", tc.appId, tc.endpointId)
			req := httptest.NewRequest(tc.method, url, tc.body)
			req.SetBasicAuth("test", "test")
			req.Header.Add("Content-Type", "application/json")

			w := httptest.NewRecorder()
			rctx := chi.NewRouteContext()
			rctx.URLParams.Add("appID", tc.appId)
			rctx.URLParams.Add("endpointID", tc.endpointId)

			req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
			req = req.WithContext(context.WithValue(req.Context(), appCtx, &datastore.Application{
				UID:       appId,
				GroupID:   groupID,
				Title:     "Valid application update",
				Endpoints: []datastore.Endpoint{},
			}))

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

			d := stripTimestamp(t, "endpoint", w.Body)

			w.Body = d
			verifyMatch(t, *w)
		})
	}

}

func Test_applicationHandler_GetDashboardSummary(t *testing.T) {

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

	groupID := "1234567890"

	group := &datastore.Group{
		UID:  groupID,
		Name: "Valid group",
	}

	app = newApplicationHandler(eventRepo, eventDeliveryRepo, appRepo, groupRepo, apiKeyRepo, eventQueue, logger, tracer, cache)

	tt := []struct {
		name       string
		method     string
		statusCode int
		dbFn       func(eventRepo *mocks.MockEventRepository, appRepo *mocks.MockApplicationRepository, orgRepo *mocks.MockGroupRepository)
	}{
		{
			name:       "valid groups",
			method:     http.MethodGet,
			statusCode: http.StatusOK,
			dbFn: func(eventRepo *mocks.MockEventRepository, appRepo *mocks.MockApplicationRepository, orgRepo *mocks.MockGroupRepository) {
				appRepo.EXPECT().
					CountGroupApplications(gomock.Any(), gomock.Any()).Times(1).
					Return(int64(5), nil)
				eventRepo.EXPECT().
					LoadEventIntervals(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Times(1).
					Return([]datastore.EventInterval{
						{
							Data: datastore.EventIntervalData{
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
			request := httptest.NewRequest(tc.method, fmt.Sprintf("/api/v1/dashboard/%s/summary?startDate=%s&type=daily", groupID, time.Now().Format(format)), nil)
			responseRecorder := httptest.NewRecorder()
			rctx := chi.NewRouteContext()
			rctx.URLParams.Add("groupID", groupID)

			request = request.WithContext(context.WithValue(request.Context(), chi.RouteCtxKey, rctx))
			request = request.WithContext(context.WithValue(request.Context(), groupCtx, group))

			if tc.dbFn != nil {
				tc.dbFn(eventRepo, appRepo, groupRepo)
			}

			(http.HandlerFunc(app.GetDashboardSummary)).
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

	groupRepo := mocks.NewMockGroupRepository(ctrl)
	appRepo := mocks.NewMockApplicationRepository(ctrl)
	eventRepo := mocks.NewMockEventRepository(ctrl)
	eventDeliveryRepo := mocks.NewMockEventDeliveryRepository(ctrl)
	eventQueue := mocks.NewMockQueuer(ctrl)
	logger := logger.NewNoopLogger()
	tracer := mocks.NewMockTracer(ctrl)
	apiKeyRepo := mocks.NewMockAPIKeyRepository(ctrl)
	cache := mcache.NewMemoryCache()

	groupID := "1234567890"

	group := &datastore.Group{
		UID:  groupID,
		Name: "Valid group",
	}

	app = newApplicationHandler(eventRepo, eventDeliveryRepo, appRepo, groupRepo, apiKeyRepo, eventQueue, logger, tracer, cache)

	tt := []struct {
		name       string
		method     string
		statusCode int
		dbFn       func(appRepo *mocks.MockApplicationRepository, groupRepo *mocks.MockGroupRepository)
	}{
		{
			name: "valid groups" +
				"",
			method:     http.MethodGet,
			statusCode: http.StatusOK,
			dbFn: func(appRepo *mocks.MockApplicationRepository, groupRepo *mocks.MockGroupRepository) {
				appRepo.EXPECT().
					LoadApplicationsPagedByGroupId(gomock.Any(), gomock.Any(), gomock.Any()).Times(1).
					Return([]datastore.Application{
						{
							UID:       "validID",
							GroupID:   groupID,
							Title:     "Valid application - 0",
							Endpoints: []datastore.Endpoint{},
						},
					},
						datastore.PaginationData{},
						nil)

			},
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			request := httptest.NewRequest(tc.method, fmt.Sprintf("/api/v1/dashboard/%s/apps?page=1", groupID), nil)
			responseRecorder := httptest.NewRecorder()
			rctx := chi.NewRouteContext()
			rctx.URLParams.Add("groupID", groupID)

			request = request.WithContext(context.WithValue(request.Context(), chi.RouteCtxKey, rctx))
			request = request.WithContext(context.WithValue(request.Context(), groupCtx, group))

			pageable := datastore.Pageable{
				Page:    1,
				PerPage: 10,
			}
			request = request.WithContext(context.WithValue(request.Context(), pageableCtx, pageable))

			if tc.dbFn != nil {
				tc.dbFn(appRepo, groupRepo)
			}

			fetchGroupApps(appRepo)(http.HandlerFunc(app.GetPaginatedApps)).
				ServeHTTP(responseRecorder, request)

			if responseRecorder.Code != tc.statusCode {
				t.Errorf("Want status '%d', got '%d'", tc.statusCode, responseRecorder.Code)
			}

			verifyMatch(t, *responseRecorder)
		})
	}

}
