//go:build integration
// +build integration

package server

import (
	"net/http"
	"net/http/httptest"
	"testing"

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
	DB        datastore.DatabaseClient
	Router    http.Handler
	ConvoyApp *applicationHandler
}

func (c *ConfigurationIntegrationTestSuite) SetupSuite() {
	c.DB = getDB()
	c.ConvoyApp = buildApplication()
	c.Router = buildRoutes(c.ConvoyApp)
}

func (c *ConfigurationIntegrationTestSuite) SetupTest() {
	testdb.PurgeDB(c.DB)

	// Setup Config.
	err := config.LoadConfig("./testdata/Auth_Config/full-convoy.json")
	require.NoError(c.T(), err)

	initRealmChain(c.T(), c.DB.APIRepo(), c.DB.UserRepo(), c.ConvoyApp.cache)
}

func (c *ConfigurationIntegrationTestSuite) TearDownTest() {
	testdb.PurgeDB(c.DB)
}

func (c *ConfigurationIntegrationTestSuite) Test_LoadConfiguration() {
	config, err := testdb.SeedConfiguration(c.DB)
	require.NoError(c.T(), err)

	// Arrange Request
	url := "/ui/configuration"
	req := createRequest(http.MethodGet, url, nil)
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
	require.Equal(c.T(), convoy.GetVersion(), newConfig[0].ApiVersion)
}

func (c *ConfigurationIntegrationTestSuite) Test_CreateConfiguration() {
	// Arrange Request
	bodyStr := `{
		"is_analytics_enabled": true
	}`

	body := serialize(bodyStr)
	req := createRequest(http.MethodPost, "/ui/configuration", body)
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

func TestConfigurationIntegrationTestSuite(t *testing.T) {
	suite.Run(t, new(ConfigurationIntegrationTestSuite))
}
