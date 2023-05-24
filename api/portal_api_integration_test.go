//go:build integration
// +build integration

package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/frain-dev/convoy/api/testdb"
	"github.com/frain-dev/convoy/auth"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/database"
	"github.com/frain-dev/convoy/database/postgres"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/internal/pkg/metrics"
	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type PortalEndpointIntegrationTestSuite struct {
	suite.Suite
	DB             database.Database
	Router         http.Handler
	ConvoyApp      *ApplicationHandler
	DefaultOrg     *datastore.Organisation
	DefaultProject *datastore.Project
	DefaultUser    *datastore.User
	APIKey         string
	PersonalAPIKey string
}

func (s *PortalEndpointIntegrationTestSuite) SetupSuite() {
	s.DB = getDB()
	s.ConvoyApp = buildServer()
	s.Router = s.ConvoyApp.BuildRoutes()
}

func (s *PortalEndpointIntegrationTestSuite) SetupTest() {
	testdb.PurgeDB(s.T(), s.DB)

	user, err := testdb.SeedDefaultUser(s.ConvoyApp.A.DB)
	require.NoError(s.T(), err)
	s.DefaultUser = user

	org, err := testdb.SeedDefaultOrganisation(s.ConvoyApp.A.DB, user)
	require.NoError(s.T(), err)
	s.DefaultOrg = org

	// Setup Default Project.
	s.DefaultProject, err = testdb.SeedDefaultProject(s.ConvoyApp.A.DB, s.DefaultOrg.UID)
	require.NoError(s.T(), err)

	// Seed Auth
	role := auth.Role{
		Type:    auth.RoleAdmin,
		Project: s.DefaultProject.UID,
	}

	_, s.APIKey, err = testdb.SeedAPIKey(s.ConvoyApp.A.DB, role, "", "test", "", "")
	require.NoError(s.T(), err)

	_, s.PersonalAPIKey, err = testdb.SeedAPIKey(s.ConvoyApp.A.DB, auth.Role{}, ulid.Make().String(), "test-personal-key", string(datastore.PersonalKey), s.DefaultUser.UID)
	require.NoError(s.T(), err)

	// Setup Config.
	err = config.LoadConfig("./testdata/Auth_Config/full-convoy.json")
	require.NoError(s.T(), err)

	apiRepo := postgres.NewAPIKeyRepo(s.ConvoyApp.A.DB)
	userRepo := postgres.NewUserRepo(s.ConvoyApp.A.DB)
	initRealmChain(s.T(), apiRepo, userRepo, s.ConvoyApp.A.Cache)
}

func (s *PortalEndpointIntegrationTestSuite) TearDownTest() {
	testdb.PurgeDB(s.T(), s.DB)
	metrics.Reset()
}

func (s *PortalEndpointIntegrationTestSuite) Test_GetEndpoint_EndpointNotFound() {
	appID := "123"
	expectedStatusCode := http.StatusNotFound

	endpoint, err := testdb.SeedEndpoint(s.ConvoyApp.A.DB, s.DefaultProject, "", "", "", true, datastore.ActiveEndpointStatus)
	require.NoError(s.T(), err)

	portalLink, err := testdb.SeedPortalLink(s.ConvoyApp.A.DB, s.DefaultProject, []string{endpoint.UID})
	require.NoError(s.T(), err)

	// Arrange Request.
	url := fmt.Sprintf("/portal-api/endpoints/%s?token=%s", appID, portalLink.Token)
	req := createRequest(http.MethodGet, url, s.APIKey, nil)
	w := httptest.NewRecorder()

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)
}

