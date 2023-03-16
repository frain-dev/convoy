//go:build integration
// +build integration

package api

import (
	"context"
	"fmt"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

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

type SubscriptionIntegrationTestSuite struct {
	suite.Suite
	DB             database.Database
	Router         http.Handler
	ConvoyApp      *ApplicationHandler
	DefaultOrg     *datastore.Organisation
	DefaultProject *datastore.Project
	APIKey         string
}

func (s *SubscriptionIntegrationTestSuite) SetupSuite() {
	s.DB = getDB()
	s.ConvoyApp = buildServer()
	s.Router = s.ConvoyApp.BuildRoutes()
}

func (s *SubscriptionIntegrationTestSuite) SetupTest() {
	testdb.PurgeDB(s.T(), s.DB)

	user, err := testdb.SeedDefaultUser(s.ConvoyApp.A.DB)
	require.NoError(s.T(), err)

	org, err := testdb.SeedDefaultOrganisation(s.ConvoyApp.A.DB, user)
	require.NoError(s.T(), err)
	s.DefaultOrg = org

	// Setup Default Project.
	s.DefaultProject, err = testdb.SeedDefaultProject(s.ConvoyApp.A.DB, org.UID)
	require.NoError(s.T(), err)
	fmt.Printf("%+v\n", s.DefaultProject)

	// Seed Auth
	role := auth.Role{
		Type:    auth.RoleAdmin,
		Project: s.DefaultProject.UID,
	}

	_, s.APIKey, err = testdb.SeedAPIKey(s.ConvoyApp.A.DB, role, "", "test", "", "")
	require.NoError(s.T(), err)

	// Setup Config.
	err = config.LoadConfig("./testdata/Auth_Config/full-convoy.json")
	require.NoError(s.T(), err)

	apiRepo := postgres.NewAPIKeyRepo(s.ConvoyApp.A.DB)
	userRepo := postgres.NewUserRepo(s.ConvoyApp.A.DB)

	initRealmChain(s.T(), apiRepo, userRepo, s.ConvoyApp.A.Cache)
}

func (s *SubscriptionIntegrationTestSuite) TearDownTest() {
	testdb.PurgeDB(s.T(), s.DB)
	metrics.Reset()
}

func (s *SubscriptionIntegrationTestSuite) Test_CreateSubscription() {
	endpoint, _ := testdb.SeedEndpoint(s.ConvoyApp.A.DB, s.DefaultProject, ulid.Make().String(), "", "", false, datastore.ActiveEndpointStatus)
	body := serialize(`{
		"name": "sub-1",
		"type": "incoming",
		"app_id": "%s",
		"project_id": "%s",
		"endpoint_id": "%s",
		"alert_config": {
			"threshold": "1h",
			"count": 10
		},
		"retry_config": {
			"type": "linear",
			"retry_count": 2,
			"duration": "10s",
			"interval_seconds": 10
		},
		"filter_config": {
			"event_types": [
				"user.created",
				"user.updated"
			]
		},
		"rate_limit_config": {
			"count": 100,
			"duration": 5
		},
		"disable_endpoint": true
	}`, endpoint.UID, s.DefaultProject.UID, endpoint.UID)

	url := fmt.Sprintf("/api/v1/projects/%s/subscriptions", s.DefaultProject.UID)
	req := createRequest(http.MethodPost, url, s.APIKey, body)
	w := httptest.NewRecorder()

	// Act
	s.Router.ServeHTTP(w, req)

	// Assert
	// require.Equal(s.T(), http.StatusCreated, w.Code)

	var subscription *datastore.Subscription
	parseResponse(s.T(), w.Result(), &subscription)

	subRepo := postgres.NewSubscriptionRepo(s.ConvoyApp.A.DB)
	dbSub, err := subRepo.FindSubscriptionByID(context.Background(), s.DefaultProject.UID, subscription.UID)
	require.NoError(s.T(), err)

	require.NotEmpty(s.T(), subscription.UID)
	require.Equal(s.T(), dbSub.Name, subscription.Name)
	require.Equal(s.T(), len(dbSub.FilterConfig.EventTypes), len(subscription.FilterConfig.EventTypes))
	require.Equal(s.T(), dbSub.RateLimitConfig.Count, subscription.RateLimitConfig.Count)
}

func (s *SubscriptionIntegrationTestSuite) Test_CreateSubscription_IncomingProject() {
	project, err := testdb.SeedProject(s.ConvoyApp.A.DB, ulid.Make().String(), "test_project", s.DefaultOrg.UID, datastore.IncomingProject, &datastore.DefaultProjectConfig)
	require.NoError(s.T(), err)

	// Seed Auth
	role := auth.Role{
		Type:    auth.RoleAdmin,
		Project: project.UID,
	}

	_, apiKey, _ := testdb.SeedAPIKey(s.ConvoyApp.A.DB, role, "", "test", "", "")

	endpoint, _ := testdb.SeedEndpoint(s.ConvoyApp.A.DB, project, ulid.Make().String(), "", "", false, datastore.ActiveEndpointStatus)
	source, _ := testdb.SeedSource(s.ConvoyApp.A.DB, project, ulid.Make().String(), "", "", nil)
	bodyStr := fmt.Sprintf(`{
		"name": "sub-1",
		"type": "incoming",
		"app_id": "%s",
        "source_id":"%s",
		"project_id": "%s",
		"endpoint_id": "%s",
		"alert_config": {
			"threshold": "1h",
			"count": 10
		},
		"retry_config": {
			"type": "linear",
			"retry_count": 2,
			"duration": "10s"
		},
		"filter_config": {
			"event_types": [
				"user.created",
				"user.updated"
			]
		},
		"rate_limit_config": {
			"count": 100,
			"duration": 5
		}
	}`, endpoint.UID, source.UID, project.UID, endpoint.UID)

	url := fmt.Sprintf("/api/v1/projects/%s/subscriptions", project.UID)
	body := serialize(bodyStr)
	req := createRequest(http.MethodPost, url, apiKey, body)
	w := httptest.NewRecorder()

	// Act
	s.Router.ServeHTTP(w, req)

	// Assert
	require.Equal(s.T(), http.StatusCreated, w.Code)

	var subscription *datastore.Subscription
	parseResponse(s.T(), w.Result(), &subscription)

	subRepo := postgres.NewSubscriptionRepo(s.ConvoyApp.A.DB)
	dbSub, err := subRepo.FindSubscriptionByID(context.Background(), project.UID, subscription.UID)
	require.NoError(s.T(), err)

	require.NotEmpty(s.T(), subscription.UID)
	require.Equal(s.T(), dbSub.Name, subscription.Name)
	require.Equal(s.T(), len(dbSub.FilterConfig.EventTypes), len(subscription.FilterConfig.EventTypes))
	require.Equal(s.T(), dbSub.RateLimitConfig.Count, subscription.RateLimitConfig.Count)
}

func (s *SubscriptionIntegrationTestSuite) Test_CreateSubscription_IncomingProject_RedirectToProjects() {
	project, err := testdb.SeedProject(s.ConvoyApp.A.DB, ulid.Make().String(), "test_project", s.DefaultOrg.UID, datastore.IncomingProject, &datastore.DefaultProjectConfig)
	require.NoError(s.T(), err)

	// Seed Auth
	role := auth.Role{
		Type:    auth.RoleAdmin,
		Project: project.UID,
	}

	_, apiKey, err := testdb.SeedAPIKey(s.ConvoyApp.A.DB, role, "", "test", "", "")
	require.NoError(s.T(), err)

	source, err := testdb.SeedSource(s.ConvoyApp.A.DB, project, ulid.Make().String(), "", "", nil)
	require.NoError(s.T(), err)

	endpoint, err := testdb.SeedEndpoint(s.ConvoyApp.A.DB, project, ulid.Make().String(), "", "", false, datastore.ActiveEndpointStatus)
	require.NoError(s.T(), err)

	bodyStr := fmt.Sprintf(`{
		"name": "sub-1",
		"type": "incoming",
		"app_id": "%s",
        "source_id":"%s",
		"project_id": "%s",
		"endpoint_id": "%s",
		"alert_config": {
			"threshold": "1h",
			"count": 10
		},
		"retry_config": {
			"type": "linear",
			"retry_count": 2,
			"duration": "10s"
		},
		"filter_config": {
			"event_types": [
				"user.created",
				"user.updated"
			]
		},
		"rate_limit_config": {
			"count": 100,
			"duration": 5
		}
	}`, endpoint.UID, source.UID, project.UID, endpoint.UID)

	url := fmt.Sprintf("/api/v1/subscriptions?projectID=%s", project.UID)
	body := serialize(bodyStr)
	req := createRequest(http.MethodPost, url, apiKey, body)
	w := httptest.NewRecorder()

	// Act
	s.Router.ServeHTTP(w, req)

	// Assert
	require.Equal(s.T(), http.StatusTemporaryRedirect, w.Code)
}

func (s *SubscriptionIntegrationTestSuite) Test_CreateSubscription_AppNotFound() {
	endpoint, _ := testdb.SeedEndpoint(s.ConvoyApp.A.DB, &datastore.Project{UID: ulid.Make().String()}, ulid.Make().String(), "", "", false, datastore.ActiveEndpointStatus)
	bodyStr := fmt.Sprintf(`{
		"name": "sub-1",
		"type": "incoming",
		"app_id": "%s",
		"endpoint_id": "%s",
		"alert_config": {
			"threshold": "1h",
			"count": 10
		},
		"retry_config": {
			"type": "linear",
			"retry_count": 2,
			"interval_seconds": 10
		},
		"filter_config": {
			"event_types": [
				"user.created",
				"user.updated"
			]
		}
	}`, ulid.Make().String(), endpoint.UID)

	url := fmt.Sprintf("/api/v1/projects/%s/subscriptions", s.DefaultProject.UID)
	body := serialize(bodyStr)
	req := createRequest(http.MethodPost, url, s.APIKey, body)
	w := httptest.NewRecorder()

	// Act
	s.Router.ServeHTTP(w, req)

	// Assert
	require.Equal(s.T(), http.StatusBadRequest, w.Code)
}

func (s *SubscriptionIntegrationTestSuite) Test_CreateSubscription_EndpointNotFound() {
	bodyStr := fmt.Sprintf(`{
		"name": "sub-1",
		"type": "incoming",
		"app_id": "%s",
		"project_id": "%s",
		"endpoint_id": "%s",
		"alert_config": {
			"threshold": "1h",
			"count": 10
		},
		"retry_config": {
			"type": "linear",
			"retry_count": 2,
			"interval_seconds": 10
		},
		"filter_config": {
			"event_types": [
				"user.created",
				"user.updated"
			]
		}
	}`, ulid.Make().String(), s.DefaultProject.UID, ulid.Make().String())

	url := fmt.Sprintf("/api/v1/projects/%s/subscriptions", s.DefaultProject.UID)
	body := serialize(bodyStr)
	req := createRequest(http.MethodPost, url, s.APIKey, body)
	w := httptest.NewRecorder()

	// Act
	s.Router.ServeHTTP(w, req)

	// Assert
	require.Equal(s.T(), http.StatusBadRequest, w.Code)
}

func (s *SubscriptionIntegrationTestSuite) Test_CreateSubscription_InvalidBody() {
	bodyStr := `{
		"name": "sub-1",
		"type": "incoming",
		"alert_config": {
			"threshold": "1h",
			"count": 10
		},
		"retry_config": {
			"type": "linear",
			"retry_count": 2,
			"interval_seconds": 10
		},
		"filter_config": {
			"event_types": [
				"user.created",
				"user.updated"
			]
		}
	}`

	url := fmt.Sprintf("/api/v1/projects/%s/subscriptions", s.DefaultProject.UID)
	body := serialize(bodyStr)
	req := createRequest(http.MethodPost, url, s.APIKey, body)
	w := httptest.NewRecorder()

	// Act
	s.Router.ServeHTTP(w, req)

	// Assert
	require.Equal(s.T(), http.StatusBadRequest, w.Code)
}

func (s *SubscriptionIntegrationTestSuite) Test_GetOneSubscription_SubscriptionNotFound() {
	subscriptionId := "123"

	// Arrange Request
	url := fmt.Sprintf("/api/v1/projects/%s/subscriptions/%s", s.DefaultProject.UID, subscriptionId)
	req := createRequest(http.MethodGet, url, s.APIKey, nil)
	w := httptest.NewRecorder()

	// Act
	s.Router.ServeHTTP(w, req)

	// Assert
	require.Equal(s.T(), http.StatusNotFound, w.Code)
}

func (s *SubscriptionIntegrationTestSuite) Test_GetOneSubscription_OutgoingProject_ValidSubscription() {
	subscriptionId := ulid.Make().String()

	project := s.DefaultProject

	// Just Before
	endpoint, err := testdb.SeedEndpoint(s.ConvoyApp.A.DB, project, ulid.Make().String(), "", "", false, datastore.ActiveEndpointStatus)
	require.NoError(s.T(), err)

	source, err := testdb.SeedSource(s.ConvoyApp.A.DB, project, ulid.Make().String(), "", "", nil)
	require.NoError(s.T(), err)

	_, err = testdb.SeedSubscription(s.ConvoyApp.A.DB, project, subscriptionId, project.Type, source, endpoint, &datastore.RetryConfiguration{}, &datastore.AlertConfiguration{}, nil)
	require.NoError(s.T(), err)

	// Arrange Request
	url := fmt.Sprintf("/api/v1/projects/%s/subscriptions/%s", s.DefaultProject.UID, subscriptionId)
	req := createRequest(http.MethodGet, url, s.APIKey, nil)
	w := httptest.NewRecorder()

	// Act
	s.Router.ServeHTTP(w, req)

	// Assert
	require.Equal(s.T(), http.StatusOK, w.Code)

	// Deep Assert
	var subscription *datastore.Subscription
	parseResponse(s.T(), w.Result(), &subscription)

	subRepo := postgres.NewSubscriptionRepo(s.ConvoyApp.A.DB)
	dbSub, err := subRepo.FindSubscriptionByID(context.Background(), project.UID, subscriptionId)
	require.NoError(s.T(), err)
	require.Equal(s.T(), subscription.UID, dbSub.UID)
	require.Equal(s.T(), subscription.Endpoint.UID, dbSub.EndpointID)
}

func (s *SubscriptionIntegrationTestSuite) Test_GetOneSubscription_IncomingProject_ValidSubscription() {
	subscriptionId := ulid.Make().String()

	project, err := testdb.SeedProject(s.ConvoyApp.A.DB, ulid.Make().String(), "test-project", s.DefaultOrg.UID, datastore.IncomingProject, &datastore.DefaultProjectConfig)
	require.NoError(s.T(), err)

	// Seed Auth
	role := auth.Role{
		Type:    auth.RoleAdmin,
		Project: project.UID,
	}

	_, apiKey, err := testdb.SeedAPIKey(s.ConvoyApp.A.DB, role, "", "test", "", "")
	require.NoError(s.T(), err)

	// Just Before
	endpoint, err := testdb.SeedEndpoint(s.ConvoyApp.A.DB, project, ulid.Make().String(), "", "", false, datastore.ActiveEndpointStatus)
	require.NoError(s.T(), err)

	source, err := testdb.SeedSource(s.ConvoyApp.A.DB, project, ulid.Make().String(), "", "", nil)
	require.NoError(s.T(), err)

	_, err = testdb.SeedSubscription(s.ConvoyApp.A.DB, project, subscriptionId, "incoming", source, endpoint, &datastore.RetryConfiguration{}, &datastore.AlertConfiguration{}, nil)
	require.NoError(s.T(), err)

	// Arrange Request
	url := fmt.Sprintf("/api/v1/projects/%s/subscriptions/%s", project.UID, subscriptionId)
	req := createRequest(http.MethodGet, url, apiKey, nil)
	w := httptest.NewRecorder()

	// Act
	s.Router.ServeHTTP(w, req)

	// Assert
	require.Equal(s.T(), http.StatusOK, w.Code)

	// Deep Assert
	var subscription *datastore.Subscription
	parseResponse(s.T(), w.Result(), &subscription)

	subRepo := postgres.NewSubscriptionRepo(s.ConvoyApp.A.DB)
	dbSub, err := subRepo.FindSubscriptionByID(context.Background(), project.UID, subscriptionId)
	require.NoError(s.T(), err)
	require.Equal(s.T(), subscription.UID, dbSub.UID)
	require.Equal(s.T(), subscription.Source.UID, dbSub.SourceID)
	require.Equal(s.T(), subscription.Endpoint.UID, dbSub.EndpointID)
}

func (s *SubscriptionIntegrationTestSuite) Test_GetSubscriptions_ValidSubscriptions() {
	r := rand.New(rand.NewSource(time.Now().Unix()))
	totalSubs := r.Intn(10)

	for i := 0; i < totalSubs; i++ {
		// Just Before
		endpoint, err := testdb.SeedEndpoint(s.ConvoyApp.A.DB, s.DefaultProject, ulid.Make().String(), "", "", false, datastore.ActiveEndpointStatus)
		require.NoError(s.T(), err)
		source, err := testdb.SeedSource(s.ConvoyApp.A.DB, s.DefaultProject, ulid.Make().String(), "", "", nil)
		require.NoError(s.T(), err)

		_, err = testdb.SeedSubscription(s.ConvoyApp.A.DB, s.DefaultProject, ulid.Make().String(), datastore.OutgoingProject, source, endpoint, &datastore.RetryConfiguration{}, &datastore.AlertConfiguration{}, nil)
		require.NoError(s.T(), err)
	}
	// Arrange Request
	url := fmt.Sprintf("/api/v1/projects/%s/subscriptions", s.DefaultProject.UID)
	req := createRequest(http.MethodGet, url, s.APIKey, nil)
	w := httptest.NewRecorder()

	// Act
	s.Router.ServeHTTP(w, req)

	// Assert
	require.Equal(s.T(), http.StatusOK, w.Code)

	// Deep Assert
	var resp pagedResponse
	parseResponse(s.T(), w.Result(), &resp)
	require.Equal(s.T(), int64(totalSubs), resp.Pagination.Total)
}

func (s *SubscriptionIntegrationTestSuite) Test_DeleteSubscription() {
	subscriptionId := ulid.Make().String()

	// Just Before
	endpoint, err := testdb.SeedEndpoint(s.ConvoyApp.A.DB, s.DefaultProject, ulid.Make().String(), "", "", false, datastore.ActiveEndpointStatus)
	require.NoError(s.T(), err)

	source, err := testdb.SeedSource(s.ConvoyApp.A.DB, s.DefaultProject, ulid.Make().String(), "", "", nil)
	require.NoError(s.T(), err)

	_, err = testdb.SeedSubscription(s.ConvoyApp.A.DB, s.DefaultProject, subscriptionId, datastore.OutgoingProject, source, endpoint, &datastore.RetryConfiguration{}, &datastore.AlertConfiguration{}, nil)
	require.NoError(s.T(), err)

	// Arrange Request.
	url := fmt.Sprintf("/api/v1/projects/%s/subscriptions/%s", s.DefaultProject.UID, subscriptionId)
	req := createRequest(http.MethodDelete, url, s.APIKey, nil)
	w := httptest.NewRecorder()

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), http.StatusOK, w.Code)

	// Deep Assert.
	subRepo := postgres.NewSubscriptionRepo(s.ConvoyApp.A.DB)
	_, err = subRepo.FindSubscriptionByID(context.Background(), s.DefaultProject.UID, subscriptionId)
	require.ErrorIs(s.T(), err, datastore.ErrSubscriptionNotFound)
}

