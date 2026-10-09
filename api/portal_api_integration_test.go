package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/frain-dev/convoy/api/testdb"
	"github.com/frain-dev/convoy/auth"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/database"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/internal/api_keys"
	"github.com/frain-dev/convoy/internal/endpoints"
	"github.com/frain-dev/convoy/internal/pkg/metrics"
	"github.com/frain-dev/convoy/internal/portal_links"
	"github.com/frain-dev/convoy/internal/users"
	log "github.com/frain-dev/convoy/pkg/logger"
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
	s.ConvoyApp = buildServer(s.T())
	s.Router = s.ConvoyApp.BuildControlPlaneRoutes()
}

func (s *PortalEndpointIntegrationTestSuite) SetupTest() {
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
		Type:    auth.RoleProjectAdmin,
		Project: s.DefaultProject.UID,
	}

	_, s.APIKey, err = testdb.SeedAPIKey(s.ConvoyApp.A.DB, role, "", "test", "", "")
	require.NoError(s.T(), err)

	_, s.PersonalAPIKey, err = testdb.SeedAPIKey(s.ConvoyApp.A.DB, auth.Role{}, ulid.Make().String(), "test-personal-key", string(datastore.PersonalKey), s.DefaultUser.UID)
	require.NoError(s.T(), err)

	// Setup Config.
	err = config.LoadConfig("./testdata/Auth_Config/full-convoy.json")
	require.NoError(s.T(), err)

	apiRepo := api_keys.New(s.ConvoyApp.A.Logger, s.ConvoyApp.A.DB)
	userRepo := users.New(log.New("convoy", log.LevelError), s.ConvoyApp.A.DB)
	portalLinkRepo := portal_links.New(s.ConvoyApp.A.Logger, s.ConvoyApp.A.DB)
	initRealmChain(s.T(), apiRepo, userRepo, portalLinkRepo, s.ConvoyApp.A.Cache)
}

func (s *PortalEndpointIntegrationTestSuite) TearDownTest() {
	metrics.Reset()
}

func (s *PortalEndpointIntegrationTestSuite) Test_GetEndpoint_EndpointNotFound() {
	appID := "123"
	expectedStatusCode := http.StatusNotFound

	endpoint, err := testdb.SeedEndpoint(s.ConvoyApp.A.DB, s.DefaultProject, "", "", "test", true, datastore.ActiveEndpointStatus)
	require.NoError(s.T(), err)

	portalLink, err := testdb.SeedPortalLink(s.ConvoyApp.A.DB, s.DefaultProject, endpoint.OwnerID)
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
	_, _ = testdb.SeedEndpoint(s.ConvoyApp.A.DB, s.DefaultProject, endpointID, "", "test", true, datastore.ActiveEndpointStatus)

	portalLink, err := testdb.SeedPortalLink(s.ConvoyApp.A.DB, s.DefaultProject, "test")
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

	endpointRepo := endpoints.New(s.ConvoyApp.A.Logger, s.ConvoyApp.A.DB)
	dbEndpoint, err := endpointRepo.FindEndpointByID(context.Background(), endpointID, s.DefaultProject.UID)
	require.NoError(s.T(), err)
	require.Equal(s.T(), endpoint.UID, dbEndpoint.UID)
	require.Equal(s.T(), endpoint.Name, dbEndpoint.Name)
}

func (s *PortalLinkIntegrationTestSuite) Test_GetPortalLinkEndpoints() {
	// Just Before
	endpoint1, err := testdb.SeedEndpoint(s.ConvoyApp.A.DB, s.DefaultProject, "", ulid.Make().String(), "test", false, datastore.ActiveEndpointStatus)
	require.NoError(s.T(), err)

	_, err = testdb.SeedEndpoint(s.ConvoyApp.A.DB, s.DefaultProject, ulid.Make().String(), "", "test", false, datastore.ActiveEndpointStatus)
	require.NoError(s.T(), err)

	portalLink, _ := testdb.SeedPortalLink(s.ConvoyApp.A.DB, s.DefaultProject, endpoint1.OwnerID)
	require.NoError(s.T(), err)

	// Arrange Request
	url := fmt.Sprintf("/portal-api/endpoints?token=%s", portalLink.Token)
	req := createRequest(http.MethodGet, url, portalLink.Token, nil)
	w := httptest.NewRecorder()

	// Act
	s.Router.ServeHTTP(w, req)

	// Assert
	require.Equal(s.T(), http.StatusOK, w.Code)

	// Deep Assert
	var respEndpoints []datastore.Endpoint
	resp := pagedResponse{Content: &respEndpoints}
	parseResponse(s.T(), w.Result(), &resp)
	require.Equal(s.T(), 2, len(respEndpoints))
}

func (s *PortalEndpointIntegrationTestSuite) Test_GetEndpoint_CrossTenant_Unauthorized() {
	ownerA := ulid.Make().String()
	ownerB := ulid.Make().String()

	// endpoint owned by A backs the portal link; endpoint owned by B is the victim.
	_, err := testdb.SeedEndpoint(s.ConvoyApp.A.DB, s.DefaultProject, "", "owner-a-ep", ownerA, true, datastore.ActiveEndpointStatus)
	require.NoError(s.T(), err)

	victim, err := testdb.SeedEndpoint(s.ConvoyApp.A.DB, s.DefaultProject, "", "owner-b-ep", ownerB, true, datastore.ActiveEndpointStatus)
	require.NoError(s.T(), err)

	portalLink, err := testdb.SeedPortalLink(s.ConvoyApp.A.DB, s.DefaultProject, ownerA)
	require.NoError(s.T(), err)

	url := fmt.Sprintf("/portal-api/endpoints/%s?token=%s", victim.UID, portalLink.Token)
	req := createRequest(http.MethodGet, url, portalLink.Token, nil)
	w := httptest.NewRecorder()

	s.Router.ServeHTTP(w, req)

	require.Equal(s.T(), http.StatusUnauthorized, w.Code)
}

