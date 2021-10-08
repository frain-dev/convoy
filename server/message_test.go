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
	"github.com/frain-dev/convoy/server/models"
	"github.com/go-chi/chi/v5"
	pager "github.com/gobeam/mongo-go-pagination"
	"github.com/golang/mock/gomock"
	"github.com/sirupsen/logrus"
)

func TestApplicationHandler_CreateAppMessage(t *testing.T) {

	orgID := "1234567890"
	appId := "12345"
	msgId := "1122333444456"

	message := &convoy.Message{
		UID:   msgId,
		AppID: appId,
	}

	type args struct {
		message *convoy.Message
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
				m, _ := app.msgRepo.(*mocks.MockMessageRepository)
				m.EXPECT().
					CreateMessage(gomock.Any(), gomock.Any()).Times(0).
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
					FindApplicationByID(gomock.Any(), gomock.Any()).Times(0).
					Return(&convoy.Application{
						UID:   appId,
						OrgID: orgID,
						Title: "Valid application",
						Endpoints: []convoy.Endpoint{
							{
								TargetURL: "http://localhost",
								Status:    convoy.ActiveEndpointStatus,
							},
						},
					}, nil)
			},
		},
		{
			name:       "valid message - no active endpoints",
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
					FindApplicationByID(gomock.Any(), gomock.Any()).Times(0).
					Return(&convoy.Application{
						UID:   appId,
						OrgID: orgID,
						Title: "Valid application",
						Endpoints: []convoy.Endpoint{
							{
								TargetURL: "http://localhost",
								Status:    convoy.InactiveEndpointStatus,
							},
						},
					}, nil)
			},
		},
		{
			name:       "valid message",
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
						UID:   appId,
						OrgID: orgID,
						Title: "Valid application",
						Endpoints: []convoy.Endpoint{
							{
								TargetURL: "http://localhost",
								Status:    convoy.ActiveEndpointStatus,
							},
						},
					}, nil)

				m, _ := app.msgRepo.(*mocks.MockMessageRepository)
				m.EXPECT().
					CreateMessage(gomock.Any(), gomock.Any()).Times(1).
					Return(nil)

			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var app *applicationHandler

			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			orgRepo := mocks.NewMockOrganisationRepository(ctrl)
			appRepo := mocks.NewMockApplicationRepository(ctrl)
			msgRepo := mocks.NewMockMessageRepository(ctrl)

			app = newApplicationHandler(msgRepo, appRepo, orgRepo)

			err := config.LoadConfig(tc.cfgPath)
			if err != nil {
				t.Error("Failed to load config file")
			}

			req := httptest.NewRequest(tc.method, "/v1/events", tc.body)
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
				logrus.Error(tc.args.message, w.Body)
				t.Errorf("Want status '%d', got '%d'", tc.statusCode, w.Code)
			}
		})
	}
}

func Test_fetchAllMessages(t *testing.T) {

	appId := "12345"
	msgId := "1122333444456"
	message := &convoy.Message{
		UID:   msgId,
		AppID: appId,
	}

	type args struct {
		message *convoy.Message
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
			name:       "valid messages",
			cfgPath:    "./testdata/Auth_Config/basic-convoy.json",
			method:     http.MethodGet,
			statusCode: http.StatusOK,
			body:       nil,
			args: args{
				message: message,
			},
			dbFn: func(app *applicationHandler) {
				m, _ := app.msgRepo.(*mocks.MockMessageRepository)
				m.EXPECT().
					LoadMessagesPaged(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Times(1).
					Return([]convoy.Message{
						*message,
					},
						pager.PaginationData{},
						nil)

			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var app *applicationHandler

			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			orgRepo := mocks.NewMockOrganisationRepository(ctrl)
			appRepo := mocks.NewMockApplicationRepository(ctrl)
			msgRepo := mocks.NewMockMessageRepository(ctrl)

			app = newApplicationHandler(msgRepo, appRepo, orgRepo)

			err := config.LoadConfig(tc.cfgPath)
			if err != nil {
				t.Error("Failed to load config file")
			}

			req := httptest.NewRequest(tc.method, "/v1/events", nil)
			req.SetBasicAuth("test", "test")
			w := httptest.NewRecorder()
			rctx := chi.NewRouteContext()
			rctx.URLParams.Add("appID", tc.args.message.AppID)

			pageable := models.Pageable{
				Page:    1,
				PerPage: 10,
			}
			req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
			req = req.WithContext(context.WithValue(req.Context(), pageableCtx, pageable))

			if tc.dbFn != nil {
				tc.dbFn(app)
			}

			router := buildRoutes(app)

			// Act
			router.ServeHTTP(w, req)

			if w.Code != tc.statusCode {
				logrus.Error(tc.args.message, w.Body)
				t.Errorf("Want status '%d', got '%d'", tc.statusCode, w.Code)
			}
		})
	}
}

