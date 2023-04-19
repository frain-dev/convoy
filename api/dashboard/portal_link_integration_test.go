//go:build integration
// +build integration

package dashboard

import (
	"context"
	"fmt"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/oklog/ulid/v2"

	"github.com/frain-dev/convoy/api/models"
	"github.com/frain-dev/convoy/api/testdb"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/database"
	"github.com/frain-dev/convoy/database/postgres"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/internal/pkg/metrics"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type PortalLinkIntegrationTestSuite struct {
	suite.Suite
	DB              database.Database
	Router          http.Handler
	ConvoyApp       *DashboardHandler
	AuthenticatorFn AuthenticatorFn
	DefaultOrg      *datastore.Organisation
	DefaultProject  *datastore.Project
	DefaultUser     *datastore.User
}

func (s *PortalLinkIntegrationTestSuite) SetupSuite() {
	s.DB = getDB()
	s.ConvoyApp = buildServer()
	s.Router = s.ConvoyApp.BuildRoutes()
}

func (s *PortalLinkIntegrationTestSuite) SetupTest() {
	testdb.PurgeDB(s.T(), s.DB)
	s.DB = getDB()

	user, err := testdb.SeedDefaultUser(s.ConvoyApp.A.DB)
	require.NoError(s.T(), err)
	s.DefaultUser = user

	org, err := testdb.SeedDefaultOrganisation(s.ConvoyApp.A.DB, user)
	require.NoError(s.T(), err)
	s.DefaultOrg = org

	// Setup Default Project.
	s.DefaultProject, err = testdb.SeedDefaultProject(s.ConvoyApp.A.DB, org.UID)
	require.NoError(s.T(), err)

	s.AuthenticatorFn = authenticateRequest(&models.LoginUser{
		Username: user.Email,
		Password: testdb.DefaultUserPassword,
	})

	// Setup Config.
	err = config.LoadConfig("../testdata/Auth_Config/full-convoy-with-jwt-realm.json")
	require.NoError(s.T(), err)

	apiRepo := postgres.NewAPIKeyRepo(s.ConvoyApp.A.DB)
	userRepo := postgres.NewUserRepo(s.ConvoyApp.A.DB)
	initRealmChain(s.T(), apiRepo, userRepo, s.ConvoyApp.A.Cache)
}

func (s *PortalLinkIntegrationTestSuite) TearDownTest() {
	testdb.PurgeDB(s.T(), s.DB)
	metrics.Reset()
}

func (s *PortalLinkIntegrationTestSuite) Test_CreatePortalLink() {
	endpoint1, _ := testdb.SeedEndpoint(s.ConvoyApp.A.DB, s.DefaultProject, "", ulid.Make().String(), "", false, datastore.ActiveEndpointStatus)
	endpoint2, _ := testdb.SeedEndpoint(s.ConvoyApp.A.DB, s.DefaultProject, "", ulid.Make().String(), "", false, datastore.ActiveEndpointStatus)
	expectedStatusCode := http.StatusCreated

	// Arrange Request
	url := fmt.Sprintf("/organisations/%s/projects/%s/portal-links",
		s.DefaultProject.OrganisationID,
		s.DefaultProject.UID)
	plainBody := fmt.Sprintf(`{
		"name": "test_portal_link",
		"endpoints": ["%s", "%s"]
	}`, endpoint1.UID, endpoint2.UID)
	body := strings.NewReader(plainBody)
	req := createRequest(http.MethodPost, url, "", body)
	err := s.AuthenticatorFn(req, s.Router)
	require.NoError(s.T(), err)

	w := httptest.NewRecorder()

	// Act
	s.Router.ServeHTTP(w, req)

	// Assert
	require.Equal(s.T(), expectedStatusCode, w.Code)

	// Deep Assert.
	var resp models.PortalLinkResponse
	parseResponse(s.T(), w.Result(), &resp)

	portalLinkRepo := postgres.NewPortalLinkRepo(s.ConvoyApp.A.DB)
	pl, err := portalLinkRepo.FindPortalLinkByID(context.Background(), resp.ProjectID, resp.UID)
	require.NoError(s.T(), err)

	require.Equal(s.T(), resp.UID, pl.UID)
	require.Equal(s.T(), resp.URL, fmt.Sprintf("https://app.convoy.io/portal?token=%s", pl.Token))
	require.Equal(s.T(), resp.Name, pl.Name)
	require.Equal(s.T(), resp.Endpoints, []string(pl.Endpoints))
	require.Equal(s.T(), 2, resp.EndpointCount)
}