func (s *PortalEndpointIntegrationTestSuite) Test_GetEndpoint_ValidEndpoint() {
	endpointID := "123456789"
	expectedStatusCode := http.StatusOK

	// Just Before.
	_, _ = testdb.SeedEndpoint(s.ConvoyApp.A.DB, s.DefaultProject, endpointID, "", "", true, datastore.ActiveEndpointStatus)

	portalLink, err := testdb.SeedPortalLink(s.ConvoyApp.A.DB, s.DefaultProject, []string{endpointID})
	require.NoError(s.T(), err)

	// Arrange Request.
	url := fmt.Sprintf("/portal-api/endpoints/%s?token=%s", endpointID, portalLink.Token)
	req := createRequest(http.MethodGet, url, s.APIKey, nil)
	w := httptest.NewRecorder()

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)

	// Deep Assert.
	var endpoint datastore.Endpoint
	parseResponse(s.T(), w.Result(), &endpoint)

	endpointRepo := postgres.NewEndpointRepo(s.ConvoyApp.A.DB)
	dbEndpoint, err := endpointRepo.FindEndpointByID(context.Background(), endpointID, s.DefaultProject.UID)
	require.NoError(s.T(), err)
	require.Equal(s.T(), endpoint.UID, dbEndpoint.UID)
	require.Equal(s.T(), endpoint.Title, dbEndpoint.Title)
}

func (s *PortalLinkIntegrationTestSuite) Test_GetPortalLinkEndpoints() {
	// Just Before
	endpoint1, err := testdb.SeedEndpoint(s.ConvoyApp.A.DB, s.DefaultProject, "", ulid.Make().String(), "", false, datastore.ActiveEndpointStatus)
	require.NoError(s.T(), err)

	endpoint2, err := testdb.SeedEndpoint(s.ConvoyApp.A.DB, s.DefaultProject, ulid.Make().String(), "", "", false, datastore.ActiveEndpointStatus)
	require.NoError(s.T(), err)

	portalLink, _ := testdb.SeedPortalLink(s.ConvoyApp.A.DB, s.DefaultProject, []string{endpoint1.UID, endpoint2.UID})
	require.NoError(s.T(), err)

	// Arrange Request
	url := fmt.Sprintf("/portal-api/endpoints?token=%s", portalLink.Token)
	req := createRequest(http.MethodGet, url, "", nil)
	w := httptest.NewRecorder()

	// Act
	s.Router.ServeHTTP(w, req)

	// Assert
	require.Equal(s.T(), http.StatusOK, w.Code)

	// Deep Assert
	var resp []datastore.Endpoint
	parseResponse(s.T(), w.Result(), &resp)
	require.Equal(s.T(), 2, len(resp))
}

func (s *PortalEndpointIntegrationTestSuite) Test_GetEndpoints_Filters() {
	s.T().Skip("Depends on #637")
}

func TestPortalEndpointIntegrationTestSuite(t *testing.T) {
	suite.Run(t, new(PortalEndpointIntegrationTestSuite))
}

type PortalEventIntegrationTestSuite struct {
	suite.Suite
	DB             database.Database
	Router         http.Handler
	ConvoyApp      *ApplicationHandler
	DefaultProject *datastore.Project
	APIKey         string
}

func (s *PortalEventIntegrationTestSuite) SetupSuite() {
	s.DB = getDB()
	s.ConvoyApp = buildServer()
	s.Router = s.ConvoyApp.BuildRoutes()
}

func (s *PortalEventIntegrationTestSuite) SetupTest() {
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
	err = config.LoadConfig("./testdata/Auth_Config/full-convoy.json")
	require.NoError(s.T(), err)

	apiRepo := postgres.NewAPIKeyRepo(s.ConvoyApp.A.DB)
	userRepo := postgres.NewUserRepo(s.ConvoyApp.A.DB)
	initRealmChain(s.T(), apiRepo, userRepo, s.ConvoyApp.A.Cache)
}

func (s *PortalEventIntegrationTestSuite) TearDownTest() {
	testdb.PurgeDB(s.T(), s.DB)
	metrics.Reset()
}

