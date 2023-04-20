//go:build integration
// +build integration

package portalapi

//import (
//	"context"
//	"fmt"
//	"math/rand"
//	"net/http"
//	"net/http/httptest"
//	"testing"
//	"time"
//
//	"github.com/oklog/ulid/v2"
//
//	"github.com/frain-dev/convoy/database"
//	"github.com/frain-dev/convoy/database/postgres"
//	"github.com/frain-dev/convoy/internal/pkg/metrics"
//
//	"github.com/frain-dev/convoy/api/testdb"
//	"github.com/frain-dev/convoy/auth"
//	"github.com/frain-dev/convoy/config"
//	"github.com/frain-dev/convoy/datastore"
//	"github.com/stretchr/testify/require"
//	"github.com/stretchr/testify/suite"
//)
//
//type SubscriptionIntegrationTestSuite struct {
//	suite.Suite
//	DB             database.Database
//	Router         http.Handler
//	ConvoyApp      *PortalLinkHandler
//	DefaultOrg     *datastore.Organisation
//	DefaultProject *datastore.Project
//	APIKey         string
//}
//
//func (s *SubscriptionIntegrationTestSuite) SetupSuite() {
//	s.DB = getDB()
//	s.ConvoyApp = buildServer()
//	s.Router = s.ConvoyApp.BuildRoutes()
//}
//
//func (s *SubscriptionIntegrationTestSuite) SetupTest() {
//	testdb.PurgeDB(s.T(), s.DB)
//
//	user, err := testdb.SeedDefaultUser(s.ConvoyApp.A.DB)
//	require.NoError(s.T(), err)
//
//	org, err := testdb.SeedDefaultOrganisation(s.ConvoyApp.A.DB, user)
//	require.NoError(s.T(), err)
//	s.DefaultOrg = org
//
//	// Setup Default Project.
//	s.DefaultProject, err = testdb.SeedDefaultProject(s.ConvoyApp.A.DB, org.UID)
//	require.NoError(s.T(), err)
//
//	// Seed Auth
//	role := auth.Role{
//		Type:    auth.RoleAdmin,
//		Project: s.DefaultProject.UID,
//	}
//
//	_, s.APIKey, err = testdb.SeedAPIKey(s.ConvoyApp.A.DB, role, "", "test", "", "")
//	require.NoError(s.T(), err)
//
//	// Setup Config.
//	err = config.LoadConfig("../testdata/Auth_Config/full-convoy.json")
//	require.NoError(s.T(), err)
//
//	apiRepo := postgres.NewAPIKeyRepo(s.ConvoyApp.A.DB)
//	userRepo := postgres.NewUserRepo(s.ConvoyApp.A.DB)
//
//	initRealmChain(s.T(), apiRepo, userRepo, s.ConvoyApp.A.Cache)
//}
//
//func (s *SubscriptionIntegrationTestSuite) TearDownTest() {
//	testdb.PurgeDB(s.T(), s.DB)
//	metrics.Reset()
//}
//
//func (s *SubscriptionIntegrationTestSuite) Test_GetOneSubscription_SubscriptionNotFound() {
//	subscriptionId := "123"
//
//	portalLink, err := testdb.SeedPortalLink(s.ConvoyApp.A.DB, s.DefaultProject, []string{})
//	require.NoError(s.T(), err)
//
//	// Arrange Request
//	url := fmt.Sprintf("/subscriptions/%s?token=%s", subscriptionId, portalLink.Token)
//	req := createRequest(http.MethodGet, url, s.APIKey, nil)
//	w := httptest.NewRecorder()
//
//	// Act
//	s.Router.ServeHTTP(w, req)
//
//	// Assert
//	require.Equal(s.T(), http.StatusNotFound, w.Code)
//}
//
//func (s *SubscriptionIntegrationTestSuite) Test_GetOneSubscription_OutgoingProject_ValidSubscription() {
//	subscriptionId := ulid.Make().String()
//
//	project := s.DefaultProject
//
//	// Just Before
//	endpoint, err := testdb.SeedEndpoint(s.ConvoyApp.A.DB, project, ulid.Make().String(), "", "", false, datastore.ActiveEndpointStatus)
//	require.NoError(s.T(), err)
//
//	source, err := testdb.SeedSource(s.ConvoyApp.A.DB, project, ulid.Make().String(), "", "", nil)
//	require.NoError(s.T(), err)
//
//	_, err = testdb.SeedSubscription(s.ConvoyApp.A.DB, project, subscriptionId, project.Type, source, endpoint, &datastore.RetryConfiguration{}, &datastore.AlertConfiguration{}, nil)
//	require.NoError(s.T(), err)
//
//	portalLink, err := testdb.SeedPortalLink(s.ConvoyApp.A.DB, s.DefaultProject, []string{})
//	require.NoError(s.T(), err)
//
//	// Arrange Request
//	url := fmt.Sprintf("/subscriptions/%s?token=%s", subscriptionId, portalLink.Token)
//	req := createRequest(http.MethodGet, url, s.APIKey, nil)
//	w := httptest.NewRecorder()
//
//	// Act
//	s.Router.ServeHTTP(w, req)
//
//	// Assert
//	require.Equal(s.T(), http.StatusOK, w.Code)
//
//	// Deep Assert
//	var subscription *datastore.Subscription
//	parseResponse(s.T(), w.Result(), &subscription)
//
//	subRepo := postgres.NewSubscriptionRepo(s.ConvoyApp.A.DB)
//	dbSub, err := subRepo.FindSubscriptionByID(context.Background(), project.UID, subscriptionId)
//	require.NoError(s.T(), err)
//	require.Equal(s.T(), subscription.UID, dbSub.UID)
//	require.Equal(s.T(), subscription.Endpoint.UID, dbSub.EndpointID)
//}
//
//func (s *SubscriptionIntegrationTestSuite) Test_GetOneSubscription_IncomingProject_ValidSubscription() {
//	subscriptionId := ulid.Make().String()
//
//	project, err := testdb.SeedProject(s.ConvoyApp.A.DB, ulid.Make().String(), "test-project", s.DefaultOrg.UID, datastore.IncomingProject, &datastore.DefaultProjectConfig)
//	require.NoError(s.T(), err)
//
//	// Seed Auth
//	role := auth.Role{
//		Type:    auth.RoleAdmin,
//		Project: project.UID,
//	}
//
//	_, apiKey, err := testdb.SeedAPIKey(s.ConvoyApp.A.DB, role, "", "test", "", "")
//	require.NoError(s.T(), err)
//
//	// Just Before
//	endpoint, err := testdb.SeedEndpoint(s.ConvoyApp.A.DB, project, ulid.Make().String(), "", "", false, datastore.ActiveEndpointStatus)
//	require.NoError(s.T(), err)
//
//	source, err := testdb.SeedSource(s.ConvoyApp.A.DB, project, ulid.Make().String(), "", "", nil)
//	require.NoError(s.T(), err)
//
//	_, err = testdb.SeedSubscription(s.ConvoyApp.A.DB, project, subscriptionId, "incoming", source, endpoint, &datastore.RetryConfiguration{}, &datastore.AlertConfiguration{}, nil)
//	require.NoError(s.T(), err)
//
//	portalLink, err := testdb.SeedPortalLink(s.ConvoyApp.A.DB, s.DefaultProject, []string{})
//	require.NoError(s.T(), err)
//
//	// Arrange Request
//	url := fmt.Sprintf("/subscriptions/%s?token=%s", subscriptionId, portalLink.Token)
//	req := createRequest(http.MethodGet, url, apiKey, nil)
//	w := httptest.NewRecorder()
//
//	// Act
//	s.Router.ServeHTTP(w, req)
//
//	// Assert
//	require.Equal(s.T(), http.StatusOK, w.Code)
//
//	// Deep Assert
//	var subscription *datastore.Subscription
//	parseResponse(s.T(), w.Result(), &subscription)
//
//	subRepo := postgres.NewSubscriptionRepo(s.ConvoyApp.A.DB)
//	dbSub, err := subRepo.FindSubscriptionByID(context.Background(), project.UID, subscriptionId)
//	require.NoError(s.T(), err)
//	require.Equal(s.T(), subscription.UID, dbSub.UID)
//	require.Equal(s.T(), subscription.Source.UID, dbSub.SourceID)
//	require.Equal(s.T(), subscription.Endpoint.UID, dbSub.EndpointID)
//}
//
//func (s *SubscriptionIntegrationTestSuite) Test_GetSubscriptions_ValidSubscriptions() {
//	r := rand.New(rand.NewSource(time.Now().Unix()))
//	totalSubs := r.Intn(10)
//
//	for i := 0; i < totalSubs; i++ {
//		// Just Before
//		endpoint, err := testdb.SeedEndpoint(s.ConvoyApp.A.DB, s.DefaultProject, ulid.Make().String(), "", "", false, datastore.ActiveEndpointStatus)
//		require.NoError(s.T(), err)
//		source, err := testdb.SeedSource(s.ConvoyApp.A.DB, s.DefaultProject, ulid.Make().String(), "", "", nil)
//		require.NoError(s.T(), err)
//
//		_, err = testdb.SeedSubscription(s.ConvoyApp.A.DB, s.DefaultProject, ulid.Make().String(), datastore.OutgoingProject, source, endpoint, &datastore.RetryConfiguration{}, &datastore.AlertConfiguration{}, nil)
//		require.NoError(s.T(), err)
//	}
//
//	portalLink, err := testdb.SeedPortalLink(s.ConvoyApp.A.DB, s.DefaultProject, []string{})
//	require.NoError(s.T(), err)
//
//	// Arrange Request
//	url := fmt.Sprintf("/subscriptions?token=%s", portalLink.Token)
//	req := createRequest(http.MethodGet, url, s.APIKey, nil)
//	w := httptest.NewRecorder()
//
//	// Act
//	s.Router.ServeHTTP(w, req)
//
//	// Assert
//	require.Equal(s.T(), http.StatusOK, w.Code)
//
//	// Deep Assert
//	var resp pagedResponse
//	parseResponse(s.T(), w.Result(), &resp)
//	require.Equal(s.T(), totalSubs, len(resp.Content.([]interface{})))
//}
//
//func TestSubscriptionIntegrationTestSuite(t *testing.T) {
//	suite.Run(t, new(SubscriptionIntegrationTestSuite))
//}
