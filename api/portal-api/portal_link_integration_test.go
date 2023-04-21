//go:build integration
// +build integration

package portalapi

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/oklog/ulid/v2"

	"github.com/frain-dev/convoy/api/testdb"
	"github.com/frain-dev/convoy/auth"
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
	DB             database.Database
	Router         http.Handler
	ConvoyApp      *PortalLinkHandler
	DefaultOrg     *datastore.Organisation
	DefaultProject *datastore.Project
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

	user, err := testdb.SeedDefaultUser(s.ConvoyApp.A.DB)
	require.NoError(s.T(), err)
	s.DefaultUser = user

	org, err := testdb.SeedDefaultOrganisation(s.ConvoyApp.A.DB, user)
	require.NoError(s.T(), err)

	// Setup Default Project.
	s.DefaultProject, err = testdb.SeedDefaultProject(s.ConvoyApp.A.DB, org.UID)
	require.NoError(s.T(), err)

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

func (s *PortalLinkIntegrationTestSuite) Test_GetPortalLinkEndpoints() {
	// Just Before
	endpoint1, err := testdb.SeedEndpoint(s.ConvoyApp.A.DB, s.DefaultProject, "", ulid.Make().String(), "", false, datastore.ActiveEndpointStatus)
	require.NoError(s.T(), err)

	endpoint2, err := testdb.SeedEndpoint(s.ConvoyApp.A.DB, s.DefaultProject, ulid.Make().String(), "", "", false, datastore.ActiveEndpointStatus)
	require.NoError(s.T(), err)

	portalLink, _ := testdb.SeedPortalLink(s.ConvoyApp.A.DB, s.DefaultProject, []string{endpoint1.UID, endpoint2.UID})
	require.NoError(s.T(), err)

	// Arrange Request
	url := fmt.Sprintf("/endpoints?token=%s", portalLink.Token)
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

func (s *PortalLinkIntegrationTestSuite) TearDownTest() {
	testdb.PurgeDB(s.T(), s.DB)
	metrics.Reset()
}

func TestPortalLinkIntegrationTestSuite(t *testing.T) {
	suite.Run(t, new(PortalLinkIntegrationTestSuite))
}
