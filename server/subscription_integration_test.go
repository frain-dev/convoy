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

	"github.com/frain-dev/convoy/auth"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/server/testdb"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type SubscriptionIntegrationTestSuite struct {
	suite.Suite
	DB           datastore.DatabaseClient
	Router       http.Handler
	ConvoyApp    *applicationHandler
	DefaultGroup *datastore.Group
	APIKey       string
}

func (s *SubscriptionIntegrationTestSuite) SetupSuite() {
	s.DB = getDB()
	s.ConvoyApp = buildApplication()
	s.Router = buildRoutes(s.ConvoyApp)
}

func (s *SubscriptionIntegrationTestSuite) SetupTest() {
	testdb.PurgeDB(s.DB)

	// Setup Default Group.
	s.DefaultGroup, _ = testdb.SeedDefaultGroup(s.DB, "")

	// Seed Auth
	role := auth.Role{
		Type:   auth.RoleAdmin,
		Groups: []string{s.DefaultGroup.UID},
	}

	_, s.APIKey, _ = testdb.SeedAPIKey(s.DB, role, "", "test", "")

	// Setup Config.
	err := config.LoadConfig("./testdata/Auth_Config/full-convoy.json")
	require.NoError(s.T(), err)

	initRealmChain(s.T(), s.DB.APIRepo(), s.DB.UserRepo(), s.ConvoyApp.cache)
}

func (s *SubscriptionIntegrationTestSuite) TearDownTest() {
	testdb.PurgeDB(s.DB)
}

func (s *SubscriptionIntegrationTestSuite) Test_CreateSubscription() {
	app, _ := testdb.SeedApplication(s.DB, s.DefaultGroup, uuid.NewString(), "", false)
	endpoint, _ := testdb.SeedEndpoint(s.DB, app, s.DefaultGroup.UID)
	bodyStr := fmt.Sprintf(`{
		"name": "sub-1",
		"type": "incoming",
		"app_id": "%s",
		"group_id": "%s",
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
	}`, app.UID, s.DefaultGroup.UID, endpoint.UID)

	body := serialize(bodyStr)
	req := createRequest(http.MethodPost, "/api/v1/subscriptions", s.APIKey, body)
	w := httptest.NewRecorder()

	// Act
	s.Router.ServeHTTP(w, req)

	// Assert
	require.Equal(s.T(), http.StatusCreated, w.Code)

	var subscription *datastore.Subscription
	parseResponse(s.T(), w.Result(), &subscription)

	dbSub, err := s.DB.SubRepo().FindSubscriptionByID(context.Background(), s.DefaultGroup.UID, subscription.UID)

	require.NoError(s.T(), err)
	require.NotEmpty(s.T(), subscription.UID)
	require.Equal(s.T(), dbSub.Name, subscription.Name)
	require.Equal(s.T(), len(dbSub.FilterConfig.EventTypes), len(subscription.FilterConfig.EventTypes))
}

func (s *SubscriptionIntegrationTestSuite) Test_CreateSubscription_IncomingGroup() {
	group, err := testdb.SeedGroup(s.DB, uuid.NewString(), "test_group", "", datastore.IncomingGroup, nil)
	require.NoError(s.T(), err)

	// Seed Auth
	role := auth.Role{
		Type:   auth.RoleAdmin,
		Groups: []string{group.UID},
	}

	_, apiKey, _ := testdb.SeedAPIKey(s.DB, role, "", "test", "")

	app, _ := testdb.SeedApplication(s.DB, group, uuid.NewString(), "", false)
	source, _ := testdb.SeedSource(s.DB, group, uuid.NewString(), "", "", nil)
	endpoint, _ := testdb.SeedEndpoint(s.DB, app, group.UID)
	bodyStr := fmt.Sprintf(`{
		"name": "sub-1",
		"type": "incoming",
		"app_id": "%s",
        "source_id":"%s",
		"group_id": "%s",
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
	}`, app.UID, source.UID, group.UID, endpoint.UID)

	body := serialize(bodyStr)
	req := createRequest(http.MethodPost, "/api/v1/subscriptions", apiKey, body)
	w := httptest.NewRecorder()

	// Act
	s.Router.ServeHTTP(w, req)

	// Assert
	require.Equal(s.T(), http.StatusCreated, w.Code)

	var subscription *datastore.Subscription
	parseResponse(s.T(), w.Result(), &subscription)

	dbSub, err := s.DB.SubRepo().FindSubscriptionByID(context.Background(), group.UID, subscription.UID)
	require.NoError(s.T(), err)

	require.NotEmpty(s.T(), subscription.UID)
	require.Equal(s.T(), dbSub.Name, subscription.Name)
	require.Equal(s.T(), len(dbSub.FilterConfig.EventTypes), len(subscription.FilterConfig.EventTypes))
}

