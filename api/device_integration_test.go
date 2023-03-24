//go:build integration
// +build integration

package api

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

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

type DeviceIntegrationTestSuite struct {
	suite.Suite
	DB              database.Database
	Router          http.Handler
	ConvoyApp       *ApplicationHandler
	AuthenticatorFn AuthenticatorFn
	DefaultOrg      *datastore.Organisation
	DefaultProject  *datastore.Project
	DefaultUser     *datastore.User
	APIKey          string
}

func (d *DeviceIntegrationTestSuite) SetupSuite() {
	d.DB = getDB()
	d.ConvoyApp = buildServer()
	d.Router = d.ConvoyApp.BuildRoutes()
}

func (d *DeviceIntegrationTestSuite) SetupTest() {
	testdb.PurgeDB(d.T(), d.DB)

	user, err := testdb.SeedDefaultUser(d.ConvoyApp.A.DB)
	require.NoError(d.T(), err)
	d.DefaultUser = user

	org, err := testdb.SeedDefaultOrganisation(d.ConvoyApp.A.DB, user)
	require.NoError(d.T(), err)
	d.DefaultOrg = org

	// Setup Default Project.
	d.DefaultProject, err = testdb.SeedDefaultProject(d.ConvoyApp.A.DB, org.UID)
	require.NoError(d.T(), err)

	d.AuthenticatorFn = authenticateRequest(&models.LoginUser{
		Username: user.Email,
		Password: testdb.DefaultUserPassword,
	})

	// Setup Config.
	err = config.LoadConfig("./testdata/Auth_Config/full-convoy-with-jwt-realm.json")
	require.NoError(d.T(), err)

	apiRepo := postgres.NewAPIKeyRepo(d.ConvoyApp.A.DB)
	userRepo := postgres.NewUserRepo(d.ConvoyApp.A.DB)
	initRealmChain(d.T(), apiRepo, userRepo, d.ConvoyApp.A.Cache)
}

func (d *DeviceIntegrationTestSuite) TearDownTest() {
	testdb.PurgeDB(d.T(), d.DB)
	metrics.Reset()
}

func (d *DeviceIntegrationTestSuite) Test_FetchDevicesByEndpointID() {
	expectedStatusCode := http.StatusOK

	endpoint, err := testdb.SeedEndpoint(d.ConvoyApp.A.DB, d.DefaultProject, "", "", "", false, datastore.ActiveEndpointStatus)
	require.NoError(d.T(), err)

	// Just Before.
	_ = testdb.SeedDevice(d.ConvoyApp.A.DB, d.DefaultProject, endpoint.UID)

	// Arrange
	url := fmt.Sprintf("/ui/organisations/%s/projects/%s/endpoints/%s/devices", d.DefaultOrg.UID, d.DefaultProject.UID, endpoint.UID)
	req := createRequest(http.MethodGet, url, d.APIKey, nil)
	err = d.AuthenticatorFn(req, d.Router)
	require.NoError(d.T(), err)
	w := httptest.NewRecorder()

	// Act.
	d.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(d.T(), expectedStatusCode, w.Code)

	// Deep Assert.
	var resp pagedResponse
	parseResponse(d.T(), w.Result(), &resp)
	require.Equal(d.T(), 1, len(resp.Content.([]interface{})))
}

func TestDeviceIntegrationTestSuite(t *testing.T) {
	suite.Run(t, new(DeviceIntegrationTestSuite))
}