func (s *PortalLinkIntegrationTestSuite) Test_GetPortalLinkByID_PortalLinkNotFound() {
	portalLinkID := "123"

	// Arrange Request
	url := fmt.Sprintf("/organisations/%s/projects/%s/portal-links/%s", s.DefaultProject.OrganisationID, s.DefaultProject.UID, portalLinkID)
	req := createRequest(http.MethodGet, url, "", nil)
	err := s.AuthenticatorFn(req, s.Router)
	require.NoError(s.T(), err)
	w := httptest.NewRecorder()

	// Act
	s.Router.ServeHTTP(w, req)

	// Assert
	require.Equal(s.T(), http.StatusNotFound, w.Code)
}

func (s *PortalLinkIntegrationTestSuite) Test_GetPortalLinkByID_ValidPortalLink() {
	// Just Before
	endpoint, _ := testdb.SeedEndpoint(s.ConvoyApp.A.DB, s.DefaultProject, "", ulid.Make().String(), "", false, datastore.ActiveEndpointStatus)
	portalLink, _ := testdb.SeedPortalLink(s.ConvoyApp.A.DB, s.DefaultProject, []string{endpoint.UID})

	// Arrange Request
	url := fmt.Sprintf("/organisations/%s/projects/%s/portal-links/%s", s.DefaultProject.OrganisationID, s.DefaultProject.UID, portalLink.UID)
	req := createRequest(http.MethodGet, url, "", nil)
	err := s.AuthenticatorFn(req, s.Router)
	require.NoError(s.T(), err)
	w := httptest.NewRecorder()

	// Act
	s.Router.ServeHTTP(w, req)

	// Assert
	require.Equal(s.T(), http.StatusOK, w.Code)

	// Deep Assert
	var resp models.PortalLinkResponse
	parseResponse(s.T(), w.Result(), &resp)

	portalLinkRepo := postgres.NewPortalLinkRepo(s.ConvoyApp.A.DB)
	pl, err := portalLinkRepo.FindPortalLinkByID(context.Background(), resp.ProjectID, resp.UID)

	require.NoError(s.T(), err)

	require.Equal(s.T(), resp.UID, pl.UID)
	require.Equal(s.T(), resp.URL, fmt.Sprintf("https://app.convoy.io/portal?token=%s", pl.Token))
	require.Equal(s.T(), resp.Name, pl.Name)
	require.Equal(s.T(), resp.Endpoints, []string(pl.Endpoints))
	require.Equal(s.T(), 1, resp.EndpointCount)
}

func (s *PortalLinkIntegrationTestSuite) Test_GetPortalLinks_ValidPortalLinks() {
	r := rand.New(rand.NewSource(time.Now().Unix()))
	totalLinks := r.Intn(5)

	// Just Before
	for i := 0; i < totalLinks; i++ {
		endpoint, err := testdb.SeedEndpoint(s.ConvoyApp.A.DB, s.DefaultProject, ulid.Make().String(), "title", "", false, datastore.ActiveEndpointStatus)
		require.NoError(s.T(), err)

		_, err = testdb.SeedPortalLink(s.ConvoyApp.A.DB, s.DefaultProject, []string{endpoint.UID})
		require.NoError(s.T(), err)
	}

	// Arrange Request
	url := fmt.Sprintf("/organisations/%s/projects/%s/portal-links", s.DefaultProject.OrganisationID, s.DefaultProject.UID)
	req := createRequest(http.MethodGet, url, "", nil)
	err := s.AuthenticatorFn(req, s.Router)
	require.NoError(s.T(), err)
	w := httptest.NewRecorder()

	// Act
	s.Router.ServeHTTP(w, req)

	// Assert
	require.Equal(s.T(), http.StatusOK, w.Code)

	// Deep Assert
	var resp pagedResponse
	parseResponse(s.T(), w.Result(), &resp)
	require.Equal(s.T(), totalLinks, len(resp.Content.([]interface{})))
}

