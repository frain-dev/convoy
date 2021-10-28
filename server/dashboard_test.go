package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/mocks"
	"github.com/golang/mock/gomock"
	"github.com/sirupsen/logrus"
)

func Test_login(t *testing.T) {
	var app *applicationHandler

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	groupRepo := mocks.NewMockGroupRepository(ctrl)
	appRepo := mocks.NewMockApplicationRepository(ctrl)
	eventRepo := mocks.NewMockEventRepository(ctrl)
	scheduleQueue := mocks.NewMockQueuer(ctrl)

	app = newApplicationHandler(eventRepo, appRepo, groupRepo, scheduleQueue)

	tests := []struct {
		name       string
		method     string
		body       *strings.Reader
		statusCode int
	}{
		{
			name:       "bad login - no request body",
			method:     http.MethodPost,
			body:       strings.NewReader(``),
			statusCode: http.StatusBadRequest,
		},
		{
			name:       "bad login - unauthorized user password",
			method:     http.MethodPost,
			body:       strings.NewReader(`{"username": "user1","password": "wrong password"}`),
			statusCode: http.StatusUnauthorized,
		},
		{
			name:       "bad login - unauthorized user name",
			method:     http.MethodPost,
			body:       strings.NewReader(`{"username": "user1000000","password": "password1"}`),
			statusCode: http.StatusUnauthorized,
		},
		{
			name:       "successful login",
			method:     http.MethodPost,
			body:       strings.NewReader(`{"username": "user1","password": "password1"}`),
			statusCode: http.StatusOK,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {

			err := config.LoadConfig("./testdata/Auth_Config/none-convoy.json")
			if err != nil {
				t.Error("Failed to load config file")
			}

			request := httptest.NewRequest(tc.method, "/v1/auth/login", tc.body)
			responseRecorder := httptest.NewRecorder()

			login()(http.HandlerFunc(app.GetAuthLogin)).
				ServeHTTP(responseRecorder, request)

			if responseRecorder.Code != tc.statusCode {
				logrus.Error(tc.name, responseRecorder.Body)
				t.Errorf("Want status '%d', got '%d'", tc.statusCode, responseRecorder.Code)
			}
		})
	}
}

func Test_fetchAllConfigDetails(t *testing.T) {
	var app *applicationHandler

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	groupRepo := mocks.NewMockGroupRepository(ctrl)
	appRepo := mocks.NewMockApplicationRepository(ctrl)
	eventRepo := mocks.NewMockEventRepository(ctrl)
	scheduleQueue := mocks.NewMockQueuer(ctrl)

	app = newApplicationHandler(eventRepo, appRepo, groupRepo, scheduleQueue)

	tests := []struct {
		name       string
		method     string
		statusCode int
	}{
		{
			name:       "successful config fetch",
			method:     http.MethodGet,
			statusCode: http.StatusOK,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {

			err := config.LoadConfig("./testdata/Auth_Config/none-convoy.json")
			if err != nil {
				t.Error("Failed to load config file")
			}

			request := httptest.NewRequest(tc.method, "/ui/dashboard/1/config", nil)
			responseRecorder := httptest.NewRecorder()

			fetchAllConfigDetails()(http.HandlerFunc(app.GetAllConfigDetails)).
				ServeHTTP(responseRecorder, request)

			if responseRecorder.Code != tc.statusCode {
				logrus.Error(tc.name, responseRecorder.Body)
				t.Errorf("Want status '%d', got '%d'", tc.statusCode, responseRecorder.Code)
			}

		})
	}
}
