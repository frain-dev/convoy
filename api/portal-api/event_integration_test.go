//go:build integration
// +build integration

package portalapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/oklog/ulid/v2"

	"github.com/frain-dev/convoy/database"
	"github.com/frain-dev/convoy/database/postgres"
	"github.com/frain-dev/convoy/internal/pkg/metrics"

	"github.com/frain-dev/convoy/api/testdb"
	"github.com/frain-dev/convoy/auth"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/datastore"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type EventIntegrationTestSuite struct {
	suite.Suite
	DB             database.Database
	Router         http.Handler
	ConvoyApp      *PortalLinkHandler
	DefaultProject *datastore.Project
	APIKey         string
}

func (s *EventIntegrationTestSuite) SetupSuite() {
	s.DB = getDB()
	s.ConvoyApp = buildServer()
	s.Router = s.ConvoyApp.BuildRoutes()
}

func (s *EventIntegrationTestSuite) SetupTest() {
	testdb.PurgeDB(s.T(), s.DB)

	user, err := testdb.SeedDefaultUser(s.ConvoyApp.A.DB)
	require.NoError(s.T(), err)

	org, err := testdb.SeedDefaultOrganisation(s.ConvoyApp.A.DB, user)
	require.NoError(s.T(), err)

	// Setup Default Project.
	s.DefaultProject, _ = testdb.SeedDefaultProject(s.ConvoyApp.A.DB, org.UID)

	// Seed Auth
	role := auth.Role{
		Type:    auth.RoleAdmin,
		Project: s.DefaultProject.UID,
	}

	_, s.APIKey, _ = testdb.SeedAPIKey(s.ConvoyApp.A.DB, role, "", "test", "", "")

	// Setup Config.
	err = config.LoadConfig("../testdata/Auth_Config/full-convoy.json")
	require.NoError(s.T(), err)

	apiRepo := postgres.NewAPIKeyRepo(s.ConvoyApp.A.DB)
	userRepo := postgres.NewUserRepo(s.ConvoyApp.A.DB)
	initRealmChain(s.T(), apiRepo, userRepo, s.ConvoyApp.A.Cache)
}

func (s *EventIntegrationTestSuite) TearDownTest() {
	testdb.PurgeDB(s.T(), s.DB)
	metrics.Reset()
}

func (s *EventIntegrationTestSuite) Test_CreateEndpointEvent() {
	endpointID := ulid.Make().String()
	expectedStatusCode := http.StatusCreated

	// Just Before.
	_, _ = testdb.SeedEndpoint(s.ConvoyApp.A.DB, s.DefaultProject, endpointID, "", "", false, datastore.ActiveEndpointStatus)

	bodyStr := `{"endpoint_id": "%s", "event_type":"*", "data":{"level":"test"}}`
	body := serialize(bodyStr, endpointID)

	url := fmt.Sprintf("/api/v1/projects/%s/events", s.DefaultProject.UID)
	req := createRequest(http.MethodPost, url, s.APIKey, body)
	w := httptest.NewRecorder()
	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)

	// Deep Assert.
	var event datastore.Event
	parseResponse(s.T(), w.Result(), &event)

	require.NotEmpty(s.T(), event.UID)
	require.Equal(s.T(), event.Endpoints[0], endpointID)
}

func (s *EventIntegrationTestSuite) Test_CreateFanoutEvent_MultipleEndpoints() {
	endpointID := ulid.Make().String()
	expectedStatusCode := http.StatusCreated
	ownerID := ulid.Make().String()

	// Just Before.
	_, _ = testdb.SeedEndpoint(s.ConvoyApp.A.DB, s.DefaultProject, endpointID, "", ownerID, false, datastore.ActiveEndpointStatus)
	_, _ = testdb.SeedEndpoint(s.ConvoyApp.A.DB, s.DefaultProject, ulid.Make().String(), "", ownerID, false, datastore.ActiveEndpointStatus)

	bodyStr := `{"owner_id":"%s", "event_type":"*", "data":{"level":"test"}}`
	body := serialize(bodyStr, ownerID)

	url := fmt.Sprintf("/api/v1/projects/%s/events/fanout", s.DefaultProject.UID)
	req := createRequest(http.MethodPost, url, s.APIKey, body)
	w := httptest.NewRecorder()
	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)

	// Deep Assert.
	var event datastore.Event
	parseResponse(s.T(), w.Result(), &event)

	require.NotEmpty(s.T(), event.UID)
	require.Equal(s.T(), event.Endpoints[0], endpointID)
	require.Equal(s.T(), 2, len(event.Endpoints))
}