func (s *PortalLinkIntegrationTestSuite) Test_GetPortalLinks_ValidPortalLinks_FilterByEndpointID() {
	r := rand.New(rand.NewSource(time.Now().Unix()))
	totalLinks := r.Intn(5)

	// Just Before
	for i := 0; i < totalLinks; i++ {
		endpoint, err := testdb.SeedEndpoint(s.ConvoyApp.A.DB, s.DefaultProject, ulid.Make().String(), "title", "", false, datastore.ActiveEndpointStatus)
		require.NoError(s.T(), err)

		_, err = testdb.SeedPortalLink(s.ConvoyApp.A.DB, s.DefaultProject, []string{endpoint.UID})
		require.NoError(s.T(), err)
	}

	endpoint, _ := testdb.SeedEndpoint(s.ConvoyApp.A.DB, s.DefaultProject, "", ulid.Make().String(), "", false, datastore.ActiveEndpointStatus)
	_, _ = testdb.SeedPortalLink(s.ConvoyApp.A.DB, s.DefaultProject, []string{endpoint.UID})

	// Arrange Request
	url := fmt.Sprintf("/organisations/%s/projects/%s/portal-links?endpointId=%s", s.DefaultProject.OrganisationID, s.DefaultProject.UID, endpoint.UID)
	req := createRequest(http.MethodGet, url, "", nil)
	err := s.AuthenticatorFn(req, s.Router)
	require.NoError(s.T(), err)
	w := httptest.NewRecorder()

	// Act
	s.Router.ServeHTTP(w, req)

	// Assert
	require.Equal(s.T(), http.StatusOK, w.Code)

	// Deep Assert
	var resp pagedResponse
	parseResponse(s.T(), w.Result(), &resp)
	require.Equal(s.T(), 1, len(resp.Content.([]interface{})))
}

func (s *PortalLinkIntegrationTestSuite) Test_UpdatePortalLinks() {
	// Just Before
	endpoint1, _ := testdb.SeedEndpoint(s.ConvoyApp.A.DB, s.DefaultProject, "", ulid.Make().String(), "", false, datastore.ActiveEndpointStatus)
	endpoint2, _ := testdb.SeedEndpoint(s.ConvoyApp.A.DB, s.DefaultProject, "", ulid.Make().String(), "", false, datastore.ActiveEndpointStatus)
	portalLink, _ := testdb.SeedPortalLink(s.ConvoyApp.A.DB, s.DefaultProject, []string{endpoint1.UID})

	url := fmt.Sprintf("/organisations/%s/projects/%s/portal-links/%s", s.DefaultProject.OrganisationID, s.DefaultProject.UID, portalLink.UID)
	bodyStr := fmt.Sprintf(`{
		    "name": "test_portal_link",
			"endpoints": ["%s"]
		}`, endpoint2.UID)

	body := serialize(bodyStr)
	req := createRequest(http.MethodPut, url, "", body)
	err := s.AuthenticatorFn(req, s.Router)
	require.NoError(s.T(), err)
	w := httptest.NewRecorder()

	// Act
	s.Router.ServeHTTP(w, req)

	// Assert
	require.Equal(s.T(), http.StatusAccepted, w.Code)

	// Deep Assert
	var resp models.PortalLinkResponse
	parseResponse(s.T(), w.Result(), &resp)

	portalLinkRepo := postgres.NewPortalLinkRepo(s.ConvoyApp.A.DB)
	pl, err := portalLinkRepo.FindPortalLinkByID(context.Background(), resp.ProjectID, resp.UID)

	require.NoError(s.T(), err)

	require.Equal(s.T(), resp.UID, pl.UID)
	require.Equal(s.T(), resp.URL, fmt.Sprintf("https://app.convoy.io/portal?token=%s", pl.Token))
	require.Equal(s.T(), resp.Name, pl.Name)
	require.Equal(s.T(), resp.Endpoints, []string(pl.Endpoints))
}

func (s *PortalLinkIntegrationTestSuite) Test_RevokePortalLink() {
	// Just Before
	endpoint1, _ := testdb.SeedEndpoint(s.ConvoyApp.A.DB, s.DefaultProject, "", ulid.Make().String(), "", false, datastore.ActiveEndpointStatus)
	portalLink, _ := testdb.SeedPortalLink(s.ConvoyApp.A.DB, s.DefaultProject, []string{endpoint1.UID})

	// Arrange Request.
	url := fmt.Sprintf("/organisations/%s/projects/%s/portal-links/%s/revoke", s.DefaultProject.OrganisationID, s.DefaultProject.UID, portalLink.UID)
	req := createRequest(http.MethodPut, url, "", nil)
	err := s.AuthenticatorFn(req, s.Router)
	require.NoError(s.T(), err)
	w := httptest.NewRecorder()

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), http.StatusOK, w.Code)

	// Deep Assert.
	plRepo := postgres.NewPortalLinkRepo(s.ConvoyApp.A.DB)
	_, err = plRepo.FindPortalLinkByID(context.Background(), s.DefaultProject.UID, portalLink.UID)
	require.ErrorIs(s.T(), err, datastore.ErrPortalLinkNotFound)
}

func TestPortalLinkIntegrationTestSuite(t *testing.T) {
	suite.Run(t, new(PortalLinkIntegrationTestSuite))
}
