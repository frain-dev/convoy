//go:build integration
// +build integration

package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/frain-dev/convoy/internal/pkg/metrics"

	"github.com/google/uuid"

	"github.com/frain-dev/convoy/auth"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/datastore"
	cm "github.com/frain-dev/convoy/datastore/mongo"
	"github.com/frain-dev/convoy/server/testdb"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type EventIntegrationTestSuite struct {
	suite.Suite
	DB           cm.Client
	Router       http.Handler
	ConvoyApp    *ApplicationHandler
	DefaultGroup *datastore.Group
	APIKey       string
}

func (s *EventIntegrationTestSuite) SetupSuite() {
	s.DB = getDB()
	s.ConvoyApp = buildServer()
	s.Router = s.ConvoyApp.BuildRoutes()
}

func (s *EventIntegrationTestSuite) SetupTest() {
	testdb.PurgeDB(s.DB)

	// Setup Default Group.
	s.DefaultGroup, _ = testdb.SeedDefaultGroup(s.ConvoyApp.A.Store, "")

	// Seed Auth
	role := auth.Role{
		Type:  auth.RoleAdmin,
		Group: s.DefaultGroup.UID,
	}

	_, s.APIKey, _ = testdb.SeedAPIKey(s.ConvoyApp.A.Store, role, "", "test", "")

	// Setup Config.
	err := config.LoadConfig("./testdata/Auth_Config/full-convoy.json")
	require.NoError(s.T(), err)

	apiRepo := cm.NewApiKeyRepo(s.ConvoyApp.A.Store)
	userRepo := cm.NewUserRepo(s.ConvoyApp.A.Store)
	initRealmChain(s.T(), apiRepo, userRepo, s.ConvoyApp.A.Cache)
}

func (s *EventIntegrationTestSuite) TearDownTest() {
	testdb.PurgeDB(s.DB)
	metrics.Reset()
}

func (s *EventIntegrationTestSuite) Test_CreateAppEvent_Valid_Event() {
	appID := uuid.NewString()
	expectedStatusCode := http.StatusCreated

	// Just Before.
	app, _ := testdb.SeedApplication(s.ConvoyApp.A.Store, s.DefaultGroup, appID, "", false)
	_, _ = testdb.SeedMultipleEndpoints(s.ConvoyApp.A.Store, app, s.DefaultGroup.UID, []string{"*"}, 2)

	bodyStr := `{"app_id":"%s", "event_type":"*", "data":{"level":"test"}}`
	body := serialize(bodyStr, appID)

	req := createRequest(http.MethodPost, "/api/v1/events", s.APIKey, body)
	w := httptest.NewRecorder()
	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)

	// Deep Assert.
	var event datastore.Event
	parseResponse(s.T(), w.Result(), &event)

	require.NotEmpty(s.T(), event.UID)
	require.Equal(s.T(), event.AppID, appID)
}

