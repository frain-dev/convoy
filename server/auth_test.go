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

func Test_fetchAuthConfig(t *testing.T) {
	var app *applicationHandler

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	orgRepo := mocks.NewMockOrganisationRepository(ctrl)
	appRepo := mocks.NewMockApplicationRepository(ctrl)
	msgRepo := mocks.NewMockMessageRepository(ctrl)

	app = newApplicationHandler(msgRepo, appRepo, orgRepo)

	tests := []struct {
		name       string
		method     string
		statusCode int
	}{
		{
			name:       "successful auth fetch",
			method:     http.MethodGet,
			statusCode: http.StatusOK,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {

			err := config.LoadConfig("./testdata/TestRequireAuth_None/convoy.json")
			if err != nil {
				t.Error("Failed to load config file")
			}

			request := httptest.NewRequest(tc.method, "/v1/auth/details", nil)
			responseRecorder := httptest.NewRecorder()

			fetchAuthConfig()(http.HandlerFunc(app.GetAuthDetails)).
				ServeHTTP(responseRecorder, request)

			if responseRecorder.Code != tc.statusCode {
				logrus.Error(tc.name, responseRecorder.Body)
				t.Errorf("Want status '%d', got '%d'", tc.statusCode, responseRecorder.Code)
			}

			verifyMatch(t, *responseRecorder)

		})
	}
}

func Test_login(t *testing.T) {
	var app *applicationHandler

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	orgRepo := mocks.NewMockOrganisationRepository(ctrl)
	appRepo := mocks.NewMockApplicationRepository(ctrl)
	msgRepo := mocks.NewMockMessageRepository(ctrl)

	app = newApplicationHandler(msgRepo, appRepo, orgRepo)

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

			err := config.LoadFromFile("./testdata/TestRequireAuth_None/convoy.json")
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