func (s *PortalEventIntegrationTestSuite) Test_GetEndpointEvent_Valid_Event() {
	eventID := ulid.Make().String()
	expectedStatusCode := http.StatusOK

	// Just Before.
	endpoint, err := testdb.SeedEndpoint(s.ConvoyApp.A.DB, s.DefaultProject, ulid.Make().String(), "", "", false, datastore.ActiveEndpointStatus)
	require.NoError(s.T(), err)

	event, err := testdb.SeedEvent(s.ConvoyApp.A.DB, endpoint, s.DefaultProject.UID, eventID, "*", "", []byte(`{}`))
	require.NoError(s.T(), err)

	portalLink, err := testdb.SeedPortalLink(s.ConvoyApp.A.DB, s.DefaultProject, []string{endpoint.UID})
	require.NoError(s.T(), err)

	url := fmt.Sprintf("/portal-api/events/%s?token=%s", eventID, portalLink.Token)
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

func (s *PortalEventIntegrationTestSuite) Test_ReplayEndpointEvent_Valid_Event() {
	eventID := ulid.Make().String()
	expectedStatusCode := http.StatusOK

	// Just Before.
	endpoint, _ := testdb.SeedEndpoint(s.ConvoyApp.A.DB, s.DefaultProject, ulid.Make().String(), "", "", false, datastore.ActiveEndpointStatus)
	_, _ = testdb.SeedEvent(s.ConvoyApp.A.DB, endpoint, s.DefaultProject.UID, eventID, "*", "", []byte(`{}`))

	portalLink, err := testdb.SeedPortalLink(s.ConvoyApp.A.DB, s.DefaultProject, []string{endpoint.UID})
	require.NoError(s.T(), err)

	url := fmt.Sprintf("/portal-api/events/%s/replay?token=%s", eventID, portalLink.Token)
	req := createRequest(http.MethodPut, url, s.APIKey, nil)
	w := httptest.NewRecorder()

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)
}

func (s *PortalEventIntegrationTestSuite) Test_GetEndpointEvent_Event_not_found() {
	expectedStatusCode := http.StatusNotFound

	endpoint, _ := testdb.SeedEndpoint(s.ConvoyApp.A.DB, s.DefaultProject, ulid.Make().String(), "", "", false, datastore.ActiveEndpointStatus)

	portalLink, err := testdb.SeedPortalLink(s.ConvoyApp.A.DB, s.DefaultProject, []string{endpoint.UID})
	require.NoError(s.T(), err)

	url := fmt.Sprintf("/portal-api/events/%s?token=%s", "123", portalLink.Token)
	req := createRequest(http.MethodGet, url, s.APIKey, nil)
	w := httptest.NewRecorder()

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)
}

func (s *PortalEventIntegrationTestSuite) Test_GetEventDelivery_Valid_EventDelivery() {
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

	portalLink, err := testdb.SeedPortalLink(s.ConvoyApp.A.DB, s.DefaultProject, []string{endpoint.UID})
	require.NoError(s.T(), err)

	url := fmt.Sprintf("/portal-api/eventdeliveries/%s?token=%s", eventDeliveryID, portalLink.Token)
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

func (s *PortalEventIntegrationTestSuite) Test_GetEventDelivery_Event_not_found() {
	expectedStatusCode := http.StatusNotFound

	endpoint, _ := testdb.SeedEndpoint(s.ConvoyApp.A.DB, s.DefaultProject, ulid.Make().String(), "", "", false, datastore.ActiveEndpointStatus)

	portalLink, err := testdb.SeedPortalLink(s.ConvoyApp.A.DB, s.DefaultProject, []string{endpoint.UID})
	require.NoError(s.T(), err)

	url := fmt.Sprintf("/portal-api/eventdeliveries/%s?token=%s", "123", portalLink.Token)
	req := createRequest(http.MethodGet, url, s.APIKey, nil)
	w := httptest.NewRecorder()

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)
}