func (s *PortalEndpointIntegrationTestSuite) Test_ExpireSecret_CrossTenant_Unauthorized() {
	ownerA := ulid.Make().String()
	ownerB := ulid.Make().String()

	_, err := testdb.SeedEndpoint(s.ConvoyApp.A.DB, s.DefaultProject, "", "owner-a-ep", ownerA, true, datastore.ActiveEndpointStatus)
	require.NoError(s.T(), err)

	victim, err := testdb.SeedEndpoint(s.ConvoyApp.A.DB, s.DefaultProject, "", "owner-b-ep", ownerB, true, datastore.ActiveEndpointStatus)
	require.NoError(s.T(), err)

	portalLink, err := testdb.SeedPortalLink(s.ConvoyApp.A.DB, s.DefaultProject, ownerA)
	require.NoError(s.T(), err)

	url := fmt.Sprintf("/portal-api/endpoints/%s/expire_secret?token=%s", victim.UID, portalLink.Token)
	req := createRequest(http.MethodPut, url, portalLink.Token, serialize(`{}`))
	w := httptest.NewRecorder()

	s.Router.ServeHTTP(w, req)

	require.Equal(s.T(), http.StatusUnauthorized, w.Code)
}

func (s *PortalEndpointIntegrationTestSuite) Test_GetSubscription_CrossTenant_Unauthorized() {
	ownerA := ulid.Make().String()
	ownerB := ulid.Make().String()

	_, err := testdb.SeedEndpoint(s.ConvoyApp.A.DB, s.DefaultProject, "", "owner-a-ep", ownerA, true, datastore.ActiveEndpointStatus)
	require.NoError(s.T(), err)

	victimEndpoint, err := testdb.SeedEndpoint(s.ConvoyApp.A.DB, s.DefaultProject, "", "owner-b-ep", ownerB, true, datastore.ActiveEndpointStatus)
	require.NoError(s.T(), err)

	victimSub, err := testdb.SeedSubscription(s.ConvoyApp.A.DB, s.DefaultProject, ulid.Make().String(), datastore.OutgoingProject, &datastore.Source{}, victimEndpoint, &datastore.RetryConfiguration{}, &datastore.AlertConfiguration{}, &datastore.FilterConfiguration{
		EventTypes: []string{"*"},
		Filter:     datastore.FilterSchema{Headers: datastore.M{}, Body: datastore.M{}},
	})
	require.NoError(s.T(), err)

	portalLink, err := testdb.SeedPortalLink(s.ConvoyApp.A.DB, s.DefaultProject, ownerA)
	require.NoError(s.T(), err)

	url := fmt.Sprintf("/portal-api/subscriptions/%s?token=%s", victimSub.UID, portalLink.Token)
	req := createRequest(http.MethodGet, url, portalLink.Token, nil)
	w := httptest.NewRecorder()

	s.Router.ServeHTTP(w, req)

	require.Equal(s.T(), http.StatusUnauthorized, w.Code)
}

func (s *PortalEndpointIntegrationTestSuite) Test_UpdateSubscription_RetargetToUnownedEndpoint_Unauthorized() {
	ownerA := ulid.Make().String()
	ownerB := ulid.Make().String()

	ownedEndpoint, err := testdb.SeedEndpoint(s.ConvoyApp.A.DB, s.DefaultProject, "", "owner-a-ep", ownerA, true, datastore.ActiveEndpointStatus)
	require.NoError(s.T(), err)

	foreignEndpoint, err := testdb.SeedEndpoint(s.ConvoyApp.A.DB, s.DefaultProject, "", "owner-b-ep", ownerB, true, datastore.ActiveEndpointStatus)
	require.NoError(s.T(), err)

	sub, err := testdb.SeedSubscription(s.ConvoyApp.A.DB, s.DefaultProject, ulid.Make().String(), datastore.OutgoingProject, &datastore.Source{}, ownedEndpoint, &datastore.RetryConfiguration{}, &datastore.AlertConfiguration{}, &datastore.FilterConfiguration{
		EventTypes: []string{"*"},
		Filter:     datastore.FilterSchema{Headers: datastore.M{}, Body: datastore.M{}},
	})
	require.NoError(s.T(), err)

	portalLink, err := testdb.SeedPortalLink(s.ConvoyApp.A.DB, s.DefaultProject, ownerA)
	require.NoError(s.T(), err)

	// The portal link owns the subscription's current endpoint, so the existing
	// ownership check passes; retargeting to an endpoint owned by another owner
	// must still be rejected.
	url := fmt.Sprintf("/portal-api/subscriptions/%s?token=%s", sub.UID, portalLink.Token)
	body := serialize(`{"endpoint_id":"%s"}`, foreignEndpoint.UID)
	req := createRequest(http.MethodPut, url, portalLink.Token, body)
	w := httptest.NewRecorder()

	s.Router.ServeHTTP(w, req)

	require.Equal(s.T(), http.StatusUnauthorized, w.Code)
}

