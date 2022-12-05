//go:build integration
// +build integration

package server

import (
	"context"
	"fmt"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/frain-dev/convoy/auth"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/datastore"
	cm "github.com/frain-dev/convoy/datastore/mongo"
	"github.com/frain-dev/convoy/internal/pkg/metrics"
	"github.com/frain-dev/convoy/server/models"
	"github.com/frain-dev/convoy/server/testdb"
	"github.com/google/uuid"
	"github.com/jaswdr/faker"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type PortalLinkIntegrationTestSuite struct {
	suite.Suite
	DB             cm.Client
	Router         http.Handler
	ConvoyApp      *ApplicationHandler
	DefaultOrg     *datastore.Organisation
	DefaultGroup   *datastore.Project
	DefaultUser    *datastore.User
	APIKey         string
	PersonalAPIKey string
}

func (s *PortalLinkIntegrationTestSuite) SetupSuite() {
	s.DB = getDB()
	s.ConvoyApp = buildServer()
	s.Router = s.ConvoyApp.BuildRoutes()
}

func (s *PortalLinkIntegrationTestSuite) SetupTest() {
	testdb.PurgeDB(s.T(), s.DB)

	// Setup Default Project.
	s.DefaultGroup, _ = testdb.SeedDefaultProject(s.ConvoyApp.A.Store, "")

	// Seed Auth
	role := auth.Role{
		Type:    auth.RoleAdmin,
		Project: s.DefaultGroup.UID,
	}

	_, s.APIKey, _ = testdb.SeedAPIKey(s.ConvoyApp.A.Store, role, "", "test", "", "")

	// Setup Config.
	err := config.LoadConfig("./testdata/Auth_Config/full-convoy.json")
	require.NoError(s.T(), err)

	apiRepo := cm.NewApiKeyRepo(s.ConvoyApp.A.Store)
	userRepo := cm.NewUserRepo(s.ConvoyApp.A.Store)
	initRealmChain(s.T(), apiRepo, userRepo, s.ConvoyApp.A.Cache)
}

func (s *PortalLinkIntegrationTestSuite) TearDownTest() {
	testdb.PurgeDB(s.T(), s.DB)
	metrics.Reset()
}

func (s *PortalLinkIntegrationTestSuite) Test_CreatePortalLink() {
	endpoint1, _ := testdb.SeedEndpoint(s.ConvoyApp.A.Store, s.DefaultGroup, "", uuid.NewString(), "", false)
	endpoint2, _ := testdb.SeedEndpoint(s.ConvoyApp.A.Store, s.DefaultGroup, "", uuid.NewString(), "", false)
	expectedStatusCode := http.StatusCreated

	// Arrange Request
	url := fmt.Sprintf("/api/v1/projects/%s/portal-links", s.DefaultGroup.UID)
	plainBody := fmt.Sprintf(`{
		"name": "test_portal_link",
		"endpoints": ["%s", "%s"]
	}`, endpoint1.UID, endpoint2.UID)
	body := strings.NewReader(plainBody)
	req := createRequest(http.MethodPost, url, s.APIKey, body)
	w := httptest.NewRecorder()

	// Act
	s.Router.ServeHTTP(w, req)

	// Assert
	require.Equal(s.T(), expectedStatusCode, w.Code)

	// Deep Assert.
	var resp models.PortalLinkResponse
	parseResponse(s.T(), w.Result(), &resp)

	portalLinkRepo := cm.NewPortalLinkRepo(s.ConvoyApp.A.Store)
	pl, err := portalLinkRepo.FindPortalLinkByID(context.Background(), resp.ProjectID, resp.UID)
	require.NoError(s.T(), err)

	require.Equal(s.T(), resp.UID, pl.UID)
	require.Equal(s.T(), resp.URL, fmt.Sprintf("https://app.convoy.io/portal?token=%s", pl.Token))
	require.Equal(s.T(), resp.Name, pl.Name)
	require.Equal(s.T(), resp.Endpoints, pl.Endpoints)
	require.Equal(s.T(), 2, resp.EndpointCount)
}

func (s *PortalLinkIntegrationTestSuite) Test_GetPortalLinkByID_PortalLinkNotFound() {
	portalLinkID := "123"

	// Arrange Request
	url := fmt.Sprintf("/api/v1/projects/%s/portal-links/%s", s.DefaultGroup.ID, portalLinkID)
	req := createRequest(http.MethodGet, url, s.APIKey, nil)
	w := httptest.NewRecorder()

	// Act
	s.Router.ServeHTTP(w, req)

	// Assert
	require.Equal(s.T(), http.StatusNotFound, w.Code)
}

func (s *PortalLinkIntegrationTestSuite) Test_GetPortalLinkByID_ValidPortalLink() {
	// Just Before
	endpoint, _ := testdb.SeedEndpoint(s.ConvoyApp.A.Store, s.DefaultGroup, "", uuid.NewString(), "", false)
	portalLink, _ := testdb.SeedPortalLink(s.ConvoyApp.A.Store, s.DefaultGroup, []string{endpoint.UID})

	// Arrange Request
	url := fmt.Sprintf("/api/v1/projects/%s/portal-links/%s", s.DefaultGroup.UID, portalLink.UID)
	req := createRequest(http.MethodGet, url, s.APIKey, nil)
	w := httptest.NewRecorder()

	// Act
	s.Router.ServeHTTP(w, req)

	// Assert
	require.Equal(s.T(), http.StatusOK, w.Code)

	// Deep Assert
	var resp models.PortalLinkResponse
	parseResponse(s.T(), w.Result(), &resp)

	portalLinkRepo := cm.NewPortalLinkRepo(s.ConvoyApp.A.Store)
	pl, err := portalLinkRepo.FindPortalLinkByID(context.Background(), resp.ProjectID, resp.UID)

	require.NoError(s.T(), err)

	require.Equal(s.T(), resp.UID, pl.UID)
	require.Equal(s.T(), resp.URL, fmt.Sprintf("https://app.convoy.io/portal?token=%s", pl.Token))
	require.Equal(s.T(), resp.Name, pl.Name)
	require.Equal(s.T(), resp.Endpoints, pl.Endpoints)
	require.Equal(s.T(), 1, resp.EndpointCount)
}

func (s *PortalLinkIntegrationTestSuite) Test_GetPortalLinks_ValidPortalLinks() {
	r := rand.New(rand.NewSource(time.Now().Unix()))
	totalLinks := r.Intn(5)

	// Just Before
	for i := 0; i < totalLinks; i++ {
		_, _ = testdb.SeedPortalLink(s.ConvoyApp.A.Store, s.DefaultGroup, []string{uuid.NewString()})
	}

	// Arrange Request
	url := fmt.Sprintf("/api/v1/projects/%s/portal-links", s.DefaultGroup.UID)
	req := createRequest(http.MethodGet, url, s.APIKey, nil)
	w := httptest.NewRecorder()

	// Act
	s.Router.ServeHTTP(w, req)

	// Assert
	require.Equal(s.T(), http.StatusOK, w.Code)

	// Deep Assert
	var resp pagedResponse
	parseResponse(s.T(), w.Result(), &resp)
	require.Equal(s.T(), int64(totalLinks), resp.Pagination.Total)
}

func (s *PortalLinkIntegrationTestSuite) Test_GetPortalLinks_ValidPortalLinks_FilterByEndpointID() {
	r := rand.New(rand.NewSource(time.Now().Unix()))
	totalLinks := r.Intn(5)

	// Just Before
	for i := 0; i < totalLinks; i++ {
		_, _ = testdb.SeedPortalLink(s.ConvoyApp.A.Store, s.DefaultGroup, []string{uuid.NewString()})
	}

	endpoint, _ := testdb.SeedEndpoint(s.ConvoyApp.A.Store, s.DefaultGroup, "", uuid.NewString(), "", false)
	_, _ = testdb.SeedPortalLink(s.ConvoyApp.A.Store, s.DefaultGroup, []string{endpoint.UID})

	// Arrange Request
	url := fmt.Sprintf("/api/v1/projects/%s/portal-links?endpointId=%s", s.DefaultGroup.UID, endpoint.UID)
	req := createRequest(http.MethodGet, url, s.APIKey, nil)
	w := httptest.NewRecorder()

	// Act
	s.Router.ServeHTTP(w, req)

	// Assert
	require.Equal(s.T(), http.StatusOK, w.Code)

	// Deep Assert
	var resp pagedResponse
	parseResponse(s.T(), w.Result(), &resp)
	require.Equal(s.T(), int64(1), resp.Pagination.Total)
}

func (s *PortalLinkIntegrationTestSuite) Test_UpdatePortalLinks() {
	// Just Before
	endpoint1, _ := testdb.SeedEndpoint(s.ConvoyApp.A.Store, s.DefaultGroup, "", uuid.NewString(), "", false)
	endpoint2, _ := testdb.SeedEndpoint(s.ConvoyApp.A.Store, s.DefaultGroup, "", uuid.NewString(), "", false)
	portalLink, _ := testdb.SeedPortalLink(s.ConvoyApp.A.Store, s.DefaultGroup, []string{endpoint1.UID})

	url := fmt.Sprintf("/api/v1/projects/%s/portal-links/%s", s.DefaultGroup.UID, portalLink.UID)
	bodyStr := fmt.Sprintf(`{
		    "name": "test_portal_link",
			"endpoints": ["%s"]
		}`, endpoint2.UID)

	body := serialize(bodyStr)
	req := createRequest(http.MethodPut, url, s.APIKey, body)
	w := httptest.NewRecorder()

	// Act
	s.Router.ServeHTTP(w, req)

	// Assert
	require.Equal(s.T(), http.StatusAccepted, w.Code)

	// Deep Assert
	var resp models.PortalLinkResponse
	parseResponse(s.T(), w.Result(), &resp)

	portalLinkRepo := cm.NewPortalLinkRepo(s.ConvoyApp.A.Store)
	pl, err := portalLinkRepo.FindPortalLinkByID(context.Background(), resp.ProjectID, resp.UID)

	require.NoError(s.T(), err)

	require.Equal(s.T(), resp.UID, pl.UID)
	require.Equal(s.T(), resp.URL, fmt.Sprintf("https://app.convoy.io/portal?token=%s", pl.Token))
	require.Equal(s.T(), resp.Name, pl.Name)
	require.Equal(s.T(), resp.Endpoints, pl.Endpoints)
}

func (s *PortalLinkIntegrationTestSuite) Test_RevokePortalLink() {
	// Just Before
	endpoint1, _ := testdb.SeedEndpoint(s.ConvoyApp.A.Store, s.DefaultGroup, "", uuid.NewString(), "", false)
	portalLink, _ := testdb.SeedPortalLink(s.ConvoyApp.A.Store, s.DefaultGroup, []string{endpoint1.UID})

	// Arrange Request.
	url := fmt.Sprintf("/api/v1/projects/%s/portal-links/%s/revoke", s.DefaultGroup.UID, portalLink.UID)
	req := createRequest(http.MethodPut, url, s.APIKey, nil)
	w := httptest.NewRecorder()

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), http.StatusOK, w.Code)

	// Deep Assert.
	plRepo := cm.NewPortalLinkRepo(s.ConvoyApp.A.Store)
	_, err := plRepo.FindPortalLinkByID(context.Background(), s.DefaultGroup.UID, portalLink.UID)
	require.ErrorIs(s.T(), err, datastore.ErrPortalLinkNotFound)
}

