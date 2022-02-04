package server

import (
	"net/http"
	"net/http/httptest"
	"testing"

	mcache "github.com/frain-dev/convoy/cache/memory"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/logger"
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
	logger := logger.NewNoopLogger()
	tracer := mocks.NewMockTracer(ctrl)
	apiKeyRepo := mocks.NewMockAPIKeyRepository(ctrl)
	cache := mcache.NewMemoryCache()

	app = newApplicationHandler(eventRepo, eventDeliveryRepo, appRepo, groupRepo, apiKeyRepo, eventQueue, logger, tracer, cache)

	tests := []struct {
		name       string
		method     string
		statusCode int
		dbFn       func(app *applicationHandler)
	}{
		{
			name:       "successful config fetch",
			method:     http.MethodGet,
			statusCode: http.StatusOK,
			dbFn: func(app *applicationHandler) {
				g, _ := app.groupRepo.(*mocks.MockGroupRepository)

				g.EXPECT().
					FetchGroupByID(gomock.Any(), gomock.Any()).Times(1).
					Return(&datastore.Group{
						Config: &datastore.GroupConfig{},
					}, nil)
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := config.LoadConfig("./testdata/Auth_Config/none-convoy.json", provideFakeOverride())
			if err != nil {
				t.Errorf("Failed to load config file: %v", err)
			}
			initRealmChain(t, app.apiKeyRepo)

			if tc.dbFn != nil {
				tc.dbFn(app)
			}

			req := httptest.NewRequest(tc.method, "/ui/dashboard/config?groupID=12345", nil)
			responseRecorder := httptest.NewRecorder()

			requireGroup(app.groupRepo)(http.HandlerFunc(app.GetAllConfigDetails)).
				ServeHTTP(responseRecorder, req)

			if responseRecorder.Code != tc.statusCode {
				log.Error(tc.name, responseRecorder.Body)
				t.Errorf("Want status '%d', got '%d'", tc.statusCode, responseRecorder.Code)
			}

		})
	}
}
