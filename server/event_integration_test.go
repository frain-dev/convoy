//go:build integration
// +build integration

package server

import (
	"fmt"
	"github.com/google/uuid"
	"net/http"
	"net/http/httptest"
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
	app, _ := testdb.SeedApplication(s.DB, s.DefaultGroup, appID, "", false)
	_, _ = testdb.SeedMultipleEndpoints(s.DB, app, s.DefaultGroup.UID, []string{"*"}, 2)

	bodyStr := `{"app_id":"%s", "event_type":"*", "data":{"level":"test"}}`
	body := serialize(bodyStr, appID)

	req := createRequest(http.MethodPost, "/api/v1/events", body)
	w := httptest.NewRecorder()
	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)

	// Deep Assert.
	var event datastore.Event
	parseResponse(s.T(), w.Result(), &event)

	require.NotEmpty(s.T(), event.UID)
	require.Equal(s.T(), event.AppMetadata.UID, appID)
}

func (s *EventIntegrationTestSuite) Test_CreateAppEvent_App_has_no_endpoint() {
	appID := uuid.NewString()
	expectedStatusCode := http.StatusBadRequest

	// Just Before.
	_, _ = testdb.SeedApplication(s.DB, s.DefaultGroup, appID, "", false)

	bodyStr := `{"app_id":"%s", "event_type":"*", "data":{"level":"test"}}`
	body := serialize(bodyStr, appID)

	req := createRequest(http.MethodPost, "/api/v1/events", body)
	w := httptest.NewRecorder()
	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)
}

func (s *EventIntegrationTestSuite) Test_CreateAppEvent_App_is_disabled() {
	appID := uuid.NewString()
	expectedStatusCode := http.StatusCreated

	// Just Before.
	app, _ := testdb.SeedApplication(s.DB, s.DefaultGroup, appID, "", true)
	_, _ = testdb.SeedMultipleEndpoints(s.DB, app, s.DefaultGroup.UID, []string{"*"}, 2)

	bodyStr := `{"app_id":"%s", "event_type":"*", "data":{"level":"test"}}`
	body := serialize(bodyStr, appID)

	req := createRequest(http.MethodPost, "/api/v1/events", body)
	w := httptest.NewRecorder()
	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)
}

func (s *EventIntegrationTestSuite) Test_GetAppEvent_Valid_Event() {
	eventID := uuid.NewString()
	expectedStatusCode := http.StatusOK

	// Just Before.
	app, _ := testdb.SeedApplication(s.DB, s.DefaultGroup, uuid.NewString(), "", false)
	event, _ := testdb.SeedEvent(s.DB, app, s.DefaultGroup.UID, eventID, "*", []byte(`{}`))

	url := fmt.Sprintf("/api/v1/events/%s", eventID)
	req := createRequest(http.MethodGet, url, nil)
	w := httptest.NewRecorder()

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)

	// Deep Assert.
	var respEvent datastore.Event
	parseResponse(s.T(), w.Result(), &respEvent)
	require.Equal(s.T(), event.UID, respEvent.UID)
}

func (s *EventIntegrationTestSuite) Test_ReplayAppEvent_Valid_Event() {
	eventID := uuid.NewString()
	expectedStatusCode := http.StatusOK

	// Just Before.
	app, _ := testdb.SeedApplication(s.DB, s.DefaultGroup, uuid.NewString(), "", false)
	_, _ = testdb.SeedEvent(s.DB, app, s.DefaultGroup.UID, eventID, "*", []byte(`{}`))

	url := fmt.Sprintf("/api/v1/events/%s/replay", eventID)
	req := createRequest(http.MethodPut, url, nil)
	w := httptest.NewRecorder()

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)
}

func (s *EventIntegrationTestSuite) Test_GetAppEvent_Event_not_found() {
	eventID := uuid.NewString()
	expectedStatusCode := http.StatusNotFound

	url := fmt.Sprintf("/api/v1/events/%s", eventID)
	req := createRequest(http.MethodGet, url, nil)
	w := httptest.NewRecorder()

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)
}

func (s *EventIntegrationTestSuite) Test_GetEventDelivery_Valid_EventDelivery() {
	eventDeliveryID := uuid.NewString()
	expectedStatusCode := http.StatusOK

	// Just Before.
	app, _ := testdb.SeedApplication(s.DB, s.DefaultGroup, uuid.NewString(), "", false)
	eventDelivery, _ := testdb.SeedEventDelivery(s.DB, app, &datastore.Event{}, &datastore.Endpoint{}, s.DefaultGroup.UID, eventDeliveryID, datastore.SuccessEventStatus)

	url := fmt.Sprintf("/api/v1/eventdeliveries/%s", eventDeliveryID)
	req := createRequest(http.MethodGet, url, nil)
	w := httptest.NewRecorder()

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)

	// Deep Assert.
	var respEventDelivery datastore.EventDelivery
	parseResponse(s.T(), w.Result(), &respEventDelivery)
	require.Equal(s.T(), eventDelivery.UID, respEventDelivery.UID)
}

