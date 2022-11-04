//go:build integration
// +build integration

package server

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/frain-dev/convoy/internal/pkg/metrics"

	"github.com/google/uuid"

	"github.com/frain-dev/convoy/auth"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/datastore"
	cm "github.com/frain-dev/convoy/datastore/mongo"
	"github.com/frain-dev/convoy/server/testdb"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type AppPortalIntegrationTestSuite struct {
	suite.Suite
	DB           cm.Client
	Router       http.Handler
	ConvoyApp    *ApplicationHandler
	DefaultGroup *datastore.Group
	APIKey       string
}

func (s *AppPortalIntegrationTestSuite) SetupSuite() {
	s.DB = getDB()
	s.ConvoyApp = buildServer()
	s.Router = s.ConvoyApp.BuildRoutes()
}

func (s *AppPortalIntegrationTestSuite) SetupTest() {
	testdb.PurgeDB(s.T(), s.DB)

	// Setup Default Group.
	s.DefaultGroup, _ = testdb.SeedDefaultGroup(s.ConvoyApp.A.Store, "")

	// Setup Config.
	err := config.LoadConfig("./testdata/Auth_Config/full-convoy.json")
	require.NoError(s.T(), err)

	apiRepo := cm.NewApiKeyRepo(s.ConvoyApp.A.Store)
	userRepo := cm.NewUserRepo(s.ConvoyApp.A.Store)
	initRealmChain(s.T(), apiRepo, userRepo, s.ConvoyApp.A.Cache)
}

func (s *AppPortalIntegrationTestSuite) TearDownTest() {
	testdb.PurgeDB(s.T(), s.DB)
	metrics.Reset()
}

func TestAppPortalIntegrationSuiteTest(t *testing.T) {
	suite.Run(t, new(AppPortalIntegrationTestSuite))
}

func (s *AppPortalIntegrationTestSuite) Test_GetEndpointEvents() {
	expectedStatusCode := http.StatusOK

	// Just Before.
	endpoint1, err := testdb.SeedEndpoint(s.ConvoyApp.A.Store, s.DefaultGroup, uuid.NewString(), "", false)
	require.NoError(s.T(), err)

	endpoint2, err := testdb.SeedEndpoint(s.ConvoyApp.A.Store, s.DefaultGroup, uuid.NewString(), "", false)
	require.NoError(s.T(), err)

	for i := 0; i < 5; i++ {
		_, err = testdb.SeedEvent(s.ConvoyApp.A.Store, endpoint1, s.DefaultGroup.UID, uuid.NewString(), "*", "", []byte(`{}`))
		require.NoError(s.T(), err)

	}

	event, err := testdb.SeedEvent(s.ConvoyApp.A.Store, endpoint2, s.DefaultGroup.UID, uuid.NewString(), "*", "", []byte(`{}`))
	require.NoError(s.T(), err)

	role := auth.Role{
		Type:     auth.RoleAdmin,
		Group:    s.DefaultGroup.UID,
		Endpoint: endpoint2.UID,
	}

	// generate an app portal key
	_, key, err := testdb.SeedAPIKey(s.ConvoyApp.A.Store, role, uuid.NewString(), "test", "app_portal", "")
	require.NoError(s.T(), err)

	req := createRequest(http.MethodGet, "/portal/events", key, nil)
	w := httptest.NewRecorder()

	// Act
	s.Router.ServeHTTP(w, req)

	// Assert
	require.Equal(s.T(), expectedStatusCode, w.Code)

	var respEvents []datastore.Event
	resp := &pagedResponse{Content: &respEvents}

	parseResponse(s.T(), w.Result(), &resp)
	require.Equal(s.T(), int64(1), resp.Pagination.Total)
	require.Equal(s.T(), 1, len(respEvents))
	require.Equal(s.T(), event.UID, respEvents[0].UID)
}

func (s *AppPortalIntegrationTestSuite) Test_GetEndpointSubscriptions() {
	expectedStatusCode := http.StatusOK

	// Just Before.
	endpoint1, err := testdb.SeedEndpoint(s.ConvoyApp.A.Store, s.DefaultGroup, uuid.NewString(), "", false)
	require.NoError(s.T(), err)

	endpoint2, err := testdb.SeedEndpoint(s.ConvoyApp.A.Store, s.DefaultGroup, uuid.NewString(), "", false)
	require.NoError(s.T(), err)

	source := &datastore.Source{UID: uuid.NewString()}

	// seed subscriptions
	for i := 0; i < 5; i++ {
		_, err = testdb.SeedSubscription(s.ConvoyApp.A.Store, s.DefaultGroup, uuid.NewString(), datastore.OutgoingGroup, source, endpoint1, &datastore.RetryConfiguration{}, &datastore.AlertConfiguration{}, &datastore.FilterConfiguration{}, "")
		require.NoError(s.T(), err)

	}

	sub, err := testdb.SeedSubscription(s.ConvoyApp.A.Store, s.DefaultGroup, uuid.NewString(), datastore.OutgoingGroup, source, endpoint2, &datastore.RetryConfiguration{}, &datastore.AlertConfiguration{}, &datastore.FilterConfiguration{}, "")
	require.NoError(s.T(), err)

	role := auth.Role{
		Type:     auth.RoleAdmin,
		Group:    s.DefaultGroup.UID,
		Endpoint: endpoint2.UID,
	}

	// generate an app portal key
	_, key, err := testdb.SeedAPIKey(s.ConvoyApp.A.Store, role, uuid.NewString(), "test", "app_portal", "")
	require.NoError(s.T(), err)

	req := createRequest(http.MethodGet, "/portal/subscriptions", key, nil)
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
