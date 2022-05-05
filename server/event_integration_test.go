//go:build integration
// +build integration

package server

import (
	"github.com/google/uuid"
	"net/http"
	"testing"

	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/server/testdb"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type EventIntegrationTestSuite struct {
	suite.Suite
	DB           datastore.DatabaseClient
	Router       http.Handler
	ConvoyApp    *applicationHandler
	DefaultGroup *datastore.Group
}

func (s *EventIntegrationTestSuite) SetupSuite() {
	s.DB = getDB()
	s.ConvoyApp = buildApplication()
	s.Router = buildRoutes(s.ConvoyApp)
}

func (s *EventIntegrationTestSuite) SetupTest() {
	testdb.PurgeDB(s.DB)

	// Setup Default Group.
	s.DefaultGroup, _ = testdb.SeedDefaultGroup(s.DB)

	// Setup Config.
	err := config.LoadConfig("./testdata/Auth_Config/full-convoy.json")
	require.NoError(s.T(), err)

	initRealmChain(s.T(), s.DB.APIRepo())
}

func (s *EventIntegrationTestSuite) TearDownTest() {
	testdb.PurgeDB(s.DB)
}

func (s *EventIntegrationTestSuite) Test_CreateAppEvent_Valid_Event() {
	appID := uuid.NewString()
	expectedStatusCode := http.StatusCreated

	// Just Before.
	app, _ := testdb.SeedApplication(s.DB, s.DefaultGroup, appID, false)
	_, _ = testdb.SeedMultipleEndpoints(s.DB, app, []string{"*"}, 2)

	body := M{
		"app_id":     appID,
		"event_type": "*",
		"data":       `{"level":"test"}`,
	}

	req, w := newRequestAndResponder(http.MethodPost, "/api/v1/events", serialize(s.T(), body))
	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)

	// Deep Assert.
	var event datastore.Event
	parseResponse(s.T(), w.Result(), &event)

	require.Equal(s.T(), event.AppMetadata.UID, appID)
	require.Equal(s.T(), 0, event.MatchedEndpoints)
	require.Equal(s.T(), string(event.EventType), body["event_type"])
}

func (s *EventIntegrationTestSuite) Test_CreateAppEvent_App_has_no_endpoint() {
	appID := uuid.NewString()
	expectedStatusCode := http.StatusBadRequest

	// Just Before.
	_, _ = testdb.SeedApplication(s.DB, s.DefaultGroup, appID, false)

	body := M{
		"app_id":     appID,
		"event_type": "*",
		"data":       `{"level":"test"}`,
	}

	req, w := newRequestAndResponder(http.MethodPost, "/api/v1/events", serialize(s.T(), body))
	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)
}

func (s *EventIntegrationTestSuite) Test_CreateAppEvent_App_is_disabled() {
	appID := uuid.NewString()
	expectedStatusCode := http.StatusBadRequest

	// Just Before.
	app, _ := testdb.SeedApplication(s.DB, s.DefaultGroup, appID, true)
	_, _ = testdb.SeedMultipleEndpoints(s.DB, app, []string{"*"}, 2)

	body := M{
		"app_id":     appID,
		"event_type": "*",
		"data":       `{"level":"test"}`,
	}

	req, w := newRequestAndResponder(http.MethodPost, "/api/v1/events", serialize(s.T(), body))
	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)
}

func TestEventIntegrationSuiteTest(t *testing.T) {
	suite.Run(t, new(EventIntegrationTestSuite))
}