func (s *EventIntegrationTestSuite) Test_CreateEndpointEvent_With_App_ID_Valid_Event() {
	endpointID := ulid.Make().String()
	appID := ulid.Make().String()
	expectedStatusCode := http.StatusCreated

	// Just Before.
	// Create an Endpoint with an app ID
	endpoint := &datastore.Endpoint{
		UID:       endpointID,
		Title:     fmt.Sprintf("TestEndpoint-%s", endpointID),
		ProjectID: s.DefaultProject.UID,
		AppID:     appID,
		Secrets: datastore.Secrets{
			{UID: ulid.Make().String()},
		},
		Status: datastore.ActiveEndpointStatus,
	}

	err := postgres.NewEndpointRepo(s.ConvoyApp.A.DB).CreateEndpoint(context.TODO(), endpoint, s.DefaultProject.UID)
	require.NoError(s.T(), err)

	bodyStr := `{"app_id":"%s", "event_type":"*", "data":{"level":"test"}}`
	body := serialize(bodyStr, appID)

	url := fmt.Sprintf("/api/v1/projects/%s/events", s.DefaultProject.UID)
	req := createRequest(http.MethodPost, url, s.APIKey, body)
	w := httptest.NewRecorder()
	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)

	// Deep Assert.
	var event datastore.Event
	parseResponse(s.T(), w.Result(), &event)

	require.NotEmpty(s.T(), event.UID)
	require.Equal(s.T(), event.Endpoints[0], endpointID)
}

func (s *EventIntegrationTestSuite) Test_CreateEndpointEvent_Endpoint_is_disabled() {
	endpointID := ulid.Make().String()
	expectedStatusCode := http.StatusCreated

	// Just Before.
	_, _ = testdb.SeedEndpoint(s.ConvoyApp.A.DB, s.DefaultProject, endpointID, "", "", true, datastore.ActiveEndpointStatus)

	bodyStr := `{"endpoint_id": "%s", "event_type":"*", "data":{"level":"test"}}`
	body := serialize(bodyStr, endpointID)

	url := fmt.Sprintf("/api/v1/projects/%s/events", s.DefaultProject.UID)
	req := createRequest(http.MethodPost, url, s.APIKey, body)
	w := httptest.NewRecorder()
	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)
}

func (s *EventIntegrationTestSuite) Test_GetEndpointEvent_Valid_Event() {
	eventID := ulid.Make().String()
	expectedStatusCode := http.StatusOK

	// Just Before.
	endpoint, err := testdb.SeedEndpoint(s.ConvoyApp.A.DB, s.DefaultProject, ulid.Make().String(), "", "", false, datastore.ActiveEndpointStatus)
	require.NoError(s.T(), err)

	event, err := testdb.SeedEvent(s.ConvoyApp.A.DB, endpoint, s.DefaultProject.UID, eventID, "*", "", []byte(`{}`))
	require.NoError(s.T(), err)

	url := fmt.Sprintf("/api/v1/projects/%s/events/%s", s.DefaultProject.UID, eventID)
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

func (s *EventIntegrationTestSuite) Test_CreateEndpointEvent_Valid_Event_RedirectToProjects() {
	endpointID := ulid.Make().String()
	expectedStatusCode := http.StatusTemporaryRedirect

	// Just Before.
	_, _ = testdb.SeedEndpoint(s.ConvoyApp.A.DB, s.DefaultProject, endpointID, "", "", false, datastore.ActiveEndpointStatus)

	bodyStr := `{"app_id":"%s", "event_type":"*", "data":{"level":"test"}}`
	body := serialize(bodyStr, endpointID)

	url := fmt.Sprintf("/api/v1/events?projectID=%s", s.DefaultProject.UID)
	req := createRequest(http.MethodPost, url, s.APIKey, body)
	w := httptest.NewRecorder()
	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)
}