func Test_resendMessage(t *testing.T) {

	appId := "12345"
	msgId := "1122333444456"

	type args struct {
		message *convoy.Message
	}

	tests := []struct {
		name       string
		cfgPath    string
		method     string
		statusCode int
		args       args
		body       *strings.Reader
		dbFn       func(*convoy.Message, *applicationHandler)
	}{
		{
			name:       "invalid event to resend - already successful",
			cfgPath:    "./testdata/Auth_Config/full-convoy.json",
			method:     http.MethodPut,
			statusCode: http.StatusBadRequest,
			body:       nil,
			args: args{
				message: &convoy.Message{
					UID:    msgId,
					AppID:  appId,
					Status: convoy.SuccessMessageStatus,
				},
			},
			dbFn: func(msg *convoy.Message, app *applicationHandler) {
				m, _ := app.msgRepo.(*mocks.MockMessageRepository)
				m.EXPECT().
					FindMessageByID(gomock.Any(), gomock.Any()).Times(1).
					Return(msg, nil)

				m.EXPECT().
					UpdateStatusOfMessages(gomock.Any(), gomock.Any(), gomock.Any()).Times(0).
					Return(nil)

			},
		},
		{
			name:       "invalid event to resend - not failed",
			cfgPath:    "./testdata/Auth_Config/full-convoy.json",
			method:     http.MethodPut,
			statusCode: http.StatusBadRequest,
			body:       nil,
			args: args{
				message: &convoy.Message{
					UID:    msgId,
					AppID:  appId,
					Status: convoy.ProcessingMessageStatus,
				},
			},
			dbFn: func(msg *convoy.Message, app *applicationHandler) {
				m, _ := app.msgRepo.(*mocks.MockMessageRepository)
				m.EXPECT().
					FindMessageByID(gomock.Any(), gomock.Any()).Times(1).
					Return(msg, nil)

				m.EXPECT().
					UpdateStatusOfMessages(gomock.Any(), gomock.Any(), gomock.Any()).Times(0).
					Return(nil)

			},
		},
		{
			name:       "valid event to resend - previously failed",
			cfgPath:    "./testdata/Auth_Config/basic-convoy.json",
			method:     http.MethodPut,
			statusCode: http.StatusOK,
			body:       nil,
			args: args{
				message: &convoy.Message{
					UID:    msgId,
					AppID:  appId,
					Status: convoy.FailureMessageStatus,
					AppMetadata: &convoy.AppMetadata{
						Endpoints: []convoy.EndpointMetadata{
							{TargetURL: "http://localhost"},
						},
					},
				},
			},
			dbFn: func(msg *convoy.Message, app *applicationHandler) {
				m, _ := app.msgRepo.(*mocks.MockMessageRepository)
				m.EXPECT().
					FindMessageByID(gomock.Any(), gomock.Any()).Times(1).
					Return(msg, nil)

				m.EXPECT().
					UpdateStatusOfMessages(gomock.Any(), gomock.Any(), gomock.Any()).Times(1).
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

			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var app *applicationHandler

			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			orgRepo := mocks.NewMockOrganisationRepository(ctrl)
			appRepo := mocks.NewMockApplicationRepository(ctrl)
			msgRepo := mocks.NewMockMessageRepository(ctrl)

			app = newApplicationHandler(msgRepo, appRepo, orgRepo)

			url := fmt.Sprintf("/v1/events/%s/resend", tc.args.message.UID)
			req := httptest.NewRequest(tc.method, url, nil)
			req.SetBasicAuth("test", "test")
			req.Header.Add("Content-Type", "application/json")

			w := httptest.NewRecorder()
			rctx := chi.NewRouteContext()
			rctx.URLParams.Add("appID", tc.args.message.AppID)

			req = req.WithContext(context.WithValue(req.Context(), msgCtx, tc.args.message))

			if tc.dbFn != nil {
				tc.dbFn(tc.args.message, app)
			}

			err := config.LoadConfig(tc.cfgPath)
			if err != nil {
				t.Error("Failed to load config file")
			}

			router := buildRoutes(app)

			// Act
			router.ServeHTTP(w, req)

			if w.Code != tc.statusCode {
				logrus.Error(tc.args.message, w.Body)
				t.Errorf("Want status '%d', got '%d'", tc.statusCode, w.Code)
			}

			verifyMatch(t, *w)
		})
	}
}