func (s *SubscriptionIntegrationTestSuite) Test_CreateSubscription_AppNotFound() {
	app, _ := testdb.SeedApplication(s.DB, &datastore.Group{UID: uuid.NewString()}, uuid.NewString(), "", false)
	endpoint, _ := testdb.SeedEndpoint(s.DB, app, s.DefaultGroup.UID)
	bodyStr := fmt.Sprintf(`{
		"name": "sub-1",
		"type": "incoming",
		"app_id": "%s",
		"group_id": "%s",
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
	}`, uuid.NewString(), s.DefaultGroup.UID, endpoint.UID)

	body := serialize(bodyStr)
	req := createRequest(http.MethodPost, "/api/v1/subscriptions", s.APIKey, body)
	w := httptest.NewRecorder()

	// Act
	s.Router.ServeHTTP(w, req)

	// Assert
	require.Equal(s.T(), http.StatusBadRequest, w.Code)
}

func (s *SubscriptionIntegrationTestSuite) Test_CreateSubscription_AppDoesNotBelongToGroup() {
	app, _ := testdb.SeedApplication(s.DB, &datastore.Group{UID: uuid.NewString()}, uuid.NewString(), "", false)
	endpoint, _ := testdb.SeedEndpoint(s.DB, app, s.DefaultGroup.UID)
	bodyStr := fmt.Sprintf(`{
		"name": "sub-1",
		"type": "incoming",
		"app_id": "%s",
		"group_id": "%s",
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
	}`, app.UID, s.DefaultGroup.UID, endpoint.UID)

	body := serialize(bodyStr)
	req := createRequest(http.MethodPost, "/api/v1/subscriptions", s.APIKey, body)
	w := httptest.NewRecorder()

	// Act
	s.Router.ServeHTTP(w, req)

	// Assert
	require.Equal(s.T(), http.StatusUnauthorized, w.Code)
}

func (s *SubscriptionIntegrationTestSuite) Test_CreateSubscription_EndpointNotFound() {
	app, _ := testdb.SeedApplication(s.DB, s.DefaultGroup, uuid.NewString(), "", false)
	bodyStr := fmt.Sprintf(`{
		"name": "sub-1",
		"type": "incoming",
		"app_id": "%s",
		"group_id": "%s",
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
	}`, app.UID, s.DefaultGroup.UID, uuid.NewString())

	body := serialize(bodyStr)
	req := createRequest(http.MethodPost, "/api/v1/subscriptions", s.APIKey, body)
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

	body := serialize(bodyStr)
	req := createRequest(http.MethodPost, "/api/v1/subscriptions", s.APIKey, body)
	w := httptest.NewRecorder()

	// Act
	s.Router.ServeHTTP(w, req)

	// Assert
	require.Equal(s.T(), http.StatusBadRequest, w.Code)
}

func (s *SubscriptionIntegrationTestSuite) Test_GetOneSubscription_SubscriptionNotFound() {
	subscriptionId := "123"

	// Arrange Request
	url := fmt.Sprintf("/api/v1/subscriptions/%s", subscriptionId)
	req := createRequest(http.MethodGet, url, s.APIKey, nil)
	w := httptest.NewRecorder()

	// Act
	s.Router.ServeHTTP(w, req)

	// Assert
	require.Equal(s.T(), http.StatusNotFound, w.Code)
}