func (s *EventIntegrationTestSuite) Test_CreateAppEvent_App_has_no_endpoint() {
	appID := uuid.NewString()
	expectedStatusCode := http.StatusBadRequest

	// Just Before.
	_, _ = testdb.SeedApplication(s.ConvoyApp.A.Store, s.DefaultGroup, appID, "", false)

	bodyStr := `{"app_id":"%s", "event_type":"*", "data":{"level":"test"}}`
	body := serialize(bodyStr, appID)

	req := createRequest(http.MethodPost, "/api/v1/events", s.APIKey, body)
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
	app, _ := testdb.SeedApplication(s.ConvoyApp.A.Store, s.DefaultGroup, appID, "", true)
	_, _ = testdb.SeedMultipleEndpoints(s.ConvoyApp.A.Store, app, s.DefaultGroup.UID, []string{"*"}, 2)

	bodyStr := `{"app_id":"%s", "event_type":"*", "data":{"level":"test"}}`
	body := serialize(bodyStr, appID)

	req := createRequest(http.MethodPost, "/api/v1/events", s.APIKey, body)
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
	app, _ := testdb.SeedApplication(s.ConvoyApp.A.Store, s.DefaultGroup, uuid.NewString(), "", false)
	event, _ := testdb.SeedEvent(s.ConvoyApp.A.Store, app, s.DefaultGroup.UID, eventID, "*", []byte(`{}`))

	url := fmt.Sprintf("/api/v1/events/%s", eventID)
	req := createRequest(http.MethodGet, url, s.APIKey, nil)
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
	app, _ := testdb.SeedApplication(s.ConvoyApp.A.Store, s.DefaultGroup, uuid.NewString(), "", false)
	_, _ = testdb.SeedEvent(s.ConvoyApp.A.Store, app, s.DefaultGroup.UID, eventID, "*", []byte(`{}`))

	url := fmt.Sprintf("/api/v1/events/%s/replay", eventID)
	req := createRequest(http.MethodPut, url, s.APIKey, nil)
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
	req := createRequest(http.MethodGet, url, s.APIKey, nil)
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
	app, _ := testdb.SeedApplication(s.ConvoyApp.A.Store, s.DefaultGroup, uuid.NewString(), "", false)
	eventDelivery, _ := testdb.SeedEventDelivery(s.ConvoyApp.A.Store, app, &datastore.Event{}, &datastore.Endpoint{}, s.DefaultGroup.UID, eventDeliveryID, datastore.SuccessEventStatus, &datastore.Subscription{})

	url := fmt.Sprintf("/api/v1/eventdeliveries/%s", eventDeliveryID)
	req := createRequest(http.MethodGet, url, s.APIKey, nil)
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
	req := createRequest(http.MethodGet, url, s.APIKey, nil)
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
	app, _ := testdb.SeedApplication(s.ConvoyApp.A.Store, s.DefaultGroup, uuid.NewString(), "", false)
	_, _ = testdb.SeedEndpoint(s.ConvoyApp.A.Store, app, s.DefaultGroup.UID)
	subscription, _ := testdb.SeedSubscription(s.ConvoyApp.A.Store, app, s.DefaultGroup, uuid.NewString(), datastore.OutgoingGroup, &datastore.Source{}, &datastore.Endpoint{}, &datastore.RetryConfiguration{}, &datastore.AlertConfiguration{}, &datastore.FilterConfiguration{}, "")
	eventDelivery, _ := testdb.SeedEventDelivery(s.ConvoyApp.A.Store, app, &datastore.Event{}, &app.Endpoints[0], s.DefaultGroup.UID, eventDeliveryID, datastore.FailureEventStatus, subscription)

	url := fmt.Sprintf("/api/v1/eventdeliveries/%s/resend", eventDeliveryID)
	req := createRequest(http.MethodPut, url, s.APIKey, nil)
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
	app, _ := testdb.SeedApplication(s.ConvoyApp.A.Store, s.DefaultGroup, uuid.NewString(), "", false)
	_, _ = testdb.SeedEndpoint(s.ConvoyApp.A.Store, app, s.DefaultGroup.UID)
	event, _ := testdb.SeedEvent(s.ConvoyApp.A.Store, app, s.DefaultGroup.UID, uuid.NewString(), "*", []byte(`{}`))
	_, _ = testdb.SeedEventDelivery(s.ConvoyApp.A.Store, app, event, &app.Endpoints[0], s.DefaultGroup.UID, eventDeliveryID, datastore.FailureEventStatus, &datastore.Subscription{})
	_, _ = testdb.SeedEventDelivery(s.ConvoyApp.A.Store, app, event, &app.Endpoints[0], s.DefaultGroup.UID, eventDeliveryID, datastore.FailureEventStatus, &datastore.Subscription{})
	_, _ = testdb.SeedEventDelivery(s.ConvoyApp.A.Store, app, event, &app.Endpoints[0], s.DefaultGroup.UID, eventDeliveryID, datastore.FailureEventStatus, &datastore.Subscription{})

	url := fmt.Sprintf("/api/v1/eventdeliveries/batchretry?appId=%s&eventId=%s&status=%s", app.UID, event.UID, datastore.FailureEventStatus)
	req := createRequest(http.MethodPost, url, s.APIKey, nil)
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
	app, _ := testdb.SeedApplication(s.ConvoyApp.A.Store, s.DefaultGroup, uuid.NewString(), "", false)
	_, _ = testdb.SeedEndpoint(s.ConvoyApp.A.Store, app, s.DefaultGroup.UID)
	event, _ := testdb.SeedEvent(s.ConvoyApp.A.Store, app, s.DefaultGroup.UID, uuid.NewString(), "*", []byte(`{}`))
	subscription, _ := testdb.SeedSubscription(s.ConvoyApp.A.Store, app, s.DefaultGroup, uuid.NewString(), datastore.OutgoingGroup, &datastore.Source{}, &datastore.Endpoint{}, &datastore.RetryConfiguration{}, &datastore.AlertConfiguration{}, &datastore.FilterConfiguration{}, "")
	_, _ = testdb.SeedEventDelivery(s.ConvoyApp.A.Store, app, event, &app.Endpoints[0], s.DefaultGroup.UID, eventDeliveryID, datastore.FailureEventStatus, subscription)
	_, _ = testdb.SeedEventDelivery(s.ConvoyApp.A.Store, app, event, &app.Endpoints[0], s.DefaultGroup.UID, eventDeliveryID, datastore.FailureEventStatus, subscription)
	_, _ = testdb.SeedEventDelivery(s.ConvoyApp.A.Store, app, event, &app.Endpoints[0], s.DefaultGroup.UID, eventDeliveryID, datastore.FailureEventStatus, subscription)

	url := fmt.Sprintf("/api/v1/eventdeliveries/countbatchretryevents?appId=%s&eventId=%s&status=%s", app.UID, event.UID, datastore.FailureEventStatus)
	req := createRequest(http.MethodGet, url, s.APIKey, nil)
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
	expectedStatusCode := http.StatusOK
	expectedMessage := "3 successful, 0 failed"

	// Just Before.
	app, _ := testdb.SeedApplication(s.ConvoyApp.A.Store, s.DefaultGroup, uuid.NewString(), "", false)
	_, _ = testdb.SeedEndpoint(s.ConvoyApp.A.Store, app, s.DefaultGroup.UID)
	event, _ := testdb.SeedEvent(s.ConvoyApp.A.Store, app, s.DefaultGroup.UID, uuid.NewString(), "*", []byte(`{}`))
	subscription, _ := testdb.SeedSubscription(s.ConvoyApp.A.Store, app, s.DefaultGroup, uuid.NewString(), datastore.OutgoingGroup, &datastore.Source{}, &datastore.Endpoint{}, &datastore.RetryConfiguration{}, &datastore.AlertConfiguration{}, &datastore.FilterConfiguration{}, "")
	e1, _ := testdb.SeedEventDelivery(s.ConvoyApp.A.Store, app, event, &app.Endpoints[0], s.DefaultGroup.UID, uuid.NewString(), datastore.SuccessEventStatus, subscription)
	e2, _ := testdb.SeedEventDelivery(s.ConvoyApp.A.Store, app, event, &app.Endpoints[0], s.DefaultGroup.UID, uuid.NewString(), datastore.SuccessEventStatus, subscription)
	e3, _ := testdb.SeedEventDelivery(s.ConvoyApp.A.Store, app, event, &app.Endpoints[0], s.DefaultGroup.UID, uuid.NewString(), datastore.SuccessEventStatus, subscription)

	url := "/api/v1/eventdeliveries/forceresend"

	bodyStr := `{"ids":["%s", "%s", "%s"]}`
	body := serialize(bodyStr, e1.UID, e2.UID, e3.UID)

	req := createRequest(http.MethodPost, url, s.APIKey, body)
	w := httptest.NewRecorder()

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)

	var response map[string]interface{}
	err := json.NewDecoder(w.Body).Decode(&response)
	require.NoError(s.T(), err)

	require.Equal(s.T(), expectedMessage, response["message"].(string))
}