func (s *EventIntegrationTestSuite) Test_ReplayEndpointEvent_Valid_Event() {
	eventID := ulid.Make().String()
	expectedStatusCode := http.StatusOK

	// Just Before.
	endpoint, _ := testdb.SeedEndpoint(s.ConvoyApp.A.DB, s.DefaultProject, ulid.Make().String(), "", "", false, datastore.ActiveEndpointStatus)
	_, _ = testdb.SeedEvent(s.ConvoyApp.A.DB, endpoint, s.DefaultProject.UID, eventID, "*", "", []byte(`{}`))

	url := fmt.Sprintf("/api/v1/projects/%s/events/%s/replay", s.DefaultProject.UID, eventID)
	req := createRequest(http.MethodPut, url, s.APIKey, nil)
	w := httptest.NewRecorder()

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)
}

func (s *EventIntegrationTestSuite) Test_GetEndpointEvent_Event_not_found() {
	eventID := ulid.Make().String()
	expectedStatusCode := http.StatusNotFound

	url := fmt.Sprintf("/api/v1/projects/%s/events/%s", s.DefaultProject.UID, eventID)
	req := createRequest(http.MethodGet, url, s.APIKey, nil)
	w := httptest.NewRecorder()

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)
}

func (s *EventIntegrationTestSuite) Test_GetEventDelivery_Valid_EventDelivery() {
	eventDeliveryID := ulid.Make().String()
	expectedStatusCode := http.StatusOK

	// Just Before.
	endpoint, _ := testdb.SeedEndpoint(s.ConvoyApp.A.DB, s.DefaultProject, ulid.Make().String(), "", "", false, datastore.ActiveEndpointStatus)
	event, _ := testdb.SeedEvent(s.ConvoyApp.A.DB, endpoint, s.DefaultProject.UID, ulid.Make().String(), "*", "", []byte(`{}`))
	subscription, err := testdb.SeedSubscription(s.ConvoyApp.A.DB, s.DefaultProject, ulid.Make().String(), datastore.OutgoingProject, &datastore.Source{}, endpoint, &datastore.RetryConfiguration{}, &datastore.AlertConfiguration{}, &datastore.FilterConfiguration{
		EventTypes: []string{"*"},
		Filter:     datastore.FilterSchema{Headers: datastore.M{}, Body: datastore.M{}},
	})
	require.NoError(s.T(), err)

	eventDelivery, err := testdb.SeedEventDelivery(s.ConvoyApp.A.DB, event, endpoint, s.DefaultProject.UID, eventDeliveryID, datastore.FailureEventStatus, subscription)
	require.NoError(s.T(), err)

	url := fmt.Sprintf("/api/v1/projects/%s/eventdeliveries/%s", s.DefaultProject.UID, eventDeliveryID)
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

func (s *EventIntegrationTestSuite) Test_GetEventDelivery_Valid_EventDelivery_RedirectToProjects() {
	eventDeliveryID := ulid.Make().String()
	expectedStatusCode := http.StatusTemporaryRedirect

	// Just Before.
	endpoint, _ := testdb.SeedEndpoint(s.ConvoyApp.A.DB, s.DefaultProject, ulid.Make().String(), "", "", false, datastore.ActiveEndpointStatus)
	_, _ = testdb.SeedEventDelivery(s.ConvoyApp.A.DB, &datastore.Event{}, endpoint, s.DefaultProject.UID, eventDeliveryID, datastore.SuccessEventStatus, &datastore.Subscription{})

	url := fmt.Sprintf("/api/v1/eventdeliveries/%s?groupID=%s", eventDeliveryID, s.DefaultProject.UID)
	req := createRequest(http.MethodGet, url, s.APIKey, nil)
	w := httptest.NewRecorder()

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)
}

func (s *EventIntegrationTestSuite) Test_GetEventDelivery_Event_not_found() {
	eventDeliveryID := ulid.Make().String()
	expectedStatusCode := http.StatusNotFound

	url := fmt.Sprintf("/api/v1/projects/%s/eventdeliveries/%s", s.DefaultProject.UID, eventDeliveryID)
	req := createRequest(http.MethodGet, url, s.APIKey, nil)
	w := httptest.NewRecorder()

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)
}

