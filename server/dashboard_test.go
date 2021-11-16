package server

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/mocks"
	"github.com/golang/mock/gomock"
	log "github.com/sirupsen/logrus"
)

func Test_fetchAllConfigDetails(t *testing.T) {
	var app *applicationHandler

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	groupRepo := mocks.NewMockGroupRepository(ctrl)
	appRepo := mocks.NewMockApplicationRepository(ctrl)
	eventRepo := mocks.NewMockEventRepository(ctrl)
	eventDeliveryRepo := mocks.NewMockEventDeliveryRepository(ctrl)
	eventQueue := mocks.NewMockQueuer(ctrl)

	app = newApplicationHandler(eventRepo, eventDeliveryRepo, appRepo, groupRepo, eventQueue)

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
				t.Errorf("Failed to load config file: %v", err)
			}
			initRealmChain(t)

			request := httptest.NewRequest(tc.method, "/ui/dashboard/1/config", nil)
			responseRecorder := httptest.NewRecorder()

			fetchAllConfigDetails()(http.HandlerFunc(app.GetAllConfigDetails)).
				ServeHTTP(responseRecorder, request)

			if responseRecorder.Code != tc.statusCode {
				log.Error(tc.name, responseRecorder.Body)
				t.Errorf("Want status '%d', got '%d'", tc.statusCode, responseRecorder.Code)
			}

		})
	}
}