func (s *EventIntegrationTestSuite) Test_GetEventDelivery_Event_not_found() {
	eventDeliveryID := uuid.NewString()
	expectedStatusCode := http.StatusNotFound

	url := fmt.Sprintf("/api/v1/eventdeliveries/%s", eventDeliveryID)
	req := createRequest(http.MethodGet, url, nil)
	w := httptest.NewRecorder()

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)
}

func (s *EventIntegrationTestSuite) Test_ResendEventDelivery_Valid_Resend() {
	eventDeliveryID := uuid.NewString()
	expectedStatusCode := http.StatusOK

	// Just Before.
	app, _ := testdb.SeedApplication(s.DB, s.DefaultGroup, uuid.NewString(), "", false)
	_, _ = testdb.SeedEndpoint(s.DB, app, s.DefaultGroup.UID, []string{"*"})
	eventDelivery, _ := testdb.SeedEventDelivery(s.DB, app, &datastore.Event{}, &app.Endpoints[0], s.DefaultGroup.UID, eventDeliveryID, datastore.FailureEventStatus)

	url := fmt.Sprintf("/api/v1/eventdeliveries/%s/resend", eventDeliveryID)
	req := createRequest(http.MethodPut, url, nil)
	w := httptest.NewRecorder()

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)

	// Deep Assert.
	var respEventDelivery datastore.EventDelivery
	parseResponse(s.T(), w.Result(), &respEventDelivery)
	require.Equal(s.T(), datastore.ScheduledEventStatus, respEventDelivery.Status)
	require.Equal(s.T(), eventDelivery.UID, respEventDelivery.UID)
}

func (s *EventIntegrationTestSuite) Test_BatchRetryEventDelivery_Valid_EventDeliveries() {
	eventDeliveryID := uuid.NewString()
	expectedStatusCode := http.StatusOK

	// Just Before.
	app, _ := testdb.SeedApplication(s.DB, s.DefaultGroup, uuid.NewString(), "", false)
	_, _ = testdb.SeedEndpoint(s.DB, app, s.DefaultGroup.UID, []string{"*"})
	event, _ := testdb.SeedEvent(s.DB, app, s.DefaultGroup.UID, uuid.NewString(), "*", []byte(`{}`))
	_, _ = testdb.SeedEventDelivery(s.DB, app, event, &app.Endpoints[0], s.DefaultGroup.UID, eventDeliveryID, datastore.FailureEventStatus)
	_, _ = testdb.SeedEventDelivery(s.DB, app, event, &app.Endpoints[0], s.DefaultGroup.UID, eventDeliveryID, datastore.FailureEventStatus)
	_, _ = testdb.SeedEventDelivery(s.DB, app, event, &app.Endpoints[0], s.DefaultGroup.UID, eventDeliveryID, datastore.FailureEventStatus)

	url := fmt.Sprintf("/api/v1/eventdeliveries/batchretry?appId=%s&eventId=%s&status=%s", app.UID, event.UID, datastore.FailureEventStatus)
	req := createRequest(http.MethodPost, url, nil)
	w := httptest.NewRecorder()

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)
}

func (s *EventIntegrationTestSuite) Test_CountAffectedEventDeliveries_Valid_Filters() {
	eventDeliveryID := uuid.NewString()
	expectedStatusCode := http.StatusOK

	// Just Before.
	app, _ := testdb.SeedApplication(s.DB, s.DefaultGroup, uuid.NewString(), "", false)
	_, _ = testdb.SeedEndpoint(s.DB, app, s.DefaultGroup.UID, []string{"*"})
	event, _ := testdb.SeedEvent(s.DB, app, s.DefaultGroup.UID, uuid.NewString(), "*", []byte(`{}`))
	_, _ = testdb.SeedEventDelivery(s.DB, app, event, &app.Endpoints[0], s.DefaultGroup.UID, eventDeliveryID, datastore.FailureEventStatus)
	_, _ = testdb.SeedEventDelivery(s.DB, app, event, &app.Endpoints[0], s.DefaultGroup.UID, eventDeliveryID, datastore.FailureEventStatus)
	_, _ = testdb.SeedEventDelivery(s.DB, app, event, &app.Endpoints[0], s.DefaultGroup.UID, eventDeliveryID, datastore.FailureEventStatus)

	url := fmt.Sprintf("/api/v1/eventdeliveries/countbatchretryevents?appId=%s&eventId=%s&status=%s", app.UID, event.UID, datastore.FailureEventStatus)
	req := createRequest(http.MethodGet, url, nil)
	w := httptest.NewRecorder()

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)

	// Deep Assert.
	var num struct {
		Num int `json:"num"`
	}
	parseResponse(s.T(), w.Result(), &num)
	require.Equal(s.T(), 3, num.Num)
}

