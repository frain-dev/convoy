//go:build integration
// +build integration

package server

import (
	"context"
	"fmt"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/frain-dev/convoy/internal/pkg/metrics"

	"github.com/frain-dev/convoy/auth"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/datastore"
	cm "github.com/frain-dev/convoy/datastore/mongo"
	"github.com/frain-dev/convoy/server/testdb"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type SubscriptionIntegrationTestSuite struct {
	suite.Suite
	DB             cm.Client
	Router         http.Handler
	ConvoyApp      *ApplicationHandler
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

	// Setup Default Project.
	s.DefaultProject, _ = testdb.SeedDefaultProject(s.ConvoyApp.A.Store, "")
	fmt.Printf("%+v\n", s.DefaultProject)

	// Seed Auth
	role := auth.Role{
		Type:    auth.RoleAdmin,
		Project: s.DefaultProject.UID,
	}

	_, s.APIKey, _ = testdb.SeedAPIKey(s.ConvoyApp.A.Store, role, "", "test", "", "")

	// Setup Config.
	err := config.LoadConfig("./testdata/Auth_Config/full-convoy.json")
	require.NoError(s.T(), err)

	apiRepo := cm.NewApiKeyRepo(s.ConvoyApp.A.Store)
	userRepo := cm.NewUserRepo(s.ConvoyApp.A.Store)

	initRealmChain(s.T(), apiRepo, userRepo, s.ConvoyApp.A.Cache)
}

func (s *SubscriptionIntegrationTestSuite) TearDownTest() {
	testdb.PurgeDB(s.T(), s.DB)
	metrics.Reset()
}

func (s *SubscriptionIntegrationTestSuite) Test_CreateSubscription() {
	endpoint, _ := testdb.SeedEndpoint(s.ConvoyApp.A.Store, s.DefaultProject, uuid.NewString(), "", "", false, datastore.ActiveEndpointStatus)
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

	subRepo := cm.NewSubscriptionRepo(s.ConvoyApp.A.Store)
	dbSub, err := subRepo.FindSubscriptionByID(context.Background(), s.DefaultProject.UID, subscription.UID)

	require.NoError(s.T(), err)
	require.NotEmpty(s.T(), subscription.UID)
	require.Equal(s.T(), dbSub.Name, subscription.Name)
	require.Equal(s.T(), len(dbSub.FilterConfig.EventTypes), len(subscription.FilterConfig.EventTypes))
	require.Equal(s.T(), dbSub.RateLimitConfig.Count, subscription.RateLimitConfig.Count)
	require.Equal(s.T(), dbSub.DisableEndpoint, subscription.DisableEndpoint)
}

func (s *SubscriptionIntegrationTestSuite) Test_CreateSubscription_IncomingProject() {
	project, err := testdb.SeedProject(s.ConvoyApp.A.Store, uuid.NewString(), "test_project", "", datastore.IncomingProject, nil)
	require.NoError(s.T(), err)

	// Seed Auth
	role := auth.Role{
		Type:    auth.RoleAdmin,
		Project: project.UID,
	}

	_, apiKey, _ := testdb.SeedAPIKey(s.ConvoyApp.A.Store, role, "", "test", "", "")

	endpoint, _ := testdb.SeedEndpoint(s.ConvoyApp.A.Store, project, uuid.NewString(), "", "", false, datastore.ActiveEndpointStatus)
	source, _ := testdb.SeedSource(s.ConvoyApp.A.Store, project, uuid.NewString(), "", "", nil)
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

	subRepo := cm.NewSubscriptionRepo(s.ConvoyApp.A.Store)
	dbSub, err := subRepo.FindSubscriptionByID(context.Background(), project.UID, subscription.UID)
	require.NoError(s.T(), err)

	require.NotEmpty(s.T(), subscription.UID)
	require.Equal(s.T(), dbSub.Name, subscription.Name)
	require.Equal(s.T(), len(dbSub.FilterConfig.EventTypes), len(subscription.FilterConfig.EventTypes))
	require.Equal(s.T(), dbSub.RateLimitConfig.Count, subscription.RateLimitConfig.Count)
}

func (s *SubscriptionIntegrationTestSuite) Test_CreateSubscription_IncomingProject_RedirectToProjects() {
	project, err := testdb.SeedProject(s.ConvoyApp.A.Store, uuid.NewString(), "test_project", "", datastore.IncomingProject, nil)
	require.NoError(s.T(), err)

	// Seed Auth
	role := auth.Role{
		Type:    auth.RoleAdmin,
		Project: project.UID,
	}

	_, apiKey, _ := testdb.SeedAPIKey(s.ConvoyApp.A.Store, role, "", "test", "", "")

	source, _ := testdb.SeedSource(s.ConvoyApp.A.Store, project, uuid.NewString(), "", "", nil)
	endpoint, _ := testdb.SeedEndpoint(s.ConvoyApp.A.Store, project, uuid.NewString(), "", "", false, datastore.ActiveEndpointStatus)
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
	endpoint, _ := testdb.SeedEndpoint(s.ConvoyApp.A.Store, &datastore.Project{UID: uuid.NewString()}, uuid.NewString(), "", "", false, datastore.ActiveEndpointStatus)
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
	}`, uuid.NewString(), endpoint.UID)

	url := fmt.Sprintf("/api/v1/projects/%s/subscriptions", s.DefaultProject.UID)
	body := serialize(bodyStr)
	req := createRequest(http.MethodPost, url, s.APIKey, body)
	w := httptest.NewRecorder()

	// Act
	s.Router.ServeHTTP(w, req)

	// Assert
	require.Equal(s.T(), http.StatusBadRequest, w.Code)
}

func (s *SubscriptionIntegrationTestSuite) Test_CreateSubscription_EndpointDoesNotBelongToProject() {
	endpoint, _ := testdb.SeedEndpoint(s.ConvoyApp.A.Store, &datastore.Project{UID: uuid.NewString()}, uuid.NewString(), "", "", false, datastore.ActiveEndpointStatus)
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
			"duration": "10s"
		},
		"filter_config": {
			"event_types": [
				"user.created",
				"user.updated"
			]
		}
	}`, endpoint.UID, s.DefaultProject.UID, endpoint.UID)

	url := fmt.Sprintf("/api/v1/projects/%s/subscriptions", s.DefaultProject.UID)
	body := serialize(bodyStr)
	req := createRequest(http.MethodPost, url, s.APIKey, body)
	w := httptest.NewRecorder()

	// Act
	s.Router.ServeHTTP(w, req)

	// Assert
	require.Equal(s.T(), http.StatusUnauthorized, w.Code)
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
	}`, uuid.NewString(), s.DefaultProject.UID, uuid.NewString())

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
	subscriptionId := "123456789"

	project := s.DefaultProject

	// Just Before
	endpoint, _ := testdb.SeedEndpoint(s.ConvoyApp.A.Store, project, uuid.NewString(), "", "", false, datastore.ActiveEndpointStatus)
	source, _ := testdb.SeedSource(s.ConvoyApp.A.Store, project, uuid.NewString(), "", "", nil)
	_, _ = testdb.SeedSubscription(s.ConvoyApp.A.Store, project, subscriptionId, project.Type, source, endpoint, &datastore.RetryConfiguration{}, &datastore.AlertConfiguration{}, &datastore.FilterConfiguration{})

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

	subRepo := cm.NewSubscriptionRepo(s.ConvoyApp.A.Store)
	dbSub, err := subRepo.FindSubscriptionByID(context.Background(), project.UID, subscriptionId)
	require.NoError(s.T(), err)
	require.Equal(s.T(), subscription.UID, dbSub.UID)
	require.Equal(s.T(), subscription.Endpoint.UID, dbSub.EndpointID)
}

func (s *SubscriptionIntegrationTestSuite) Test_GetOneSubscription_IncomingProject_ValidSubscription() {
	subscriptionId := "123456789"

	project, err := testdb.SeedProject(s.ConvoyApp.A.Store, uuid.NewString(), "test-project", "", datastore.IncomingProject, nil)
	require.NoError(s.T(), err)

	// Seed Auth
	role := auth.Role{
		Type:    auth.RoleAdmin,
		Project: project.UID,
	}

	_, apiKey, _ := testdb.SeedAPIKey(s.ConvoyApp.A.Store, role, "", "test", "", "")

	// Just Before
	endpoint, _ := testdb.SeedEndpoint(s.ConvoyApp.A.Store, project, uuid.NewString(), "", "", false, datastore.ActiveEndpointStatus)
	source, _ := testdb.SeedSource(s.ConvoyApp.A.Store, project, uuid.NewString(), "", "", nil)
	_, _ = testdb.SeedSubscription(s.ConvoyApp.A.Store, project, subscriptionId, "incoming", source, endpoint, &datastore.RetryConfiguration{}, &datastore.AlertConfiguration{}, &datastore.FilterConfiguration{})

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

	subRepo := cm.NewSubscriptionRepo(s.ConvoyApp.A.Store)
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
		endpoint, _ := testdb.SeedEndpoint(s.ConvoyApp.A.Store, s.DefaultProject, uuid.NewString(), "", "", false, datastore.ActiveEndpointStatus)
		source, _ := testdb.SeedSource(s.ConvoyApp.A.Store, s.DefaultProject, uuid.NewString(), "", "", nil)
		_, _ = testdb.SeedSubscription(s.ConvoyApp.A.Store, s.DefaultProject, uuid.NewString(), datastore.OutgoingProject, source, endpoint, &datastore.RetryConfiguration{}, &datastore.AlertConfiguration{}, &datastore.FilterConfiguration{})
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
	subscriptionId := "123456789"

	// Just Before
	endpoint, _ := testdb.SeedEndpoint(s.ConvoyApp.A.Store, s.DefaultProject, uuid.NewString(), "", "", false, datastore.ActiveEndpointStatus)
	source, _ := testdb.SeedSource(s.ConvoyApp.A.Store, s.DefaultProject, uuid.NewString(), "", "", nil)
	_, _ = testdb.SeedSubscription(s.ConvoyApp.A.Store, s.DefaultProject, subscriptionId, datastore.OutgoingProject, source, endpoint, &datastore.RetryConfiguration{}, &datastore.AlertConfiguration{}, &datastore.FilterConfiguration{})

	// Arrange Request.
	url := fmt.Sprintf("/api/v1/projects/%s/subscriptions/%s", s.DefaultProject.UID, subscriptionId)
	req := createRequest(http.MethodDelete, url, s.APIKey, nil)
	w := httptest.NewRecorder()

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), http.StatusOK, w.Code)

	// Deep Assert.
	subRepo := cm.NewSubscriptionRepo(s.ConvoyApp.A.Store)
	_, err := subRepo.FindSubscriptionByID(context.Background(), s.DefaultProject.UID, subscriptionId)
	require.ErrorIs(s.T(), err, datastore.ErrSubscriptionNotFound)
}

func (s *SubscriptionIntegrationTestSuite) Test_UpdateSubscription() {
	subscriptionId := "123456789"

	// Just Before
	endpoint, _ := testdb.SeedEndpoint(s.ConvoyApp.A.Store, s.DefaultProject, uuid.NewString(), "", "", false, datastore.ActiveEndpointStatus)
	source, _ := testdb.SeedSource(s.ConvoyApp.A.Store, s.DefaultProject, uuid.NewString(), "", "", nil)
	_, _ = testdb.SeedSubscription(s.ConvoyApp.A.Store, s.DefaultProject, subscriptionId, datastore.OutgoingProject, source, endpoint, &datastore.RetryConfiguration{}, &datastore.AlertConfiguration{}, &datastore.FilterConfiguration{})

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

	subRepo := cm.NewSubscriptionRepo(s.ConvoyApp.A.Store)
	dbSub, err := subRepo.FindSubscriptionByID(context.Background(), s.DefaultProject.UID, subscriptionId)
	require.NoError(s.T(), err)
	require.Equal(s.T(), 2, len(dbSub.FilterConfig.EventTypes))
	require.Equal(s.T(), "1h", dbSub.AlertConfig.Threshold)
	require.Equal(s.T(), subscription.RetryConfig.Duration, dbSub.RetryConfig.Duration)
	require.Equal(s.T(), subscription.DisableEndpoint, dbSub.DisableEndpoint)
}

func TestSubscriptionIntegrationTestSuite(t *testing.T) {
	suite.Run(t, new(SubscriptionIntegrationTestSuite))
}