func (s *EventIntegrationTestSuite) Test_ResendEventDelivery_Valid_Resend() {
	eventDeliveryID := ulid.Make().String()
	expectedStatusCode := http.StatusOK

	// Just Before.
	endpoint, err := testdb.SeedEndpoint(s.ConvoyApp.A.DB, s.DefaultProject, ulid.Make().String(), "", "", false, datastore.ActiveEndpointStatus)
	require.NoError(s.T(), err)

	event, err := testdb.SeedEvent(s.ConvoyApp.A.DB, endpoint, s.DefaultProject.UID, ulid.Make().String(), "*", "", []byte(`{}`))
	require.NoError(s.T(), err)

	subscription, err := testdb.SeedSubscription(s.ConvoyApp.A.DB, s.DefaultProject, ulid.Make().String(), datastore.OutgoingProject, &datastore.Source{}, endpoint, &datastore.RetryConfiguration{}, &datastore.AlertConfiguration{}, &datastore.FilterConfiguration{
		EventTypes: []string{"*"},
		Filter:     datastore.FilterSchema{Headers: datastore.M{}, Body: datastore.M{}},
	})
	require.NoError(s.T(), err)

	eventDelivery, err := testdb.SeedEventDelivery(s.ConvoyApp.A.DB, event, endpoint, s.DefaultProject.UID, eventDeliveryID, datastore.FailureEventStatus, subscription)
	require.NoError(s.T(), err)

	url := fmt.Sprintf("/api/v1/projects/%s/eventdeliveries/%s/resend", s.DefaultProject.UID, eventDeliveryID)
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
	expectedStatusCode := http.StatusOK

	// Just Before.
	endpoint, _ := testdb.SeedEndpoint(s.ConvoyApp.A.DB, s.DefaultProject, ulid.Make().String(), "", "", false, datastore.ActiveEndpointStatus)
	event, _ := testdb.SeedEvent(s.ConvoyApp.A.DB, endpoint, s.DefaultProject.UID, ulid.Make().String(), "*", "", []byte(`{}`))
	subscription, err := testdb.SeedSubscription(s.ConvoyApp.A.DB, s.DefaultProject, ulid.Make().String(), datastore.OutgoingProject, &datastore.Source{}, endpoint, &datastore.RetryConfiguration{}, &datastore.AlertConfiguration{}, &datastore.FilterConfiguration{
		EventTypes: []string{"*"},
		Filter:     datastore.FilterSchema{Headers: datastore.M{}, Body: datastore.M{}},
	})
	require.NoError(s.T(), err)

	_, err = testdb.SeedEventDelivery(s.ConvoyApp.A.DB, event, endpoint, s.DefaultProject.UID, ulid.Make().String(), datastore.FailureEventStatus, subscription)
	require.NoError(s.T(), err)

	_, err = testdb.SeedEventDelivery(s.ConvoyApp.A.DB, event, endpoint, s.DefaultProject.UID, ulid.Make().String(), datastore.FailureEventStatus, subscription)
	require.NoError(s.T(), err)

	_, err = testdb.SeedEventDelivery(s.ConvoyApp.A.DB, event, endpoint, s.DefaultProject.UID, ulid.Make().String(), datastore.FailureEventStatus, subscription)
	require.NoError(s.T(), err)

	url := fmt.Sprintf("/api/v1/projects/%s/eventdeliveries/batchretry?endpointId=%s&eventId=%s&status=%s", s.DefaultProject.UID, endpoint.UID, event.UID, datastore.FailureEventStatus)
	req := createRequest(http.MethodPost, url, s.APIKey, nil)
	w := httptest.NewRecorder()

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)
}

