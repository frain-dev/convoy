package server

import (
	"context"
	"fmt"
	"github.com/go-chi/chi/v5"
	pager "github.com/gobeam/mongo-go-pagination"
	"github.com/golang/mock/gomock"
	"github.com/hookcamp/hookcamp"
	"github.com/hookcamp/hookcamp/config"
	"github.com/hookcamp/hookcamp/mocks"
	"github.com/hookcamp/hookcamp/server/models"
	"github.com/sirupsen/logrus"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func Test_ensureNewMessage(t *testing.T) {

	var app *applicationHandler

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	orgRepo := mocks.NewMockOrganisationRepository(ctrl)
	appRepo := mocks.NewMockApplicationRepository(ctrl)
	msgRepo := mocks.NewMockMessageRepository(ctrl)

	orgID := "1234567890"
	appId := "12345"
	msgId := "1122333444456"

	app = newApplicationHandler(msgRepo, appRepo, orgRepo)

	message := &hookcamp.Message{
		UID:   msgId,
		AppID: appId,
	}

	type args struct {
		message *hookcamp.Message
	}

	tests := []struct {
		name       string
		method     string
		statusCode int
		args       args
		body       *strings.Reader
		dbFn       func(msgRepo *mocks.MockMessageRepository, appRepo *mocks.MockApplicationRepository, orgRepo *mocks.MockOrganisationRepository)
	}{
		{
			name:       "invalid message - no event type",
			method:     http.MethodPost,
			statusCode: http.StatusBadRequest,
			body:       strings.NewReader(`{ "data": {}}`),
			args: args{

				message: message,
			},
			dbFn: func(msgRepo *mocks.MockMessageRepository, appRepo *mocks.MockApplicationRepository, orgRepo *mocks.MockOrganisationRepository) {
				appRepo.EXPECT().
					FindApplicationByID(gomock.Any(), gomock.Any()).Times(0).
					Return(&hookcamp.Application{
						UID:       appId,
						OrgID:     orgID,
						Title:     "Valid application",
						Endpoints: []hookcamp.Endpoint{},
					}, nil)
				msgRepo.EXPECT().
					CreateMessage(gomock.Any(), gomock.Any()).Times(0).
					Return(nil)

			},
		},
		{
			name:       "valid message",
			method:     http.MethodPost,
			statusCode: http.StatusCreated,
			body:       strings.NewReader(`{"event_type": "test.event", "data": { "Hello": "World", "Test": "Data" }}`),
			args: args{
				message: message,
			},
			dbFn: func(msgRepo *mocks.MockMessageRepository, appRepo *mocks.MockApplicationRepository, orgRepo *mocks.MockOrganisationRepository) {
				appRepo.EXPECT().
					FindApplicationByID(gomock.Any(), gomock.Any()).Times(1).
					Return(&hookcamp.Application{
						UID:   appId,
						OrgID: orgID,
						Title: "Valid application",
						Endpoints: []hookcamp.Endpoint{
							{
								TargetURL: "http://localhost",
							},
						},
					}, nil)
				msgRepo.EXPECT().
					CreateMessage(gomock.Any(), gomock.Any()).Times(1).
					Return(nil)

			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {

			err := config.LoadFromFile("./testdata/TestRequireAuth_None/hookcamp.json")
			if err != nil {
				t.Error("Failed to load config file")
			}

			request := httptest.NewRequest(tc.method, fmt.Sprintf("/v1/apps/%s/events", tc.args.message.AppID), tc.body)
			responseRecorder := httptest.NewRecorder()
			rctx := chi.NewRouteContext()
			rctx.URLParams.Add("appID", tc.args.message.AppID)

			request = request.WithContext(context.WithValue(request.Context(), chi.RouteCtxKey, rctx))

			if tc.dbFn != nil {
				tc.dbFn(msgRepo, appRepo, orgRepo)
			}

			ensureNewMessage(appRepo, msgRepo)(http.HandlerFunc(app.CreateAppMessage)).
				ServeHTTP(responseRecorder, request)

			if responseRecorder.Code != tc.statusCode {
				logrus.Error(tc.args.message, responseRecorder.Body)
				t.Errorf("Want status '%d', got '%d'", tc.statusCode, responseRecorder.Code)
			}
		})
	}
}

func Test_fetchAllMessages(t *testing.T) {
	var app *applicationHandler

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	orgRepo := mocks.NewMockOrganisationRepository(ctrl)
	appRepo := mocks.NewMockApplicationRepository(ctrl)
	msgRepo := mocks.NewMockMessageRepository(ctrl)

	appId := "12345"
	msgId := "1122333444456"

	app = newApplicationHandler(msgRepo, appRepo, orgRepo)

	message := &hookcamp.Message{
		UID:   msgId,
		AppID: appId,
	}

	type args struct {
		message *hookcamp.Message
	}

	tests := []struct {
		name       string
		method     string
		statusCode int
		args       args
		body       *strings.Reader
		dbFn       func(msgRepo *mocks.MockMessageRepository, appRepo *mocks.MockApplicationRepository, orgRepo *mocks.MockOrganisationRepository)
	}{
		{
			name:       "valid messages",
			method:     http.MethodGet,
			statusCode: http.StatusOK,
			body:       nil,
			args: args{
				message: message,
			},
			dbFn: func(msgRepo *mocks.MockMessageRepository, appRepo *mocks.MockApplicationRepository, orgRepo *mocks.MockOrganisationRepository) {
				msgRepo.EXPECT().
					LoadMessagesPaged(gomock.Any(), gomock.Any(), gomock.Any()).Times(1).
					Return([]hookcamp.Message{
						*message,
					},
						pager.PaginationData{},
						nil)

			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			request := httptest.NewRequest(tc.method, fmt.Sprintf("/v1/apps/%s/messages", tc.args.message.AppID), nil)
			responseRecorder := httptest.NewRecorder()
			rctx := chi.NewRouteContext()
			rctx.URLParams.Add("appID", tc.args.message.AppID)

			pageable := models.Pageable{
				Page:    1,
				PerPage: 10,
			}
			request = request.WithContext(context.WithValue(request.Context(), chi.RouteCtxKey, rctx))
			request = request.WithContext(context.WithValue(request.Context(), pageableCtx, pageable))

			if tc.dbFn != nil {
				tc.dbFn(msgRepo, appRepo, orgRepo)
			}

			fetchAllMessages(msgRepo)(http.HandlerFunc(app.GetAppMessagesPaged)).
				ServeHTTP(responseRecorder, request)

			if responseRecorder.Code != tc.statusCode {
				logrus.Error(tc.args.message, responseRecorder.Body)
				t.Errorf("Want status '%d', got '%d'", tc.statusCode, responseRecorder.Code)
			}
		})
	}
}
