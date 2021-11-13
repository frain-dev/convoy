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
	log "github.com/sirupsen/logrus"
)

func TestApplicationHandler_CreateAppEvent(t *testing.T) {

	var app *applicationHandler

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	groupRepo := mocks.NewMockGroupRepository(ctrl)
	appRepo := mocks.NewMockApplicationRepository(ctrl)
	eventRepo := mocks.NewMockEventRepository(ctrl)
	eventDeliveryRepo := mocks.NewMockEventDeliveryRepository(ctrl)
	eventQueue := mocks.NewMockQueuer(ctrl)

	app = newApplicationHandler(eventRepo, eventDeliveryRepo, appRepo, groupRepo, eventQueue)

	groupId := "1234567890"
	appId := "12345"
	msgId := "1122333444456"

	message := &convoy.Event{
		UID: msgId,
		AppMetadata: &convoy.AppMetadata{
			UID: appId,
		},
	}

	type args struct {
		message *convoy.Event
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
			cfgPath:    "./testdata/Auth_Config/full-convoy.json",
			method:     http.MethodPost,
			statusCode: http.StatusBadRequest,
			body:       strings.NewReader(`{"data": {}`),
			args: args{
				message: message,
			},
			dbFn: func(app *applicationHandler) {},
		},
		{
			name:       "invalid message - no app_id",
			cfgPath:    "./testdata/Auth_Config/full-convoy.json",
			method:     http.MethodPost,
			statusCode: http.StatusBadRequest,
			body:       strings.NewReader(`{ "event_type: "test", "data": {}}`),
			args: args{
				message: message,
			},
			dbFn: func(app *applicationHandler) {},
		},
		{
			name:       "invalid message - no data field",
			cfgPath:    "./testdata/Auth_Config/full-convoy.json",
			method:     http.MethodPost,
			statusCode: http.StatusBadRequest,
			body:       strings.NewReader(`{ "app_id": "", "event_type: "test" }`),
			args: args{
				message: message,
			},
			dbFn: func(app *applicationHandler) {},
		},
		{
			name:       "invalid message - no event type",
			cfgPath:    "./testdata/Auth_Config/full-convoy.json",
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

			},
		},
		{
			name:       "valid message - no endpoints",
			cfgPath:    "./testdata/Auth_Config/full-convoy.json",
			method:     http.MethodPost,
			statusCode: http.StatusBadRequest,
			body:       strings.NewReader(`{"app_id": "12345", "event_type": "test",  "data": {}}`),
			args: args{
				message: message,
			},
			dbFn: func(app *applicationHandler) {
				a, _ := app.appRepo.(*mocks.MockApplicationRepository)
				a.EXPECT().
					FindApplicationByID(gomock.Any(), gomock.Any()).Times(1).
					Return(&convoy.Application{
						UID:       appId,
						GroupID:   groupId,
						Title:     "Valid application",
						Endpoints: []convoy.Endpoint{},
					}, nil)
			},
		},
		{
			name:       "valid message - no active endpoints",
			cfgPath:    "./testdata/Auth_Config/full-convoy.json",
			method:     http.MethodPost,
			statusCode: http.StatusCreated,
			body:       strings.NewReader(`{"app_id": "12345", "event_type": "test",  "data": {}}`),
			args: args{
				message: message,
			},
			dbFn: func(app *applicationHandler) {
				a, _ := app.appRepo.(*mocks.MockApplicationRepository)
				a.EXPECT().
					FindApplicationByID(gomock.Any(), gomock.Any()).Times(1).
					Return(&convoy.Application{
						UID:     appId,
						GroupID: groupId,
						Title:   "Valid application",
						Endpoints: []convoy.Endpoint{
							{
								TargetURL: "http://localhost",
								Status:    convoy.InactiveEndpointStatus,
							},
						},
					}, nil)

				o, _ := app.eventRepo.(*mocks.MockEventRepository)
				o.EXPECT().
					CreateEvent(gomock.Any(), gomock.Any()).Times(1).
					Return(nil)
			},
		},
		{
			name:       "valid message - no matching endpoints",
			cfgPath:    "./testdata/Auth_Config/full-convoy.json",
			method:     http.MethodPost,
			statusCode: http.StatusCreated,
			body:       strings.NewReader(`{"app_id": "12345", "event_type": "test.event", "data": { "Hello": "World", "Test": "Data" }}`),
			args: args{
				message: message,
			},
			dbFn: func(app *applicationHandler) {
				a, _ := app.appRepo.(*mocks.MockApplicationRepository)
				a.EXPECT().
					FindApplicationByID(gomock.Any(), gomock.Any()).Times(1).
					Return(&convoy.Application{
						UID:     appId,
						GroupID: groupId,
						Title:   "Valid application",
						Endpoints: []convoy.Endpoint{
							{
								TargetURL: "http://localhost",
								Status:    convoy.ActiveEndpointStatus,
							},
						},
					}, nil)

				m, _ := app.eventRepo.(*mocks.MockEventRepository)
				m.EXPECT().
					CreateEvent(gomock.Any(), gomock.Any()).Times(1).
					Return(nil)

			},
		},
		{
			name:       "valid message - matching endpoints",
			cfgPath:    "./testdata/Auth_Config/full-convoy.json",
			method:     http.MethodPost,
			statusCode: http.StatusCreated,
			body:       strings.NewReader(`{"app_id": "12345", "event_type": "test.event", "data": { "Hello": "World", "Test": "Data" }}`),
			args: args{
				message: message,
			},
			dbFn: func(app *applicationHandler) {
				a, _ := app.appRepo.(*mocks.MockApplicationRepository)
				a.EXPECT().
					FindApplicationByID(gomock.Any(), gomock.Any()).Times(1).
					Return(&convoy.Application{
						UID:     appId,
						GroupID: groupId,
						Title:   "Valid application",
						Endpoints: []convoy.Endpoint{
							{
								TargetURL: "http://localhost",
								Status:    convoy.ActiveEndpointStatus,
								Events:    []string{"test.event"},
							},
						},
					}, nil)

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
					Write(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Times(1).
					Return(nil)

			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {

			err := config.LoadConfig(tc.cfgPath)
			if err != nil {
				t.Error("Failed to load config file")
			}

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

	groupRepo := mocks.NewMockGroupRepository(ctrl)
	appRepo := mocks.NewMockApplicationRepository(ctrl)
	eventRepo := mocks.NewMockEventRepository(ctrl)
	eventDeliveryRepo := mocks.NewMockEventDeliveryRepository(ctrl)
	eventQueue := mocks.NewMockQueuer(ctrl)

	app = newApplicationHandler(eventRepo, eventDeliveryRepo, appRepo, groupRepo, eventQueue)

	appID := "12345"
	eventID := "1122333444456"
	eventDeliveryID := "2134453454"

	type args struct {
		event   *convoy.Event
		message *convoy.EventDelivery
	}

	tests := []struct {
		name       string
		cfgPath    string
		method     string
		statusCode int
		args       args
		body       *strings.Reader
		dbFn       func(*convoy.Event, *convoy.EventDelivery, *applicationHandler)
	}{
		{
			name:       "invalid resend - event successful",
			cfgPath:    "./testdata/Auth_Config/full-convoy.json",
			method:     http.MethodPut,
			statusCode: http.StatusBadRequest,
			body:       nil,
			args: args{
				event: &convoy.Event{
					UID: eventID,
				},
				message: &convoy.EventDelivery{
					UID: eventDeliveryID,
					EventMetadata: &convoy.EventMetadata{
						UID: eventID,
					},
					Status: convoy.SuccessEventStatus,
					AppMetadata: &convoy.AppMetadata{
						UID: appID,
					},
				},
			},
			dbFn: func(ev *convoy.Event, msg *convoy.EventDelivery, app *applicationHandler) {

				m, _ := app.eventDeliveryRepo.(*mocks.MockEventDeliveryRepository)
				m.EXPECT().
					FindEventDeliveryByID(gomock.Any(), gomock.Any()).Times(1).
					Return(msg, nil)

				m.EXPECT().
					UpdateStatusOfEventDelivery(gomock.Any(), gomock.Any(), gomock.Any()).Times(0).
					Return(nil)

			},
		},
		{
			name:       "invalid resend - event not failed",
			cfgPath:    "./testdata/Auth_Config/full-convoy.json",
			method:     http.MethodPut,
			statusCode: http.StatusBadRequest,
			body:       nil,
			args: args{
				event: &convoy.Event{
					UID: eventID,
				},
				message: &convoy.EventDelivery{
					UID: eventDeliveryID,
					EventMetadata: &convoy.EventMetadata{
						UID: eventID,
					},
					Status: convoy.ProcessingEventStatus,
					AppMetadata: &convoy.AppMetadata{
						UID: appID,
					},
				},
			},
			dbFn: func(ev *convoy.Event, msg *convoy.EventDelivery, app *applicationHandler) {

				m, _ := app.eventDeliveryRepo.(*mocks.MockEventDeliveryRepository)
				m.EXPECT().
					FindEventDeliveryByID(gomock.Any(), gomock.Any()).Times(1).
					Return(msg, nil)
			},
		},
		{
			name:       "invalid  resend - pending endpoint",
			cfgPath:    "./testdata/Auth_Config/full-convoy.json",
			method:     http.MethodPut,
			statusCode: http.StatusBadRequest,
			body:       nil,
			args: args{
				event: &convoy.Event{
					UID: eventID,
				},
				message: &convoy.EventDelivery{
					UID: eventDeliveryID,
					EventMetadata: &convoy.EventMetadata{
						UID: eventID,
					},
					Status: convoy.FailureEventStatus,
					EndpointMetadata: &convoy.EndpointMetadata{
						TargetURL: "http://localhost",
						Status:    convoy.PendingEndpointStatus,
					},
					AppMetadata: &convoy.AppMetadata{
						UID: appID,
					},
				},
			},
			dbFn: func(ev *convoy.Event, msg *convoy.EventDelivery, app *applicationHandler) {
				m, _ := app.eventDeliveryRepo.(*mocks.MockEventDeliveryRepository)
				m.EXPECT().
					FindEventDeliveryByID(gomock.Any(), gomock.Any()).Times(1).
					Return(msg, nil)

				a, _ := app.appRepo.(*mocks.MockApplicationRepository)
				a.EXPECT().
					FindApplicationEndpointByID(gomock.Any(), gomock.Any(), gomock.Any()).Times(1).
					Return(&convoy.Endpoint{
						TargetURL: "http://localhost",
						Status:    convoy.PendingEndpointStatus,
					}, nil)
			},
		},
		{
			name:       "valid resend - previously failed and inactive endpoint",
			cfgPath:    "./testdata/Auth_Config/basic-convoy.json",
			method:     http.MethodPut,
			statusCode: http.StatusOK,
			body:       nil,
			args: args{
				event: &convoy.Event{
					UID: eventID,
				},
				message: &convoy.EventDelivery{
					UID: eventDeliveryID,
					EventMetadata: &convoy.EventMetadata{
						UID: eventID,
					},
					Status: convoy.FailureEventStatus,
					EndpointMetadata: &convoy.EndpointMetadata{
						TargetURL: "http://localhost",
						Status:    convoy.InactiveEndpointStatus,
					},
					AppMetadata: &convoy.AppMetadata{
						UID: appID,
					},
				},
			},
			dbFn: func(ev *convoy.Event, msg *convoy.EventDelivery, app *applicationHandler) {
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
						&convoy.Endpoint{
							Status: convoy.InactiveEndpointStatus,
						},
						nil,
					)

				a.EXPECT().
					UpdateApplicationEndpointsStatus(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Times(1).
					Return(nil)

				q, _ := app.eventQueue.(*mocks.MockQueuer)
				q.EXPECT().
					Write(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Times(1).
					Return(nil)
			},
		},
		{
			name:       "valid resend - previously failed - active endpoint",
			cfgPath:    "./testdata/Auth_Config/basic-convoy.json",
			method:     http.MethodPut,
			statusCode: http.StatusOK,
			body:       nil,
			args: args{
				event: &convoy.Event{
					UID: eventID,
				},
				message: &convoy.EventDelivery{
					UID: eventDeliveryID,
					EventMetadata: &convoy.EventMetadata{
						UID: eventID,
					},
					Status: convoy.FailureEventStatus,
					EndpointMetadata: &convoy.EndpointMetadata{
						TargetURL: "http://localhost",
						Status:    convoy.ActiveEndpointStatus,
					},
					AppMetadata: &convoy.AppMetadata{
						UID: appID,
					},
				},
			},
			dbFn: func(ev *convoy.Event, msg *convoy.EventDelivery, app *applicationHandler) {
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
						&convoy.Endpoint{
							TargetURL: "http://localhost",
							Status:    convoy.ActiveEndpointStatus,
						},
						nil,
					)

				q, _ := app.eventQueue.(*mocks.MockQueuer)
				q.EXPECT().
					Write(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Times(1).
					Return(nil)
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

			req = req.WithContext(context.WithValue(req.Context(), eventCtx, tc.args.message))

			if tc.dbFn != nil {
				tc.dbFn(tc.args.event, tc.args.message, app)
			}

			err := config.LoadConfig(tc.cfgPath)
			if err != nil {
				t.Error("Failed to load config file")
			}

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