func (s *EventIntegrationTestSuite) Test_CountAffectedEventDeliveries_Valid_Filters() {
	expectedStatusCode := http.StatusOK

	// Just Before.
	endpoint, _ := testdb.SeedEndpoint(s.ConvoyApp.A.DB, s.DefaultProject, ulid.Make().String(), "", "", false, datastore.ActiveEndpointStatus)
	event, _ := testdb.SeedEvent(s.ConvoyApp.A.DB, endpoint, s.DefaultProject.UID, ulid.Make().String(), "*", "", []byte(`{}`))
	subscription, err := testdb.SeedSubscription(s.ConvoyApp.A.DB, s.DefaultProject, ulid.Make().String(), datastore.OutgoingProject, &datastore.Source{}, endpoint, &datastore.RetryConfiguration{}, &datastore.AlertConfiguration{}, &datastore.FilterConfiguration{
		EventTypes: []string{"*"},
		Filter:     datastore.FilterSchema{Headers: datastore.M{}, Body: datastore.M{}},
	})
	require.NoError(s.T(), err)

	_, err = testdb.SeedEventDelivery(s.ConvoyApp.A.DB, event, endpoint, s.DefaultProject.UID, ulid.Make().String(), datastore.FailureEventStatus, subscription)
	require.NoError(s.T(), err)

	_, err = testdb.SeedEventDelivery(s.ConvoyApp.A.DB, event, endpoint, s.DefaultProject.UID, ulid.Make().String(), datastore.FailureEventStatus, subscription)
	require.NoError(s.T(), err)

	_, err = testdb.SeedEventDelivery(s.ConvoyApp.A.DB, event, endpoint, s.DefaultProject.UID, ulid.Make().String(), datastore.FailureEventStatus, subscription)
	require.NoError(s.T(), err)

	url := fmt.Sprintf("/api/v1/projects/%s/eventdeliveries/countbatchretryevents?endpointId=%s&eventId=%s&status=%s", s.DefaultProject.UID, endpoint.UID, event.UID, datastore.FailureEventStatus)
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
	endpoint, _ := testdb.SeedEndpoint(s.ConvoyApp.A.DB, s.DefaultProject, ulid.Make().String(), "", "", false, datastore.ActiveEndpointStatus)
	event, _ := testdb.SeedEvent(s.ConvoyApp.A.DB, endpoint, s.DefaultProject.UID, ulid.Make().String(), "*", "", []byte(`{}`))
	subscription, err := testdb.SeedSubscription(s.ConvoyApp.A.DB, s.DefaultProject, ulid.Make().String(), datastore.OutgoingProject, &datastore.Source{}, endpoint, &datastore.RetryConfiguration{}, &datastore.AlertConfiguration{}, &datastore.FilterConfiguration{
		EventTypes: []string{"*"},
		Filter:     datastore.FilterSchema{Headers: datastore.M{}, Body: datastore.M{}},
	})
	require.NoError(s.T(), err)

	e1, err := testdb.SeedEventDelivery(s.ConvoyApp.A.DB, event, endpoint, s.DefaultProject.UID, ulid.Make().String(), datastore.SuccessEventStatus, subscription)
	require.NoError(s.T(), err)

	e2, _ := testdb.SeedEventDelivery(s.ConvoyApp.A.DB, event, endpoint, s.DefaultProject.UID, ulid.Make().String(), datastore.SuccessEventStatus, subscription)
	e3, _ := testdb.SeedEventDelivery(s.ConvoyApp.A.DB, event, endpoint, s.DefaultProject.UID, ulid.Make().String(), datastore.SuccessEventStatus, subscription)

	url := fmt.Sprintf("/api/v1/projects/%s/eventdeliveries/forceresend", s.DefaultProject.UID)

	bodyStr := `{"ids":["%s", "%s", "%s"]}`
	body := serialize(bodyStr, e1.UID, e2.UID, e3.UID)

	req := createRequest(http.MethodPost, url, s.APIKey, body)
	w := httptest.NewRecorder()

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)

	var response map[string]interface{}
	err = json.NewDecoder(w.Body).Decode(&response)
	require.NoError(s.T(), err)

	require.Equal(s.T(), expectedMessage, response["message"].(string))
}

