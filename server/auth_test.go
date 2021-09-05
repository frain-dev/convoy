package server

import (
	"net/http"
	"net/http/httptest"
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

			err := config.LoadFromFile("./testdata/TestRequireAuth_None/convoy.json")
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
