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

func Test_fetchAllConfigDetails(t *testing.T) {
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