func (s *SubscriptionIntegrationTestSuite) Test_GetOneSubscription_OutgoingGroup_ValidSubscription() {
	subscriptionId := "123456789"

	group := s.DefaultGroup

	// Just Before
	app, _ := testdb.SeedApplication(s.DB, group, uuid.NewString(), "", false)
	endpoint, _ := testdb.SeedEndpoint(s.DB, app, group.UID)
	source, _ := testdb.SeedSource(s.DB, group, uuid.NewString(), "", "", nil)
	_, _ = testdb.SeedSubscription(s.DB, app, group, subscriptionId, group.Type, source, endpoint, &datastore.RetryConfiguration{}, &datastore.AlertConfiguration{}, &datastore.FilterConfiguration{}, "")

	// Arrange Request
	url := fmt.Sprintf("/api/v1/subscriptions/%s", subscriptionId)
	req := createRequest(http.MethodGet, url, s.APIKey, nil)
	w := httptest.NewRecorder()

	// Act
	s.Router.ServeHTTP(w, req)

	// Assert
	require.Equal(s.T(), http.StatusOK, w.Code)

	// Deep Assert
	var subscription *datastore.Subscription
	parseResponse(s.T(), w.Result(), &subscription)

	dbSub, err := s.DB.SubRepo().FindSubscriptionByID(context.Background(), group.UID, subscriptionId)
	require.NoError(s.T(), err)
	require.Equal(s.T(), subscription.UID, dbSub.UID)
	require.Equal(s.T(), subscription.Endpoint.UID, dbSub.EndpointID)
}

func (s *SubscriptionIntegrationTestSuite) Test_GetOneSubscription_IncomingGroup_ValidSubscription() {
	subscriptionId := "123456789"

	group, err := testdb.SeedGroup(s.DB, uuid.NewString(), "test-group", "", datastore.IncomingGroup, nil)
	require.NoError(s.T(), err)

	// Seed Auth
	role := auth.Role{
		Type:   auth.RoleAdmin,
		Groups: []string{group.UID},
	}

	_, apiKey, _ := testdb.SeedAPIKey(s.DB, role, "", "test", "")

	// Just Before
	app, _ := testdb.SeedApplication(s.DB, group, uuid.NewString(), "", false)
	endpoint, _ := testdb.SeedEndpoint(s.DB, app, group.UID)
	source, _ := testdb.SeedSource(s.DB, group, uuid.NewString(), "", "", nil)
	_, _ = testdb.SeedSubscription(s.DB, app, group, subscriptionId, "incoming", source, endpoint, &datastore.RetryConfiguration{}, &datastore.AlertConfiguration{}, &datastore.FilterConfiguration{}, "")

	// Arrange Request
	url := fmt.Sprintf("/api/v1/subscriptions/%s", subscriptionId)
	req := createRequest(http.MethodGet, url, apiKey, nil)
	w := httptest.NewRecorder()

	// Act
	s.Router.ServeHTTP(w, req)

	// Assert
	require.Equal(s.T(), http.StatusOK, w.Code)

	// Deep Assert
	var subscription *datastore.Subscription
	parseResponse(s.T(), w.Result(), &subscription)

	dbSub, err := s.DB.SubRepo().FindSubscriptionByID(context.Background(), group.UID, subscriptionId)
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
		app, _ := testdb.SeedApplication(s.DB, s.DefaultGroup, uuid.NewString(), "", false)
		endpoint, _ := testdb.SeedEndpoint(s.DB, app, s.DefaultGroup.UID)
		source, _ := testdb.SeedSource(s.DB, s.DefaultGroup, uuid.NewString(), "", "", nil)
		_, _ = testdb.SeedSubscription(s.DB, app, s.DefaultGroup, uuid.NewString(), datastore.OutgoingGroup, source, endpoint, &datastore.RetryConfiguration{}, &datastore.AlertConfiguration{}, &datastore.FilterConfiguration{}, "")
	}
	// Arrange Request
	url := "/api/v1/subscriptions"
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
	app, _ := testdb.SeedApplication(s.DB, s.DefaultGroup, uuid.NewString(), "", false)
	endpoint, _ := testdb.SeedEndpoint(s.DB, app, s.DefaultGroup.UID)
	source, _ := testdb.SeedSource(s.DB, s.DefaultGroup, uuid.NewString(), "", "", nil)
	_, _ = testdb.SeedSubscription(s.DB, app, s.DefaultGroup, subscriptionId, datastore.OutgoingGroup, source, endpoint, &datastore.RetryConfiguration{}, &datastore.AlertConfiguration{}, &datastore.FilterConfiguration{}, "")

	// Arrange Request.
	url := fmt.Sprintf("/api/v1/subscriptions/%s", subscriptionId)
	req := createRequest(http.MethodDelete, url, s.APIKey, nil)
	w := httptest.NewRecorder()

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), http.StatusOK, w.Code)

	// Deep Assert.
	_, err := s.DB.SubRepo().FindSubscriptionByID(context.Background(), s.DefaultGroup.UID, subscriptionId)
	require.ErrorIs(s.T(), err, datastore.ErrSubscriptionNotFound)
}

