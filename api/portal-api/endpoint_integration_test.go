//go:build integration
// +build integration

package portalapi

import (
	"context"
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

type EndpointIntegrationTestSuite struct {
	suite.Suite
	DB             database.Database
	Router         http.Handler
	ConvoyApp      *PortalLinkHandler
	DefaultOrg     *datastore.Organisation
	DefaultProject *datastore.Project
	DefaultUser    *datastore.User
	APIKey         string
	PersonalAPIKey string
}

func (s *EndpointIntegrationTestSuite) SetupSuite() {
	s.DB = getDB()
	s.ConvoyApp = buildServer()
	s.Router = s.ConvoyApp.BuildRoutes()
}

func (s *EndpointIntegrationTestSuite) SetupTest() {
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
	err = config.LoadConfig("../testdata/Auth_Config/full-convoy.json")
	require.NoError(s.T(), err)

	apiRepo := postgres.NewAPIKeyRepo(s.ConvoyApp.A.DB)
	userRepo := postgres.NewUserRepo(s.ConvoyApp.A.DB)
	initRealmChain(s.T(), apiRepo, userRepo, s.ConvoyApp.A.Cache)
}

func (s *EndpointIntegrationTestSuite) TearDownTest() {
	testdb.PurgeDB(s.T(), s.DB)
	metrics.Reset()
}

func (s *EndpointIntegrationTestSuite) Test_GetEndpoint_EndpointNotFound() {
	appID := "123"
	expectedStatusCode := http.StatusNotFound

	endpoint, err := testdb.SeedEndpoint(s.ConvoyApp.A.DB, s.DefaultProject, "", "", "", true, datastore.ActiveEndpointStatus)
	require.NoError(s.T(), err)

	portalLink, err := testdb.SeedPortalLink(s.ConvoyApp.A.DB, s.DefaultProject, []string{endpoint.UID})
	require.NoError(s.T(), err)

	// Arrange Request.
	url := fmt.Sprintf("/endpoints/%s?token=%s", appID, portalLink.Token)
	req := createRequest(http.MethodGet, url, s.APIKey, nil)
	w := httptest.NewRecorder()

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)
}

func (s *EndpointIntegrationTestSuite) Test_GetEndpoint_ValidEndpoint() {
	endpointID := "123456789"
	expectedStatusCode := http.StatusOK

	// Just Before.
	_, _ = testdb.SeedEndpoint(s.ConvoyApp.A.DB, s.DefaultProject, endpointID, "", "", true, datastore.ActiveEndpointStatus)

	portalLink, err := testdb.SeedPortalLink(s.ConvoyApp.A.DB, s.DefaultProject, []string{endpointID})
	require.NoError(s.T(), err)

	// Arrange Request.
	url := fmt.Sprintf("/endpoints/%s?token=%s", endpointID, portalLink.Token)
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

func (s *EndpointIntegrationTestSuite) Test_GetEndpoints_Filters() {
	s.T().Skip("Depends on #637")
}

func TestEndpointIntegrationTestSuite(t *testing.T) {
	suite.Run(t, new(EndpointIntegrationTestSuite))
}