func (s *PortalEventIntegrationTestSuite) Test_ResendEventDelivery_Valid_Resend() {
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

	portalLink, err := testdb.SeedPortalLink(s.ConvoyApp.A.DB, s.DefaultProject, []string{endpoint.UID})
	require.NoError(s.T(), err)

	url := fmt.Sprintf("/portal-api/eventdeliveries/%s/resend?token=%s", eventDelivery.UID, portalLink.Token)
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

func (s *PortalEventIntegrationTestSuite) Test_BatchRetryEventDelivery_Valid_EventDeliveries() {
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

	portalLink, err := testdb.SeedPortalLink(s.ConvoyApp.A.DB, s.DefaultProject, []string{endpoint.UID})
	require.NoError(s.T(), err)

	url := fmt.Sprintf("/portal-api/eventdeliveries/batchretry?endpointId=%s&eventId=%s&status=%s&token=%s", endpoint.UID, event.UID, datastore.FailureEventStatus, portalLink.Token)
	req := createRequest(http.MethodPost, url, s.APIKey, nil)
	w := httptest.NewRecorder()

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)
}

func (s *PortalEventIntegrationTestSuite) Test_CountAffectedEventDeliveries_Valid_Filters() {
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

	portalLink, err := testdb.SeedPortalLink(s.ConvoyApp.A.DB, s.DefaultProject, []string{endpoint.UID})
	require.NoError(s.T(), err)

	url := fmt.Sprintf("/portal-api/eventdeliveries/countbatchretryevents?endpointId=%s&eventId=%s&status=%s&token=%s", endpoint.UID, event.UID, datastore.FailureEventStatus, portalLink.Token)
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

func (s *PortalEventIntegrationTestSuite) Test_ForceResendEventDeliveries_Valid_EventDeliveries() {
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

	portalLink, err := testdb.SeedPortalLink(s.ConvoyApp.A.DB, s.DefaultProject, []string{endpoint.UID})
	require.NoError(s.T(), err)

	url := fmt.Sprintf("/portal-api/eventdeliveries/forceresend?token=%s", portalLink.Token)

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

func (s *PortalEventIntegrationTestSuite) Test_GetEventsPaged() {
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

	_, err = testdb.SeedSource(s.ConvoyApp.A.DB, s.DefaultProject, sourceID, ulid.Make().String(), "", vc, "")
	require.NoError(s.T(), err)

	_, err = testdb.SeedEvent(s.ConvoyApp.A.DB, endpoint1, s.DefaultProject.UID, eventID, "*", sourceID, []byte(`{}`))
	require.NoError(s.T(), err)

	_, err = testdb.SeedEvent(s.ConvoyApp.A.DB, endpoint1, s.DefaultProject.UID, ulid.Make().String(), "*", sourceID, []byte(`{}`))
	require.NoError(s.T(), err)

	endpoint2, err := testdb.SeedEndpoint(s.ConvoyApp.A.DB, s.DefaultProject, ulid.Make().String(), "", "", false, datastore.ActiveEndpointStatus)
	require.NoError(s.T(), err)

	e2, err := testdb.SeedEvent(s.ConvoyApp.A.DB, endpoint2, s.DefaultProject.UID, ulid.Make().String(), "*", sourceID, []byte(`{}`))
	require.NoError(s.T(), err)

	portalLink, err := testdb.SeedPortalLink(s.ConvoyApp.A.DB, s.DefaultProject, []string{endpoint2.UID})
	require.NoError(s.T(), err)

	url := fmt.Sprintf("/portal-api/events?endpointId=%s&sourceId=%s&token=%s", endpoint1.UID, sourceID, portalLink.Token)
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
	require.Equal(s.T(), 1, len(respEvents))

	v := []string{e2.UID}
	for i := range respEvents {
		require.Contains(s.T(), v, respEvents[i].UID)
	}
}

func (s *PortalEventIntegrationTestSuite) Test_GetEventDeliveriesPaged() {
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

	_, err = testdb.SeedEventDelivery(s.ConvoyApp.A.DB, event1, endpoint1, s.DefaultProject.UID, eventDeliveryID, datastore.FailureEventStatus, subscription)
	require.NoError(s.T(), err)

	_, err = testdb.SeedEventDelivery(s.ConvoyApp.A.DB, event1, endpoint1, s.DefaultProject.UID, ulid.Make().String(), datastore.FailureEventStatus, subscription)
	require.NoError(s.T(), err)

	endpoint2, err := testdb.SeedEndpoint(s.ConvoyApp.A.DB, s.DefaultProject, ulid.Make().String(), "", "", false, datastore.ActiveEndpointStatus)
	require.NoError(s.T(), err)

	event2, err := testdb.SeedEvent(s.ConvoyApp.A.DB, endpoint2, s.DefaultProject.UID, ulid.Make().String(), "*", "", []byte(`{}`))
	require.NoError(s.T(), err)

	d2, err := testdb.SeedEventDelivery(s.ConvoyApp.A.DB, event2, endpoint2, s.DefaultProject.UID, ulid.Make().String(), datastore.FailureEventStatus, subscription)
	require.NoError(s.T(), err)

	portalLink, err := testdb.SeedPortalLink(s.ConvoyApp.A.DB, s.DefaultProject, []string{endpoint2.UID})
	require.NoError(s.T(), err)

	url := fmt.Sprintf("/portal-api/eventdeliveries?endpointId=%s&token=%s", endpoint1.UID, portalLink.Token)
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
	require.Equal(s.T(), 1, len(respEvents))

	v := []*datastore.EventDelivery{d2}
	for i, delivery := range v {
		require.Equal(s.T(), respEvents[i].UID, delivery.UID)
	}
}

func (s *PortalLinkIntegrationTestSuite) Test_GetPortalLinkEndpointSubscriptions() {
	expectedStatusCode := http.StatusOK

	// Just Before.
	endpoint1, err := testdb.SeedEndpoint(s.ConvoyApp.A.DB, s.DefaultProject, ulid.Make().String(), "", "", false, datastore.ActiveEndpointStatus)
	require.NoError(s.T(), err)

	endpoint2, err := testdb.SeedEndpoint(s.ConvoyApp.A.DB, s.DefaultProject, ulid.Make().String(), "", "", false, datastore.ActiveEndpointStatus)
	require.NoError(s.T(), err)

	portalLink, err := testdb.SeedPortalLink(s.ConvoyApp.A.DB, s.DefaultProject, []string{endpoint2.UID})
	require.NoError(s.T(), err)

	vc := &datastore.VerifierConfig{
		Type: datastore.BasicAuthVerifier,
		BasicAuth: &datastore.BasicAuth{
			UserName: "Convoy",
			Password: "Convoy",
		},
	}

	source, err := testdb.SeedSource(s.ConvoyApp.A.DB, s.DefaultProject, "", ulid.Make().String(), "", vc, "")
	require.NoError(s.T(), err)

	// seed subscriptions
	for i := 0; i < 5; i++ {
		_, err = testdb.SeedSubscription(s.ConvoyApp.A.DB, s.DefaultProject, ulid.Make().String(), datastore.OutgoingProject, source, endpoint1, &datastore.RetryConfiguration{}, &datastore.AlertConfiguration{}, nil)
		require.NoError(s.T(), err)

	}

	sub, err := testdb.SeedSubscription(s.ConvoyApp.A.DB, s.DefaultProject, ulid.Make().String(), datastore.OutgoingProject, source, endpoint2, &datastore.RetryConfiguration{}, &datastore.AlertConfiguration{}, nil)
	require.NoError(s.T(), err)

	req := createRequest(http.MethodGet, fmt.Sprintf("/portal-api/subscriptions?token=%s", portalLink.Token), "", nil)
	w := httptest.NewRecorder()

	// Act
	s.Router.ServeHTTP(w, req)

	// Assert
	require.Equal(s.T(), expectedStatusCode, w.Code)

	var respSubs []datastore.Subscription
	resp := &pagedResponse{Content: &respSubs}

	parseResponse(s.T(), w.Result(), &resp)
	require.Equal(s.T(), 1, len(respSubs))
	require.Equal(s.T(), sub.UID, respSubs[0].UID)
}

func TestPortalEventIntegrationTestSuite(t *testing.T) {
	suite.Run(t, new(PortalEventIntegrationTestSuite))
}
