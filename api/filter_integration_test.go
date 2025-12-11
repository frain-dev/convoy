package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/frain-dev/convoy/api/models"
	"github.com/frain-dev/convoy/api/testdb"
	"github.com/frain-dev/convoy/auth"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/database"
	"github.com/frain-dev/convoy/database/postgres"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/internal/pkg/metrics"
	"github.com/frain-dev/convoy/internal/portal_links"
)

type FilterIntegrationTestSuite struct {
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

func (s *FilterIntegrationTestSuite) SetupSuite() {
	// s.DB = getDB()
	s.ConvoyApp = buildServer(s.T())
	s.Router = s.ConvoyApp.BuildControlPlaneRoutes()
}

func (s *FilterIntegrationTestSuite) SetupTest() {
	user, err := testdb.SeedDefaultUser(s.ConvoyApp.A.DB)
	require.NoError(s.T(), err)
	s.DefaultUser = user

	org, err := testdb.SeedDefaultOrganisation(s.ConvoyApp.A.DB, user)
	require.NoError(s.T(), err)
	s.DefaultOrg = org

	// Setup Default Project.
	s.DefaultProject, err = testdb.SeedDefaultProject(s.ConvoyApp.A.DB, s.DefaultOrg.UID)
	require.NoError(s.T(), err)

	// Seed Auth
	role := auth.Role{
		Type:    auth.RoleProjectAdmin,
		Project: s.DefaultProject.UID,
	}

	_, s.APIKey, err = testdb.SeedAPIKey(s.ConvoyApp.A.DB, role, "", "test", "", "")
	require.NoError(s.T(), err)

	// Setup authenticator
	s.AuthenticatorFn = authenticateRequest(&models.LoginUser{
		Username: user.Email,
		Password: testdb.DefaultUserPassword,
	})

	// Setup Config.
	err = config.LoadConfig("./testdata/Auth_Config/full-convoy.json")
	require.NoError(s.T(), err)

	apiRepo := postgres.NewAPIKeyRepo(s.ConvoyApp.A.DB)
	userRepo := postgres.NewUserRepo(s.ConvoyApp.A.DB)
	portalLinkRepo := portal_links.New(s.ConvoyApp.A.Logger, s.ConvoyApp.A.DB)
	initRealmChain(s.T(), apiRepo, userRepo, portalLinkRepo, s.ConvoyApp.A.Cache)
}

func (s *FilterIntegrationTestSuite) TearDownTest() {
	// testdb.PurgeDB(s.T(), s.DB)
	metrics.Reset()
}

func (s *FilterIntegrationTestSuite) Test_CreateFilter() {
	// Setup test data
	endpoint, err := testdb.SeedEndpoint(s.ConvoyApp.A.DB, s.DefaultProject, "", "", "", false, datastore.ActiveEndpointStatus)
	require.NoError(s.T(), err)

	source, err := testdb.SeedSource(s.ConvoyApp.A.DB, s.DefaultProject, "", "", "", nil, "", "")
	require.NoError(s.T(), err)

	subscription, err := testdb.SeedSubscription(
		s.ConvoyApp.A.DB,
		s.DefaultProject,
		ulid.Make().String(),
		datastore.OutgoingProject,
		source,
		endpoint,
		&datastore.RetryConfiguration{},
		&datastore.AlertConfiguration{},
		&datastore.FilterConfiguration{
			EventTypes: []string{"user.active"},
		},
	)
	require.NoError(s.T(), err)

	// First seed the event type to ensure it exists
	eventType, err := testdb.SeedEventType(s.ConvoyApp.A.DB, s.DefaultProject.UID, ulid.Make().String(), "user.created", "User created event", "user")
	require.NoError(s.T(), err)

	// Test data
	newFilter := models.CreateFilterRequest{
		EventType: eventType.Name,
		Headers:   datastore.M{"x-api-key": "abc123"},
		Body:      datastore.M{"user.active": true},
	}

	// Create request
	body, err := json.Marshal(newFilter)
	require.NoError(s.T(), err)

	url := fmt.Sprintf("/api/v1/projects/%s/subscriptions/%s/filters", s.DefaultProject.UID, subscription.UID)
	req := createRequest(http.MethodPost, url, "", bytes.NewReader(body))
	err = s.AuthenticatorFn(req, s.Router)
	require.NoError(s.T(), err)

	w := httptest.NewRecorder()
	s.Router.ServeHTTP(w, req)

	require.Equal(s.T(), http.StatusCreated, w.Code)

	var response models.FilterResponse
	parseResponse(s.T(), w.Result(), &response)

	// Verify filter created correctly
	require.Equal(s.T(), newFilter.EventType, response.EventTypeFilter.EventType)
	require.Equal(s.T(), newFilter.Headers, response.EventTypeFilter.Headers)
	require.Equal(s.T(), newFilter.Body, response.EventTypeFilter.Body)
	require.Equal(s.T(), subscription.UID, response.EventTypeFilter.SubscriptionID)
}

func (s *FilterIntegrationTestSuite) Test_GetFilter() {
	// Setup test data
	endpoint, err := testdb.SeedEndpoint(s.ConvoyApp.A.DB, s.DefaultProject, "", "", "", false, datastore.ActiveEndpointStatus)
	require.NoError(s.T(), err)

	source, err := testdb.SeedSource(s.ConvoyApp.A.DB, s.DefaultProject, "", "", "", nil, "", "")
	require.NoError(s.T(), err)

	subscription, err := testdb.SeedSubscription(
		s.ConvoyApp.A.DB,
		s.DefaultProject,
		ulid.Make().String(),
		datastore.OutgoingProject,
		source,
		endpoint,
		&datastore.RetryConfiguration{},
		&datastore.AlertConfiguration{},
		nil,
	)
	require.NoError(s.T(), err)

	headers := datastore.M{"x-api-key": "abc123"}
	body := datastore.M{"user.active": true}
	eventType := "user.created"

	filter, err := testdb.SeedFilter(s.ConvoyApp.A.DB, subscription.UID, "", eventType, headers, body)
	require.NoError(s.T(), err)

	// Create request
	url := fmt.Sprintf("/api/v1/projects/%s/subscriptions/%s/filters/%s", s.DefaultProject.UID, subscription.UID, filter.UID)
	req := createRequest(http.MethodGet, url, s.APIKey, nil)
	err = s.AuthenticatorFn(req, s.Router)
	require.NoError(s.T(), err)

	w := httptest.NewRecorder()

	// Act
	s.Router.ServeHTTP(w, req)

	// Assert
	require.Equal(s.T(), http.StatusOK, w.Code)

	var response models.FilterResponse
	parseResponse(s.T(), w.Result(), &response)

	// Verify filter retrieved correctly
	require.Equal(s.T(), filter.UID, response.EventTypeFilter.UID)
	require.Equal(s.T(), eventType, response.EventTypeFilter.EventType)
	require.Equal(s.T(), headers, response.EventTypeFilter.Headers)
	require.Equal(s.T(), body, response.EventTypeFilter.Body)
}

func (s *FilterIntegrationTestSuite) Test_GetFilters() {
	// Setup test data
	endpoint, err := testdb.SeedEndpoint(s.ConvoyApp.A.DB, s.DefaultProject, "", "", "", false, datastore.ActiveEndpointStatus)
	require.NoError(s.T(), err)

	source, err := testdb.SeedSource(s.ConvoyApp.A.DB, s.DefaultProject, "", "", "", nil, "", "")
	require.NoError(s.T(), err)

	subscription, err := testdb.SeedSubscription(
		s.ConvoyApp.A.DB,
		s.DefaultProject,
		ulid.Make().String(),
		datastore.OutgoingProject,
		source,
		endpoint,
		&datastore.RetryConfiguration{},
		&datastore.AlertConfiguration{},
		nil,
	)
	require.NoError(s.T(), err)

	// Create event types
	eventType1, err := testdb.SeedEventType(s.ConvoyApp.A.DB, s.DefaultProject.UID, ulid.Make().String(), "user.created", "User created event", "user")
	require.NoError(s.T(), err)

	eventType2, err := testdb.SeedEventType(s.ConvoyApp.A.DB, s.DefaultProject.UID, ulid.Make().String(), "user.updated", "User updated event", "user")
	require.NoError(s.T(), err)

	eventType3, err := testdb.SeedEventType(s.ConvoyApp.A.DB, s.DefaultProject.UID, ulid.Make().String(), "user.deleted", "User deleted event", "user")
	require.NoError(s.T(), err)

	// Create multiple filters with specific UIDs
	filter1UID := ulid.Make().String()
	filter1, err := testdb.SeedFilter(s.ConvoyApp.A.DB, subscription.UID, filter1UID, eventType1.Name,
		datastore.M{"x-api-key": "abc123"}, datastore.M{"user.active": true})
	require.NoError(s.T(), err)

	filter2UID := ulid.Make().String()
	filter2, err := testdb.SeedFilter(s.ConvoyApp.A.DB, subscription.UID, filter2UID, eventType2.Name,
		datastore.M{"x-api-key": "def456"}, datastore.M{"user.email": "test@example.com"})
	require.NoError(s.T(), err)

	filter3UID := ulid.Make().String()
	filter3, err := testdb.SeedFilter(s.ConvoyApp.A.DB, subscription.UID, filter3UID, eventType3.Name,
		datastore.M{"x-api-key": "ghi789"}, datastore.M{"user.id": "123"})
	require.NoError(s.T(), err)

	s.T().Run("get all filters", func(t *testing.T) {
		// Create request
		url := fmt.Sprintf("/api/v1/projects/%s/subscriptions/%s/filters", s.DefaultProject.UID, subscription.UID)
		req := createRequest(http.MethodGet, url, "", nil)
		err = s.AuthenticatorFn(req, s.Router)
		require.NoError(t, err)

		w := httptest.NewRecorder()
		s.Router.ServeHTTP(w, req)

		// Log the response for debugging
		t.Logf("Response: %s", w.Body.String())

		// Assert
		require.Equal(t, http.StatusOK, w.Code)

		// Try parsing as array directly
		var filters []models.FilterResponse
		parseResponse(t, w.Result(), &filters)

		// Verify our filters are in the response
		filterUIDs := []string{filter1.UID, filter2.UID, filter3.UID}
		foundFilters := 0

		for _, filter := range filters {
			if contains(filterUIDs, filter.EventTypeFilter.UID) {
				foundFilters++
			}
		}

		// Verify we found all our filters
		require.Equal(t, 3, foundFilters, "Expected to find all 3 of our seeded filters")

		// Verify event types match what we expected
		eventTypes := []string{}
		for _, filter := range filters {
			if contains(filterUIDs, filter.EventTypeFilter.UID) {
				eventTypes = append(eventTypes, filter.EventTypeFilter.EventType)
			}
		}
		require.Contains(t, eventTypes, eventType1.Name)
		require.Contains(t, eventTypes, eventType2.Name)
		require.Contains(t, eventTypes, eventType3.Name)
	})
}

// Helper function to check if a slice contains a string
func contains(slice []string, str string) bool {
	for _, item := range slice {
		if item == str {
			return true
		}
	}
	return false
}

func (s *FilterIntegrationTestSuite) Test_UpdateFilter() {
	// Setup test data
	endpoint, err := testdb.SeedEndpoint(s.ConvoyApp.A.DB, s.DefaultProject, "", "", "", false, datastore.ActiveEndpointStatus)
	require.NoError(s.T(), err)

	source, err := testdb.SeedSource(s.ConvoyApp.A.DB, s.DefaultProject, "", "", "", nil, "", "")
	require.NoError(s.T(), err)

	subscription, err := testdb.SeedSubscription(
		s.ConvoyApp.A.DB,
		s.DefaultProject,
		ulid.Make().String(),
		datastore.OutgoingProject,
		source,
		endpoint,
		&datastore.RetryConfiguration{},
		&datastore.AlertConfiguration{},
		nil,
	)
	require.NoError(s.T(), err)

	// First seed the event type to ensure it exists - use a fixed UID to avoid duplication
	eventTypeUID := ulid.Make().String()
	eventType, err := testdb.SeedEventType(s.ConvoyApp.A.DB, s.DefaultProject.UID, eventTypeUID, "user.created", "User created event", "user")
	require.NoError(s.T(), err)

	// Create a filter to update with a specific UID
	filterUID := ulid.Make().String()
	filter, err := testdb.SeedFilter(s.ConvoyApp.A.DB, subscription.UID, filterUID, eventType.Name,
		datastore.M{"x-api-key": "abc123"}, datastore.M{"user.active": true})
	require.NoError(s.T(), err)

	s.T().Run("update a filter", func(t *testing.T) {
		// Log the filter for debugging
		t.Logf("Filter to update: %+v", filter)
		t.Logf("Event type: %+v", eventType)

		// Updated filter data - include the event type in the update
		updateFilter := models.UpdateFilterRequest{
			EventType: eventType.Name, // Include the event type
			Headers:   datastore.M{"x-api-key": "new-key"},
			Body:      datastore.M{"user.email": "new@example.com"},
		}

		// Create request
		body, err := json.Marshal(updateFilter)
		require.NoError(t, err)

		url := fmt.Sprintf("/api/v1/projects/%s/subscriptions/%s/filters/%s", s.DefaultProject.UID, subscription.UID, filter.UID)
		t.Logf("Update URL: %s", url)

		req := createRequest(http.MethodPut, url, "", bytes.NewReader(body))
		err = s.AuthenticatorFn(req, s.Router)
		require.NoError(t, err)

		w := httptest.NewRecorder()
		s.Router.ServeHTTP(w, req)

		// Log the response for debugging
		t.Logf("Response: %s", w.Body.String())

		// Assert
		require.Equal(t, http.StatusOK, w.Code)

		var response models.FilterResponse
		parseResponse(t, w.Result(), &response)

		// Verify filter was updated correctly
		require.Equal(t, filter.UID, response.EventTypeFilter.UID)
		require.Equal(t, filter.EventType, response.EventTypeFilter.EventType) // event type shouldn't change
		require.Equal(t, updateFilter.Headers, response.EventTypeFilter.Headers)
		require.Equal(t, updateFilter.Body, response.EventTypeFilter.Body)
	})
}

func (s *FilterIntegrationTestSuite) Test_DeleteFilter() {
	// Setup test data
	endpoint, err := testdb.SeedEndpoint(s.ConvoyApp.A.DB, s.DefaultProject, "", "", "", false, datastore.ActiveEndpointStatus)
	require.NoError(s.T(), err)

	source, err := testdb.SeedSource(s.ConvoyApp.A.DB, s.DefaultProject, "", "", "", nil, "", "")
	require.NoError(s.T(), err)

	subscription, err := testdb.SeedSubscription(
		s.ConvoyApp.A.DB,
		s.DefaultProject,
		ulid.Make().String(),
		datastore.OutgoingProject,
		source,
		endpoint,
		&datastore.RetryConfiguration{},
		&datastore.AlertConfiguration{},
		nil,
	)
	require.NoError(s.T(), err)

	// Create a filter to delete
	filter, err := testdb.SeedFilter(s.ConvoyApp.A.DB, subscription.UID, "", "user.created",
		datastore.M{"x-api-key": "abc123"}, datastore.M{"user.active": true})
	require.NoError(s.T(), err)

	// Create request to delete
	url := fmt.Sprintf("/api/v1/projects/%s/subscriptions/%s/filters/%s", s.DefaultProject.UID, subscription.UID, filter.UID)
	req := createRequest(http.MethodDelete, url, s.APIKey, nil)
	err = s.AuthenticatorFn(req, s.Router)
	require.NoError(s.T(), err)

	w := httptest.NewRecorder()

	// Act
	s.Router.ServeHTTP(w, req)

	// Assert
	require.Equal(s.T(), http.StatusOK, w.Code)

	// Verify filter is actually deleted
	// by attempting to get it (should return 404)
	req = createRequest(http.MethodGet, url, s.APIKey, nil)
	err = s.AuthenticatorFn(req, s.Router)
	require.NoError(s.T(), err)

	w = httptest.NewRecorder()
	s.Router.ServeHTTP(w, req)
	require.Equal(s.T(), http.StatusNotFound, w.Code)
}

func (s *FilterIntegrationTestSuite) Test_TestFilter() {
	// Setup test data
	endpoint, err := testdb.SeedEndpoint(s.ConvoyApp.A.DB, s.DefaultProject, "", "", "", false, datastore.ActiveEndpointStatus)
	require.NoError(s.T(), err)

	source, err := testdb.SeedSource(s.ConvoyApp.A.DB, s.DefaultProject, "", "", "", nil, "", "")
	require.NoError(s.T(), err)

	subscription, err := testdb.SeedSubscription(
		s.ConvoyApp.A.DB,
		s.DefaultProject,
		ulid.Make().String(),
		datastore.OutgoingProject,
		source,
		endpoint,
		&datastore.RetryConfiguration{},
		&datastore.AlertConfiguration{},
		nil,
	)
	require.NoError(s.T(), err)

	// First seed the event type to ensure it exists
	eventType, err := testdb.SeedEventType(s.ConvoyApp.A.DB, s.DefaultProject.UID, ulid.Make().String(), "user.created", "User created event", "user")
	require.NoError(s.T(), err)

	// Create a filter for testing with specific criteria
	filter, err := testdb.SeedFilter(s.ConvoyApp.A.DB, subscription.UID, "", eventType.Name,
		nil, datastore.M{"user.active": true})
	require.NoError(s.T(), err)

	s.T().Run("test a filter", func(t *testing.T) {
		// Log the filter for debugging
		t.Logf("Filter: %+v", filter)

		// Test payload that should match the filter criteria exactly
		testPayload := models.TestFilterRequest{
			Payload: map[string]interface{}{
				"user": map[string]interface{}{
					"active": true,
				},
			},
		}

		// Create request
		body, err := json.Marshal(testPayload)
		require.NoError(t, err)

		url := fmt.Sprintf("/api/v1/projects/%s/subscriptions/%s/filters/test/%s", s.DefaultProject.UID, subscription.UID, eventType.Name)
		req := createRequest(http.MethodPost, url, "", bytes.NewReader(body))
		err = s.AuthenticatorFn(req, s.Router)
		require.NoError(t, err)

		w := httptest.NewRecorder()
		s.Router.ServeHTTP(w, req)

		// Log the response for debugging
		t.Logf("Response: %s", w.Body.String())

		// Assert
		require.Equal(t, http.StatusOK, w.Code)

		var response models.TestFilterResponse
		parseResponse(t, w.Result(), &response)

		// Verify the filter matched
		require.True(t, response.IsMatch)

		// Test a payload that should not match
		testPayload = models.TestFilterRequest{
			Payload: map[string]interface{}{
				"user": map[string]interface{}{
					"active": false,
				},
			},
		}

		// Create request
		body, err = json.Marshal(testPayload)
		require.NoError(t, err)

		req = createRequest(http.MethodPost, url, "", bytes.NewReader(body))
		err = s.AuthenticatorFn(req, s.Router)
		require.NoError(t, err)

		w = httptest.NewRecorder()
		s.Router.ServeHTTP(w, req)

		// Log the response for debugging
		t.Logf("Response (should not match): %s", w.Body.String())

		// Assert
		require.Equal(t, http.StatusOK, w.Code)

		parseResponse(t, w.Result(), &response)

		// Verify the filter did not match
		require.False(t, response.IsMatch)
	})
}

func (s *FilterIntegrationTestSuite) Test_BulkCreateFilters() {
	// Setup test data
	endpoint, err := testdb.SeedEndpoint(s.ConvoyApp.A.DB, s.DefaultProject, "", "", "", false, datastore.ActiveEndpointStatus)
	require.NoError(s.T(), err)

	source, err := testdb.SeedSource(s.ConvoyApp.A.DB, s.DefaultProject, "", "", "", nil, "", "")
	require.NoError(s.T(), err)

	subscription, err := testdb.SeedSubscription(
		s.ConvoyApp.A.DB,
		s.DefaultProject,
		ulid.Make().String(),
		datastore.OutgoingProject,
		source,
		endpoint,
		&datastore.RetryConfiguration{},
		&datastore.AlertConfiguration{},
		nil,
	)
	require.NoError(s.T(), err)

	// First seed the event types to ensure they exist
	eventType1, err := testdb.SeedEventType(s.ConvoyApp.A.DB, s.DefaultProject.UID, ulid.Make().String(), "user.created", "User created event", "user")
	require.NoError(s.T(), err)

	eventType2, err := testdb.SeedEventType(s.ConvoyApp.A.DB, s.DefaultProject.UID, ulid.Make().String(), "user.deleted", "User deleted event", "user")
	require.NoError(s.T(), err)

	s.T().Run("bulk create filters", func(t *testing.T) {
		// Test data - bulk create multiple filters
		newFilters := []models.CreateFilterRequest{
			{
				EventType: eventType1.Name,
				Headers:   datastore.M{"x-api-key": "abc123"},
				Body:      datastore.M{"user.active": true},
			},
			{
				EventType: eventType2.Name,
				Headers:   datastore.M{"x-api-key": "ghi789"},
				Body:      datastore.M{"user.id": "123"},
			},
		}

		// Create request
		body, err := json.Marshal(newFilters)
		require.NoError(t, err)

		url := fmt.Sprintf("/api/v1/projects/%s/subscriptions/%s/filters/bulk", s.DefaultProject.UID, subscription.UID)
		req := createRequest(http.MethodPost, url, "", bytes.NewReader(body))
		err = s.AuthenticatorFn(req, s.Router)
		require.NoError(t, err)

		w := httptest.NewRecorder()
		s.Router.ServeHTTP(w, req)

		// Assert
		require.Equal(t, http.StatusCreated, w.Code)

		var response []models.FilterResponse
		parseResponse(t, w.Result(), &response)

		// Verify filters created correctly
		require.Equal(t, len(newFilters), len(response))

		// Verify that the filters match what was requested
		for i, filter := range response {
			require.Equal(t, newFilters[i].EventType, filter.EventTypeFilter.EventType)
			require.Equal(t, newFilters[i].Headers, filter.EventTypeFilter.Headers)
			require.Equal(t, newFilters[i].Body, filter.EventTypeFilter.Body)
			require.Equal(t, subscription.UID, filter.EventTypeFilter.SubscriptionID)
		}
	})
}

func (s *FilterIntegrationTestSuite) Test_BulkUpdateFilters() {
	// Setup test data
	endpoint, err := testdb.SeedEndpoint(s.ConvoyApp.A.DB, s.DefaultProject, "", "", "", false, datastore.ActiveEndpointStatus)
	require.NoError(s.T(), err)

	source, err := testdb.SeedSource(s.ConvoyApp.A.DB, s.DefaultProject, "", "", "", nil, "", "")
	require.NoError(s.T(), err)

	subscription, err := testdb.SeedSubscription(
		s.ConvoyApp.A.DB,
		s.DefaultProject,
		ulid.Make().String(),
		datastore.OutgoingProject,
		source,
		endpoint,
		&datastore.RetryConfiguration{},
		&datastore.AlertConfiguration{},
		nil,
	)
	require.NoError(s.T(), err)

	// First seed the event types to ensure they exist
	eventType1, err := testdb.SeedEventType(s.ConvoyApp.A.DB, s.DefaultProject.UID, ulid.Make().String(), "user.created", "User created event", "user")
	require.NoError(s.T(), err)

	eventType2, err := testdb.SeedEventType(s.ConvoyApp.A.DB, s.DefaultProject.UID, ulid.Make().String(), "user.deleted", "User deleted event", "user")
	require.NoError(s.T(), err)

	// Create filters to update with specific UIDs
	filter1UID := ulid.Make().String()
	filter1, err := testdb.SeedFilter(s.ConvoyApp.A.DB, subscription.UID, filter1UID, eventType1.Name,
		datastore.M{"x-api-key": "abc123"}, datastore.M{"user.active": true})
	require.NoError(s.T(), err)

	filter2UID := ulid.Make().String()
	filter2, err := testdb.SeedFilter(s.ConvoyApp.A.DB, subscription.UID, filter2UID, eventType2.Name,
		datastore.M{"x-api-key": "def456"}, datastore.M{"user.email": "test@example.com"})
	require.NoError(s.T(), err)

	s.T().Run("bulk update filters", func(t *testing.T) {
		// Log the filters for debugging
		t.Logf("Filter 1 to update: %+v", filter1)
		t.Logf("Filter 2 to update: %+v", filter2)

		// Prepare bulk update data
		updateFilters := []models.BulkUpdateFilterRequest{
			{
				UID:     filter1.UID,
				Headers: datastore.M{"x-api-key": "new-key-1"},
				Body:    datastore.M{"user.active": false},
			},
			{
				UID:     filter2.UID,
				Headers: datastore.M{"x-api-key": "new-key-2"},
				Body:    datastore.M{"user.email": "updated@example.com"},
			},
		}

		// Create request
		body, err := json.Marshal(updateFilters)
		require.NoError(t, err)

		url := fmt.Sprintf("/api/v1/projects/%s/subscriptions/%s/filters/bulk_update", s.DefaultProject.UID, subscription.UID)
		t.Logf("Bulk update URL: %s", url)

		req := createRequest(http.MethodPost, url, "", bytes.NewReader(body))
		err = s.AuthenticatorFn(req, s.Router)
		require.NoError(t, err)

		w := httptest.NewRecorder()
		s.Router.ServeHTTP(w, req)

		// Log the response for debugging
		t.Logf("Response: %s", w.Body.String())

		// Assert
		require.Equal(t, http.StatusOK, w.Code)

		var response []models.FilterResponse
		parseResponse(t, w.Result(), &response)

		// Verify filters updated correctly
		require.Equal(t, len(updateFilters), len(response))

		// Map response filters by UID for easier validation
		filtersByUID := make(map[string]models.FilterResponse)
		for _, filter := range response {
			filtersByUID[filter.EventTypeFilter.UID] = filter
		}

		// Verify first filter updates - convert to JSON and back to normalize types for comparison
		updatedFilter1 := filtersByUID[filter1.UID]
		headersJSON1, err := json.Marshal(updatedFilter1.EventTypeFilter.Headers)
		require.NoError(t, err)
		var normalizedHeaders1 map[string]interface{}
		err = json.Unmarshal(headersJSON1, &normalizedHeaders1)
		require.NoError(t, err)

		bodyJSON1, err := json.Marshal(updatedFilter1.EventTypeFilter.Body)
		require.NoError(t, err)
		var normalizedBody1 map[string]interface{}
		err = json.Unmarshal(bodyJSON1, &normalizedBody1)
		require.NoError(t, err)

		expectedHeadersJSON1, err := json.Marshal(updateFilters[0].Headers)
		require.NoError(t, err)
		var expectedNormalizedHeaders1 map[string]interface{}
		err = json.Unmarshal(expectedHeadersJSON1, &expectedNormalizedHeaders1)
		require.NoError(t, err)

		expectedBodyJSON1, err := json.Marshal(updateFilters[0].Body)
		require.NoError(t, err)
		var expectedNormalizedBody1 map[string]interface{}
		err = json.Unmarshal(expectedBodyJSON1, &expectedNormalizedBody1)
		require.NoError(t, err)

		require.Equal(t, filter1.EventType, updatedFilter1.EventTypeFilter.EventType) // Event type shouldn't change
		require.Equal(t, expectedNormalizedHeaders1, normalizedHeaders1)
		require.Equal(t, expectedNormalizedBody1, normalizedBody1)

		// Verify second filter updates - convert to JSON and back to normalize types for comparison
		updatedFilter2 := filtersByUID[filter2.UID]
		headersJSON2, err := json.Marshal(updatedFilter2.EventTypeFilter.Headers)
		require.NoError(t, err)
		var normalizedHeaders2 map[string]interface{}
		err = json.Unmarshal(headersJSON2, &normalizedHeaders2)
		require.NoError(t, err)

		bodyJSON2, err := json.Marshal(updatedFilter2.EventTypeFilter.Body)
		require.NoError(t, err)
		var normalizedBody2 map[string]interface{}
		err = json.Unmarshal(bodyJSON2, &normalizedBody2)
		require.NoError(t, err)

		expectedHeadersJSON2, err := json.Marshal(updateFilters[1].Headers)
		require.NoError(t, err)
		var expectedNormalizedHeaders2 map[string]interface{}
		err = json.Unmarshal(expectedHeadersJSON2, &expectedNormalizedHeaders2)
		require.NoError(t, err)

		expectedBodyJSON2, err := json.Marshal(updateFilters[1].Body)
		require.NoError(t, err)
		var expectedNormalizedBody2 map[string]interface{}
		err = json.Unmarshal(expectedBodyJSON2, &expectedNormalizedBody2)
		require.NoError(t, err)

		require.Equal(t, filter2.EventType, updatedFilter2.EventTypeFilter.EventType) // Event type shouldn't change
		require.Equal(t, expectedNormalizedHeaders2, normalizedHeaders2)
		require.Equal(t, expectedNormalizedBody2, normalizedBody2)
	})
}

func TestFilterIntegrationTestSuite(t *testing.T) {
	suite.Run(t, new(FilterIntegrationTestSuite))
}