func (s *EventIntegrationTestSuite) Test_ForceResendEventDeliveries_Valid_EventDeliveries() {
	eventDeliveryID := uuid.NewString()
	expectedStatusCode := http.StatusOK

	// Just Before.
	app, _ := testdb.SeedApplication(s.DB, s.DefaultGroup, uuid.NewString(), "", false)
	_, _ = testdb.SeedEndpoint(s.DB, app, s.DefaultGroup.UID, []string{"*"})
	event, _ := testdb.SeedEvent(s.DB, app, s.DefaultGroup.UID, uuid.NewString(), "*", []byte(`{}`))
	e1, _ := testdb.SeedEventDelivery(s.DB, app, event, &app.Endpoints[0], s.DefaultGroup.UID, eventDeliveryID, datastore.SuccessEventStatus)
	e2, _ := testdb.SeedEventDelivery(s.DB, app, event, &app.Endpoints[0], s.DefaultGroup.UID, eventDeliveryID, datastore.SuccessEventStatus)
	e3, _ := testdb.SeedEventDelivery(s.DB, app, event, &app.Endpoints[0], s.DefaultGroup.UID, eventDeliveryID, datastore.SuccessEventStatus)

	url := fmt.Sprintf("/api/v1/eventdeliveries/forceresend")

	bodyStr := `{"ids":["%s", "%s", "%s"]}`
	body := serialize(bodyStr, e1.UID, e2.UID, e3.UID)

	req := createRequest(http.MethodPost, url, body)
	w := httptest.NewRecorder()

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)
}

func (s *EventIntegrationTestSuite) Test_GetEventsPaged() {
	eventID := uuid.NewString()
	expectedStatusCode := http.StatusOK

	// Just Before.
	app1, _ := testdb.SeedApplication(s.DB, s.DefaultGroup, uuid.NewString(), "", false)
	e1, _ := testdb.SeedEvent(s.DB, app1, s.DefaultGroup.UID, eventID, "*", []byte(`{}`))
	e2, _ := testdb.SeedEvent(s.DB, app1, s.DefaultGroup.UID, eventID, "*", []byte(`{}`))

	app2, _ := testdb.SeedApplication(s.DB, s.DefaultGroup, uuid.NewString(), "", false)
	_, _ = testdb.SeedEvent(s.DB, app2, s.DefaultGroup.UID, eventID, "*", []byte(`{}`))

	url := fmt.Sprintf("/api/v1/events?appId=%s", app1.UID)
	req := createRequest(http.MethodGet, url, nil)
	w := httptest.NewRecorder()

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)

	// Deep Assert.
	var respEvents []datastore.Event
	resp := pagedResponse{Content: &respEvents}
	parseResponse(s.T(), w.Result(), &resp)
	require.Equal(s.T(), int64(2), resp.Pagination.Total)
	require.Equal(s.T(), 2, len(respEvents))

	v := []*datastore.Event{e1, e2}
	for i, delivery := range v {
		require.Equal(s.T(), respEvents[i].UID, delivery.UID)
	}
}

func (s *EventIntegrationTestSuite) GetEventDeliveriesPaged() {
	eventDeliveryID := uuid.NewString()
	expectedStatusCode := http.StatusOK

	// Just Before.
	app1, _ := testdb.SeedApplication(s.DB, s.DefaultGroup, uuid.NewString(), "", false)
	event1, _ := testdb.SeedEvent(s.DB, app1, s.DefaultGroup.UID, uuid.NewString(), "*", []byte(`{}`))
	d1, _ := testdb.SeedEventDelivery(s.DB, app1, event1, &app1.Endpoints[0], s.DefaultGroup.UID, eventDeliveryID, datastore.FailureEventStatus)
	d2, _ := testdb.SeedEventDelivery(s.DB, app1, event1, &app1.Endpoints[0], s.DefaultGroup.UID, eventDeliveryID, datastore.FailureEventStatus)

	app2, _ := testdb.SeedApplication(s.DB, s.DefaultGroup, uuid.NewString(), "", false)
	event2, _ := testdb.SeedEvent(s.DB, app2, s.DefaultGroup.UID, uuid.NewString(), "*", []byte(`{}`))
	_, _ = testdb.SeedEventDelivery(s.DB, app2, event2, &app2.Endpoints[0], s.DefaultGroup.UID, eventDeliveryID, datastore.FailureEventStatus)

	url := fmt.Sprintf("/api/v1/eventdeliveries?appId=%s", app1.UID)
	req := createRequest(http.MethodGet, url, nil)
	w := httptest.NewRecorder()

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)

	// Deep Assert.
	var respEvents []datastore.EventDelivery
	resp := pagedResponse{Content: &respEvents}
	parseResponse(s.T(), w.Result(), &respEvents)
	require.Equal(s.T(), 2, resp.Pagination.Total)

	v := []*datastore.EventDelivery{d1, d2}
	for i, delivery := range v {
		require.Equal(s.T(), respEvents[i].UID, delivery.UID)
	}
}

func TestEventIntegrationSuiteTest(t *testing.T) {
	suite.Run(t, new(EventIntegrationTestSuite))
}