func (s *PortalEndpointIntegrationTestSuite) Test_UpdateSubscription_RetargetToOwnedEndpoint_Allowed() {
	ownerA := ulid.Make().String()

	ownedEndpoint, err := testdb.SeedEndpoint(s.ConvoyApp.A.DB, s.DefaultProject, "", "owner-a-ep-1", ownerA, true, datastore.ActiveEndpointStatus)
	require.NoError(s.T(), err)

	otherOwnedEndpoint, err := testdb.SeedEndpoint(s.ConvoyApp.A.DB, s.DefaultProject, "", "owner-a-ep-2", ownerA, true, datastore.ActiveEndpointStatus)
	require.NoError(s.T(), err)

	sub, err := testdb.SeedSubscription(s.ConvoyApp.A.DB, s.DefaultProject, ulid.Make().String(), datastore.OutgoingProject, &datastore.Source{}, ownedEndpoint, &datastore.RetryConfiguration{}, &datastore.AlertConfiguration{}, &datastore.FilterConfiguration{
		EventTypes: []string{"*"},
		Filter:     datastore.FilterSchema{Headers: datastore.M{}, Body: datastore.M{}},
	})
	require.NoError(s.T(), err)

	portalLink, err := testdb.SeedPortalLink(s.ConvoyApp.A.DB, s.DefaultProject, ownerA)
	require.NoError(s.T(), err)

	// Retargeting to another endpoint the portal link owns is allowed.
	url := fmt.Sprintf("/portal-api/subscriptions/%s?token=%s", sub.UID, portalLink.Token)
	body := serialize(`{"endpoint_id":"%s"}`, otherOwnedEndpoint.UID)
	req := createRequest(http.MethodPut, url, portalLink.Token, body)
	w := httptest.NewRecorder()

	s.Router.ServeHTTP(w, req)

	require.Equal(s.T(), http.StatusAccepted, w.Code)
}

func (s *PortalEndpointIntegrationTestSuite) Test_GetFilter_CrossTenant_Unauthorized() {
	ownerA := ulid.Make().String()
	ownerB := ulid.Make().String()

	_, err := testdb.SeedEndpoint(s.ConvoyApp.A.DB, s.DefaultProject, "", "owner-a-ep", ownerA, true, datastore.ActiveEndpointStatus)
	require.NoError(s.T(), err)

	victimEndpoint, err := testdb.SeedEndpoint(s.ConvoyApp.A.DB, s.DefaultProject, "", "owner-b-ep", ownerB, true, datastore.ActiveEndpointStatus)
	require.NoError(s.T(), err)

	victimSub, err := testdb.SeedSubscription(s.ConvoyApp.A.DB, s.DefaultProject, ulid.Make().String(), datastore.OutgoingProject, &datastore.Source{}, victimEndpoint, &datastore.RetryConfiguration{}, &datastore.AlertConfiguration{}, &datastore.FilterConfiguration{
		EventTypes: []string{"*"},
		Filter:     datastore.FilterSchema{Headers: datastore.M{}, Body: datastore.M{}},
	})
	require.NoError(s.T(), err)

	portalLink, err := testdb.SeedPortalLink(s.ConvoyApp.A.DB, s.DefaultProject, ownerA)
	require.NoError(s.T(), err)

	// The owner gate runs as route middleware, so even a non-existent filter id is
	// rejected before the handler when the subscription is not owned by the caller.
	url := fmt.Sprintf("/portal-api/subscriptions/%s/filters/%s?token=%s", victimSub.UID, ulid.Make().String(), portalLink.Token)
	req := createRequest(http.MethodGet, url, portalLink.Token, nil)
	w := httptest.NewRecorder()

	s.Router.ServeHTTP(w, req)

	require.Equal(s.T(), http.StatusUnauthorized, w.Code)
}

func (s *PortalEndpointIntegrationTestSuite) Test_GetFilter_ControlPlane_CrossTenant_Unauthorized() {
	ownerA := ulid.Make().String()
	ownerB := ulid.Make().String()

	_, err := testdb.SeedEndpoint(s.ConvoyApp.A.DB, s.DefaultProject, "", "owner-a-ep", ownerA, true, datastore.ActiveEndpointStatus)
	require.NoError(s.T(), err)

	victimEndpoint, err := testdb.SeedEndpoint(s.ConvoyApp.A.DB, s.DefaultProject, "", "owner-b-ep", ownerB, true, datastore.ActiveEndpointStatus)
	require.NoError(s.T(), err)

	victimSub, err := testdb.SeedSubscription(s.ConvoyApp.A.DB, s.DefaultProject, ulid.Make().String(), datastore.OutgoingProject, &datastore.Source{}, victimEndpoint, &datastore.RetryConfiguration{}, &datastore.AlertConfiguration{}, &datastore.FilterConfiguration{
		EventTypes: []string{"*"},
		Filter:     datastore.FilterSchema{Headers: datastore.M{}, Body: datastore.M{}},
	})
	require.NoError(s.T(), err)

	portalLink, err := testdb.SeedPortalLink(s.ConvoyApp.A.DB, s.DefaultProject, ownerA)
	require.NoError(s.T(), err)

	// The filter handlers are also mounted on the control plane; the portal owner gate
	// must apply there too, not only on /portal-api.
	url := fmt.Sprintf("/api/v1/projects/%s/subscriptions/%s/filters/%s", s.DefaultProject.UID, victimSub.UID, ulid.Make().String())
	req := createRequest(http.MethodGet, url, portalLink.Token, nil)
	w := httptest.NewRecorder()

	s.Router.ServeHTTP(w, req)

	require.Equal(s.T(), http.StatusUnauthorized, w.Code)
}

func (s *PortalEndpointIntegrationTestSuite) Test_GetEndpoints_Filters() {
	s.T().Skip("Depends on #637")
}