func (s *SubscriptionIntegrationTestSuite) Test_UpdateSubscription() {
	subscriptionId := ulid.Make().String()

	// Just Before
	endpoint, err := testdb.SeedEndpoint(s.ConvoyApp.A.DB, s.DefaultProject, ulid.Make().String(), "", "", false, datastore.ActiveEndpointStatus)
	require.NoError(s.T(), err)

	source, err := testdb.SeedSource(s.ConvoyApp.A.DB, s.DefaultProject, ulid.Make().String(), "", "", nil)
	require.NoError(s.T(), err)

	_, err = testdb.SeedSubscription(s.ConvoyApp.A.DB, s.DefaultProject, subscriptionId, datastore.OutgoingProject, source, endpoint, &datastore.RetryConfiguration{}, &datastore.AlertConfiguration{}, nil)
	require.NoError(s.T(), err)

	// Arrange Request
	url := fmt.Sprintf("/api/v1/projects/%s/subscriptions/%s", s.DefaultProject.UID, subscriptionId)
	bodyStr := `{
		"alert_config": {
			"threshold": "1h",
			"count": 10
		},
		"retry_config": {
			"type": "linear",
			"retry_count": 3,
			"duration": "2s"
		},
		"filter_config": {
			"event_types": [
				"user.created",
				"user.updated"
			]
		},
		"disable_endpoint": false
	}`

	body := serialize(bodyStr)
	req := createRequest(http.MethodPut, url, s.APIKey, body)
	w := httptest.NewRecorder()

	// Act
	s.Router.ServeHTTP(w, req)

	// Assert
	require.Equal(s.T(), http.StatusAccepted, w.Code)

	// Deep Asset
	var subscription *datastore.Subscription
	parseResponse(s.T(), w.Result(), &subscription)

	subRepo := postgres.NewSubscriptionRepo(s.ConvoyApp.A.DB)
	dbSub, err := subRepo.FindSubscriptionByID(context.Background(), s.DefaultProject.UID, subscriptionId)
	require.NoError(s.T(), err)
	require.Equal(s.T(), 2, len(dbSub.FilterConfig.EventTypes))
	require.Equal(s.T(), "1h", dbSub.AlertConfig.Threshold)
	require.Equal(s.T(), subscription.RetryConfig.Duration, dbSub.RetryConfig.Duration)
}

func TestSubscriptionIntegrationTestSuite(t *testing.T) {
	suite.Run(t, new(SubscriptionIntegrationTestSuite))
}
