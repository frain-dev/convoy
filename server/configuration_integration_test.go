//go:build integration
// +build integration

package server

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/frain-dev/convoy/internal/pkg/metrics"
	"github.com/frain-dev/convoy/internal/pkg/server"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/server/models"
	"github.com/frain-dev/convoy/server/testdb"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type ConfigurationIntegrationTestSuite struct {
	suite.Suite
	DB              datastore.DatabaseClient
	Router          http.Handler
	ConvoyApp       *server.Server
	AuthenticatorFn AuthenticatorFn
	DefaultOrg      *datastore.Organisation
	DefaultGroup    *datastore.Group
	DefaultUser     *datastore.User
}

func (c *ConfigurationIntegrationTestSuite) SetupSuite() {
	c.DB = getDB()
	c.ConvoyApp = buildServer()
	c.Router = BuildRoutes(c.ConvoyApp)
}

func (c *ConfigurationIntegrationTestSuite) SetupTest() {
	testdb.PurgeDB(c.DB)

	// Setup Default Group
	c.DefaultGroup, _ = testdb.SeedDefaultGroup(c.DB, "")

	user, err := testdb.SeedDefaultUser(c.DB)
	require.NoError(c.T(), err)
	c.DefaultUser = user

	org, err := testdb.SeedDefaultOrganisation(c.DB, user)
	require.NoError(c.T(), err)
	c.DefaultOrg = org

	c.AuthenticatorFn = authenticateRequest(&models.LoginUser{
		Username: user.Email,
		Password: testdb.DefaultUserPassword,
	})

	// Setup Config.
	err = config.LoadConfig("./testdata/Auth_Config/full-convoy-with-jwt-realm.json")
	require.NoError(c.T(), err)

	initRealmChain(c.T(), c.DB.APIRepo(), c.DB.UserRepo(), c.ConvoyApp.Cache)
}

func (c *ConfigurationIntegrationTestSuite) TearDownTest() {
	testdb.PurgeDB(c.DB)
	metrics.Reset()
}

func (c *ConfigurationIntegrationTestSuite) Test_LoadConfiguration() {
	config, err := testdb.SeedConfiguration(c.DB)
	require.NoError(c.T(), err)

	// Arrange Request
	url := "/ui/configuration"
	req := createRequest(http.MethodGet, url, "", nil)
	err = c.AuthenticatorFn(req, c.Router)
	require.NoError(c.T(), err)

	w := httptest.NewRecorder()

	// Act
	c.Router.ServeHTTP(w, req)

	// Assert
	require.Equal(c.T(), http.StatusOK, w.Code)

	var newConfig []*models.ConfigurationResponse
	parseResponse(c.T(), w.Result(), &newConfig)

	require.NotEmpty(c.T(), newConfig[0].UID)
	require.Equal(c.T(), config.UID, newConfig[0].UID)
	require.Equal(c.T(), config.IsAnalyticsEnabled, newConfig[0].IsAnalyticsEnabled)
	require.Equal(c.T(), config.StoragePolicy.OnPrem.Path, convoy.DefaultOnPremDir)
	require.Equal(c.T(), convoy.GetVersion(), newConfig[0].ApiVersion)
}

func (c *ConfigurationIntegrationTestSuite) Test_CreateConfiguration() {
	// Arrange Request
	bodyStr := `{
		"is_analytics_enabled": true
	}`

	body := serialize(bodyStr)
	req := createRequest(http.MethodPost, "/ui/configuration", "", body)
	err := c.AuthenticatorFn(req, c.Router)
	require.NoError(c.T(), err)
	w := httptest.NewRecorder()

	// Act
	c.Router.ServeHTTP(w, req)

	// Assert
	require.Equal(c.T(), http.StatusCreated, w.Code)

	var config datastore.Configuration
	parseResponse(c.T(), w.Result(), &config)

	require.NotEmpty(c.T(), config.UID)
	require.True(c.T(), config.IsAnalyticsEnabled)
}

func (c *ConfigurationIntegrationTestSuite) Test_UpdateConfiguration() {
	_, err := testdb.SeedConfiguration(c.DB)
	require.NoError(c.T(), err)

	// Arrange Request
	bodyStr := `{
		"is_analytics_enabled": false,
		"storage_policy": {
			"type": "on_prem",
			"on_prem":{
				"path":"/tmp"
			}
		}
	}`

	body := serialize(bodyStr)
	req := createRequest(http.MethodPut, "/ui/configuration", "", body)
	err = c.AuthenticatorFn(req, c.Router)
	require.NoError(c.T(), err)
	w := httptest.NewRecorder()

	// Act
	c.Router.ServeHTTP(w, req)

	// Assert
	require.Equal(c.T(), http.StatusAccepted, w.Code)

	var config datastore.Configuration
	parseResponse(c.T(), w.Result(), &config)

	require.NotEmpty(c.T(), config.UID)
	require.Equal(c.T(), "/tmp", config.StoragePolicy.OnPrem.Path)
	require.False(c.T(), config.IsAnalyticsEnabled)

}

func TestConfigurationIntegrationTestSuite(t *testing.T) {
	suite.Run(t, new(ConfigurationIntegrationTestSuite))
}