func (s *PortalEndpointIntegrationTestSuite) Test_GetPortalLicenseFeatures_SelfHosted() {
	portalLink, err := testdb.SeedPortalLink(s.ConvoyApp.A.DB, s.DefaultProject, "test")
	require.NoError(s.T(), err)

	// Self-hosted suite config: features come from the deployment licenser,
	// resolved from the portal token. The suite's noop licenser serves an empty
	// feature payload, so assert the route is wired and returns a successful
	// envelope; feature-map content is covered by the unit tests in
	// api/handlers/license_test.go.
	url := fmt.Sprintf("/portal-api/license/features?token=%s", portalLink.Token)
	req := createRequest(http.MethodGet, url, portalLink.Token, nil)
	w := httptest.NewRecorder()

	s.Router.ServeHTTP(w, req)

	require.Equal(s.T(), http.StatusOK, w.Code)

	var resp struct {
		Status bool `json:"status"`
	}
	require.NoError(s.T(), json.NewDecoder(w.Result().Body).Decode(&resp))
	require.True(s.T(), resp.Status)
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
	s.ConvoyApp = buildServer(s.T())
	s.Router = s.ConvoyApp.BuildControlPlaneRoutes()
}

func (s *PortalEventIntegrationTestSuite) SetupTest() {

	user, err := testdb.SeedDefaultUser(s.ConvoyApp.A.DB)
	require.NoError(s.T(), err)

	org, err := testdb.SeedDefaultOrganisation(s.ConvoyApp.A.DB, user)
	require.NoError(s.T(), err)

	// Setup Default Project.
	s.DefaultProject, _ = testdb.SeedDefaultProject(s.ConvoyApp.A.DB, org.UID)

	// Seed Auth
	role := auth.Role{
		Type:    auth.RoleProjectAdmin,
		Project: s.DefaultProject.UID,
	}

	_, s.APIKey, _ = testdb.SeedAPIKey(s.ConvoyApp.A.DB, role, "", "test", "", "")

	// Setup Config.
	err = config.LoadConfig("./testdata/Auth_Config/full-convoy.json")
	require.NoError(s.T(), err)

	apiRepo := api_keys.New(s.ConvoyApp.A.Logger, s.ConvoyApp.A.DB)
	userRepo := users.New(log.New("convoy", log.LevelError), s.ConvoyApp.A.DB)
	portalLinkRepo := portal_links.New(s.ConvoyApp.A.Logger, s.ConvoyApp.A.DB)
	initRealmChain(s.T(), apiRepo, userRepo, portalLinkRepo, s.ConvoyApp.A.Cache)
}

func (s *PortalEventIntegrationTestSuite) TearDownTest() {

	metrics.Reset()
}

