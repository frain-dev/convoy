package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/frain-dev/convoy/auth/realm_chain"
	mcache "github.com/frain-dev/convoy/cache/memory"
	nooplimiter "github.com/frain-dev/convoy/limiter/noop"
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
	buf := b.Bytes()
	err := json.NewDecoder(b).Decode(&res)
	if err != nil {
		t.Errorf("could not stripTimestamp: %s", err)
		t.FailNow()
	}

	if res.Data == nil {
		return bytes.NewBuffer(buf)
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

func provideApplication(ctrl *gomock.Controller) *applicationHandler {
	groupRepo := mocks.NewMockGroupRepository(ctrl)
	appRepo := mocks.NewMockApplicationRepository(ctrl)
	eventRepo := mocks.NewMockEventRepository(ctrl)
	eventDeliveryRepo := mocks.NewMockEventDeliveryRepository(ctrl)
	eventQueue := mocks.NewMockQueuer(ctrl)
	logger := logger.NewNoopLogger()
	tracer := mocks.NewMockTracer(ctrl)
	apiKeyRepo := mocks.NewMockAPIKeyRepository(ctrl)
	cache := mcache.NewMemoryCache()
	limiter := nooplimiter.NewNoopLimiter()
	return newApplicationHandler(eventRepo, eventDeliveryRepo, appRepo, groupRepo, apiKeyRepo, eventQueue, logger, tracer, cache, limiter)
}

func TestApplicationHandler_GetApp(t *testing.T) {

	var app *applicationHandler

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	app = provideApplication(ctrl)

	groupID := "1234567890"
	group := &datastore.Group{UID: groupID}

	validID := "123456789"

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

func TestApplicationHandler_GetApps(t *testing.T) {

	var app *applicationHandler

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	app = provideApplication(ctrl)

	groupID := "1234567890"
	group := &datastore.Group{UID: groupID}

	validID := "123456789"

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
		{
			name:       "should_fail_to_fetch_applications",
			cfgPath:    "./testdata/Auth_Config/no-auth-convoy.json",
			method:     http.MethodGet,
			statusCode: http.StatusBadRequest,
			dbFn: func(app *applicationHandler) {
				a, _ := app.appRepo.(*mocks.MockApplicationRepository)
				a.EXPECT().
					LoadApplicationsPaged(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Times(1).
					Return(nil, datastore.PaginationData{}, errors.New("failed to load"))

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

			err := config.LoadConfig(tc.cfgPath)
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

	bodyReader := strings.NewReader(`{ "group_id": "` + groupID + `", "name": "ABC_DEF_TEST", "secret": "12345" ,"slack_webhook_url":"https://google.com"}`)

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
			app = provideApplication(ctrl)

			// Arrange
			req := httptest.NewRequest(tc.method, "/api/v1/applications", tc.body)
			req.SetBasicAuth("test", "test")
			req.Header.Add("Content-Type", "application/json")
			w := httptest.NewRecorder()

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

			// Act.
			router.ServeHTTP(w, req)

			if w.Code != tc.statusCode {
				t.Errorf("Want status '%d', got '%d'", tc.statusCode, w.Code)
			}

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

		{
			name:       "valid request - disable application",
			cfgPath:    "/testdata/Auth_Config/no-auth-convoy.json",
			method:     http.MethodPut,
			statusCode: http.StatusAccepted,
			appId:      appId,
			body:       strings.NewReader(`{"name": "ABC", "is_disabled": true }`),
			dbFn: func(app *applicationHandler) {
				a, _ := app.appRepo.(*mocks.MockApplicationRepository)
				a.EXPECT().
					UpdateApplication(gomock.Any(), gomock.Any()).Times(1).
					Return(nil)

				a.EXPECT().
					FindApplicationByID(gomock.Any(), gomock.Any()).Times(1).
					Return(&datastore.Application{
						UID:        appId,
						GroupID:    groupID,
						Title:      "Valid application update",
						Endpoints:  []datastore.Endpoint{},
						IsDisabled: false,
					}, nil)

				o, _ := app.groupRepo.(*mocks.MockGroupRepository)

				o.EXPECT().
					LoadGroups(gomock.Any(), gomock.Any()).Times(1).
					Return([]*datastore.Group{group}, nil)
			},
		},

		{
			name:       "valid request - enable disabled application",
			cfgPath:    "/testdata/Auth_Config/no-auth-convoy.json",
			method:     http.MethodPut,
			statusCode: http.StatusAccepted,
			appId:      appId,
			body:       strings.NewReader(`{"name": "ABC", "is_disabled": false }`),
			dbFn: func(app *applicationHandler) {
				a, _ := app.appRepo.(*mocks.MockApplicationRepository)
				a.EXPECT().
					UpdateApplication(gomock.Any(), gomock.Any()).Times(1).
					Return(nil)

				a.EXPECT().
					FindApplicationByID(gomock.Any(), gomock.Any()).Times(1).
					Return(&datastore.Application{
						UID:        appId,
						GroupID:    groupID,
						Title:      "Valid application update",
						Endpoints:  []datastore.Endpoint{},
						IsDisabled: true,
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
			app = provideApplication(ctrl)

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

			err := config.LoadConfig(tc.cfgPath)
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

func Test_applicationHandler_DeleteApp(t *testing.T) {

	groupID := "1234567890"
	group := &datastore.Group{UID: groupID}

	appId := "12345"

	tt := []struct {
		name       string
		cfgPath    string
		statusCode int
		appId      string
		dbFn       func(app *applicationHandler)
	}{
		{
			name:       "should_delete_app",
			cfgPath:    "./testdata/Auth_Config/no-auth-convoy.json",
			statusCode: http.StatusOK,
			appId:      appId,
			dbFn: func(app *applicationHandler) {
				a, _ := app.appRepo.(*mocks.MockApplicationRepository)
				obj := &datastore.Application{
					UID:       appId,
					GroupID:   groupID,
					Title:     "Valid application delete",
					Endpoints: []datastore.Endpoint{},
				}

				a.EXPECT().
					DeleteApplication(gomock.Any(), obj).Times(1).
					Return(nil)

				a.EXPECT().
					FindApplicationByID(gomock.Any(), gomock.Any()).Times(1).
					Return(obj, nil)

				o, _ := app.groupRepo.(*mocks.MockGroupRepository)

				o.EXPECT().
					LoadGroups(gomock.Any(), gomock.Any()).Times(1).
					Return([]*datastore.Group{group}, nil)
			},
		},
		{
			name:       "should_fail_to_delete_app",
			cfgPath:    "./testdata/Auth_Config/no-auth-convoy.json",
			statusCode: http.StatusBadRequest,
			appId:      appId,
			dbFn: func(app *applicationHandler) {
				a, _ := app.appRepo.(*mocks.MockApplicationRepository)

				obj := &datastore.Application{
					UID:       appId,
					GroupID:   groupID,
					Title:     "Valid application update",
					Endpoints: []datastore.Endpoint{},
				}

				a.EXPECT().
					DeleteApplication(gomock.Any(), obj).Times(1).
					Return(errors.New("failed to delete app endpoint"))

				a.EXPECT().
					FindApplicationByID(gomock.Any(), gomock.Any()).Times(1).
					Return(obj, nil)

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
			app = provideApplication(ctrl)

			url := fmt.Sprintf("/api/v1/applications/%s", tc.appId)
			req := httptest.NewRequest(http.MethodDelete, url, &bytes.Buffer{})
			req.Header.Add("Content-Type", "application/json")

			w := httptest.NewRecorder()

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
	app = provideApplication(ctrl)

	groupID := "1234567890"
	group := &datastore.Group{UID: groupID}

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
			body:       strings.NewReader(`{"url": "https://google.com", "description": "Test","rate_limit":300,"rate_limit_duration":"1h","secret":"abc"}`),
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
						Title:     "Valid application endpoint",
						Endpoints: []datastore.Endpoint{},
					}, nil)

				o, _ := app.groupRepo.(*mocks.MockGroupRepository)

				o.EXPECT().
					LoadGroups(gomock.Any(), gomock.Any()).Times(1).
					Return([]*datastore.Group{group}, nil)
			},
		},
		{
			name:       "should_error_for_invalid_rate_limit_duration",
			cfgPath:    "./testdata/Auth_Config/no-auth-convoy.json",
			method:     http.MethodPost,
			statusCode: http.StatusBadRequest,
			appId:      appId,
			body:       strings.NewReader(`{"url": "https://google.com", "description": "Test","rate_limit":300,"rate_limit_duration":"1"}`),
			dbFn: func(app *applicationHandler) {
				a, _ := app.appRepo.(*mocks.MockApplicationRepository)
				a.EXPECT().
					FindApplicationByID(gomock.Any(), gomock.Any()).Times(1).
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

			err := config.LoadConfig(tc.cfgPath)
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

			w.Body = stripTimestamp(t, "endpoint", w.Body)
			verifyMatch(t, *w)
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
						Endpoints: []datastore.Endpoint{{UID: endpointId}},
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
			body:       strings.NewReader(`{"url": "https://google.com", "description": "Correct endpoint","events":["payment.created"],"rate_limit_duration":"1h","http_timeout":"10s","rate_limit":3000}`),
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
		{
			name:       "should_error_for_endpoint_not_found",
			cfgPath:    "./testdata/Auth_Config/no-auth-convoy.json",
			method:     http.MethodPut,
			statusCode: http.StatusBadRequest,
			appId:      appId,
			endpointId: endpointId,
			body:       strings.NewReader(`{"url": "https://google.com", "description": "Correct endpoint","events":["payment.created"],"rate_limit_duration":"1h","http_timeout":"10s","rate_limit":3000}`),
			dbFn: func(app *applicationHandler) {
				a, _ := app.appRepo.(*mocks.MockApplicationRepository)

				a.EXPECT().
					FindApplicationByID(gomock.Any(), gomock.Any()).Times(1).
					Return(&datastore.Application{
						UID:     appId,
						GroupID: groupID,
						Title:   "Valid application update",
						Endpoints: []datastore.Endpoint{
							{
								UID:         "123",
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
		{
			name:       "should_error_for_invalid_rate_limit_duration",
			cfgPath:    "./testdata/Auth_Config/no-auth-convoy.json",
			method:     http.MethodPut,
			statusCode: http.StatusBadRequest,
			appId:      appId,
			endpointId: endpointId,
			body:       strings.NewReader(`{"url": "https://google.com", "description": "Correct endpoint", "rate_limit_duration":"1"}`),
			dbFn: func(app *applicationHandler) {
				a, _ := app.appRepo.(*mocks.MockApplicationRepository)
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
			app = provideApplication(ctrl)

			url := fmt.Sprintf("/api/v1/applications/%s/endpoints/%s", tc.appId, tc.endpointId)
			req := httptest.NewRequest(tc.method, url, tc.body)
			req.SetBasicAuth("test", "test")
			req.Header.Add("Content-Type", "application/json")

			w := httptest.NewRecorder()

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

			if w.Code != tc.statusCode {
				t.Errorf("Want status '%d', got '%d'", tc.statusCode, w.Code)
			}

			d := stripTimestamp(t, "endpoint", w.Body)

			w.Body = d
			verifyMatch(t, *w)
		})
	}

}

func TestApplicationHandler_GetAppEndpoint(t *testing.T) {
	groupID := "1234567890"
	group := &datastore.Group{UID: groupID}

	tt := []struct {
		name       string
		cfgPath    string
		appID      string
		endpointID string
		statusCode int
		id         string
		dbFn       func(app *applicationHandler)
	}{
		{
			name:       "should_get_application_endpoint",
			cfgPath:    "./testdata/Auth_Config/no-auth-convoy.json",
			statusCode: http.StatusOK,
			appID:      "a-123",
			endpointID: "def",
			dbFn: func(app *applicationHandler) {
				a, _ := app.appRepo.(*mocks.MockApplicationRepository)

				a.EXPECT().
					FindApplicationByID(gomock.Any(), gomock.Any()).Times(1).
					Return(&datastore.Application{
						UID:     "a-123",
						GroupID: groupID,
						Title:   "Valid application update",
						Endpoints: []datastore.Endpoint{
							{
								UID:         "abc",
								TargetURL:   "http://amazon.com",
								Description: "desc",
							},
							{
								UID:         "def",
								TargetURL:   "http://google.com",
								Description: "desc",
							},
							{
								UID:         "123",
								TargetURL:   "http://",
								Description: "deleted endpoint",
								DeletedAt:   primitive.NewDateTimeFromTime(time.Now()),
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

			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			app := provideApplication(ctrl)

			url := fmt.Sprintf("/api/v1/applications/%s/endpoints", tc.appID)
			req := httptest.NewRequest(http.MethodGet, url, &bytes.Buffer{})
			req.Header.Add("Content-Type", "application/json")

			w := httptest.NewRecorder()

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

			if w.Code != tc.statusCode {
				t.Errorf("Want status '%d', got '%d'", tc.statusCode, w.Code)
			}

			verifyMatch(t, *w)
		})
	}
}

func TestApplicationHandler_GetAppEndpoints(t *testing.T) {
	groupID := "1234567890"
	group := &datastore.Group{UID: groupID}

	tt := []struct {
		name       string
		cfgPath    string
		appID      string
		statusCode int
		id         string
		dbFn       func(app *applicationHandler)
	}{
		{
			name:       "should_get_application_endpoints",
			cfgPath:    "./testdata/Auth_Config/no-auth-convoy.json",
			statusCode: http.StatusOK,
			appID:      "a-123",
			dbFn: func(app *applicationHandler) {
				a, _ := app.appRepo.(*mocks.MockApplicationRepository)

				a.EXPECT().
					FindApplicationByID(gomock.Any(), gomock.Any()).Times(1).
					Return(&datastore.Application{
						UID:     "a-123",
						GroupID: groupID,
						Title:   "Valid application update",
						Endpoints: []datastore.Endpoint{
							{
								UID:         "abc",
								TargetURL:   "http://amazon.com",
								Description: "desc",
							},
							{
								UID:         "abc",
								TargetURL:   "http://google.com",
								Description: "desc",
							},
							{
								UID:         "abc",
								TargetURL:   "http://",
								Description: "deleted endpoint",
								DeletedAt:   primitive.NewDateTimeFromTime(time.Now()),
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

			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			app := provideApplication(ctrl)

			url := fmt.Sprintf("/api/v1/applications/%s/endpoints", tc.appID)
			req := httptest.NewRequest(http.MethodGet, url, &bytes.Buffer{})
			req.Header.Add("Content-Type", "application/json")

			w := httptest.NewRecorder()

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

			if w.Code != tc.statusCode {
				t.Errorf("Want status '%d', got '%d'", tc.statusCode, w.Code)
			}
			fmt.Println("gg", w.Body.String())
			verifyMatch(t, *w)
		})
	}
}

func Test_applicationHandler_GetDashboardSummary(t *testing.T) {

	var app *applicationHandler

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	app = provideApplication(ctrl)

	groupID := "1234567890"

	group := &datastore.Group{
		UID:  groupID,
		Name: "Valid group",
	}

	tt := []struct {
		name       string
		method     string
		statusCode int
		dbFn       func(app *applicationHandler)
	}{
		{
			name:       "valid groups",
			method:     http.MethodGet,
			statusCode: http.StatusOK,
			dbFn: func(app *applicationHandler) {
				app.appRepo.(*mocks.MockApplicationRepository).EXPECT().
					CountGroupApplications(gomock.Any(), gomock.Any()).Times(1).
					Return(int64(5), nil)
				app.eventRepo.(*mocks.MockEventRepository).EXPECT().
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
				tc.dbFn(app)
			}

			(http.HandlerFunc(app.GetDashboardSummary)).
				ServeHTTP(responseRecorder, request)

			if responseRecorder.Code != tc.statusCode {
				t.Errorf("Want status '%d', got '%d'", tc.statusCode, responseRecorder.Code)
			}
		})
	}

}

func Test_applicationHandler_DeleteAppEndpoint(t *testing.T) {

	groupID := "1234567890"
	group := &datastore.Group{UID: groupID}

	appId := "12345"
	endpointId := "9999900000-8888"

	tt := []struct {
		name       string
		cfgPath    string
		statusCode int
		appId      string
		endpointId string
		dbFn       func(app *applicationHandler)
	}{
		{
			name:       "should_delete_app_endpoint",
			cfgPath:    "./testdata/Auth_Config/no-auth-convoy.json",
			statusCode: http.StatusOK,
			appId:      appId,
			endpointId: endpointId,
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
		{
			name:       "should_fail_to_delete_app_endpoint",
			cfgPath:    "./testdata/Auth_Config/no-auth-convoy.json",
			statusCode: http.StatusBadRequest,
			appId:      appId,
			endpointId: endpointId,
			dbFn: func(app *applicationHandler) {
				a, _ := app.appRepo.(*mocks.MockApplicationRepository)
				a.EXPECT().
					UpdateApplication(gomock.Any(), gomock.Any()).Times(1).
					Return(errors.New("failed to delete app endpoint"))

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
		{
			name:       "should_error_for_endpoint_not_found",
			cfgPath:    "./testdata/Auth_Config/no-auth-convoy.json",
			statusCode: http.StatusBadRequest,
			appId:      appId,
			endpointId: endpointId,
			dbFn: func(app *applicationHandler) {
				a, _ := app.appRepo.(*mocks.MockApplicationRepository)

				a.EXPECT().
					FindApplicationByID(gomock.Any(), gomock.Any()).Times(1).
					Return(&datastore.Application{
						UID:     appId,
						GroupID: groupID,
						Title:   "Valid application update",
						Endpoints: []datastore.Endpoint{
							{
								UID:         "123",
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
			app = provideApplication(ctrl)

			url := fmt.Sprintf("/api/v1/applications/%s/endpoints/%s", tc.appId, tc.endpointId)
			req := httptest.NewRequest(http.MethodDelete, url, &bytes.Buffer{})
			req.SetBasicAuth("test", "test")
			req.Header.Add("Content-Type", "application/json")

			w := httptest.NewRecorder()

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

			if w.Code != tc.statusCode {
				t.Errorf("Want status '%d', got '%d'", tc.statusCode, w.Code)
			}

			verifyMatch(t, *w)
		})
	}

}

func Test_applicationHandler_GetPaginatedApps(t *testing.T) {

	var app *applicationHandler

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	app = provideApplication(ctrl)

	groupID := "1234567890"

	group := &datastore.Group{
		UID:  groupID,
		Name: "Valid group",
	}

	tt := []struct {
		name       string
		method     string
		statusCode int
		dbFn       func(app *applicationHandler)
	}{
		{
			name: "valid groups" +
				"",
			method:     http.MethodGet,
			statusCode: http.StatusOK,
			dbFn: func(app *applicationHandler) {
				app.appRepo.(*mocks.MockApplicationRepository).EXPECT().
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
				tc.dbFn(app)
			}

			fetchGroupApps(app.appRepo.(*mocks.MockApplicationRepository))(http.HandlerFunc(app.GetPaginatedApps)).
				ServeHTTP(responseRecorder, request)

			if responseRecorder.Code != tc.statusCode {
				t.Errorf("Want status '%d', got '%d'", tc.statusCode, responseRecorder.Code)
			}

			verifyMatch(t, *responseRecorder)
		})
	}

}
