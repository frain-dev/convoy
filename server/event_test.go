package server

import (
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
	log "github.com/sirupsen/logrus"
)

func TestApplicationHandler_CreateAppEvent(t *testing.T) {

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

	appId := "12345"
	msgId := "1122333444456"

	message := &datastore.Event{
		UID: msgId,
		AppMetadata: &datastore.AppMetadata{
			UID: appId,
		},
	}

	type args struct {
		message *datastore.Event
	}

	tests := []struct {
		name       string
		cfgPath    string
		method     string
		statusCode int
		args       args
		body       *strings.Reader
		dbFn       func(app *applicationHandler)
	}{
		{
			name:       "invalid message - malformed request",
			cfgPath:    "./testdata/Auth_Config/no-auth-convoy.json",
			method:     http.MethodPost,
			statusCode: http.StatusBadRequest,
			body:       strings.NewReader(`{"data": {}`),
			args: args{
				message: message,
			},
			dbFn: func(app *applicationHandler) {
				o, _ := app.groupRepo.(*mocks.MockGroupRepository)

				o.EXPECT().
					LoadGroups(gomock.Any(), gomock.Any()).Times(1).
					Return([]*datastore.Group{group}, nil)
			},
		},
		{
			name:       "invalid message - no app_id",
			cfgPath:    "./testdata/Auth_Config/no-auth-convoy.json",
			method:     http.MethodPost,
			statusCode: http.StatusBadRequest,
			body:       strings.NewReader(`{ "event_type: "test", "data": {}}`),
			args: args{
				message: message,
			},
			dbFn: func(app *applicationHandler) {
				o, _ := app.groupRepo.(*mocks.MockGroupRepository)

				o.EXPECT().
					LoadGroups(gomock.Any(), gomock.Any()).Times(1).
					Return([]*datastore.Group{group}, nil)

			},
		},
		{
			name:       "invalid message - no data field",
			cfgPath:    "./testdata/Auth_Config/no-auth-convoy.json",
			method:     http.MethodPost,
			statusCode: http.StatusBadRequest,
			body:       strings.NewReader(`{ "app_id": "", "event_type: "test" }`),
			args: args{
				message: message,
			},
			dbFn: func(app *applicationHandler) {
				o, _ := app.groupRepo.(*mocks.MockGroupRepository)

				o.EXPECT().
					LoadGroups(gomock.Any(), gomock.Any()).Times(1).
					Return([]*datastore.Group{group}, nil)
			},
		},
		{
			name:       "invalid message - no event type",
			cfgPath:    "./testdata/Auth_Config/no-auth-convoy.json",
			method:     http.MethodPost,
			statusCode: http.StatusBadRequest,
			body:       strings.NewReader(`{ "data": {}}`),
			args: args{
				message: message,
			},
			dbFn: func(app *applicationHandler) {
				m, _ := app.eventRepo.(*mocks.MockEventRepository)
				m.EXPECT().
					CreateEvent(gomock.Any(), gomock.Any()).Times(0).
					Return(nil)

				o, _ := app.groupRepo.(*mocks.MockGroupRepository)

				o.EXPECT().
					LoadGroups(gomock.Any(), gomock.Any()).Times(1).
					Return([]*datastore.Group{group}, nil)
			},
		},
		{
			name:       "valid message - no endpoints",
			cfgPath:    "./testdata/Auth_Config/no-auth-convoy.json",
			method:     http.MethodPost,
			statusCode: http.StatusBadRequest,
			body:       strings.NewReader(`{"app_id": "12345", "event_type": "test",  "data": {}}`),
			args: args{
				message: message,
			},
			dbFn: func(app *applicationHandler) {
				c, _ := app.cache.(*mocks.MockCache)
				c.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any()).Times(1)

				a, _ := app.appRepo.(*mocks.MockApplicationRepository)
				a.EXPECT().
					FindApplicationByID(gomock.Any(), gomock.Any()).Times(1).
					Return(&datastore.Application{
						UID:       appId,
						GroupID:   groupId,
						Title:     "Valid application",
						Endpoints: []datastore.Endpoint{},
					}, nil)

				c.EXPECT().Set(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Times(1)
				o, _ := app.groupRepo.(*mocks.MockGroupRepository)

				o.EXPECT().
					LoadGroups(gomock.Any(), gomock.Any()).Times(1).
					Return([]*datastore.Group{group}, nil)
			},
		},
		{
			name:       "valid message - no active endpoints",
			cfgPath:    "./testdata/Auth_Config/no-auth-convoy.json",
			method:     http.MethodPost,
			statusCode: http.StatusCreated,
			body:       strings.NewReader(`{"app_id": "12345", "event_type": "test",  "data": {}}`),
			args: args{
				message: message,
			},
			dbFn: func(app *applicationHandler) {
				c, _ := app.cache.(*mocks.MockCache)
				c.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any()).Times(1)

				a, _ := app.appRepo.(*mocks.MockApplicationRepository)
				a.EXPECT().
					FindApplicationByID(gomock.Any(), gomock.Any()).Times(1).
					Return(&datastore.Application{
						UID:     appId,
						GroupID: groupId,
						Title:   "Valid application",
						Endpoints: []datastore.Endpoint{
							{
								TargetURL: "http://localhost",
								Status:    datastore.InactiveEndpointStatus,
							},
						},
					}, nil)
				c.EXPECT().Set(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Times(1)

				e, _ := app.eventRepo.(*mocks.MockEventRepository)
				e.EXPECT().
					CreateEvent(gomock.Any(), gomock.Any()).Times(1).
					Return(nil)

				o, _ := app.groupRepo.(*mocks.MockGroupRepository)

				o.EXPECT().
					LoadGroups(gomock.Any(), gomock.Any()).Times(1).
					Return([]*datastore.Group{group}, nil)
			},
		},
		{
			name:       "valid message - no matching endpoints",
			cfgPath:    "./testdata/Auth_Config/no-auth-convoy.json",
			method:     http.MethodPost,
			statusCode: http.StatusCreated,
			body:       strings.NewReader(`{"app_id": "12345", "event_type": "test.event", "data": { "Hello": "World", "Test": "Data" }}`),
			args: args{
				message: message,
			},
			dbFn: func(app *applicationHandler) {
				c, _ := app.cache.(*mocks.MockCache)
				c.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any()).Times(1)

				a, _ := app.appRepo.(*mocks.MockApplicationRepository)
				a.EXPECT().
					FindApplicationByID(gomock.Any(), gomock.Any()).Times(1).
					Return(&datastore.Application{
						UID:     appId,
						GroupID: groupId,
						Title:   "Valid application",
						Endpoints: []datastore.Endpoint{
							{
								TargetURL: "http://localhost",
								Status:    datastore.ActiveEndpointStatus,
							},
						},
					}, nil)

				c.EXPECT().Set(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Times(1)

				m, _ := app.eventRepo.(*mocks.MockEventRepository)
				m.EXPECT().
					CreateEvent(gomock.Any(), gomock.Any()).Times(1).
					Return(nil)

				o, _ := app.groupRepo.(*mocks.MockGroupRepository)

				o.EXPECT().
					LoadGroups(gomock.Any(), gomock.Any()).Times(1).
					Return([]*datastore.Group{group}, nil)
			},
		},
		{
			name:       "valid message - matching endpoints",
			cfgPath:    "./testdata/Auth_Config/no-auth-convoy.json",
			method:     http.MethodPost,
			statusCode: http.StatusCreated,
			body:       strings.NewReader(`{"app_id": "12345", "event_type": "test.event", "data": { "Hello": "World", "Test": "Data" }}`),
			args: args{
				message: message,
			},
			dbFn: func(app *applicationHandler) {
				c, _ := app.cache.(*mocks.MockCache)
				c.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any()).Times(1)

				a, _ := app.appRepo.(*mocks.MockApplicationRepository)
				a.EXPECT().
					FindApplicationByID(gomock.Any(), gomock.Any()).Times(1).
					Return(&datastore.Application{
						UID:     appId,
						GroupID: groupId,
						Title:   "Valid application",
						Endpoints: []datastore.Endpoint{
							{
								TargetURL: "http://localhost",
								Status:    datastore.ActiveEndpointStatus,
								Events:    []string{"test.event"},
							},
						},
					}, nil)
				c.EXPECT().Set(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Times(1)

				m, _ := app.eventRepo.(*mocks.MockEventRepository)
				m.EXPECT().
					CreateEvent(gomock.Any(), gomock.Any()).Times(1).
					Return(nil)

				ed, _ := app.eventDeliveryRepo.(*mocks.MockEventDeliveryRepository)
				ed.EXPECT().
					CreateEventDelivery(gomock.Any(), gomock.Any()).
					Return(nil)

				q, _ := app.eventQueue.(*mocks.MockQueuer)
				q.EXPECT().
					WriteEvent(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Times(1).
					Return(nil)
				q.EXPECT().
					WriteEventDelivery(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Times(1).
					Return(nil)

				o, _ := app.groupRepo.(*mocks.MockGroupRepository)

				o.EXPECT().
					LoadGroups(gomock.Any(), gomock.Any()).Times(1).
					Return([]*datastore.Group{group}, nil)
			},
		},
		{
			name:       "valid message - matching inactive endpoints",
			cfgPath:    "./testdata/Auth_Config/no-auth-convoy.json",
			method:     http.MethodPost,
			statusCode: http.StatusCreated,
			body:       strings.NewReader(`{"app_id": "12345", "event_type": "test.event", "data": { "Hello": "World", "Test": "Data" }}`),
			args: args{
				message: message,
			},
			dbFn: func(app *applicationHandler) {
				c, _ := app.cache.(*mocks.MockCache)
				c.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any()).Times(1)

				a, _ := app.appRepo.(*mocks.MockApplicationRepository)
				a.EXPECT().
					FindApplicationByID(gomock.Any(), gomock.Any()).Times(1).
					Return(&datastore.Application{
						UID:     appId,
						GroupID: groupId,
						Title:   "Valid application",
						Endpoints: []datastore.Endpoint{
							{
								TargetURL: "http://localhost",
								Status:    datastore.InactiveEndpointStatus,
								Events:    []string{"test.event"},
							},
						},
					}, nil)
				c.EXPECT().Set(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Times(1)

				m, _ := app.eventRepo.(*mocks.MockEventRepository)
				m.EXPECT().
					CreateEvent(gomock.Any(), gomock.Any()).Times(1).
					Return(nil)

				ed, _ := app.eventDeliveryRepo.(*mocks.MockEventDeliveryRepository)
				ed.EXPECT().
					CreateEventDelivery(gomock.Any(), gomock.Any()).
					Return(nil)

				o, _ := app.groupRepo.(*mocks.MockGroupRepository)

				o.EXPECT().
					LoadGroups(gomock.Any(), gomock.Any()).Times(1).
					Return([]*datastore.Group{group}, nil)
			},
		},
		{
			name:       "valid message - matching inactive and active endpoints",
			cfgPath:    "./testdata/Auth_Config/no-auth-convoy.json",
			method:     http.MethodPost,
			statusCode: http.StatusCreated,
			body:       strings.NewReader(`{"app_id": "12345", "event_type": "test.event", "data": { "Hello": "World", "Test": "Data" }}`),
			args: args{
				message: message,
			},
			dbFn: func(app *applicationHandler) {
				c, _ := app.cache.(*mocks.MockCache)
				c.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any()).Times(1)

				a, _ := app.appRepo.(*mocks.MockApplicationRepository)
				a.EXPECT().
					FindApplicationByID(gomock.Any(), gomock.Any()).Times(1).
					Return(&datastore.Application{
						UID:     appId,
						GroupID: groupId,
						Title:   "Valid application",
						Endpoints: []datastore.Endpoint{
							{
								TargetURL: "http://localhost",
								Status:    datastore.InactiveEndpointStatus,
								Events:    []string{"test.event"},
							},
							{
								TargetURL: "http://localhost",
								Status:    datastore.ActiveEndpointStatus,
								Events:    []string{"test.event"},
							},
						},
					}, nil)
				c.EXPECT().Set(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Times(1)

				m, _ := app.eventRepo.(*mocks.MockEventRepository)
				m.EXPECT().
					CreateEvent(gomock.Any(), gomock.Any()).Times(1).
					Return(nil)

				ed, _ := app.eventDeliveryRepo.(*mocks.MockEventDeliveryRepository)

				ed.EXPECT().
					CreateEventDelivery(gomock.Any(), gomock.Any()).Times(2).
					Return(nil)

				q, _ := app.eventQueue.(*mocks.MockQueuer)
				q.EXPECT().
					WriteEvent(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Times(1).
					Return(nil)
				q.EXPECT().
					WriteEventDelivery(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Times(1).
					Return(nil)

				o, _ := app.groupRepo.(*mocks.MockGroupRepository)

				o.EXPECT().
					LoadGroups(gomock.Any(), gomock.Any()).Times(1).
					Return([]*datastore.Group{group}, nil)
			},
		},
		{
			name:       "valid message - disabled application",
			cfgPath:    "./testdata/Auth_Config/no-auth-convoy.json",
			method:     http.MethodPost,
			statusCode: http.StatusBadRequest,
			body:       strings.NewReader(`{"app_id": "12345", "event_type": "test",  "data": {}}`),
			args: args{
				message: message,
			},
			dbFn: func(app *applicationHandler) {
				c, _ := app.cache.(*mocks.MockCache)
				c.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any()).Times(1)

				a, _ := app.appRepo.(*mocks.MockApplicationRepository)
				a.EXPECT().
					FindApplicationByID(gomock.Any(), gomock.Any()).Times(1).
					Return(&datastore.Application{
						UID:        appId,
						GroupID:    groupId,
						Title:      "Valid application",
						IsDisabled: true,
						Endpoints:  []datastore.Endpoint{},
					}, nil)
				c.EXPECT().Set(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Times(1)

				o, _ := app.groupRepo.(*mocks.MockGroupRepository)

				o.EXPECT().
					LoadGroups(gomock.Any(), gomock.Any()).Times(1).
					Return([]*datastore.Group{group}, nil)
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := config.LoadConfig(tc.cfgPath)
			if err != nil {
				t.Errorf("Failed to load config file: %v", err)
			}
			initRealmChain(t, app.apiKeyRepo)

			req := httptest.NewRequest(tc.method, "/api/v1/events", tc.body)
			req.SetBasicAuth("test", "test")
			req.Header.Add("Content-Type", "application/json")
			w := httptest.NewRecorder()

			if tc.dbFn != nil {
				tc.dbFn(app)
			}

			router := buildRoutes(app)

			// Act
			router.ServeHTTP(w, req)

			if w.Code != tc.statusCode {
				log.Error(tc.args.message, w.Body)
				t.Errorf("Want status '%d', got '%d'", tc.statusCode, w.Code)
			}
		})
	}
}

func Test_resendEventDelivery(t *testing.T) {

	var app *applicationHandler

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	app = provideApplication(ctrl)

	group := &datastore.Group{Name: "default-group", UID: "1234567890"}

	appID := "12345"
	eventID := "1122333444456"
	eventDeliveryID := "2134453454"

	type args struct {
		event   *datastore.Event
		message *datastore.EventDelivery
	}

	tests := []struct {
		name       string
		cfgPath    string
		method     string
		statusCode int
		args       args
		body       *strings.Reader
		dbFn       func(*datastore.Event, *datastore.EventDelivery, *applicationHandler)
	}{
		{
			name:       "invalid resend - event successful",
			cfgPath:    "./testdata/Auth_Config/no-auth-convoy.json",
			method:     http.MethodPut,
			statusCode: http.StatusBadRequest,
			body:       nil,
			args: args{
				event: &datastore.Event{
					UID: eventID,
				},
				message: &datastore.EventDelivery{
					UID: eventDeliveryID,
					EventMetadata: &datastore.EventMetadata{
						UID: eventID,
					},
					Status: datastore.SuccessEventStatus,
					AppMetadata: &datastore.AppMetadata{
						UID: appID,
					},
				},
			},
			dbFn: func(ev *datastore.Event, msg *datastore.EventDelivery, app *applicationHandler) {

				m, _ := app.eventDeliveryRepo.(*mocks.MockEventDeliveryRepository)
				m.EXPECT().
					FindEventDeliveryByID(gomock.Any(), gomock.Any()).Times(1).
					Return(msg, nil)

				m.EXPECT().
					UpdateStatusOfEventDelivery(gomock.Any(), gomock.Any(), gomock.Any()).Times(0).
					Return(nil)

				o, _ := app.groupRepo.(*mocks.MockGroupRepository)
				o.EXPECT().
					LoadGroups(gomock.Any(), gomock.Any()).Times(1).
					Return([]*datastore.Group{group}, nil)
			},
		},
		{
			name:       "invalid resend - event not failed",
			cfgPath:    "./testdata/Auth_Config/no-auth-convoy.json",
			method:     http.MethodPut,
			statusCode: http.StatusBadRequest,
			body:       nil,
			args: args{
				event: &datastore.Event{
					UID: eventID,
				},
				message: &datastore.EventDelivery{
					UID: eventDeliveryID,
					EventMetadata: &datastore.EventMetadata{
						UID: eventID,
					},
					Status: datastore.ProcessingEventStatus,
					AppMetadata: &datastore.AppMetadata{
						UID: appID,
					},
				},
			},
			dbFn: func(ev *datastore.Event, msg *datastore.EventDelivery, app *applicationHandler) {

				m, _ := app.eventDeliveryRepo.(*mocks.MockEventDeliveryRepository)
				m.EXPECT().
					FindEventDeliveryByID(gomock.Any(), gomock.Any()).Times(1).
					Return(msg, nil)

				o, _ := app.groupRepo.(*mocks.MockGroupRepository)

				o.EXPECT().
					LoadGroups(gomock.Any(), gomock.Any()).Times(1).
					Return([]*datastore.Group{group}, nil)
			},
		},
		{
			name:       "invalid  resend - pending endpoint",
			cfgPath:    "./testdata/Auth_Config/no-auth-convoy.json",
			method:     http.MethodPut,
			statusCode: http.StatusBadRequest,
			body:       nil,
			args: args{
				event: &datastore.Event{
					UID: eventID,
				},
				message: &datastore.EventDelivery{
					UID: eventDeliveryID,
					EventMetadata: &datastore.EventMetadata{
						UID: eventID,
					},
					Status: datastore.FailureEventStatus,
					EndpointMetadata: &datastore.EndpointMetadata{
						TargetURL: "http://localhost",
						Status:    datastore.PendingEndpointStatus,
					},
					AppMetadata: &datastore.AppMetadata{
						UID: appID,
					},
				},
			},
			dbFn: func(ev *datastore.Event, msg *datastore.EventDelivery, app *applicationHandler) {
				m, _ := app.eventDeliveryRepo.(*mocks.MockEventDeliveryRepository)
				m.EXPECT().
					FindEventDeliveryByID(gomock.Any(), gomock.Any()).Times(1).
					Return(msg, nil)

				a, _ := app.appRepo.(*mocks.MockApplicationRepository)
				a.EXPECT().
					FindApplicationEndpointByID(gomock.Any(), gomock.Any(), gomock.Any()).Times(1).
					Return(&datastore.Endpoint{
						TargetURL: "http://localhost",
						Status:    datastore.PendingEndpointStatus,
					}, nil)

				o, _ := app.groupRepo.(*mocks.MockGroupRepository)

				o.EXPECT().
					LoadGroups(gomock.Any(), gomock.Any()).Times(1).
					Return([]*datastore.Group{group}, nil)
			},
		},
		{
			name:       "valid resend - previously failed and inactive endpoint",
			cfgPath:    "./testdata/Auth_Config/no-auth-convoy.json",
			method:     http.MethodPut,
			statusCode: http.StatusOK,
			body:       nil,
			args: args{
				event: &datastore.Event{
					UID: eventID,
				},
				message: &datastore.EventDelivery{
					UID: eventDeliveryID,
					EventMetadata: &datastore.EventMetadata{
						UID: eventID,
					},
					Status: datastore.FailureEventStatus,
					EndpointMetadata: &datastore.EndpointMetadata{
						TargetURL: "http://localhost",
						Status:    datastore.InactiveEndpointStatus,
					},
					AppMetadata: &datastore.AppMetadata{
						UID: appID,
					},
				},
			},
			dbFn: func(ev *datastore.Event, msg *datastore.EventDelivery, app *applicationHandler) {
				m, _ := app.eventDeliveryRepo.(*mocks.MockEventDeliveryRepository)
				m.EXPECT().
					FindEventDeliveryByID(gomock.Any(), gomock.Any()).Times(1).
					Return(msg, nil)

				m.EXPECT().
					UpdateStatusOfEventDelivery(gomock.Any(), gomock.Any(), gomock.Any()).Times(1).
					Return(nil)

				a, _ := app.appRepo.(*mocks.MockApplicationRepository)
				a.EXPECT().
					FindApplicationEndpointByID(gomock.Any(), gomock.Any(), gomock.Any()).Times(1).
					Return(
						&datastore.Endpoint{
							Status: datastore.InactiveEndpointStatus,
						},
						nil,
					)

				a.EXPECT().
					UpdateApplicationEndpointsStatus(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Times(1).
					Return(nil)

				q, _ := app.eventQueue.(*mocks.MockQueuer)
				q.EXPECT().
					WriteEvent(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Times(1).
					Return(nil)
				q.EXPECT().
					WriteEventDelivery(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Times(1).
					Return(nil)

				o, _ := app.groupRepo.(*mocks.MockGroupRepository)

				o.EXPECT().
					LoadGroups(gomock.Any(), gomock.Any()).Times(1).
					Return([]*datastore.Group{group}, nil)
			},
		},
		{
			name:       "valid resend - previously failed - active endpoint",
			cfgPath:    "./testdata/Auth_Config/no-auth-convoy.json",
			method:     http.MethodPut,
			statusCode: http.StatusOK,
			body:       nil,
			args: args{
				event: &datastore.Event{
					UID: eventID,
				},
				message: &datastore.EventDelivery{
					UID: eventDeliveryID,
					EventMetadata: &datastore.EventMetadata{
						UID: eventID,
					},
					Status: datastore.FailureEventStatus,
					EndpointMetadata: &datastore.EndpointMetadata{
						TargetURL: "http://localhost",
						Status:    datastore.ActiveEndpointStatus,
					},
					AppMetadata: &datastore.AppMetadata{
						UID: appID,
					},
				},
			},
			dbFn: func(ev *datastore.Event, msg *datastore.EventDelivery, app *applicationHandler) {
				m, _ := app.eventDeliveryRepo.(*mocks.MockEventDeliveryRepository)
				m.EXPECT().
					FindEventDeliveryByID(gomock.Any(), gomock.Any()).Times(1).
					Return(msg, nil)

				m.EXPECT().
					UpdateStatusOfEventDelivery(gomock.Any(), gomock.Any(), gomock.Any()).Times(1).
					Return(nil)

				a, _ := app.appRepo.(*mocks.MockApplicationRepository)
				a.EXPECT().
					FindApplicationEndpointByID(gomock.Any(), gomock.Any(), gomock.Any()).Times(1).
					Return(
						&datastore.Endpoint{
							TargetURL: "http://localhost",
							Status:    datastore.ActiveEndpointStatus,
						},
						nil,
					)

				q, _ := app.eventQueue.(*mocks.MockQueuer)
				q.EXPECT().
					WriteEvent(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Times(1).
					Return(nil)
				q.EXPECT().
					WriteEventDelivery(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Times(1).
					Return(nil)
				o, _ := app.groupRepo.(*mocks.MockGroupRepository)

				o.EXPECT().
					LoadGroups(gomock.Any(), gomock.Any()).Times(1).
					Return([]*datastore.Group{group}, nil)
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			url := fmt.Sprintf("/api/v1/eventdeliveries/%s/resend", tc.args.message.UID)
			req := httptest.NewRequest(tc.method, url, nil)
			req.SetBasicAuth("test", "test")
			req.Header.Add("Content-Type", "application/json")

			w := httptest.NewRecorder()
			rctx := chi.NewRouteContext()
			rctx.URLParams.Add("appID", tc.args.message.AppMetadata.UID)

			if tc.dbFn != nil {
				tc.dbFn(tc.args.event, tc.args.message, app)
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
				log.Error(tc.args.message, w.Body)
				t.Errorf("Want status '%d', got '%d'", tc.statusCode, w.Code)
			}

			verifyMatch(t, *w)
		})
	}
}

func TestApplicationHandler_BatchRetryEventDelivery(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	app := provideApplication(ctrl)

	group := &datastore.Group{Name: "default-group", UID: "1234567890"}

	type args struct {
		event   *datastore.Event
		message []datastore.EventDelivery
	}
	tests := []struct {
		name       string
		cfgPath    string
		method     string
		urlQuery   string
		statusCode int
		args       args
		body       *strings.Reader
		dbFn       func(*http.Request, *datastore.Event, []datastore.EventDelivery, *applicationHandler)
	}{
		{
			name:       "should_batch_retry_all_successfully",
			cfgPath:    "./testdata/Auth_Config/no-auth-convoy.json",
			method:     http.MethodPost,
			urlQuery:   "?appId=1234&eventId=abc&status=Scheduled&status=Discarded",
			statusCode: http.StatusOK,
			args: args{
				event: &datastore.Event{
					UID: "1111",
				},
				message: []datastore.EventDelivery{
					{
						UID:    "123",
						Status: datastore.FailureEventStatus,
						EventMetadata: &datastore.EventMetadata{
							UID: "abcd",
						},
						EndpointMetadata: &datastore.EndpointMetadata{
							UID: "1234",
						},
						AppMetadata: &datastore.AppMetadata{
							UID: "123",
						},
					},
				},
			},
			body: strings.NewReader(`{"ids":["1234","12345"]}`),
			dbFn: func(r *http.Request, ev *datastore.Event, msg []datastore.EventDelivery, app *applicationHandler) {
				ctx := r.Context()

				ctx = setGroupInContext(ctx, group)
				*r = *r.WithContext(ctx)

				e, _ := app.eventDeliveryRepo.(*mocks.MockEventDeliveryRepository)
				e.EXPECT().
					LoadEventDeliveriesPaged(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Times(1).
					Return(msg, datastore.PaginationData{
						Total:     5,
						Page:      1,
						PerPage:   10,
						Prev:      0,
						Next:      0,
						TotalPage: 1,
					}, nil)

				e.EXPECT().
					UpdateStatusOfEventDelivery(gomock.Any(), gomock.Any(), gomock.Any()).Times(1).
					Return(nil)

				a, _ := app.appRepo.(*mocks.MockApplicationRepository)
				a.EXPECT().
					FindApplicationEndpointByID(gomock.Any(), gomock.Any(), gomock.Any()).Times(1).
					Return(
						&datastore.Endpoint{
							Status: datastore.InactiveEndpointStatus,
						},
						nil,
					)

				a.EXPECT().
					UpdateApplicationEndpointsStatus(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Times(1).
					Return(nil)

				o, _ := app.groupRepo.(*mocks.MockGroupRepository)
				o.EXPECT().
					LoadGroups(gomock.Any(), gomock.Any()).Times(1).
					Return([]*datastore.Group{group}, nil)

				q, _ := app.eventQueue.(*mocks.MockQueuer)
				q.EXPECT().
					WriteEvent(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Times(1).
					Return(nil)
				q.EXPECT().
					WriteEventDelivery(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Times(1).
					Return(nil)
			},
		},
		{
			name:       "should_batch_retry_one_successfully",
			cfgPath:    "./testdata/Auth_Config/no-auth-convoy.json",
			urlQuery:   "?appId=1234&eventId=abc&status=Scheduled&status=Discarded",
			method:     http.MethodPost,
			statusCode: http.StatusOK,
			args: args{
				event: &datastore.Event{
					UID: "1111",
				},
				message: []datastore.EventDelivery{
					{
						UID:    "123",
						Status: datastore.FailureEventStatus,
						EventMetadata: &datastore.EventMetadata{
							UID: "abcd",
						},
						EndpointMetadata: &datastore.EndpointMetadata{
							UID: "1234",
						},
						AppMetadata: &datastore.AppMetadata{
							UID: "123",
						},
					},
					{
						UID:    "123",
						Status: datastore.SuccessEventStatus,
						EventMetadata: &datastore.EventMetadata{
							UID: "abcd",
						},
						EndpointMetadata: &datastore.EndpointMetadata{
							UID: "1234",
						},
						AppMetadata: &datastore.AppMetadata{
							UID: "123",
						},
					},
				},
			},
			body: strings.NewReader(`{"ids":["1234","12345"]}`),
			dbFn: func(r *http.Request, ev *datastore.Event, msg []datastore.EventDelivery, app *applicationHandler) {
				ctx := r.Context()

				ctx = setGroupInContext(ctx, group)
				*r = *r.WithContext(ctx)

				e, _ := app.eventDeliveryRepo.(*mocks.MockEventDeliveryRepository)
				e.EXPECT().
					LoadEventDeliveriesPaged(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Times(1).
					Return(msg, datastore.PaginationData{
						Total:     5,
						Page:      1,
						PerPage:   10,
						Prev:      0,
						Next:      0,
						TotalPage: 1,
					}, nil)

				e.EXPECT().
					UpdateStatusOfEventDelivery(gomock.Any(), gomock.Any(), gomock.Any()).Times(1).
					Return(nil)

				a, _ := app.appRepo.(*mocks.MockApplicationRepository)
				a.EXPECT().
					FindApplicationEndpointByID(gomock.Any(), gomock.Any(), gomock.Any()).Times(1).
					Return(
						&datastore.Endpoint{
							Status: datastore.InactiveEndpointStatus,
						},
						nil,
					)

				a.EXPECT().
					UpdateApplicationEndpointsStatus(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Times(1).
					Return(nil)

				o, _ := app.groupRepo.(*mocks.MockGroupRepository)
				o.EXPECT().
					LoadGroups(gomock.Any(), gomock.Any()).Times(1).
					Return([]*datastore.Group{group}, nil)

				q, _ := app.eventQueue.(*mocks.MockQueuer)
				q.EXPECT().
					WriteEvent(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Times(1).
					Return(nil)
				q.EXPECT().
					WriteEventDelivery(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Times(1).
					Return(nil)
			},
		},
		{
			name:       "should_fail_to_write_to_queue",
			cfgPath:    "./testdata/Auth_Config/no-auth-convoy.json",
			urlQuery:   "?appId=1234&eventId=abc&status=Scheduled&status=Discarded",
			method:     http.MethodPost,
			statusCode: http.StatusOK,
			args: args{
				event: &datastore.Event{
					UID: "1111",
				},
				message: []datastore.EventDelivery{
					{
						UID:    "123",
						Status: datastore.FailureEventStatus,
						EventMetadata: &datastore.EventMetadata{
							UID: "abcd",
						},
						EndpointMetadata: &datastore.EndpointMetadata{
							UID: "1234",
						},
						AppMetadata: &datastore.AppMetadata{
							UID: "123",
						},
					},
				},
			},
			body: strings.NewReader(`{"ids":["1234","12345"]}`),
			dbFn: func(r *http.Request, ev *datastore.Event, msg []datastore.EventDelivery, app *applicationHandler) {
				ctx := r.Context()

				ctx = setGroupInContext(ctx, group)
				*r = *r.WithContext(ctx)

				e, _ := app.eventDeliveryRepo.(*mocks.MockEventDeliveryRepository)
				e.EXPECT().
					LoadEventDeliveriesPaged(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Times(1).
					Return(msg, datastore.PaginationData{
						Total:     5,
						Page:      1,
						PerPage:   10,
						Prev:      0,
						Next:      0,
						TotalPage: 1,
					}, nil)

				e.EXPECT().
					UpdateStatusOfEventDelivery(gomock.Any(), gomock.Any(), gomock.Any()).Times(1).
					Return(nil)

				a, _ := app.appRepo.(*mocks.MockApplicationRepository)
				a.EXPECT().
					FindApplicationEndpointByID(gomock.Any(), gomock.Any(), gomock.Any()).Times(1).
					Return(
						&datastore.Endpoint{
							Status: datastore.InactiveEndpointStatus,
						},
						nil,
					)

				a.EXPECT().
					UpdateApplicationEndpointsStatus(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Times(1).
					Return(nil)

				o, _ := app.groupRepo.(*mocks.MockGroupRepository)
				o.EXPECT().
					LoadGroups(gomock.Any(), gomock.Any()).Times(1).
					Return([]*datastore.Group{group}, nil)

				q, _ := app.eventQueue.(*mocks.MockQueuer)
				q.EXPECT().
					WriteEvent(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Times(1).
					Return(errors.New("failed to write to queue"))
				q.EXPECT().
					WriteEventDelivery(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Times(1).
					Return(nil)
			},
		},
		{
			name:       "should_fail_to_update_status_of_event_delivery",
			cfgPath:    "./testdata/Auth_Config/no-auth-convoy.json",
			urlQuery:   "?appId=1234&eventId=abc&status=Scheduled&status=Discarded",
			method:     http.MethodPost,
			statusCode: http.StatusOK,
			args: args{
				event: &datastore.Event{
					UID: "1111",
				},
				message: []datastore.EventDelivery{
					{
						UID:    "123",
						Status: datastore.FailureEventStatus,
						EventMetadata: &datastore.EventMetadata{
							UID: "abcd",
						},
						EndpointMetadata: &datastore.EndpointMetadata{
							UID: "1234",
						},
						AppMetadata: &datastore.AppMetadata{
							UID: "123",
						},
					},
				},
			},
			body: strings.NewReader(`{"ids":["1234","12345"]}`),
			dbFn: func(r *http.Request, ev *datastore.Event, msg []datastore.EventDelivery, app *applicationHandler) {
				ctx := r.Context()

				ctx = setGroupInContext(ctx, group)
				*r = *r.WithContext(ctx)

				e, _ := app.eventDeliveryRepo.(*mocks.MockEventDeliveryRepository)
				e.EXPECT().
					LoadEventDeliveriesPaged(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Times(1).
					Return(msg, datastore.PaginationData{
						Total:     5,
						Page:      1,
						PerPage:   10,
						Prev:      0,
						Next:      0,
						TotalPage: 1,
					}, nil)

				e.EXPECT().
					UpdateStatusOfEventDelivery(gomock.Any(), gomock.Any(), gomock.Any()).Times(1).
					Return(errors.New("failed to update status of event delivery"))

				a, _ := app.appRepo.(*mocks.MockApplicationRepository)
				a.EXPECT().
					FindApplicationEndpointByID(gomock.Any(), gomock.Any(), gomock.Any()).Times(1).
					Return(
						&datastore.Endpoint{
							Status: datastore.InactiveEndpointStatus,
						},
						nil,
					)

				a.EXPECT().
					UpdateApplicationEndpointsStatus(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Times(1).
					Return(nil)

				o, _ := app.groupRepo.(*mocks.MockGroupRepository)
				o.EXPECT().
					LoadGroups(gomock.Any(), gomock.Any()).Times(1).
					Return([]*datastore.Group{group}, nil)
			},
		},
		{
			name:       "should_fail_to_update_app_endpoints_status",
			cfgPath:    "./testdata/Auth_Config/no-auth-convoy.json",
			urlQuery:   "?appId=1234&eventId=abc&status=Scheduled&status=Discarded",
			method:     http.MethodPost,
			statusCode: http.StatusOK,
			args: args{
				event: &datastore.Event{
					UID: "1111",
				},
				message: []datastore.EventDelivery{
					{
						UID:    "123",
						Status: datastore.FailureEventStatus,
						EventMetadata: &datastore.EventMetadata{
							UID: "abcd",
						},
						EndpointMetadata: &datastore.EndpointMetadata{
							UID: "1234",
						},
						AppMetadata: &datastore.AppMetadata{
							UID: "123",
						},
					},
				},
			},
			body: strings.NewReader(`{"ids":["12345"]}`),
			dbFn: func(r *http.Request, ev *datastore.Event, msg []datastore.EventDelivery, app *applicationHandler) {
				ctx := r.Context()

				ctx = setGroupInContext(ctx, group)
				*r = *r.WithContext(ctx)

				e, _ := app.eventDeliveryRepo.(*mocks.MockEventDeliveryRepository)
				e.EXPECT().
					LoadEventDeliveriesPaged(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Times(1).
					Return(msg, datastore.PaginationData{
						Total:     5,
						Page:      1,
						PerPage:   10,
						Prev:      0,
						Next:      0,
						TotalPage: 1,
					}, nil)

				a, _ := app.appRepo.(*mocks.MockApplicationRepository)
				a.EXPECT().
					FindApplicationEndpointByID(gomock.Any(), gomock.Any(), gomock.Any()).Times(1).
					Return(
						&datastore.Endpoint{
							Status: datastore.InactiveEndpointStatus,
						},
						nil,
					)

				a.EXPECT().
					UpdateApplicationEndpointsStatus(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Times(1).
					Return(errors.New("failed to update endpoint status"))

				o, _ := app.groupRepo.(*mocks.MockGroupRepository)
				o.EXPECT().
					LoadGroups(gomock.Any(), gomock.Any()).Times(1).
					Return([]*datastore.Group{group}, nil)
			},
		},
		{
			name:       "should_fail_to_find_app_endpoint",
			cfgPath:    "./testdata/Auth_Config/no-auth-convoy.json",
			urlQuery:   "?appId=1234&eventId=abc&status=Scheduled&status=Discarded",
			method:     http.MethodPost,
			statusCode: http.StatusOK,
			args: args{
				event: &datastore.Event{
					UID: "1111",
				},
				message: []datastore.EventDelivery{
					{
						UID:    "123",
						Status: datastore.FailureEventStatus,
						EventMetadata: &datastore.EventMetadata{
							UID: "abcd",
						},
						EndpointMetadata: &datastore.EndpointMetadata{
							UID: "1234",
						},
						AppMetadata: &datastore.AppMetadata{
							UID: "123",
						},
					},
					{
						UID:    "123",
						Status: datastore.SuccessEventStatus,
						EventMetadata: &datastore.EventMetadata{
							UID: "abcd",
						},
						EndpointMetadata: &datastore.EndpointMetadata{
							UID: "1234",
						},
						AppMetadata: &datastore.AppMetadata{
							UID: "123",
						},
					},
				},
			},
			body: strings.NewReader(`{"ids":["1234","12345"]}`),
			dbFn: func(r *http.Request, ev *datastore.Event, msg []datastore.EventDelivery, app *applicationHandler) {
				ctx := r.Context()

				ctx = setGroupInContext(ctx, group)
				*r = *r.WithContext(ctx)

				e, _ := app.eventDeliveryRepo.(*mocks.MockEventDeliveryRepository)
				e.EXPECT().
					LoadEventDeliveriesPaged(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Times(1).
					Return(msg, datastore.PaginationData{
						Total:     5,
						Page:      1,
						PerPage:   10,
						Prev:      0,
						Next:      0,
						TotalPage: 1,
					}, nil)

				a, _ := app.appRepo.(*mocks.MockApplicationRepository)
				a.EXPECT().
					FindApplicationEndpointByID(gomock.Any(), gomock.Any(), gomock.Any()).Times(1).
					Return(
						nil,
						errors.New("failed to find app endpoint"),
					)

				o, _ := app.groupRepo.(*mocks.MockGroupRepository)
				o.EXPECT().
					LoadGroups(gomock.Any(), gomock.Any()).Times(1).
					Return([]*datastore.Group{group}, nil)
			},
		},
		{
			name:       "should_fail_to_load_event_deliveries",
			cfgPath:    "./testdata/Auth_Config/no-auth-convoy.json",
			urlQuery:   "?appId=1234&eventId=abc&status=Scheduled&status=Discarded",
			method:     http.MethodPost,
			statusCode: http.StatusInternalServerError,
			args: args{
				event: &datastore.Event{
					UID: "1111",
				},
				message: nil,
			},
			body: strings.NewReader(`{"ids":["1234","12345"]}`),
			dbFn: func(r *http.Request, ev *datastore.Event, msg []datastore.EventDelivery, app *applicationHandler) {
				ctx := r.Context()

				ctx = setGroupInContext(ctx, group)
				*r = *r.WithContext(ctx)

				e, _ := app.eventDeliveryRepo.(*mocks.MockEventDeliveryRepository)
				e.EXPECT().
					LoadEventDeliveriesPaged(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Times(1).
					Return(nil, datastore.PaginationData{}, errors.New("failed to load events"))

				o, _ := app.groupRepo.(*mocks.MockGroupRepository)
				o.EXPECT().
					LoadGroups(gomock.Any(), gomock.Any()).Times(1).
					Return([]*datastore.Group{group}, nil)
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			url := "/api/v1/eventdeliveries/batchretry" + tc.urlQuery
			req := httptest.NewRequest(tc.method, url, tc.body)
			req.SetBasicAuth("test", "test")
			req.Header.Add("Content-Type", "application/json")

			w := httptest.NewRecorder()

			if tc.dbFn != nil {
				tc.dbFn(req, tc.args.event, tc.args.message, app)
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

func TestApplicationHandler_CountAffectedEventDeliveries(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	app := provideApplication(ctrl)

	group := &datastore.Group{Name: "default-group", UID: "1234567890"}

	tests := []struct {
		name       string
		cfgPath    string
		method     string
		urlQuery   string
		statusCode int
		body       *strings.Reader
		dbFn       func(*http.Request, *applicationHandler)
	}{
		{
			name:       "should_count_affected_event_deliveries_successfully",
			cfgPath:    "./testdata/Auth_Config/no-auth-convoy.json",
			urlQuery:   "?appId=1234&eventId=abc&status=Scheduled&status=Discarded",
			method:     http.MethodGet,
			statusCode: http.StatusOK,

			body: strings.NewReader(`{"ids":["1234","12345"]}`),
			dbFn: func(r *http.Request, app *applicationHandler) {
				ctx := r.Context()

				ctx = setGroupInContext(ctx, group)
				*r = *r.WithContext(ctx)

				e, _ := app.eventDeliveryRepo.(*mocks.MockEventDeliveryRepository)
				e.EXPECT().
					CountEventDeliveries(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Times(1).
					Return(int64(10), nil)

				o, _ := app.groupRepo.(*mocks.MockGroupRepository)
				o.EXPECT().
					LoadGroups(gomock.Any(), gomock.Any()).Times(1).
					Return([]*datastore.Group{group}, nil)
			},
		},
		{
			name:       "should_fail_to_count_affected_event_deliveries",
			cfgPath:    "./testdata/Auth_Config/no-auth-convoy.json",
			urlQuery:   "?appId=1234&eventId=abc&status=Scheduled&status=Discarded",
			method:     http.MethodGet,
			statusCode: http.StatusInternalServerError,
			body:       strings.NewReader(`{"ids":["1234","12345"]}`),
			dbFn: func(r *http.Request, app *applicationHandler) {
				ctx := r.Context()

				ctx = setGroupInContext(ctx, group)
				*r = *r.WithContext(ctx)

				e, _ := app.eventDeliveryRepo.(*mocks.MockEventDeliveryRepository)
				e.EXPECT().
					CountEventDeliveries(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Times(1).
					Return(int64(0), errors.New("failed to count deliveries"))

				o, _ := app.groupRepo.(*mocks.MockGroupRepository)
				o.EXPECT().
					LoadGroups(gomock.Any(), gomock.Any()).Times(1).
					Return([]*datastore.Group{group}, nil)
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			url := "/api/v1/eventdeliveries/countbatchretryevents" + tc.urlQuery
			req := httptest.NewRequest(tc.method, url, tc.body)
			req.SetBasicAuth("test", "test")
			req.Header.Add("Content-Type", "application/json")

			w := httptest.NewRecorder()

			if tc.dbFn != nil {
				tc.dbFn(req, app)
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

func TestApplicationHandler_ForceResendEventDelivery(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	app := provideApplication(ctrl)
	group := &datastore.Group{Name: "default-group", UID: "1234567890"}

	type args struct {
		event   *datastore.Event
		message []datastore.EventDelivery
	}
	tests := []struct {
		name       string
		cfgPath    string
		method     string
		statusCode int
		args       args
		body       *strings.Reader
		dbFn       func(*datastore.Event, []datastore.EventDelivery, *applicationHandler)
	}{
		{
			name:       "should_force_resend_all_successfully",
			cfgPath:    "./testdata/Auth_Config/no-auth-convoy.json",
			method:     http.MethodPost,
			statusCode: http.StatusOK,
			args: args{
				event: &datastore.Event{
					UID: "1111",
				},
				message: []datastore.EventDelivery{
					{
						UID:    "123",
						Status: datastore.SuccessEventStatus,
						EventMetadata: &datastore.EventMetadata{
							UID: "abcd",
						},
						EndpointMetadata: &datastore.EndpointMetadata{
							UID: "1234",
						},
						AppMetadata: &datastore.AppMetadata{
							UID: "123",
						},
					},
					{
						UID:    "1234",
						Status: datastore.SuccessEventStatus,
						EventMetadata: &datastore.EventMetadata{
							UID: "abcd",
						},
						EndpointMetadata: &datastore.EndpointMetadata{
							UID: "1234",
						},
						AppMetadata: &datastore.AppMetadata{
							UID: "123",
						},
					},

					{
						UID:    "12345",
						Status: datastore.SuccessEventStatus,
						EventMetadata: &datastore.EventMetadata{
							UID: "abcd",
						},
						EndpointMetadata: &datastore.EndpointMetadata{
							UID: "1234",
						},
						AppMetadata: &datastore.AppMetadata{
							UID: "123",
						},
					},
				},
			},
			body: strings.NewReader(`{"ids":["123","1234","12345"]}`),
			dbFn: func(ev *datastore.Event, msg []datastore.EventDelivery, app *applicationHandler) {
				e, _ := app.eventDeliveryRepo.(*mocks.MockEventDeliveryRepository)
				e.EXPECT().
					FindEventDeliveriesByIDs(gomock.Any(), gomock.Any()).Times(1).
					Return(msg, nil)

				e.EXPECT().
					UpdateStatusOfEventDelivery(gomock.Any(), gomock.Any(), gomock.Any()).Times(3).
					Return(nil)

				a, _ := app.appRepo.(*mocks.MockApplicationRepository)
				a.EXPECT().
					FindApplicationEndpointByID(gomock.Any(), gomock.Any(), gomock.Any()).Times(3).
					Return(
						&datastore.Endpoint{
							Status: datastore.ActiveEndpointStatus,
						},
						nil,
					)

				o, _ := app.groupRepo.(*mocks.MockGroupRepository)
				o.EXPECT().
					LoadGroups(gomock.Any(), gomock.Any()).Times(1).
					Return([]*datastore.Group{group}, nil)

				q, _ := app.eventQueue.(*mocks.MockQueuer)
				q.EXPECT().
					WriteEvent(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Times(3).
					Return(nil)
				q.EXPECT().
					WriteEventDelivery(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Times(1).
					Return(nil)
			},
		},
		{
			name:       "should_error_for_discarded_event_status",
			cfgPath:    "./testdata/Auth_Config/no-auth-convoy.json",
			method:     http.MethodPost,
			statusCode: http.StatusOK,
			args: args{
				event: &datastore.Event{
					UID: "1111",
				},
				message: []datastore.EventDelivery{
					{
						UID:    "123",
						Status: datastore.SuccessEventStatus,
						EventMetadata: &datastore.EventMetadata{
							UID: "abcd",
						},
						EndpointMetadata: &datastore.EndpointMetadata{
							UID: "1234",
						},
						AppMetadata: &datastore.AppMetadata{
							UID: "123",
						},
					},
					{
						UID:    "1234",
						Status: datastore.DiscardedEventStatus,
						EventMetadata: &datastore.EventMetadata{
							UID: "abcd",
						},
						EndpointMetadata: &datastore.EndpointMetadata{
							UID: "1234",
						},
						AppMetadata: &datastore.AppMetadata{
							UID: "123",
						},
					},

					{
						UID:    "12345",
						Status: datastore.SuccessEventStatus,
						EventMetadata: &datastore.EventMetadata{
							UID: "abcd",
						},
						EndpointMetadata: &datastore.EndpointMetadata{
							UID: "1234",
						},
						AppMetadata: &datastore.AppMetadata{
							UID: "123",
						},
					},
				},
			},
			body: strings.NewReader(`{"ids":["123","1234","12345"]}`),
			dbFn: func(ev *datastore.Event, msg []datastore.EventDelivery, app *applicationHandler) {
				e, _ := app.eventDeliveryRepo.(*mocks.MockEventDeliveryRepository)
				e.EXPECT().
					FindEventDeliveriesByIDs(gomock.Any(), gomock.Any()).Times(1).
					Return(msg, nil)

				e.EXPECT().
					UpdateStatusOfEventDelivery(gomock.Any(), gomock.Any(), gomock.Any()).Times(2).
					Return(nil)

				a, _ := app.appRepo.(*mocks.MockApplicationRepository)
				a.EXPECT().
					FindApplicationEndpointByID(gomock.Any(), gomock.Any(), gomock.Any()).Times(2).
					Return(
						&datastore.Endpoint{
							Status: datastore.ActiveEndpointStatus,
						},
						nil,
					)

				o, _ := app.groupRepo.(*mocks.MockGroupRepository)
				o.EXPECT().
					LoadGroups(gomock.Any(), gomock.Any()).Times(1).
					Return([]*datastore.Group{group}, nil)

				q, _ := app.eventQueue.(*mocks.MockQueuer)
				q.EXPECT().
					WriteEvent(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Times(2).
					Return(nil)
				q.EXPECT().
					WriteEventDelivery(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Times(1).
					Return(nil)
			},
		},
		{
			name:       "should_error_for_endpoint_not_found",
			cfgPath:    "./testdata/Auth_Config/no-auth-convoy.json",
			method:     http.MethodPost,
			statusCode: http.StatusOK,
			args: args{
				event: &datastore.Event{
					UID: "1111",
				},
				message: []datastore.EventDelivery{
					{
						UID:    "123",
						Status: datastore.SuccessEventStatus,
						EventMetadata: &datastore.EventMetadata{
							UID: "abcd",
						},
						EndpointMetadata: &datastore.EndpointMetadata{
							UID: "1234",
						},
						AppMetadata: &datastore.AppMetadata{
							UID: "123",
						},
					},
					{
						UID:    "1234",
						Status: datastore.SuccessEventStatus,
						EventMetadata: &datastore.EventMetadata{
							UID: "abcd",
						},
						EndpointMetadata: &datastore.EndpointMetadata{
							UID: "1234",
						},
						AppMetadata: &datastore.AppMetadata{
							UID: "123",
						},
					},

					{
						UID:    "12345",
						Status: datastore.SuccessEventStatus,
						EventMetadata: &datastore.EventMetadata{
							UID: "abcd",
						},
						EndpointMetadata: &datastore.EndpointMetadata{
							UID: "1234",
						},
						AppMetadata: &datastore.AppMetadata{
							UID: "123",
						},
					},
				},
			},
			body: strings.NewReader(`{"ids":["123","1234","12345"]}`),
			dbFn: func(ev *datastore.Event, msg []datastore.EventDelivery, app *applicationHandler) {
				e, _ := app.eventDeliveryRepo.(*mocks.MockEventDeliveryRepository)
				e.EXPECT().
					FindEventDeliveriesByIDs(gomock.Any(), gomock.Any()).Times(1).
					Return(msg, nil)

				a, _ := app.appRepo.(*mocks.MockApplicationRepository)
				a.EXPECT().
					FindApplicationEndpointByID(gomock.Any(), gomock.Any(), gomock.Any()).Times(3).
					Return(
						nil,
						errors.New("cannot find endpoint"),
					)

				o, _ := app.groupRepo.(*mocks.MockGroupRepository)
				o.EXPECT().
					LoadGroups(gomock.Any(), gomock.Any()).Times(1).
					Return([]*datastore.Group{group}, nil)
			},
		},
		{
			name:       "should_error_for_inactive_endpoint",
			cfgPath:    "./testdata/Auth_Config/no-auth-convoy.json",
			method:     http.MethodPost,
			statusCode: http.StatusOK,
			args: args{
				event: &datastore.Event{
					UID: "1111",
				},
				message: []datastore.EventDelivery{
					{
						UID:    "1234",
						Status: datastore.SuccessEventStatus,
						EventMetadata: &datastore.EventMetadata{
							UID: "abcd",
						},
						EndpointMetadata: &datastore.EndpointMetadata{
							UID: "1234",
						},
						AppMetadata: &datastore.AppMetadata{
							UID: "123",
						},
					},
					{
						UID:    "12345",
						Status: datastore.SuccessEventStatus,
						EventMetadata: &datastore.EventMetadata{
							UID: "abcd",
						},
						EndpointMetadata: &datastore.EndpointMetadata{
							UID: "1234",
						},
						AppMetadata: &datastore.AppMetadata{
							UID: "123",
						},
					},
				},
			},
			body: strings.NewReader(`{"ids":["123","1234","12345"]}`),
			dbFn: func(ev *datastore.Event, msg []datastore.EventDelivery, app *applicationHandler) {
				e, _ := app.eventDeliveryRepo.(*mocks.MockEventDeliveryRepository)
				e.EXPECT().
					FindEventDeliveriesByIDs(gomock.Any(), gomock.Any()).Times(1).
					Return(msg, nil)

				a, _ := app.appRepo.(*mocks.MockApplicationRepository)
				a.EXPECT().
					FindApplicationEndpointByID(gomock.Any(), gomock.Any(), gomock.Any()).Times(2).
					Return(
						&datastore.Endpoint{Status: datastore.InactiveEndpointStatus},
						nil,
					)

				o, _ := app.groupRepo.(*mocks.MockGroupRepository)
				o.EXPECT().
					LoadGroups(gomock.Any(), gomock.Any()).Times(1).
					Return([]*datastore.Group{group}, nil)
			},
		},
		{
			name:       "should_error_for_failed_to_update_event_delivery_status",
			cfgPath:    "./testdata/Auth_Config/no-auth-convoy.json",
			method:     http.MethodPost,
			statusCode: http.StatusOK,
			args: args{
				event: &datastore.Event{
					UID: "1111",
				},
				message: []datastore.EventDelivery{
					{
						UID:    "1234",
						Status: datastore.SuccessEventStatus,
						EventMetadata: &datastore.EventMetadata{
							UID: "abcd",
						},
						EndpointMetadata: &datastore.EndpointMetadata{
							UID: "1234",
						},
						AppMetadata: &datastore.AppMetadata{
							UID: "123",
						},
					},
					{
						UID:    "12345",
						Status: datastore.SuccessEventStatus,
						EventMetadata: &datastore.EventMetadata{
							UID: "abcd",
						},
						EndpointMetadata: &datastore.EndpointMetadata{
							UID: "1234",
						},
						AppMetadata: &datastore.AppMetadata{
							UID: "123",
						},
					},
				},
			},
			body: strings.NewReader(`{"ids":["123","1234","12345"]}`),
			dbFn: func(ev *datastore.Event, msg []datastore.EventDelivery, app *applicationHandler) {
				e, _ := app.eventDeliveryRepo.(*mocks.MockEventDeliveryRepository)
				e.EXPECT().
					FindEventDeliveriesByIDs(gomock.Any(), gomock.Any()).Times(1).
					Return(msg, nil)

				e.EXPECT().
					UpdateStatusOfEventDelivery(gomock.Any(), gomock.Any(), gomock.Any()).Times(2).
					Return(errors.New("update failed"))

				a, _ := app.appRepo.(*mocks.MockApplicationRepository)
				a.EXPECT().
					FindApplicationEndpointByID(gomock.Any(), gomock.Any(), gomock.Any()).Times(2).
					Return(
						&datastore.Endpoint{Status: datastore.ActiveEndpointStatus},
						nil,
					)

				o, _ := app.groupRepo.(*mocks.MockGroupRepository)
				o.EXPECT().
					LoadGroups(gomock.Any(), gomock.Any()).Times(1).
					Return([]*datastore.Group{group}, nil)
			},
		},
		{
			name:       "should_error_for_failed_to_fetch_deliveries",
			cfgPath:    "./testdata/Auth_Config/no-auth-convoy.json",
			method:     http.MethodPost,
			statusCode: http.StatusInternalServerError,
			args: args{
				event: &datastore.Event{
					UID: "1111",
				},
				message: []datastore.EventDelivery{},
			},
			body: strings.NewReader(`{"ids":["1234","12345"]}`),
			dbFn: func(ev *datastore.Event, msg []datastore.EventDelivery, app *applicationHandler) {
				e, _ := app.eventDeliveryRepo.(*mocks.MockEventDeliveryRepository)
				e.EXPECT().
					FindEventDeliveriesByIDs(gomock.Any(), gomock.Any()).Times(1).
					Return(nil, errors.New("failed to find event deliveries"))

				o, _ := app.groupRepo.(*mocks.MockGroupRepository)
				o.EXPECT().
					LoadGroups(gomock.Any(), gomock.Any()).Times(1).
					Return([]*datastore.Group{group}, nil)

			},
		},
		{
			name:       "should_error_for_malformed_body",
			cfgPath:    "./testdata/Auth_Config/no-auth-convoy.json",
			method:     http.MethodPost,
			statusCode: http.StatusBadRequest,
			body:       strings.NewReader(`{"ids":"12345"}`),
			dbFn: func(ev *datastore.Event, msg []datastore.EventDelivery, app *applicationHandler) {
				o, _ := app.groupRepo.(*mocks.MockGroupRepository)
				o.EXPECT().
					LoadGroups(gomock.Any(), gomock.Any()).Times(1).
					Return([]*datastore.Group{group}, nil)
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			url := "/api/v1/eventdeliveries/forceresend"
			req := httptest.NewRequest(tc.method, url, tc.body)
			req.SetBasicAuth("test", "test")
			req.Header.Add("Content-Type", "application/json")

			w := httptest.NewRecorder()

			if tc.dbFn != nil {
				tc.dbFn(tc.args.event, tc.args.message, app)
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
			fmt.Println("resp", w.Body.String())
			verifyMatch(t, *w)
		})
	}
}