func (s *PortalEventIntegrationTestSuite) Test_GetEndpointEvent_Valid_Event() {
	eventID := ulid.Make().String()
	expectedStatusCode := http.StatusOK

	// Just Before.
	endpoint, err := testdb.SeedEndpoint(s.ConvoyApp.A.DB, s.DefaultProject, ulid.Make().String(), "", "test", false, datastore.ActiveEndpointStatus)
	require.NoError(s.T(), err)

	event, err := testdb.SeedEvent(s.ConvoyApp.A.DB, endpoint, s.DefaultProject.UID, eventID, "*", "", []byte(`{}`))
	require.NoError(s.T(), err)

	portalLink, err := testdb.SeedPortalLink(s.ConvoyApp.A.DB, s.DefaultProject, endpoint.OwnerID)
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
	endpoint, _ := testdb.SeedEndpoint(s.ConvoyApp.A.DB, s.DefaultProject, ulid.Make().String(), "", "test", false, datastore.ActiveEndpointStatus)
	_, _ = testdb.SeedEvent(s.ConvoyApp.A.DB, endpoint, s.DefaultProject.UID, eventID, "*", "", []byte(`{}`))

	portalLink, err := testdb.SeedPortalLink(s.ConvoyApp.A.DB, s.DefaultProject, endpoint.OwnerID)
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

	endpoint, _ := testdb.SeedEndpoint(s.ConvoyApp.A.DB, s.DefaultProject, ulid.Make().String(), "", "test", false, datastore.ActiveEndpointStatus)

	portalLink, err := testdb.SeedPortalLink(s.ConvoyApp.A.DB, s.DefaultProject, endpoint.OwnerID)
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
	endpoint, _ := testdb.SeedEndpoint(s.ConvoyApp.A.DB, s.DefaultProject, ulid.Make().String(), "", "test", false, datastore.ActiveEndpointStatus)
	event, _ := testdb.SeedEvent(s.ConvoyApp.A.DB, endpoint, s.DefaultProject.UID, ulid.Make().String(), "*", "", []byte(`{}`))
	subscription, err := testdb.SeedSubscription(s.ConvoyApp.A.DB, s.DefaultProject, ulid.Make().String(), datastore.OutgoingProject, &datastore.Source{}, endpoint, &datastore.RetryConfiguration{}, &datastore.AlertConfiguration{}, &datastore.FilterConfiguration{
		EventTypes: []string{"*"},
		Filter:     datastore.FilterSchema{Headers: datastore.M{}, Body: datastore.M{}},
	})
	require.NoError(s.T(), err)

	eventDelivery, err := testdb.SeedEventDelivery(s.ConvoyApp.A.DB, event, endpoint, s.DefaultProject.UID, eventDeliveryID, datastore.FailureEventStatus, subscription)
	require.NoError(s.T(), err)

	portalLink, err := testdb.SeedPortalLink(s.ConvoyApp.A.DB, s.DefaultProject, endpoint.OwnerID)
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

	endpoint, _ := testdb.SeedEndpoint(s.ConvoyApp.A.DB, s.DefaultProject, ulid.Make().String(), "", "test", false, datastore.ActiveEndpointStatus)

	portalLink, err := testdb.SeedPortalLink(s.ConvoyApp.A.DB, s.DefaultProject, endpoint.OwnerID)
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
	endpoint, err := testdb.SeedEndpoint(s.ConvoyApp.A.DB, s.DefaultProject, ulid.Make().String(), "", "test", false, datastore.ActiveEndpointStatus)
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

	portalLink, err := testdb.SeedPortalLink(s.ConvoyApp.A.DB, s.DefaultProject, endpoint.OwnerID)
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
	endpoint, _ := testdb.SeedEndpoint(s.ConvoyApp.A.DB, s.DefaultProject, ulid.Make().String(), "", "test", false, datastore.ActiveEndpointStatus)
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

	portalLink, err := testdb.SeedPortalLink(s.ConvoyApp.A.DB, s.DefaultProject, endpoint.OwnerID)
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
	endpoint, _ := testdb.SeedEndpoint(s.ConvoyApp.A.DB, s.DefaultProject, ulid.Make().String(), "", "test", false, datastore.ActiveEndpointStatus)
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

	portalLink, err := testdb.SeedPortalLink(s.ConvoyApp.A.DB, s.DefaultProject, endpoint.OwnerID)
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
	endpoint, _ := testdb.SeedEndpoint(s.ConvoyApp.A.DB, s.DefaultProject, ulid.Make().String(), "", "test", false, datastore.ActiveEndpointStatus)
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

	portalLink, err := testdb.SeedPortalLink(s.ConvoyApp.A.DB, s.DefaultProject, endpoint.OwnerID)
	require.NoError(s.T(), err)

	url := fmt.Sprintf("/portal-api/eventdeliveries/forceresend?token=%s", portalLink.Token)

	body := serialize(`{"ids":["%s", "%s", "%s"]}`, e1.UID, e2.UID, e3.UID)

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

func (s *PortalEventIntegrationTestSuite) Test_GetEndpointEvent_CrossTenant_Unauthorized() {
	ownerA := ulid.Make().String()

	// portal link owns endpoint A; the event belongs to endpoint B (a different owner).
	_, err := testdb.SeedEndpoint(s.ConvoyApp.A.DB, s.DefaultProject, "", "owner-a-ep", ownerA, false, datastore.ActiveEndpointStatus)
	require.NoError(s.T(), err)

	victimEndpoint, err := testdb.SeedEndpoint(s.ConvoyApp.A.DB, s.DefaultProject, "", "owner-b-ep", ulid.Make().String(), false, datastore.ActiveEndpointStatus)
	require.NoError(s.T(), err)

	event, err := testdb.SeedEvent(s.ConvoyApp.A.DB, victimEndpoint, s.DefaultProject.UID, ulid.Make().String(), "*", "", []byte(`{}`))
	require.NoError(s.T(), err)

	portalLink, err := testdb.SeedPortalLink(s.ConvoyApp.A.DB, s.DefaultProject, ownerA)
	require.NoError(s.T(), err)

	url := fmt.Sprintf("/portal-api/events/%s?token=%s", event.UID, portalLink.Token)
	req := createRequest(http.MethodGet, url, portalLink.Token, nil)
	w := httptest.NewRecorder()

	s.Router.ServeHTTP(w, req)

	require.Equal(s.T(), http.StatusUnauthorized, w.Code)
}

func (s *PortalEventIntegrationTestSuite) Test_GetEventDelivery_CrossTenant_Unauthorized() {
	ownerA := ulid.Make().String()

	_, err := testdb.SeedEndpoint(s.ConvoyApp.A.DB, s.DefaultProject, "", "owner-a-ep", ownerA, false, datastore.ActiveEndpointStatus)
	require.NoError(s.T(), err)

	victimEndpoint, err := testdb.SeedEndpoint(s.ConvoyApp.A.DB, s.DefaultProject, "", "owner-b-ep", ulid.Make().String(), false, datastore.ActiveEndpointStatus)
	require.NoError(s.T(), err)

	event, err := testdb.SeedEvent(s.ConvoyApp.A.DB, victimEndpoint, s.DefaultProject.UID, ulid.Make().String(), "*", "", []byte(`{}`))
	require.NoError(s.T(), err)

	subscription, err := testdb.SeedSubscription(s.ConvoyApp.A.DB, s.DefaultProject, ulid.Make().String(), datastore.OutgoingProject, &datastore.Source{}, victimEndpoint, &datastore.RetryConfiguration{}, &datastore.AlertConfiguration{}, &datastore.FilterConfiguration{
		EventTypes: []string{"*"},
		Filter:     datastore.FilterSchema{Headers: datastore.M{}, Body: datastore.M{}},
	})
	require.NoError(s.T(), err)

	eventDelivery, err := testdb.SeedEventDelivery(s.ConvoyApp.A.DB, event, victimEndpoint, s.DefaultProject.UID, ulid.Make().String(), datastore.FailureEventStatus, subscription)
	require.NoError(s.T(), err)

	portalLink, err := testdb.SeedPortalLink(s.ConvoyApp.A.DB, s.DefaultProject, ownerA)
	require.NoError(s.T(), err)

	url := fmt.Sprintf("/portal-api/eventdeliveries/%s?token=%s", eventDelivery.UID, portalLink.Token)
	req := createRequest(http.MethodGet, url, portalLink.Token, nil)
	w := httptest.NewRecorder()

	s.Router.ServeHTTP(w, req)

	require.Equal(s.T(), http.StatusUnauthorized, w.Code)
}

func (s *PortalEventIntegrationTestSuite) Test_ResendEventDelivery_CrossTenant_Unauthorized() {
	ownerA := ulid.Make().String()

	_, err := testdb.SeedEndpoint(s.ConvoyApp.A.DB, s.DefaultProject, "", "owner-a-ep", ownerA, false, datastore.ActiveEndpointStatus)
	require.NoError(s.T(), err)

	victimEndpoint, err := testdb.SeedEndpoint(s.ConvoyApp.A.DB, s.DefaultProject, "", "owner-b-ep", ulid.Make().String(), false, datastore.ActiveEndpointStatus)
	require.NoError(s.T(), err)

	event, err := testdb.SeedEvent(s.ConvoyApp.A.DB, victimEndpoint, s.DefaultProject.UID, ulid.Make().String(), "*", "", []byte(`{}`))
	require.NoError(s.T(), err)

	subscription, err := testdb.SeedSubscription(s.ConvoyApp.A.DB, s.DefaultProject, ulid.Make().String(), datastore.OutgoingProject, &datastore.Source{}, victimEndpoint, &datastore.RetryConfiguration{}, &datastore.AlertConfiguration{}, &datastore.FilterConfiguration{
		EventTypes: []string{"*"},
		Filter:     datastore.FilterSchema{Headers: datastore.M{}, Body: datastore.M{}},
	})
	require.NoError(s.T(), err)

	eventDelivery, err := testdb.SeedEventDelivery(s.ConvoyApp.A.DB, event, victimEndpoint, s.DefaultProject.UID, ulid.Make().String(), datastore.FailureEventStatus, subscription)
	require.NoError(s.T(), err)

	portalLink, err := testdb.SeedPortalLink(s.ConvoyApp.A.DB, s.DefaultProject, ownerA)
	require.NoError(s.T(), err)

	url := fmt.Sprintf("/portal-api/eventdeliveries/%s/resend?token=%s", eventDelivery.UID, portalLink.Token)
	req := createRequest(http.MethodPut, url, portalLink.Token, nil)
	w := httptest.NewRecorder()

	s.Router.ServeHTTP(w, req)

	require.Equal(s.T(), http.StatusUnauthorized, w.Code)
}

func (s *PortalEventIntegrationTestSuite) Test_ForceResendEventDeliveries_CrossTenant_Unauthorized() {
	ownerA := ulid.Make().String()

	_, err := testdb.SeedEndpoint(s.ConvoyApp.A.DB, s.DefaultProject, "", "owner-a-ep", ownerA, false, datastore.ActiveEndpointStatus)
	require.NoError(s.T(), err)

	victimEndpoint, err := testdb.SeedEndpoint(s.ConvoyApp.A.DB, s.DefaultProject, "", "owner-b-ep", ulid.Make().String(), false, datastore.ActiveEndpointStatus)
	require.NoError(s.T(), err)

	event, err := testdb.SeedEvent(s.ConvoyApp.A.DB, victimEndpoint, s.DefaultProject.UID, ulid.Make().String(), "*", "", []byte(`{}`))
	require.NoError(s.T(), err)

	subscription, err := testdb.SeedSubscription(s.ConvoyApp.A.DB, s.DefaultProject, ulid.Make().String(), datastore.OutgoingProject, &datastore.Source{}, victimEndpoint, &datastore.RetryConfiguration{}, &datastore.AlertConfiguration{}, &datastore.FilterConfiguration{
		EventTypes: []string{"*"},
		Filter:     datastore.FilterSchema{Headers: datastore.M{}, Body: datastore.M{}},
	})
	require.NoError(s.T(), err)

	victimDelivery, err := testdb.SeedEventDelivery(s.ConvoyApp.A.DB, event, victimEndpoint, s.DefaultProject.UID, ulid.Make().String(), datastore.SuccessEventStatus, subscription)
	require.NoError(s.T(), err)

	portalLink, err := testdb.SeedPortalLink(s.ConvoyApp.A.DB, s.DefaultProject, ownerA)
	require.NoError(s.T(), err)

	url := fmt.Sprintf("/portal-api/eventdeliveries/forceresend?token=%s", portalLink.Token)
	body := serialize(`{"ids":["%s"]}`, victimDelivery.UID)

	req := createRequest(http.MethodPost, url, portalLink.Token, body)
	w := httptest.NewRecorder()

	s.Router.ServeHTTP(w, req)

	require.Equal(s.T(), http.StatusUnauthorized, w.Code)
}

func (s *PortalEventIntegrationTestSuite) Test_PortalEventWrites_RequireEnabledProject() {
	endpoint, err := testdb.SeedEndpoint(s.ConvoyApp.A.DB, s.DefaultProject, "", "", "test", true, datastore.ActiveEndpointStatus)
	require.NoError(s.T(), err)

	portalLink, err := testdb.SeedPortalLink(s.ConvoyApp.A.DB, s.DefaultProject, endpoint.OwnerID)
	require.NoError(s.T(), err)

	originalLicenser := s.ConvoyApp.A.Licenser
	s.ConvoyApp.A.Licenser = projectDisabledLicenser{Licenser: originalLicenser, disabledProjectID: s.DefaultProject.UID}
	defer func() { s.ConvoyApp.A.Licenser = originalLicenser }()

	tests := []struct {
		name   string
		method string
		path   string
	}{
		{name: "create event", method: http.MethodPost, path: "/portal-api/events?token=%s"},
		{name: "batch replay events", method: http.MethodPost, path: "/portal-api/events/batchreplay?token=%s"},
		{name: "replay event", method: http.MethodPut, path: "/portal-api/events/" + ulid.Make().String() + "/replay?token=%s"},
		{name: "force resend event deliveries", method: http.MethodPost, path: "/portal-api/eventdeliveries/forceresend?token=%s"},
		{name: "batch retry event deliveries", method: http.MethodPost, path: "/portal-api/eventdeliveries/batchretry?token=%s"},
		{name: "resend event delivery", method: http.MethodPut, path: "/portal-api/eventdeliveries/" + ulid.Make().String() + "/resend?token=%s"},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			req := createRequest(tt.method, fmt.Sprintf(tt.path, portalLink.Token), portalLink.Token, serialize(`{}`))
			w := httptest.NewRecorder()

			s.Router.ServeHTTP(w, req)

			require.Equal(s.T(), http.StatusBadRequest, w.Code)
			require.Contains(s.T(), w.Body.String(), "this project has been disabled")
		})
	}
}