func (s *EventIntegrationTestSuite) Test_GetEventsPaged() {
	eventID := ulid.Make().String()
	sourceID := ulid.Make().String()
	expectedStatusCode := http.StatusOK

	// Just Before.
	endpoint1, err := testdb.SeedEndpoint(s.ConvoyApp.A.DB, s.DefaultProject, ulid.Make().String(), "", "", false, datastore.ActiveEndpointStatus)
	require.NoError(s.T(), err)

	vc := &datastore.VerifierConfig{
		Type: datastore.BasicAuthVerifier,
		BasicAuth: &datastore.BasicAuth{
			UserName: "Convoy",
			Password: "Convoy",
		},
	}

	_, err = testdb.SeedSource(s.ConvoyApp.A.DB, s.DefaultProject, sourceID, ulid.Make().String(), "", vc)
	require.NoError(s.T(), err)

	e1, err := testdb.SeedEvent(s.ConvoyApp.A.DB, endpoint1, s.DefaultProject.UID, eventID, "*", sourceID, []byte(`{}`))
	require.NoError(s.T(), err)

	e2, err := testdb.SeedEvent(s.ConvoyApp.A.DB, endpoint1, s.DefaultProject.UID, ulid.Make().String(), "*", sourceID, []byte(`{}`))
	require.NoError(s.T(), err)

	endpoint2, err := testdb.SeedEndpoint(s.ConvoyApp.A.DB, s.DefaultProject, ulid.Make().String(), "", "", false, datastore.ActiveEndpointStatus)
	require.NoError(s.T(), err)

	_, err = testdb.SeedEvent(s.ConvoyApp.A.DB, endpoint2, s.DefaultProject.UID, ulid.Make().String(), "*", sourceID, []byte(`{}`))
	require.NoError(s.T(), err)

	url := fmt.Sprintf("/api/v1/projects/%s/events?endpointId=%s&sourceId=%s", s.DefaultProject.UID, endpoint1.UID, sourceID)
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
	require.Equal(s.T(), 2, len(respEvents))

	v := []string{e1.UID, e2.UID}
	for i := range respEvents {
		require.Contains(s.T(), v, respEvents[i].UID)
	}
}

func (s *EventIntegrationTestSuite) Test_GetEventDeliveriesPaged() {
	eventDeliveryID := ulid.Make().String()
	expectedStatusCode := http.StatusOK

	// Just Before.
	endpoint1, err := testdb.SeedEndpoint(s.ConvoyApp.A.DB, s.DefaultProject, ulid.Make().String(), "", "", false, datastore.ActiveEndpointStatus)
	require.NoError(s.T(), err)

	subscription, err := testdb.SeedSubscription(s.ConvoyApp.A.DB, s.DefaultProject, ulid.Make().String(), datastore.OutgoingProject, &datastore.Source{}, endpoint1, &datastore.RetryConfiguration{}, &datastore.AlertConfiguration{}, &datastore.FilterConfiguration{
		EventTypes: []string{"*"},
		Filter:     datastore.FilterSchema{Headers: datastore.M{}, Body: datastore.M{}},
	})
	require.NoError(s.T(), err)

	event1, err := testdb.SeedEvent(s.ConvoyApp.A.DB, endpoint1, s.DefaultProject.UID, ulid.Make().String(), "*", "", []byte(`{}`))
	require.NoError(s.T(), err)

	d1, err := testdb.SeedEventDelivery(s.ConvoyApp.A.DB, event1, endpoint1, s.DefaultProject.UID, eventDeliveryID, datastore.FailureEventStatus, subscription)
	require.NoError(s.T(), err)

	d2, err := testdb.SeedEventDelivery(s.ConvoyApp.A.DB, event1, endpoint1, s.DefaultProject.UID, ulid.Make().String(), datastore.FailureEventStatus, subscription)
	require.NoError(s.T(), err)

	endpoint2, err := testdb.SeedEndpoint(s.ConvoyApp.A.DB, s.DefaultProject, ulid.Make().String(), "", "", false, datastore.ActiveEndpointStatus)
	require.NoError(s.T(), err)

	event2, err := testdb.SeedEvent(s.ConvoyApp.A.DB, endpoint2, s.DefaultProject.UID, ulid.Make().String(), "*", "", []byte(`{}`))
	require.NoError(s.T(), err)

	_, err = testdb.SeedEventDelivery(s.ConvoyApp.A.DB, event2, endpoint2, s.DefaultProject.UID, ulid.Make().String(), datastore.FailureEventStatus, subscription)
	require.NoError(s.T(), err)

	url := fmt.Sprintf("/api/v1/projects/%s/eventdeliveries?endpointId=%s", s.DefaultProject.UID, endpoint1.UID)
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
	require.Equal(s.T(), 2, len(respEvents))

	v := []*datastore.EventDelivery{d2, d1}
	for i, delivery := range v {
		require.Equal(s.T(), respEvents[i].UID, delivery.UID)
	}
}

func TestEventIntegrationSuiteTest(t *testing.T) {
	suite.Run(t, new(EventIntegrationTestSuite))
}