func (s *SubscriptionIntegrationTestSuite) Test_UpdateSubscription() {
	subscriptionId := "123456789"

	// Just Before
	app, _ := testdb.SeedApplication(s.DB, s.DefaultGroup, uuid.NewString(), "", false)
	endpoint, _ := testdb.SeedEndpoint(s.DB, app, s.DefaultGroup.UID)
	source, _ := testdb.SeedSource(s.DB, s.DefaultGroup, uuid.NewString(), "", "", nil)
	_, _ = testdb.SeedSubscription(s.DB, app, s.DefaultGroup, subscriptionId, datastore.OutgoingGroup, source, endpoint, &datastore.RetryConfiguration{}, &datastore.AlertConfiguration{}, &datastore.FilterConfiguration{}, "")

	// Arrange Request
	url := fmt.Sprintf("/api/v1/subscriptions/%s", subscriptionId)
	bodyStr := `{
		"alert_config": {
			"threshold": "1h",
			"count": 10
		},
		"retry_config": {
			"type": "linear",
			"retry_count": 2,
			"duration": "1h"
		},
		"filter_config": {
			"event_types": [
				"user.created",
				"user.updated"
			]
		}
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

	dbSub, err := s.DB.SubRepo().FindSubscriptionByID(context.Background(), s.DefaultGroup.UID, subscriptionId)
	require.NoError(s.T(), err)
	require.Equal(s.T(), 2, len(dbSub.FilterConfig.EventTypes))
	require.Equal(s.T(), "1h", dbSub.AlertConfig.Threshold)
	require.Equal(s.T(), "1h", dbSub.RetryConfig.Duration)
}

func (s *SubscriptionIntegrationTestSuite) Test_ToggleSubscriptionStatus_ActiveStatus() {
	subscriptionId := "123456789"

	// Just Before
	app, _ := testdb.SeedApplication(s.DB, s.DefaultGroup, uuid.NewString(), "", false)
	endpoint, _ := testdb.SeedEndpoint(s.DB, app, s.DefaultGroup.UID)
	source, _ := testdb.SeedSource(s.DB, s.DefaultGroup, uuid.NewString(), "", "", nil)
	_, _ = testdb.SeedSubscription(s.DB, app, s.DefaultGroup, subscriptionId, datastore.OutgoingGroup, source, endpoint, &datastore.RetryConfiguration{}, &datastore.AlertConfiguration{}, &datastore.FilterConfiguration{}, datastore.ActiveSubscriptionStatus)

	// Arrange Request
	url := fmt.Sprintf("/api/v1/subscriptions/%s/toggle_status", subscriptionId)
	req := createRequest(http.MethodPut, url, s.APIKey, nil)
	w := httptest.NewRecorder()

	// Act
	s.Router.ServeHTTP(w, req)

	// Assert
	require.Equal(s.T(), http.StatusAccepted, w.Code)

	// Deep Asset
	var subscription *datastore.Subscription
	parseResponse(s.T(), w.Result(), &subscription)

	dbSub, err := s.DB.SubRepo().FindSubscriptionByID(context.Background(), s.DefaultGroup.UID, subscriptionId)
	require.NoError(s.T(), err)
	require.Equal(s.T(), subscriptionId, dbSub.UID)
	require.Equal(s.T(), datastore.InactiveSubscriptionStatus, dbSub.Status)
}

func (s *SubscriptionIntegrationTestSuite) Test_ToggleSubscriptionStatus_InactiveStatus() {
	subscriptionId := "123456789"

	// Just Before
	app, _ := testdb.SeedApplication(s.DB, s.DefaultGroup, uuid.NewString(), "", false)
	endpoint, _ := testdb.SeedEndpoint(s.DB, app, s.DefaultGroup.UID)
	source, _ := testdb.SeedSource(s.DB, s.DefaultGroup, uuid.NewString(), "", "", nil)
	_, _ = testdb.SeedSubscription(s.DB, app, s.DefaultGroup, subscriptionId, datastore.OutgoingGroup, source, endpoint, &datastore.RetryConfiguration{}, &datastore.AlertConfiguration{}, &datastore.FilterConfiguration{}, datastore.InactiveSubscriptionStatus)

	// Arrange Request
	url := fmt.Sprintf("/api/v1/subscriptions/%s/toggle_status", subscriptionId)
	req := createRequest(http.MethodPut, url, s.APIKey, nil)
	w := httptest.NewRecorder()

	// Act
	s.Router.ServeHTTP(w, req)

	// Assert
	require.Equal(s.T(), http.StatusAccepted, w.Code)

	// Deep Asset
	var subscription *datastore.Subscription
	parseResponse(s.T(), w.Result(), &subscription)

	dbSub, err := s.DB.SubRepo().FindSubscriptionByID(context.Background(), s.DefaultGroup.UID, subscriptionId)
	require.NoError(s.T(), err)
	require.Equal(s.T(), subscriptionId, dbSub.UID)
	require.Equal(s.T(), datastore.ActiveSubscriptionStatus, dbSub.Status)
}

func (s *SubscriptionIntegrationTestSuite) Test_ToggleSubscriptionStatus_PendingStatus() {
	subscriptionId := "123456789"

	// Just Before
	app, _ := testdb.SeedApplication(s.DB, s.DefaultGroup, uuid.NewString(), "", false)
	endpoint, _ := testdb.SeedEndpoint(s.DB, app, s.DefaultGroup.UID)
	source, _ := testdb.SeedSource(s.DB, s.DefaultGroup, uuid.NewString(), "", "", nil)
	_, _ = testdb.SeedSubscription(s.DB, app, s.DefaultGroup, subscriptionId, datastore.OutgoingGroup, source, endpoint, &datastore.RetryConfiguration{}, &datastore.AlertConfiguration{}, &datastore.FilterConfiguration{}, datastore.PendingSubscriptionStatus)

	// Arrange Request
	url := fmt.Sprintf("/api/v1/subscriptions/%s/toggle_status", subscriptionId)
	req := createRequest(http.MethodPut, url, s.APIKey, nil)
	w := httptest.NewRecorder()

	// Act
	s.Router.ServeHTTP(w, req)

	// Assert
	require.Equal(s.T(), http.StatusBadRequest, w.Code)
}

func (s *SubscriptionIntegrationTestSuite) Test_ToggleSubscriptionStatus_UnknownStatus() {
	subscriptionId := "123456789"

	// Just Before
	app, _ := testdb.SeedApplication(s.DB, s.DefaultGroup, uuid.NewString(), "", false)
	endpoint, _ := testdb.SeedEndpoint(s.DB, app, s.DefaultGroup.UID)
	source, _ := testdb.SeedSource(s.DB, s.DefaultGroup, uuid.NewString(), "", "", nil)
	_, _ = testdb.SeedSubscription(s.DB, app, s.DefaultGroup, subscriptionId, datastore.OutgoingGroup, source, endpoint, &datastore.RetryConfiguration{}, &datastore.AlertConfiguration{}, &datastore.FilterConfiguration{}, "random")

	// Arrange Request
	url := fmt.Sprintf("/api/v1/subscriptions/%s/toggle_status", subscriptionId)
	req := createRequest(http.MethodPut, url, s.APIKey, nil)
	w := httptest.NewRecorder()

	// Act
	s.Router.ServeHTTP(w, req)

	// Assert
	require.Equal(s.T(), http.StatusBadRequest, w.Code)
}

func TestSubscriptionIntegrationTestSuite(t *testing.T) {
	suite.Run(t, new(SubscriptionIntegrationTestSuite))
}