func (s *PortalEventIntegrationTestSuite) Test_TestSubscriptionFunction_RequiresEnabledProject() {
	endpoint, err := testdb.SeedEndpoint(s.ConvoyApp.A.DB, s.DefaultProject, "", "", "test", true, datastore.ActiveEndpointStatus)
	require.NoError(s.T(), err)

	portalLink, err := testdb.SeedPortalLink(s.ConvoyApp.A.DB, s.DefaultProject, endpoint.OwnerID)
	require.NoError(s.T(), err)

	originalLicenser := s.ConvoyApp.A.Licenser
	s.ConvoyApp.A.Licenser = projectDisabledLicenser{Licenser: originalLicenser, disabledProjectID: s.DefaultProject.UID}
	defer func() { s.ConvoyApp.A.Licenser = originalLicenser }()

	body := serialize(`{
		"function": "function transform(payload) { return payload; }",
		"payload": {"name": "test"}
	}`)
	url := fmt.Sprintf("/portal-api/subscriptions/test_function?token=%s", portalLink.Token)
	req := createRequest(http.MethodPost, url, portalLink.Token, body)
	w := httptest.NewRecorder()

	s.Router.ServeHTTP(w, req)

	require.Equal(s.T(), http.StatusBadRequest, w.Code)
}

func (s *PortalEventIntegrationTestSuite) Test_GetEventsPaged() {
	ownerID := "portal-owner-" + ulid.Make().String()
	sourceID := ulid.Make().String()

	vc := &datastore.VerifierConfig{
		Type: datastore.BasicAuthVerifier,
		BasicAuth: &datastore.BasicAuth{
			UserName: "Convoy",
			Password: "Convoy",
		},
	}
	_, err := testdb.SeedSource(s.ConvoyApp.A.DB, s.DefaultProject, sourceID, ulid.Make().String(), "", vc, "", "")
	require.NoError(s.T(), err)

	allowedA, err := testdb.SeedEndpoint(s.ConvoyApp.A.DB, s.DefaultProject, ulid.Make().String(), "allowed-a", ownerID, false, datastore.ActiveEndpointStatus)
	require.NoError(s.T(), err)
	allowedB, err := testdb.SeedEndpoint(s.ConvoyApp.A.DB, s.DefaultProject, ulid.Make().String(), "allowed-b", ownerID, false, datastore.ActiveEndpointStatus)
	require.NoError(s.T(), err)
	outside, err := testdb.SeedEndpoint(s.ConvoyApp.A.DB, s.DefaultProject, ulid.Make().String(), "outside", "other-owner-"+ulid.Make().String(), false, datastore.ActiveEndpointStatus)
	require.NoError(s.T(), err)

	eventA, err := testdb.SeedEvent(s.ConvoyApp.A.DB, allowedA, s.DefaultProject.UID, ulid.Make().String(), "*", sourceID, []byte(`{}`))
	require.NoError(s.T(), err)
	eventB, err := testdb.SeedEvent(s.ConvoyApp.A.DB, allowedB, s.DefaultProject.UID, ulid.Make().String(), "*", sourceID, []byte(`{}`))
	require.NoError(s.T(), err)
	_, err = testdb.SeedEvent(s.ConvoyApp.A.DB, outside, s.DefaultProject.UID, ulid.Make().String(), "*", sourceID, []byte(`{}`))
	require.NoError(s.T(), err)

	portalLink, err := testdb.SeedPortalLink(s.ConvoyApp.A.DB, s.DefaultProject, ownerID)
	require.NoError(s.T(), err)

	fetch := func(query string) []datastore.Event {
		s.T().Helper()
		url := fmt.Sprintf("/portal-api/events?%ssourceId=%s&token=%s", query, sourceID, portalLink.Token)
		req := createRequest(http.MethodGet, url, portalLink.Token, nil)
		w := httptest.NewRecorder()
		s.Router.ServeHTTP(w, req)
		require.Equal(s.T(), http.StatusOK, w.Code)

		var respEvents []datastore.Event
		resp := pagedResponse{Content: &respEvents}
		parseResponse(s.T(), w.Result(), &resp)
		return respEvents
	}

	unfiltered := fetch("")
	require.Len(s.T(), unfiltered, 2)
	unfilteredUIDs := map[string]struct{}{unfiltered[0].UID: {}, unfiltered[1].UID: {}}
	_, okA := unfilteredUIDs[eventA.UID]
	_, okB := unfilteredUIDs[eventB.UID]
	require.True(s.T(), okA && okB)

	filteredAllowed := fetch(fmt.Sprintf("endpointId=%s&", allowedA.UID))
	require.Len(s.T(), filteredAllowed, 1)
	require.Equal(s.T(), eventA.UID, filteredAllowed[0].UID)

	filteredOutside := fetch(fmt.Sprintf("endpointId=%s&", outside.UID))
	require.Len(s.T(), filteredOutside, 0)
}

