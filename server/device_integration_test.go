//go:build integration
// +build integration

package server

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/datastore"
	cm "github.com/frain-dev/convoy/datastore/mongo"
	convoyMongo "github.com/frain-dev/convoy/datastore/mongo"
	"github.com/frain-dev/convoy/internal/pkg/metrics"
	"github.com/frain-dev/convoy/server/models"
	"github.com/frain-dev/convoy/server/testdb"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type DeviceIntegrationTestSuite struct {
	suite.Suite
	DB              convoyMongo.Client
	Router          http.Handler
	ConvoyApp       *ApplicationHandler
	AuthenticatorFn AuthenticatorFn
	DefaultOrg      *datastore.Organisation
	DefaultGroup    *datastore.Group
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

	// Setup Default Group.
	d.DefaultGroup, _ = testdb.SeedDefaultGroup(d.ConvoyApp.A.Store, "")

	user, err := testdb.SeedDefaultUser(d.ConvoyApp.A.Store)
	require.NoError(d.T(), err)
	d.DefaultUser = user

	org, err := testdb.SeedDefaultOrganisation(d.ConvoyApp.A.Store, user)
	require.NoError(d.T(), err)
	d.DefaultOrg = org

	d.AuthenticatorFn = authenticateRequest(&models.LoginUser{
		Username: user.Email,
		Password: testdb.DefaultUserPassword,
	})

	// Setup Config.
	err = config.LoadConfig("./testdata/Auth_Config/full-convoy-with-jwt-realm.json")
	require.NoError(d.T(), err)

	apiRepo := cm.NewApiKeyRepo(d.ConvoyApp.A.Store)
	userRepo := cm.NewUserRepo(d.ConvoyApp.A.Store)
	initRealmChain(d.T(), apiRepo, userRepo, d.ConvoyApp.A.Cache)
}

func (d *DeviceIntegrationTestSuite) TearDownTest() {
	testdb.PurgeDB(d.T(), d.DB)
	metrics.Reset()
}

func (d *DeviceIntegrationTestSuite) Test_FetchDevicesByAppID() {
	expectedStatusCode := http.StatusOK

	app, err := testdb.SeedApplication(d.ConvoyApp.A.Store, d.DefaultGroup, "", "", false)
	require.NoError(d.T(), err)

	// Just Before.
	_ = testdb.SeedDevice(d.ConvoyApp.A.Store, d.DefaultGroup, app.UID)

	// Arrange
	url := fmt.Sprintf("/ui/organisations/%s/projects/%s/apps/%s/devices", d.DefaultOrg.UID, d.DefaultGroup.UID, app.UID)
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
	require.Equal(d.T(), int64(1), resp.Pagination.Total)
}

func TestDeviceIntegrationTestSuite(t *testing.T) {
	suite.Run(t, new(DeviceIntegrationTestSuite))
}