func (s *PortalLinkIntegrationTestSuite) Test_GetPortalLinkEndpoints() {
	// Just Before
	endpoint1, err := testdb.SeedEndpoint(s.ConvoyApp.A.Store, s.DefaultGroup, "", uuid.NewString(), "", false)
	require.NoError(s.T(), err)

	endpoint2, err := testdb.SeedEndpoint(s.ConvoyApp.A.Store, s.DefaultGroup, uuid.NewString(), "", "", false)
	require.NoError(s.T(), err)

	portalLink, _ := testdb.SeedPortalLink(s.ConvoyApp.A.Store, s.DefaultGroup, []string{endpoint1.UID, endpoint2.UID})
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

func (s *PortalLinkIntegrationTestSuite) Test_CreatePortalLinkEndpoint() {
	endpointTitle := fmt.Sprintf("Test-%s", uuid.New().String())
	endpointURL := faker.New().Internet().URL()
	expectedStatusCode := http.StatusCreated

	endpoint1, err := testdb.SeedEndpoint(s.ConvoyApp.A.Store, s.DefaultGroup, "", uuid.NewString(), "", false)
	require.NoError(s.T(), err)

	portalLink, _ := testdb.SeedPortalLink(s.ConvoyApp.A.Store, s.DefaultGroup, []string{endpoint1.UID})
	require.NoError(s.T(), err)

	// Arrange Request
	url := fmt.Sprintf("/portal-api/endpoints?token=%s", portalLink.Token)
	plainBody := fmt.Sprintf(`{
		"name": "%s",
		"description": "test endpoint",
		"url": "%s"
	}`, endpointTitle, endpointURL)

	body := strings.NewReader(plainBody)
	req := createRequest(http.MethodPost, url, "", body)
	w := httptest.NewRecorder()

	// Act
	s.Router.ServeHTTP(w, req)

	// Assert
	require.Equal(s.T(), expectedStatusCode, w.Code)

	// Deep Assert
	var endpoint datastore.Endpoint
	parseResponse(s.T(), w.Result(), &endpoint)

	endpointRepo := cm.NewEndpointRepo(s.ConvoyApp.A.Store)
	dbEndpoint, err := endpointRepo.FindEndpointByID(context.Background(), endpoint.UID)
	require.NoError(s.T(), err)
	require.Equal(s.T(), endpointTitle, dbEndpoint.Title)
	require.Equal(s.T(), endpointURL, dbEndpoint.TargetURL)

	portalLinkRepo := cm.NewPortalLinkRepo(s.ConvoyApp.A.Store)
	dbPLink, err := portalLinkRepo.FindPortalLinkByID(context.Background(), s.DefaultGroup.UID, portalLink.UID)
	require.NoError(s.T(), err)
	require.Equal(s.T(), 2, len(dbPLink.Endpoints))
}

func (s *PortalLinkIntegrationTestSuite) Test_GetPortalLinkEndpointEvents() {
	expectedStatusCode := http.StatusOK

	// Just Before.
	endpoint1, err := testdb.SeedEndpoint(s.ConvoyApp.A.Store, s.DefaultGroup, uuid.NewString(), "", "", false)
	require.NoError(s.T(), err)

	endpoint2, err := testdb.SeedEndpoint(s.ConvoyApp.A.Store, s.DefaultGroup, uuid.NewString(), "", "", false)
	require.NoError(s.T(), err)

	portalLink, err := testdb.SeedPortalLink(s.ConvoyApp.A.Store, s.DefaultGroup, []string{endpoint1.UID, endpoint2.UID})
	require.NoError(s.T(), err)

	for i := 0; i < 5; i++ {
		_, err = testdb.SeedEvent(s.ConvoyApp.A.Store, endpoint1, s.DefaultGroup.UID, uuid.NewString(), "*", "", []byte(`{}`))
		require.NoError(s.T(), err)

	}

	_, err = testdb.SeedEvent(s.ConvoyApp.A.Store, endpoint2, s.DefaultGroup.UID, uuid.NewString(), "*", "", []byte(`{}`))
	require.NoError(s.T(), err)

	req := createRequest(http.MethodGet, fmt.Sprintf("/portal-api/events?token=%s", portalLink.Token), "", nil)
	w := httptest.NewRecorder()

	// Act
	s.Router.ServeHTTP(w, req)

	// Assert
	require.Equal(s.T(), expectedStatusCode, w.Code)

	var respEvents []datastore.Event
	resp := &pagedResponse{Content: &respEvents}

	parseResponse(s.T(), w.Result(), &resp)
	require.Equal(s.T(), int64(6), resp.Pagination.Total)
	require.Equal(s.T(), 6, len(respEvents))
}

func (s *PortalLinkIntegrationTestSuite) Test_GetPortalLinkEndpointSubscriptions() {
	expectedStatusCode := http.StatusOK

	// Just Before.
	endpoint1, err := testdb.SeedEndpoint(s.ConvoyApp.A.Store, s.DefaultGroup, uuid.NewString(), "", "", false)
	require.NoError(s.T(), err)

	endpoint2, err := testdb.SeedEndpoint(s.ConvoyApp.A.Store, s.DefaultGroup, uuid.NewString(), "", "", false)
	require.NoError(s.T(), err)

	portalLink, err := testdb.SeedPortalLink(s.ConvoyApp.A.Store, s.DefaultGroup, []string{endpoint2.UID})
	require.NoError(s.T(), err)

	source := &datastore.Source{UID: uuid.NewString()}

	// seed subscriptions
	for i := 0; i < 5; i++ {
		_, err = testdb.SeedSubscription(s.ConvoyApp.A.Store, s.DefaultGroup, uuid.NewString(), datastore.OutgoingProject, source, endpoint1, &datastore.RetryConfiguration{}, &datastore.AlertConfiguration{}, &datastore.FilterConfiguration{}, "")
		require.NoError(s.T(), err)

	}

	sub, err := testdb.SeedSubscription(s.ConvoyApp.A.Store, s.DefaultGroup, uuid.NewString(), datastore.OutgoingProject, source, endpoint2, &datastore.RetryConfiguration{}, &datastore.AlertConfiguration{}, &datastore.FilterConfiguration{}, "")
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
	require.Equal(s.T(), int64(1), resp.Pagination.Total)
	require.Equal(s.T(), 1, len(respSubs))
	require.Equal(s.T(), sub.UID, respSubs[0].UID)
}

func TestPortalLinkIntegrationTestSuite(t *testing.T) {
	suite.Run(t, new(PortalLinkIntegrationTestSuite))
}