func (s *PortalEventIntegrationTestSuite) Test_GetEventDeliveriesPaged() {
	ownerID := "portal-owner-" + ulid.Make().String()

	allowedA, err := testdb.SeedEndpoint(s.ConvoyApp.A.DB, s.DefaultProject, ulid.Make().String(), "allowed-a", ownerID, false, datastore.ActiveEndpointStatus)
	require.NoError(s.T(), err)

	allowedB, err := testdb.SeedEndpoint(s.ConvoyApp.A.DB, s.DefaultProject, ulid.Make().String(), "allowed-b", ownerID, false, datastore.ActiveEndpointStatus)
	require.NoError(s.T(), err)

	outside, err := testdb.SeedEndpoint(s.ConvoyApp.A.DB, s.DefaultProject, ulid.Make().String(), "outside", "other-owner-"+ulid.Make().String(), false, datastore.ActiveEndpointStatus)
	require.NoError(s.T(), err)

	subA, err := testdb.SeedSubscription(s.ConvoyApp.A.DB, s.DefaultProject, ulid.Make().String(), datastore.OutgoingProject, &datastore.Source{}, allowedA, &datastore.RetryConfiguration{}, &datastore.AlertConfiguration{}, &datastore.FilterConfiguration{
		EventTypes: []string{"*"},
		Filter:     datastore.FilterSchema{Headers: datastore.M{}, Body: datastore.M{}},
	})
	require.NoError(s.T(), err)

	subB, err := testdb.SeedSubscription(s.ConvoyApp.A.DB, s.DefaultProject, ulid.Make().String(), datastore.OutgoingProject, &datastore.Source{}, allowedB, &datastore.RetryConfiguration{}, &datastore.AlertConfiguration{}, &datastore.FilterConfiguration{
		EventTypes: []string{"*"},
		Filter:     datastore.FilterSchema{Headers: datastore.M{}, Body: datastore.M{}},
	})
	require.NoError(s.T(), err)

	subOutside, err := testdb.SeedSubscription(s.ConvoyApp.A.DB, s.DefaultProject, ulid.Make().String(), datastore.OutgoingProject, &datastore.Source{}, outside, &datastore.RetryConfiguration{}, &datastore.AlertConfiguration{}, &datastore.FilterConfiguration{
		EventTypes: []string{"*"},
		Filter:     datastore.FilterSchema{Headers: datastore.M{}, Body: datastore.M{}},
	})
	require.NoError(s.T(), err)

	eventA, err := testdb.SeedEvent(s.ConvoyApp.A.DB, allowedA, s.DefaultProject.UID, ulid.Make().String(), "*", "", []byte(`{}`))
	require.NoError(s.T(), err)
	deliveryA, err := testdb.SeedEventDelivery(s.ConvoyApp.A.DB, eventA, allowedA, s.DefaultProject.UID, ulid.Make().String(), datastore.FailureEventStatus, subA)
	require.NoError(s.T(), err)

	eventB, err := testdb.SeedEvent(s.ConvoyApp.A.DB, allowedB, s.DefaultProject.UID, ulid.Make().String(), "*", "", []byte(`{}`))
	require.NoError(s.T(), err)
	deliveryB, err := testdb.SeedEventDelivery(s.ConvoyApp.A.DB, eventB, allowedB, s.DefaultProject.UID, ulid.Make().String(), datastore.FailureEventStatus, subB)
	require.NoError(s.T(), err)

	eventOutside, err := testdb.SeedEvent(s.ConvoyApp.A.DB, outside, s.DefaultProject.UID, ulid.Make().String(), "*", "", []byte(`{}`))
	require.NoError(s.T(), err)
	_, err = testdb.SeedEventDelivery(s.ConvoyApp.A.DB, eventOutside, outside, s.DefaultProject.UID, ulid.Make().String(), datastore.FailureEventStatus, subOutside)
	require.NoError(s.T(), err)

	portalLink, err := testdb.SeedPortalLink(s.ConvoyApp.A.DB, s.DefaultProject, ownerID)
	require.NoError(s.T(), err)

	fetch := func(query string) []datastore.EventDelivery {
		s.T().Helper()
		url := fmt.Sprintf("/portal-api/eventdeliveries?%stoken=%s", query, portalLink.Token)
		req := createRequest(http.MethodGet, url, portalLink.Token, nil)
		w := httptest.NewRecorder()
		s.Router.ServeHTTP(w, req)
		require.Equal(s.T(), http.StatusOK, w.Code)

		var respEvents []datastore.EventDelivery
		resp := pagedResponse{Content: &respEvents}
		parseResponse(s.T(), w.Result(), &resp)
		return respEvents
	}

	// No endpoint filter: every delivery under the portal owner, none outside.
	unfiltered := fetch("")
	require.Len(s.T(), unfiltered, 2)
	unfilteredUIDs := map[string]struct{}{unfiltered[0].UID: {}, unfiltered[1].UID: {}}
	_, okA := unfilteredUIDs[deliveryA.UID]
	_, okB := unfilteredUIDs[deliveryB.UID]
	require.True(s.T(), okA && okB)

	// Filter to an allowed endpoint: only that endpoint's deliveries.
	filteredAllowed := fetch(fmt.Sprintf("endpointId=%s&", allowedA.UID))
	require.Len(s.T(), filteredAllowed, 1)
	require.Equal(s.T(), deliveryA.UID, filteredAllowed[0].UID)

	// Filter to an endpoint outside the portal allowlist: empty, never widened.
	filteredOutside := fetch(fmt.Sprintf("endpointId=%s&", outside.UID))
	require.Len(s.T(), filteredOutside, 0)
}

func TestPortalEventIntegrationTestSuite(t *testing.T) {
	suite.Run(t, new(PortalEventIntegrationTestSuite))
}