func (s *EventIntegrationTestSuite) Test_GetEventsPaged() {
	eventID := uuid.NewString()
	expectedStatusCode := http.StatusOK

	// Just Before.
	app1, _ := testdb.SeedApplication(s.ConvoyApp.A.Store, s.DefaultGroup, uuid.NewString(), "", false)
	e1, _ := testdb.SeedEvent(s.ConvoyApp.A.Store, app1, s.DefaultGroup.UID, eventID, "*", []byte(`{}`))
	e2, _ := testdb.SeedEvent(s.ConvoyApp.A.Store, app1, s.DefaultGroup.UID, eventID, "*", []byte(`{}`))

	app2, _ := testdb.SeedApplication(s.ConvoyApp.A.Store, s.DefaultGroup, uuid.NewString(), "", false)
	_, _ = testdb.SeedEvent(s.ConvoyApp.A.Store, app2, s.DefaultGroup.UID, eventID, "*", []byte(`{}`))

	url := fmt.Sprintf("/api/v1/events?appId=%s", app1.UID)
	req := createRequest(http.MethodGet, url, s.APIKey, nil)
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

func (s *EventIntegrationTestSuite) Test_GetEventDeliveriesPaged() {
	eventDeliveryID := uuid.NewString()
	expectedStatusCode := http.StatusOK

	// Just Before.
	app1, _ := testdb.SeedApplication(s.ConvoyApp.A.Store, s.DefaultGroup, uuid.NewString(), "", false)
	endpoint1, _ := testdb.SeedEndpoint(s.ConvoyApp.A.Store, app1, s.DefaultGroup.UID)
	event1, _ := testdb.SeedEvent(s.ConvoyApp.A.Store, app1, s.DefaultGroup.UID, uuid.NewString(), "*", []byte(`{}`))
	d1, _ := testdb.SeedEventDelivery(s.ConvoyApp.A.Store, app1, event1, endpoint1, s.DefaultGroup.UID, eventDeliveryID, datastore.FailureEventStatus, &datastore.Subscription{})
	d2, _ := testdb.SeedEventDelivery(s.ConvoyApp.A.Store, app1, event1, endpoint1, s.DefaultGroup.UID, eventDeliveryID, datastore.FailureEventStatus, &datastore.Subscription{})

	app2, _ := testdb.SeedApplication(s.ConvoyApp.A.Store, s.DefaultGroup, uuid.NewString(), "", false)
	endpoint2, _ := testdb.SeedEndpoint(s.ConvoyApp.A.Store, app2, s.DefaultGroup.UID)
	event2, _ := testdb.SeedEvent(s.ConvoyApp.A.Store, app2, s.DefaultGroup.UID, uuid.NewString(), "*", []byte(`{}`))
	_, _ = testdb.SeedEventDelivery(s.ConvoyApp.A.Store, app2, event2, endpoint2, s.DefaultGroup.UID, eventDeliveryID, datastore.FailureEventStatus, &datastore.Subscription{})

	url := fmt.Sprintf("/api/v1/eventdeliveries?appId=%s", app1.UID)
	req := createRequest(http.MethodGet, url, s.APIKey, nil)
	w := httptest.NewRecorder()

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)

	// Deep Assert.
	var respEvents []datastore.EventDelivery
	resp := pagedResponse{Content: &respEvents}
	parseResponse(s.T(), w.Result(), &resp)
	require.Equal(s.T(), int64(2), resp.Pagination.Total)
	require.Equal(s.T(), 2, len(respEvents))

	v := []*datastore.EventDelivery{d1, d2}
	for i, delivery := range v {
		require.Equal(s.T(), respEvents[i].UID, delivery.UID)
	}
}

func TestEventIntegrationSuiteTest(t *testing.T) {
	suite.Run(t, new(EventIntegrationTestSuite))
}
