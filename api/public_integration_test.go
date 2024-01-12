//go:build integration
// +build integration

package api

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/frain-dev/convoy/api/models"
	"github.com/frain-dev/convoy/api/testdb"
	"github.com/frain-dev/convoy/auth"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/database"
	"github.com/frain-dev/convoy/database/postgres"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/internal/pkg/metrics"
	"github.com/jaswdr/faker"
	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type PublicEndpointIntegrationTestSuite struct {
	suite.Suite
	DB             database.Database
	Router         http.Handler
	ConvoyApp      *ApplicationHandler
	DefaultOrg     *datastore.Organisation
	DefaultProject *datastore.Project
	DefaultUser    *datastore.User
	APIKey         string
	PersonalAPIKey string
}

func (s *PublicEndpointIntegrationTestSuite) SetupSuite() {
	s.DB = getDB()
	s.ConvoyApp = buildServer()
	s.Router = s.ConvoyApp.BuildRoutes()
}

func (s *PublicEndpointIntegrationTestSuite) SetupTest() {
	testdb.PurgeDB(s.T(), s.DB)

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
		Type:    auth.RoleAdmin,
		Project: s.DefaultProject.UID,
	}

	_, s.APIKey, err = testdb.SeedAPIKey(s.ConvoyApp.A.DB, role, "", "test", "", "")
	require.NoError(s.T(), err)

	_, s.PersonalAPIKey, err = testdb.SeedAPIKey(s.ConvoyApp.A.DB, auth.Role{}, ulid.Make().String(), "test-personal-key", string(datastore.PersonalKey), s.DefaultUser.UID)
	require.NoError(s.T(), err)

	// Setup Config.
	err = config.LoadConfig("./testdata/Auth_Config/full-convoy.json")
	require.NoError(s.T(), err)

	apiRepo := postgres.NewAPIKeyRepo(s.ConvoyApp.A.DB, nil)
	userRepo := postgres.NewUserRepo(s.ConvoyApp.A.DB, nil)
	portalLinkRepo := postgres.NewPortalLinkRepo(s.ConvoyApp.A.DB, s.ConvoyApp.A.Cache)
	initRealmChain(s.T(), apiRepo, userRepo, portalLinkRepo, s.ConvoyApp.A.Cache)
}

func (s *PublicEndpointIntegrationTestSuite) TearDownTest() {
	testdb.PurgeDB(s.T(), s.DB)
	metrics.Reset()
}

func (s *PublicEndpointIntegrationTestSuite) Test_GetEndpoint_EndpointNotFound() {
	appID := "123"
	expectedStatusCode := http.StatusNotFound

	// Arrange Request.
	url := fmt.Sprintf("/api/v1/projects/%s/endpoints/%s", s.DefaultProject.UID, appID)
	req := createRequest(http.MethodGet, url, s.APIKey, nil)
	w := httptest.NewRecorder()

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)
}

func (s *PublicEndpointIntegrationTestSuite) Test_GetEndpoint_ValidEndpoint() {
	endpointID := "123456789"
	expectedStatusCode := http.StatusOK

	// Just Before.
	_, _ = testdb.SeedEndpoint(s.ConvoyApp.A.DB, s.DefaultProject, endpointID, "", "", true, datastore.ActiveEndpointStatus)

	// Arrange Request.
	url := fmt.Sprintf("/api/v1/projects/%s/endpoints/%s", s.DefaultProject.UID, endpointID)
	req := createRequest(http.MethodGet, url, s.APIKey, nil)
	w := httptest.NewRecorder()

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)

	// Deep Assert.
	var endpoint datastore.Endpoint
	parseResponse(s.T(), w.Result(), &endpoint)

	endpointRepo := postgres.NewEndpointRepo(s.ConvoyApp.A.DB, nil)
	dbEndpoint, err := endpointRepo.FindEndpointByID(context.Background(), endpointID, s.DefaultProject.UID)
	require.NoError(s.T(), err)
	require.Equal(s.T(), endpoint.UID, dbEndpoint.UID)
	require.Equal(s.T(), endpoint.Title, dbEndpoint.Title)
}

func (s *PublicEndpointIntegrationTestSuite) Test_GetEndpoint_ValidEndpoint_WithPersonalAPIKey() {
	endpointID := "123456789"
	expectedStatusCode := http.StatusOK

	// Just Before.
	_, _ = testdb.SeedEndpoint(s.ConvoyApp.A.DB, s.DefaultProject, endpointID, "", "", true, datastore.ActiveEndpointStatus)

	// Arrange Request.
	url := fmt.Sprintf("/api/v1/projects/%s/endpoints/%s", s.DefaultProject.UID, endpointID)
	req := createRequest(http.MethodGet, url, s.PersonalAPIKey, nil)
	w := httptest.NewRecorder()

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)

	// Deep Assert.
	var endpoint datastore.Endpoint
	parseResponse(s.T(), w.Result(), &endpoint)

	endpointRepo := postgres.NewEndpointRepo(s.ConvoyApp.A.DB, nil)
	dbEndpoint, err := endpointRepo.FindEndpointByID(context.Background(), endpointID, s.DefaultProject.UID)
	require.NoError(s.T(), err)
	require.Equal(s.T(), endpoint.UID, dbEndpoint.UID)
	require.Equal(s.T(), endpoint.Title, dbEndpoint.Title)
}

func (s *PublicEndpointIntegrationTestSuite) Test_GetEndpoints_ValidEndpoints() {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	totalEndpoints := r.Intn(5) + 1
	expectedStatusCode := http.StatusOK

	// Just Before.
	err := testdb.SeedMultipleEndpoints(s.ConvoyApp.A.DB, s.DefaultProject, totalEndpoints)
	require.NoError(s.T(), err)

	// Arrange.
	url := fmt.Sprintf("/api/v1/projects/%s/endpoints", s.DefaultProject.UID)
	req := createRequest(http.MethodGet, url, s.APIKey, nil)
	w := httptest.NewRecorder()

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)

	// Deep Assert.
	var resp pagedResponse
	parseResponse(s.T(), w.Result(), &resp)
	require.Equal(s.T(), totalEndpoints, len(resp.Content.([]interface{})))
}

func (s *PublicEndpointIntegrationTestSuite) Test_GetEndpoints_ValidEndpoints_WithPersonalAPIKey() {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	totalEndpoints := r.Intn(5) + 1
	expectedStatusCode := http.StatusOK

	// Just Before.
	_ = testdb.SeedMultipleEndpoints(s.ConvoyApp.A.DB, s.DefaultProject, totalEndpoints)

	// Arrange.
	url := fmt.Sprintf("/api/v1/projects/%s/endpoints", s.DefaultProject.UID)
	req := createRequest(http.MethodGet, url, s.PersonalAPIKey, nil)
	w := httptest.NewRecorder()

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)

	// Deep Assert.
	var resp pagedResponse
	parseResponse(s.T(), w.Result(), &resp)
	require.Equal(s.T(), totalEndpoints, len(resp.Content.([]interface{})))
}

func (s *PublicEndpointIntegrationTestSuite) Test_GetEndpoints_Filters() {
	s.T().Skip("Depends on #637")
}

func (s *PublicEndpointIntegrationTestSuite) Test_CreateEndpoint() {
	endpointTitle := fmt.Sprintf("Test-%s", ulid.Make().String())
	endpointURL := faker.New().Internet().URL()
	expectedStatusCode := http.StatusCreated

	// Arrange Request.
	url := fmt.Sprintf("/api/v1/projects/%s/endpoints", s.DefaultProject.UID)
	plainBody := fmt.Sprintf(`{
		"name": "%s",
		"description": "test endpoint",
		"url": "%s"
	}`, endpointTitle, endpointURL)
	body := strings.NewReader(plainBody)
	req := createRequest(http.MethodPost, url, s.APIKey, body)
	w := httptest.NewRecorder()

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)

	// Deep Assert.
	var endpoint datastore.Endpoint
	parseResponse(s.T(), w.Result(), &endpoint)

	endpointRepo := postgres.NewEndpointRepo(s.ConvoyApp.A.DB, nil)
	dbEndpoint, err := endpointRepo.FindEndpointByID(context.Background(), endpoint.UID, s.DefaultProject.UID)
	require.NoError(s.T(), err)
	require.Equal(s.T(), endpointTitle, dbEndpoint.Title)
	require.Equal(s.T(), endpointURL, dbEndpoint.TargetURL)
}

func (s *PublicEndpointIntegrationTestSuite) Test_CreateEndpointWithPersonalAPIKey() {
	endpointTitle := fmt.Sprintf("Test-%s", ulid.Make().String())
	endpointURL := faker.New().Internet().URL()
	expectedStatusCode := http.StatusCreated

	// Arrange Request.
	url := fmt.Sprintf("/api/v1/projects/%s/endpoints", s.DefaultProject.UID)
	body := serialize(`{
		"name": "%s",
		"description": "test endpoint",
		"url": "%s"
		}`, endpointTitle, endpointURL)
	req := createRequest(http.MethodPost, url, s.PersonalAPIKey, body)
	w := httptest.NewRecorder()

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)

	// Deep Assert.
	var endpoint datastore.Endpoint
	parseResponse(s.T(), w.Result(), &endpoint)

	endpointRepo := postgres.NewEndpointRepo(s.ConvoyApp.A.DB, nil)
	dbApp, err := endpointRepo.FindEndpointByID(context.Background(), endpoint.UID, s.DefaultProject.UID)
	require.NoError(s.T(), err)
	require.Equal(s.T(), endpointTitle, dbApp.Title)
	require.Equal(s.T(), endpointURL, dbApp.TargetURL)
}

func (s *PublicEndpointIntegrationTestSuite) Test_CreateEndpoint_NoName() {
	endpointTitle := ""
	expectedStatusCode := http.StatusBadRequest

	// Arrange Request.
	url := fmt.Sprintf("/api/v1/projects/%s/endpoints", s.DefaultProject.UID)
	plainBody := fmt.Sprintf(`{
		"name": "%s"
	}`, endpointTitle)
	body := strings.NewReader(plainBody)
	req := createRequest(http.MethodPost, url, s.APIKey, body)
	w := httptest.NewRecorder()

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)
}

func (s *PublicEndpointIntegrationTestSuite) Test_UpdateEndpoint_InvalidRequest() {
	endpointID := ulid.Make().String()
	expectedStatusCode := http.StatusBadRequest

	// Just Before.
	_, _ = testdb.SeedEndpoint(s.ConvoyApp.A.DB, s.DefaultProject, endpointID, "", "", true, datastore.ActiveEndpointStatus)

	// Arrange Request.
	url := fmt.Sprintf("/api/v1/projects/%s/endpoints/%s", s.DefaultProject.UID, endpointID)
	plainBody := ""
	body := strings.NewReader(plainBody)
	req := createRequest(http.MethodPut, url, s.APIKey, body)
	w := httptest.NewRecorder()

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)
}

func (s *PublicEndpointIntegrationTestSuite) Test_UpdateEndpoint() {
	title := "random-name"
	endpointURL := faker.New().Internet().URL()
	supportEmail := "10xengineer@getconvoy.io"
	endpointID := ulid.Make().String()
	expectedStatusCode := http.StatusAccepted

	// Just Before.
	_, _ = testdb.SeedEndpoint(s.ConvoyApp.A.DB, s.DefaultProject, endpointID, "", "", false, datastore.ActiveEndpointStatus)

	// Arrange Request.
	url := fmt.Sprintf("/api/v1/projects/%s/endpoints/%s", s.DefaultProject.UID, endpointID)
	plainBody := fmt.Sprintf(`{
		"name": "%s",
		"description": "test endpoint",
		"url": "%s",
		"support_email": "%s"
 	}`, title, endpointURL, supportEmail)
	body := strings.NewReader(plainBody)
	req := createRequest(http.MethodPut, url, s.APIKey, body)
	w := httptest.NewRecorder()

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)

	// Deep Assert.
	var endpoint datastore.Endpoint
	parseResponse(s.T(), w.Result(), &endpoint)

	endpointRepo := postgres.NewEndpointRepo(s.ConvoyApp.A.DB, nil)
	dbEndpoint, err := endpointRepo.FindEndpointByID(context.Background(), endpointID, s.DefaultProject.UID)
	require.NoError(s.T(), err)
	require.Equal(s.T(), endpoint.UID, dbEndpoint.UID)
	require.Equal(s.T(), title, dbEndpoint.Title)
	require.Equal(s.T(), supportEmail, dbEndpoint.SupportEmail)
	require.Equal(s.T(), endpointURL, dbEndpoint.TargetURL)
}

func (s *PublicEndpointIntegrationTestSuite) Test_UpdateEndpoint_WithPersonalAPIKey() {
	title := "random-name"
	supportEmail := "10xengineer@getconvoy.io"
	isDisabled := randBool()
	endpointID := ulid.Make().String()
	expectedStatusCode := http.StatusAccepted
	endpointURL := faker.New().Internet().URL()

	// Just Before.
	_, _ = testdb.SeedEndpoint(s.ConvoyApp.A.DB, s.DefaultProject, endpointID, "", "", isDisabled, datastore.ActiveEndpointStatus)

	// Arrange Request.
	url := fmt.Sprintf("/api/v1/projects/%s/endpoints/%s", s.DefaultProject.UID, endpointID)
	body := serialize(`{
		"name": "%s",
		"description": "test endpoint",
		"support_email": "%s",
		"url": "%s",
		"is_disabled": %t
	}`, title, supportEmail, endpointURL, !isDisabled)
	req := createRequest(http.MethodPut, url, s.PersonalAPIKey, body)
	w := httptest.NewRecorder()

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)

	// Deep Assert.
	var endpoint datastore.Endpoint
	parseResponse(s.T(), w.Result(), &endpoint)

	endpointRepo := postgres.NewEndpointRepo(s.ConvoyApp.A.DB, nil)
	dbEndpoint, err := endpointRepo.FindEndpointByID(context.Background(), endpointID, s.DefaultProject.UID)
	require.NoError(s.T(), err)
	require.Equal(s.T(), endpoint.UID, dbEndpoint.UID)
	require.Equal(s.T(), title, dbEndpoint.Title)
	require.Equal(s.T(), supportEmail, dbEndpoint.SupportEmail)
	require.Equal(s.T(), endpointURL, dbEndpoint.TargetURL)
}

func (s *PublicEndpointIntegrationTestSuite) Test_DeleteEndpoint() {
	endpointID := ulid.Make().String()
	expectedStatusCode := http.StatusOK

	// Just Before.
	_, _ = testdb.SeedEndpoint(s.ConvoyApp.A.DB, s.DefaultProject, endpointID, "", "", true, datastore.ActiveEndpointStatus)

	// Arrange Request.
	url := fmt.Sprintf("/api/v1/projects/%s/endpoints/%s", s.DefaultProject.UID, endpointID)
	req := createRequest(http.MethodDelete, url, s.APIKey, nil)
	w := httptest.NewRecorder()

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)

	// Deep Assert.
	endpointRepo := postgres.NewEndpointRepo(s.ConvoyApp.A.DB, nil)
	_, err := endpointRepo.FindEndpointByID(context.Background(), endpointID, s.DefaultProject.UID)
	require.Error(s.T(), err, datastore.ErrEndpointNotFound)
}

func (s *PublicEndpointIntegrationTestSuite) Test_DeleteEndpoint_WithPersonalAPIKey() {
	endpointID := ulid.Make().String()
	expectedStatusCode := http.StatusOK

	// Just Before.
	_, _ = testdb.SeedEndpoint(s.ConvoyApp.A.DB, s.DefaultProject, endpointID, "", "", true, datastore.ActiveEndpointStatus)

	// Arrange Request.
	url := fmt.Sprintf("/api/v1/projects/%s/endpoints/%s", s.DefaultProject.UID, endpointID)
	req := createRequest(http.MethodDelete, url, s.PersonalAPIKey, nil)
	w := httptest.NewRecorder()

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)

	// Deep Assert.
	endpointRepo := postgres.NewEndpointRepo(s.ConvoyApp.A.DB, nil)
	_, err := endpointRepo.FindEndpointByID(context.Background(), endpointID, s.DefaultProject.UID)
	require.Error(s.T(), err, datastore.ErrEndpointNotFound)
}

func (s *PublicEndpointIntegrationTestSuite) Test_CreateEndpoint_With_Custom_Authentication() {
	title := "random-name"
	f := faker.New()
	endpointURL := f.Internet().URL()
	secret := f.Lorem().Text(25)
	expectedStatusCode := http.StatusCreated

	// Arrange Request
	url := fmt.Sprintf("/api/v1/projects/%s/endpoints", s.DefaultProject.UID)
	plainBody := fmt.Sprintf(`{
		"name": "%s",
		"url": "%s",
		"secret": "%s",
		"description": "default endpoint",
		"authentication": {
			"type": "api_key",
			"api_key": {
				"header_name": "x-api-key",
				"header_value": "testapikey"
			}
		}
	}`, title, endpointURL, secret)
	body := strings.NewReader(plainBody)
	req := createRequest(http.MethodPost, url, s.APIKey, body)
	w := httptest.NewRecorder()

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)

	// Deep Assert.
	var endpoint datastore.Endpoint
	parseResponse(s.T(), w.Result(), &endpoint)

	require.Equal(s.T(), title, endpoint.Title)
	require.Equal(s.T(), endpointURL, endpoint.TargetURL)
	require.Equal(s.T(), datastore.EndpointAuthenticationType("api_key"), endpoint.Authentication.Type)
	require.Equal(s.T(), "x-api-key", endpoint.Authentication.ApiKey.HeaderName)
	require.Equal(s.T(), "testapikey", endpoint.Authentication.ApiKey.HeaderValue)
}

func (s *PublicEndpointIntegrationTestSuite) Test_ExpireEndpointSecret() {
	endpointID := ulid.Make().String()
	f := faker.New()
	secret := f.Lorem().Text(25)
	expiration := 7
	expectedStatusCode := http.StatusOK

	// Just Before.
	e, _ := testdb.SeedEndpoint(s.ConvoyApp.A.DB, s.DefaultProject, endpointID, "", "", true, datastore.ActiveEndpointStatus)
	_, _ = testdb.SeedEndpointSecret(s.ConvoyApp.A.DB, e, secret)

	// Arrange Request
	url := fmt.Sprintf("/api/v1/projects/%s/endpoints/%s/expire_secret", s.DefaultProject.UID, endpointID)
	plainBody := fmt.Sprintf(`{
		"expiration": %d
	}`, expiration)
	body := strings.NewReader(plainBody)
	req := createRequest(http.MethodPut, url, s.APIKey, body)
	w := httptest.NewRecorder()

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)

	// Deep Assert.
	var endpoint datastore.Endpoint
	parseResponse(s.T(), w.Result(), &endpoint)

	endpointRepo := postgres.NewEndpointRepo(s.ConvoyApp.A.DB, nil)
	endpoint2, err := endpointRepo.FindEndpointByID(context.Background(), endpointID, s.DefaultProject.UID)
	require.NoError(s.T(), err)
	require.NotEmpty(s.T(), endpoint2.Secrets[0].ExpiresAt)
}

func (s *PublicEndpointIntegrationTestSuite) Test_PauseEndpoint_PausedStatus() {
	endpointId := ulid.Make().String()

	// Just Before
	_, err := testdb.SeedEndpoint(s.ConvoyApp.A.DB, s.DefaultProject, endpointId, "", "", false, datastore.ActiveEndpointStatus)
	require.NoError(s.T(), err)

	// Arrange Request
	url := fmt.Sprintf("/api/v1/projects/%s/endpoints/%s/pause", s.DefaultProject.UID, endpointId)
	req := createRequest(http.MethodPut, url, s.APIKey, nil)
	w := httptest.NewRecorder()

	// Act
	s.Router.ServeHTTP(w, req)

	// Assert
	require.Equal(s.T(), http.StatusAccepted, w.Code)

	// Deep Asset
	var endpoint *datastore.Endpoint
	parseResponse(s.T(), w.Result(), &endpoint)

	endpointRepo := postgres.NewEndpointRepo(s.ConvoyApp.A.DB, nil)
	dbEndpoint, err := endpointRepo.FindEndpointByID(context.Background(), endpointId, s.DefaultProject.UID)
	require.NoError(s.T(), err)
	require.Equal(s.T(), endpointId, dbEndpoint.UID)
	require.Equal(s.T(), datastore.PausedEndpointStatus, dbEndpoint.Status)
}

func (s *PublicEndpointIntegrationTestSuite) Test_PauseEndpoint_ActiveStatus() {
	endpointId := ulid.Make().String()

	// Just Before
	_, err := testdb.SeedEndpoint(s.ConvoyApp.A.DB, s.DefaultProject, endpointId, "", "", false, datastore.PausedEndpointStatus)
	require.NoError(s.T(), err)

	// Arrange Request
	url := fmt.Sprintf("/api/v1/projects/%s/endpoints/%s/pause", s.DefaultProject.UID, endpointId)
	req := createRequest(http.MethodPut, url, s.APIKey, nil)
	w := httptest.NewRecorder()

	// Act
	s.Router.ServeHTTP(w, req)

	// Assert
	require.Equal(s.T(), http.StatusAccepted, w.Code)

	// Deep Asset
	var endpoint *datastore.Endpoint
	parseResponse(s.T(), w.Result(), &endpoint)

	endpointRepo := postgres.NewEndpointRepo(s.ConvoyApp.A.DB, nil)
	dbEndpoint, err := endpointRepo.FindEndpointByID(context.Background(), endpointId, s.DefaultProject.UID)
	require.NoError(s.T(), err)
	require.Equal(s.T(), endpointId, dbEndpoint.UID)
	require.Equal(s.T(), datastore.ActiveEndpointStatus, dbEndpoint.Status)
}

func TestPublicEndpointIntegrationTestSuite(t *testing.T) {
	suite.Run(t, new(PublicEndpointIntegrationTestSuite))
}

type PublicEventIntegrationTestSuite struct {
	suite.Suite
	DB             database.Database
	Router         http.Handler
	ConvoyApp      *ApplicationHandler
	DefaultProject *datastore.Project
	APIKey         string
}

func (s *PublicEventIntegrationTestSuite) SetupSuite() {
	s.DB = getDB()
	s.ConvoyApp = buildServer()
	s.Router = s.ConvoyApp.BuildRoutes()
}

func (s *PublicEventIntegrationTestSuite) SetupTest() {
	testdb.PurgeDB(s.T(), s.DB)

	user, err := testdb.SeedDefaultUser(s.ConvoyApp.A.DB)
	require.NoError(s.T(), err)

	org, err := testdb.SeedDefaultOrganisation(s.ConvoyApp.A.DB, user)
	require.NoError(s.T(), err)

	// Setup Default Project.
	s.DefaultProject, _ = testdb.SeedDefaultProject(s.ConvoyApp.A.DB, org.UID)

	// Seed Auth
	role := auth.Role{
		Type:    auth.RoleAdmin,
		Project: s.DefaultProject.UID,
	}

	_, s.APIKey, _ = testdb.SeedAPIKey(s.ConvoyApp.A.DB, role, "", "test", "", "")

	// Setup Config.
	err = config.LoadConfig("./testdata/Auth_Config/full-convoy.json")
	require.NoError(s.T(), err)

	apiRepo := postgres.NewAPIKeyRepo(s.ConvoyApp.A.DB, nil)
	userRepo := postgres.NewUserRepo(s.ConvoyApp.A.DB, nil)
	portalLinkRepo := postgres.NewPortalLinkRepo(s.ConvoyApp.A.DB, s.ConvoyApp.A.Cache)
	initRealmChain(s.T(), apiRepo, userRepo, portalLinkRepo, s.ConvoyApp.A.Cache)
}

func (s *PublicEventIntegrationTestSuite) TearDownTest() {
	testdb.PurgeDB(s.T(), s.DB)
	metrics.Reset()
}

func (s *PublicEventIntegrationTestSuite) Test_CreateEndpointEvent() {
	endpointID := ulid.Make().String()
	expectedStatusCode := http.StatusCreated

	// Just Before.
	_, _ = testdb.SeedEndpoint(s.ConvoyApp.A.DB, s.DefaultProject, endpointID, "", "", false, datastore.ActiveEndpointStatus)

	bodyStr := `{"endpoint_id": "%s", "event_type":"*", "data":{"level":"test"}}`
	body := serialize(bodyStr, endpointID)

	url := fmt.Sprintf("/api/v1/projects/%s/events", s.DefaultProject.UID)
	req := createRequest(http.MethodPost, url, s.APIKey, body)
	w := httptest.NewRecorder()
	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)

	//// Deep Assert.
	//var event datastore.Event
	//parseResponse(s.T(), w.Result(), &event)
	//
	//require.NotEmpty(s.T(), event.UID)
	//require.Equal(s.T(), event.Endpoinints[0], endpointID)
}

func (s *PublicEventIntegrationTestSuite) Test_CreateDynamicEvent() {
	endpointID := ulid.Make().String()
	expectedStatusCode := http.StatusCreated

	// Just Before.
	_, _ = testdb.SeedEndpoint(s.ConvoyApp.A.DB, s.DefaultProject, endpointID, "", "", false, datastore.ActiveEndpointStatus)

	bodyStr := `{
        "endpoint": {
            "url":"https://testing.com",
            "secret": "12345"
        },
        "event": {
            "event_type":"*",
            "data": {"name":"daniel"},
            "custom_headers": {"x-sig":"convoy"}
        }
}`
	body := serialize(bodyStr, endpointID)

	url := fmt.Sprintf("/api/v1/projects/%s/events/dynamic", s.DefaultProject.UID)
	req := createRequest(http.MethodPost, url, s.APIKey, body)
	w := httptest.NewRecorder()
	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)
}

func (s *PublicEventIntegrationTestSuite) Test_CreateFanoutEvent_MultipleEndpoints() {
	endpointID := ulid.Make().String()
	expectedStatusCode := http.StatusCreated
	ownerID := ulid.Make().String()

	// Just Before.
	_, _ = testdb.SeedEndpoint(s.ConvoyApp.A.DB, s.DefaultProject, endpointID, "", ownerID, false, datastore.ActiveEndpointStatus)
	_, _ = testdb.SeedEndpoint(s.ConvoyApp.A.DB, s.DefaultProject, ulid.Make().String(), "", ownerID, false, datastore.ActiveEndpointStatus)

	bodyStr := `{"owner_id":"%s", "event_type":"*", "data":{"level":"test"}}`
	body := serialize(bodyStr, ownerID)

	url := fmt.Sprintf("/api/v1/projects/%s/events/fanout", s.DefaultProject.UID)
	req := createRequest(http.MethodPost, url, s.APIKey, body)
	w := httptest.NewRecorder()
	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)

	// Deep Assert.
	var event datastore.Event
	parseResponse(s.T(), w.Result(), &event)

	require.NotEmpty(s.T(), event.UID)
	require.Equal(s.T(), event.Endpoints[0], endpointID)
	require.Equal(s.T(), 2, len(event.Endpoints))
}

func (s *PublicEventIntegrationTestSuite) Test_CreateEndpointEvent_With_App_ID_Valid_Event() {
	endpointID := ulid.Make().String()
	appID := ulid.Make().String()
	expectedStatusCode := http.StatusCreated

	// Just Before.
	// Create an Endpoint with an app ID
	endpoint := &datastore.Endpoint{
		UID:       endpointID,
		Title:     fmt.Sprintf("TestEndpoint-%s", endpointID),
		ProjectID: s.DefaultProject.UID,
		AppID:     appID,
		Secrets: datastore.Secrets{
			{UID: ulid.Make().String()},
		},
		Status: datastore.ActiveEndpointStatus,
	}

	err := postgres.NewEndpointRepo(s.ConvoyApp.A.DB, nil).CreateEndpoint(context.TODO(), endpoint, s.DefaultProject.UID)
	require.NoError(s.T(), err)

	bodyStr := `{"app_id":"%s", "event_type":"*", "data":{"level":"test"}}`
	body := serialize(bodyStr, appID)

	url := fmt.Sprintf("/api/v1/projects/%s/events", s.DefaultProject.UID)
	req := createRequest(http.MethodPost, url, s.APIKey, body)
	w := httptest.NewRecorder()
	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)

	//// Deep Assert.
	//var event datastore.Event
	//parseResponse(s.T(), w.Result(), &event)

	// require.NotEmpty(s.T(), event.UID)
	// require.Equal(s.T(), event.Endpoints[0], endpointID)
}

func (s *PublicEventIntegrationTestSuite) Test_CreateEndpointEvent_Endpoint_is_disabled() {
	endpointID := ulid.Make().String()
	expectedStatusCode := http.StatusCreated

	// Just Before.
	_, _ = testdb.SeedEndpoint(s.ConvoyApp.A.DB, s.DefaultProject, endpointID, "", "", true, datastore.ActiveEndpointStatus)

	bodyStr := `{"endpoint_id": "%s", "event_type":"*", "data":{"level":"test"}}`
	body := serialize(bodyStr, endpointID)

	url := fmt.Sprintf("/api/v1/projects/%s/events", s.DefaultProject.UID)
	req := createRequest(http.MethodPost, url, s.APIKey, body)
	w := httptest.NewRecorder()
	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)
}

func (s *PublicEventIntegrationTestSuite) Test_GetEndpointEvent_Valid_Event() {
	eventID := ulid.Make().String()
	expectedStatusCode := http.StatusOK

	// Just Before.
	endpoint, err := testdb.SeedEndpoint(s.ConvoyApp.A.DB, s.DefaultProject, ulid.Make().String(), "", "", false, datastore.ActiveEndpointStatus)
	require.NoError(s.T(), err)

	event, err := testdb.SeedEvent(s.ConvoyApp.A.DB, endpoint, s.DefaultProject.UID, eventID, "*", "", []byte(`{}`))
	require.NoError(s.T(), err)

	url := fmt.Sprintf("/api/v1/projects/%s/events/%s", s.DefaultProject.UID, eventID)
	req := createRequest(http.MethodGet, url, s.APIKey, nil)
	w := httptest.NewRecorder()

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)

	// Deep Assert.
	var respEvent datastore.Event
	parseResponse(s.T(), w.Result(), &respEvent)
	require.Equal(s.T(), event.UID, respEvent.UID)
}

func (s *PublicEventIntegrationTestSuite) Test_CreateEndpointEvent_Valid_Event_RedirectToProjects() {
	s.T().Skip("Deprecated Redirects")
	//	endpointID := ulid.Make().String()
	//	expectedStatusCode := http.StatusTemporaryRedirect
	//
	//	// Just Before.
	//	_, _ = testdb.SeedEndpoint(s.ConvoyApp.A.DB, s.DefaultProject, endpointID, "", "", false, datastore.ActiveEndpointStatus)
	//
	//	bodyStr := `{"app_id":"%s", "event_type":"*", "data":{"level":"test"}}`
	//	body := serialize(bodyStr, endpointID)
	//
	//	url := fmt.Sprintf("/api/v1/events?projectID=%s", s.DefaultProject.UID)
	//	req := createRequest(http.MethodPost, url, s.APIKey, body)
	//	w := httptest.NewRecorder()
	//	// Act.
	//	s.Router.ServeHTTP(w, req)
	//
	//	// Assert.
	//	require.Equal(s.T(), expectedStatusCode, w.Code)
}

func (s *PublicEventIntegrationTestSuite) Test_ReplayEndpointEvent_Valid_Event() {
	eventID := ulid.Make().String()
	expectedStatusCode := http.StatusOK

	// Just Before.
	endpoint, _ := testdb.SeedEndpoint(s.ConvoyApp.A.DB, s.DefaultProject, ulid.Make().String(), "", "", false, datastore.ActiveEndpointStatus)
	_, _ = testdb.SeedEvent(s.ConvoyApp.A.DB, endpoint, s.DefaultProject.UID, eventID, "*", "", []byte(`{}`))

	url := fmt.Sprintf("/api/v1/projects/%s/events/%s/replay", s.DefaultProject.UID, eventID)
	req := createRequest(http.MethodPut, url, s.APIKey, nil)
	w := httptest.NewRecorder()

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)
}

func (s *PublicEventIntegrationTestSuite) Test_GetEndpointEvent_Event_not_found() {
	eventID := ulid.Make().String()
	expectedStatusCode := http.StatusNotFound

	url := fmt.Sprintf("/api/v1/projects/%s/events/%s", s.DefaultProject.UID, eventID)
	req := createRequest(http.MethodGet, url, s.APIKey, nil)
	w := httptest.NewRecorder()

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)
}

func (s *PublicEventIntegrationTestSuite) Test_GetEventDelivery_Valid_EventDelivery() {
	eventDeliveryID := ulid.Make().String()
	expectedStatusCode := http.StatusOK

	// Just Before.
	endpoint, _ := testdb.SeedEndpoint(s.ConvoyApp.A.DB, s.DefaultProject, ulid.Make().String(), "", "", false, datastore.ActiveEndpointStatus)
	event, _ := testdb.SeedEvent(s.ConvoyApp.A.DB, endpoint, s.DefaultProject.UID, ulid.Make().String(), "*", "", []byte(`{}`))
	subscription, err := testdb.SeedSubscription(s.ConvoyApp.A.DB, s.DefaultProject, ulid.Make().String(), datastore.OutgoingProject, &datastore.Source{}, endpoint, &datastore.RetryConfiguration{}, &datastore.AlertConfiguration{}, &datastore.FilterConfiguration{
		EventTypes: []string{"*"},
		Filter:     datastore.FilterSchema{Headers: datastore.M{}, Body: datastore.M{}},
	})
	require.NoError(s.T(), err)

	eventDelivery, err := testdb.SeedEventDelivery(s.ConvoyApp.A.DB, event, endpoint, s.DefaultProject.UID, eventDeliveryID, datastore.FailureEventStatus, subscription)
	require.NoError(s.T(), err)

	url := fmt.Sprintf("/api/v1/projects/%s/eventdeliveries/%s", s.DefaultProject.UID, eventDeliveryID)
	req := createRequest(http.MethodGet, url, s.APIKey, nil)
	w := httptest.NewRecorder()

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)

	// Deep Assert.
	var respEventDelivery datastore.EventDelivery
	parseResponse(s.T(), w.Result(), &respEventDelivery)
	require.Equal(s.T(), eventDelivery.UID, respEventDelivery.UID)
}

func (s *PublicEventIntegrationTestSuite) Test_GetEventDelivery_Valid_EventDelivery_RedirectToProjects() {
	s.T().Skip("Deprecated Redirects")
	//	eventDeliveryID := ulid.Make().String()
	//	expectedStatusCode := http.StatusTemporaryRedirect
	//
	//	// Just Before.
	//	endpoint, _ := testdb.SeedEndpoint(s.ConvoyApp.A.DB, s.DefaultProject, ulid.Make().String(), "", "", false, datastore.ActiveEndpointStatus)
	//	_, _ = testdb.SeedEventDelivery(s.ConvoyApp.A.DB, &datastore.Event{}, endpoint, s.DefaultProject.UID, eventDeliveryID, datastore.SuccessEventStatus, &datastore.Subscription{})
	//
	//	url := fmt.Sprintf("/api/v1/eventdeliveries/%s?groupID=%s", eventDeliveryID, s.DefaultProject.UID)
	//	req := createRequest(http.MethodGet, url, s.APIKey, nil)
	//	w := httptest.NewRecorder()
	//
	//	// Act.
	//	s.Router.ServeHTTP(w, req)
	//
	//	// Assert.
	//	require.Equal(s.T(), expectedStatusCode, w.Code)
}

func (s *PublicEventIntegrationTestSuite) Test_GetEventDelivery_Event_not_found() {
	eventDeliveryID := ulid.Make().String()
	expectedStatusCode := http.StatusNotFound

	url := fmt.Sprintf("/api/v1/projects/%s/eventdeliveries/%s", s.DefaultProject.UID, eventDeliveryID)
	req := createRequest(http.MethodGet, url, s.APIKey, nil)
	w := httptest.NewRecorder()

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)
}

func (s *PublicEventIntegrationTestSuite) Test_ResendEventDelivery_Valid_Resend() {
	eventDeliveryID := ulid.Make().String()
	expectedStatusCode := http.StatusOK

	// Just Before.
	endpoint, err := testdb.SeedEndpoint(s.ConvoyApp.A.DB, s.DefaultProject, ulid.Make().String(), "", "", false, datastore.ActiveEndpointStatus)
	require.NoError(s.T(), err)

	event, err := testdb.SeedEvent(s.ConvoyApp.A.DB, endpoint, s.DefaultProject.UID, ulid.Make().String(), "*", "", []byte(`{}`))
	require.NoError(s.T(), err)

	subscription, err := testdb.SeedSubscription(s.ConvoyApp.A.DB, s.DefaultProject, ulid.Make().String(), datastore.OutgoingProject, &datastore.Source{}, endpoint, &datastore.RetryConfiguration{}, &datastore.AlertConfiguration{}, &datastore.FilterConfiguration{
		EventTypes: []string{"*"},
		Filter:     datastore.FilterSchema{Headers: datastore.M{}, Body: datastore.M{}},
	})
	require.NoError(s.T(), err)

	eventDelivery, err := testdb.SeedEventDelivery(s.ConvoyApp.A.DB, event, endpoint, s.DefaultProject.UID, eventDeliveryID, datastore.FailureEventStatus, subscription)
	require.NoError(s.T(), err)

	url := fmt.Sprintf("/api/v1/projects/%s/eventdeliveries/%s/resend", s.DefaultProject.UID, eventDeliveryID)
	req := createRequest(http.MethodPut, url, s.APIKey, nil)
	w := httptest.NewRecorder()

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)

	// Deep Assert.
	var respEventDelivery datastore.EventDelivery
	parseResponse(s.T(), w.Result(), &respEventDelivery)
	require.Equal(s.T(), datastore.ScheduledEventStatus, respEventDelivery.Status)
	require.Equal(s.T(), eventDelivery.UID, respEventDelivery.UID)
}

func (s *PublicEventIntegrationTestSuite) Test_BatchRetryEventDelivery_Valid_EventDeliveries() {
	expectedStatusCode := http.StatusOK

	// Just Before.
	endpoint, _ := testdb.SeedEndpoint(s.ConvoyApp.A.DB, s.DefaultProject, ulid.Make().String(), "", "", false, datastore.ActiveEndpointStatus)
	event, _ := testdb.SeedEvent(s.ConvoyApp.A.DB, endpoint, s.DefaultProject.UID, ulid.Make().String(), "*", "", []byte(`{}`))
	subscription, err := testdb.SeedSubscription(s.ConvoyApp.A.DB, s.DefaultProject, ulid.Make().String(), datastore.OutgoingProject, &datastore.Source{}, endpoint, &datastore.RetryConfiguration{}, &datastore.AlertConfiguration{}, &datastore.FilterConfiguration{
		EventTypes: []string{"*"},
		Filter:     datastore.FilterSchema{Headers: datastore.M{}, Body: datastore.M{}},
	})
	require.NoError(s.T(), err)

	_, err = testdb.SeedEventDelivery(s.ConvoyApp.A.DB, event, endpoint, s.DefaultProject.UID, ulid.Make().String(), datastore.FailureEventStatus, subscription)
	require.NoError(s.T(), err)

	_, err = testdb.SeedEventDelivery(s.ConvoyApp.A.DB, event, endpoint, s.DefaultProject.UID, ulid.Make().String(), datastore.FailureEventStatus, subscription)
	require.NoError(s.T(), err)

	_, err = testdb.SeedEventDelivery(s.ConvoyApp.A.DB, event, endpoint, s.DefaultProject.UID, ulid.Make().String(), datastore.FailureEventStatus, subscription)
	require.NoError(s.T(), err)

	url := fmt.Sprintf("/api/v1/projects/%s/eventdeliveries/batchretry?endpointId=%s&eventId=%s&status=%s", s.DefaultProject.UID, endpoint.UID, event.UID, datastore.FailureEventStatus)
	req := createRequest(http.MethodPost, url, s.APIKey, nil)
	w := httptest.NewRecorder()

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)
}

func (s *PublicEventIntegrationTestSuite) Test_ForceResendEventDeliveries_Valid_EventDeliveries() {
	expectedStatusCode := http.StatusOK
	expectedMessage := "3 successful, 0 failed"

	// Just Before.
	endpoint, _ := testdb.SeedEndpoint(s.ConvoyApp.A.DB, s.DefaultProject, ulid.Make().String(), "", "", false, datastore.ActiveEndpointStatus)
	event, _ := testdb.SeedEvent(s.ConvoyApp.A.DB, endpoint, s.DefaultProject.UID, ulid.Make().String(), "*", "", []byte(`{}`))
	subscription, err := testdb.SeedSubscription(s.ConvoyApp.A.DB, s.DefaultProject, ulid.Make().String(), datastore.OutgoingProject, &datastore.Source{}, endpoint, &datastore.RetryConfiguration{}, &datastore.AlertConfiguration{}, &datastore.FilterConfiguration{
		EventTypes: []string{"*"},
		Filter:     datastore.FilterSchema{Headers: datastore.M{}, Body: datastore.M{}},
	})
	require.NoError(s.T(), err)

	e1, err := testdb.SeedEventDelivery(s.ConvoyApp.A.DB, event, endpoint, s.DefaultProject.UID, ulid.Make().String(), datastore.SuccessEventStatus, subscription)
	require.NoError(s.T(), err)

	e2, _ := testdb.SeedEventDelivery(s.ConvoyApp.A.DB, event, endpoint, s.DefaultProject.UID, ulid.Make().String(), datastore.SuccessEventStatus, subscription)
	e3, _ := testdb.SeedEventDelivery(s.ConvoyApp.A.DB, event, endpoint, s.DefaultProject.UID, ulid.Make().String(), datastore.SuccessEventStatus, subscription)

	url := fmt.Sprintf("/api/v1/projects/%s/eventdeliveries/forceresend", s.DefaultProject.UID)

	bodyStr := `{"ids":["%s", "%s", "%s"]}`
	body := serialize(bodyStr, e1.UID, e2.UID, e3.UID)

	req := createRequest(http.MethodPost, url, s.APIKey, body)
	w := httptest.NewRecorder()

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)

	var response map[string]interface{}
	err = json.NewDecoder(w.Body).Decode(&response)
	require.NoError(s.T(), err)

	require.Equal(s.T(), expectedMessage, response["message"].(string))
}

func (s *PublicEventIntegrationTestSuite) Test_GetEventsPaged() {
	eventID := ulid.Make().String()
	sourceID := ulid.Make().String()
	expectedStatusCode := http.StatusOK

	// Just Before.
	endpoint1, err := testdb.SeedEndpoint(s.ConvoyApp.A.DB, s.DefaultProject, ulid.Make().String(), "", "", false, datastore.ActiveEndpointStatus)
	require.NoError(s.T(), err)

	vc := &datastore.VerifierConfig{
		Type: datastore.BasicAuthVerifier,
		BasicAuth: &datastore.BasicAuth{
			UserName: "Convoy",
			Password: "Convoy",
		},
	}

	_, err = testdb.SeedSource(s.ConvoyApp.A.DB, s.DefaultProject, sourceID, ulid.Make().String(), "", vc, "", "")
	require.NoError(s.T(), err)

	e1, err := testdb.SeedEvent(s.ConvoyApp.A.DB, endpoint1, s.DefaultProject.UID, eventID, "*", sourceID, []byte(`{}`))
	require.NoError(s.T(), err)

	e2, err := testdb.SeedEvent(s.ConvoyApp.A.DB, endpoint1, s.DefaultProject.UID, ulid.Make().String(), "*", sourceID, []byte(`{}`))
	require.NoError(s.T(), err)

	endpoint2, err := testdb.SeedEndpoint(s.ConvoyApp.A.DB, s.DefaultProject, ulid.Make().String(), "", "", false, datastore.ActiveEndpointStatus)
	require.NoError(s.T(), err)

	_, err = testdb.SeedEvent(s.ConvoyApp.A.DB, endpoint2, s.DefaultProject.UID, ulid.Make().String(), "*", sourceID, []byte(`{}`))
	require.NoError(s.T(), err)

	url := fmt.Sprintf("/api/v1/projects/%s/events?endpointId=%s&sourceId=%s", s.DefaultProject.UID, endpoint1.UID, sourceID)
	req := createRequest(http.MethodGet, url, s.APIKey, nil)
	w := httptest.NewRecorder()

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)

	// Deep Assert.
	var respEvents []datastore.Event
	resp := pagedResponse{Content: &respEvents}
	parseResponse(s.T(), w.Result(), &resp)
	require.Equal(s.T(), 2, len(respEvents))

	v := []string{e1.UID, e2.UID}
	for i := range respEvents {
		require.Contains(s.T(), v, respEvents[i].UID)
	}
}

func (s *PublicEventIntegrationTestSuite) Test_GetEventDeliveriesPaged() {
	eventDeliveryID := ulid.Make().String()
	expectedStatusCode := http.StatusOK

	// Just Before.
	endpoint1, err := testdb.SeedEndpoint(s.ConvoyApp.A.DB, s.DefaultProject, ulid.Make().String(), "", "", false, datastore.ActiveEndpointStatus)
	require.NoError(s.T(), err)

	subscription, err := testdb.SeedSubscription(s.ConvoyApp.A.DB, s.DefaultProject, ulid.Make().String(), datastore.OutgoingProject, &datastore.Source{}, endpoint1, &datastore.RetryConfiguration{}, &datastore.AlertConfiguration{}, &datastore.FilterConfiguration{
		EventTypes: []string{"*"},
		Filter:     datastore.FilterSchema{Headers: datastore.M{}, Body: datastore.M{}},
	})
	require.NoError(s.T(), err)

	subscription2, err := testdb.SeedSubscription(s.ConvoyApp.A.DB, s.DefaultProject, ulid.Make().String(), datastore.OutgoingProject, &datastore.Source{}, endpoint1, &datastore.RetryConfiguration{}, &datastore.AlertConfiguration{}, &datastore.FilterConfiguration{
		EventTypes: []string{"*"},
		Filter:     datastore.FilterSchema{Headers: datastore.M{}, Body: datastore.M{}},
	})
	require.NoError(s.T(), err)

	event1, err := testdb.SeedEvent(s.ConvoyApp.A.DB, endpoint1, s.DefaultProject.UID, ulid.Make().String(), "*", "", []byte(`{}`))
	require.NoError(s.T(), err)

	d1, err := testdb.SeedEventDelivery(s.ConvoyApp.A.DB, event1, endpoint1, s.DefaultProject.UID, eventDeliveryID, datastore.FailureEventStatus, subscription)
	require.NoError(s.T(), err)

	d2, err := testdb.SeedEventDelivery(s.ConvoyApp.A.DB, event1, endpoint1, s.DefaultProject.UID, ulid.Make().String(), datastore.FailureEventStatus, subscription)
	require.NoError(s.T(), err)

	_, err = testdb.SeedEventDelivery(s.ConvoyApp.A.DB, event1, endpoint1, s.DefaultProject.UID, ulid.Make().String(), datastore.FailureEventStatus, subscription2)
	require.NoError(s.T(), err)

	_, err = testdb.SeedEventDelivery(s.ConvoyApp.A.DB, event1, endpoint1, s.DefaultProject.UID, ulid.Make().String(), datastore.FailureEventStatus, subscription2)
	require.NoError(s.T(), err)

	endpoint2, err := testdb.SeedEndpoint(s.ConvoyApp.A.DB, s.DefaultProject, ulid.Make().String(), "", "", false, datastore.ActiveEndpointStatus)
	require.NoError(s.T(), err)

	event2, err := testdb.SeedEvent(s.ConvoyApp.A.DB, endpoint2, s.DefaultProject.UID, ulid.Make().String(), "*", "", []byte(`{}`))
	require.NoError(s.T(), err)

	_, err = testdb.SeedEventDelivery(s.ConvoyApp.A.DB, event2, endpoint2, s.DefaultProject.UID, ulid.Make().String(), datastore.FailureEventStatus, subscription)
	require.NoError(s.T(), err)

	url := fmt.Sprintf("/api/v1/projects/%s/eventdeliveries?endpointId=%s&subscriptionId=%s", s.DefaultProject.UID, endpoint1.UID, subscription.UID)
	req := createRequest(http.MethodGet, url, s.APIKey, nil)
	w := httptest.NewRecorder()

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)

	// Deep Assert.
	var respEvents []datastore.EventDelivery
	resp := pagedResponse{Content: &respEvents}
	parseResponse(s.T(), w.Result(), &resp)
	require.Equal(s.T(), 2, len(respEvents))

	v := []*datastore.EventDelivery{d2, d1}
	for i, delivery := range v {
		require.Equal(s.T(), respEvents[i].UID, delivery.UID)
	}
}

func TestPublicEventIntegrationTestSuite(t *testing.T) {
	suite.Run(t, new(PublicEventIntegrationTestSuite))
}

type PublicPortalLinkIntegrationTestSuite struct {
	suite.Suite
	DB             database.Database
	Router         http.Handler
	ConvoyApp      *ApplicationHandler
	DefaultOrg     *datastore.Organisation
	DefaultProject *datastore.Project
	DefaultUser    *datastore.User
	APIKey         string
	PersonalAPIKey string
}

func (s *PublicPortalLinkIntegrationTestSuite) SetupSuite() {
	s.DB = getDB()
	s.ConvoyApp = buildServer()
	s.Router = s.ConvoyApp.BuildRoutes()
}

func (s *PublicPortalLinkIntegrationTestSuite) SetupTest() {
	testdb.PurgeDB(s.T(), s.DB)

	user, err := testdb.SeedDefaultUser(s.ConvoyApp.A.DB)
	require.NoError(s.T(), err)
	s.DefaultUser = user

	org, err := testdb.SeedDefaultOrganisation(s.ConvoyApp.A.DB, user)
	require.NoError(s.T(), err)

	// Setup Default Project.
	s.DefaultProject, err = testdb.SeedDefaultProject(s.ConvoyApp.A.DB, org.UID)
	require.NoError(s.T(), err)

	// Seed Auth
	role := auth.Role{
		Type:    auth.RoleAdmin,
		Project: s.DefaultProject.UID,
	}

	_, s.APIKey, _ = testdb.SeedAPIKey(s.ConvoyApp.A.DB, role, "", "test", "", "")

	// Setup Config.
	err = config.LoadConfig("./testdata/Auth_Config/full-convoy.json")
	require.NoError(s.T(), err)

	apiRepo := postgres.NewAPIKeyRepo(s.ConvoyApp.A.DB, nil)
	userRepo := postgres.NewUserRepo(s.ConvoyApp.A.DB, nil)
	portalLinkRepo := postgres.NewPortalLinkRepo(s.ConvoyApp.A.DB, s.ConvoyApp.A.Cache)
	initRealmChain(s.T(), apiRepo, userRepo, portalLinkRepo, s.ConvoyApp.A.Cache)
}

func (s *PublicPortalLinkIntegrationTestSuite) TearDownTest() {
	testdb.PurgeDB(s.T(), s.DB)
	metrics.Reset()
}

func (s *PublicPortalLinkIntegrationTestSuite) Test_CreatePortalLink() {
	endpoint1, _ := testdb.SeedEndpoint(s.ConvoyApp.A.DB, s.DefaultProject, "", ulid.Make().String(), "", false, datastore.ActiveEndpointStatus)
	endpoint2, _ := testdb.SeedEndpoint(s.ConvoyApp.A.DB, s.DefaultProject, "", ulid.Make().String(), "", false, datastore.ActiveEndpointStatus)
	expectedStatusCode := http.StatusCreated

	// Arrange Request
	url := fmt.Sprintf("/api/v1/projects/%s/portal-links", s.DefaultProject.UID)
	plainBody := fmt.Sprintf(`{
		"name": "test_portal_link",
		"endpoints": ["%s", "%s"]
	}`, endpoint1.UID, endpoint2.UID)
	body := strings.NewReader(plainBody)
	req := createRequest(http.MethodPost, url, s.APIKey, body)
	w := httptest.NewRecorder()

	// Act
	s.Router.ServeHTTP(w, req)

	// Assert
	require.Equal(s.T(), expectedStatusCode, w.Code)

	// Deep Assert.
	var resp models.PortalLinkResponse
	parseResponse(s.T(), w.Result(), &resp)

	portalLinkRepo := postgres.NewPortalLinkRepo(s.ConvoyApp.A.DB, nil)
	pl, err := portalLinkRepo.FindPortalLinkByID(context.Background(), resp.ProjectID, resp.UID)
	require.NoError(s.T(), err)

	require.Equal(s.T(), resp.UID, pl.UID)
	require.Equal(s.T(), resp.URL, fmt.Sprintf("https://app.convoy.io/portal?token=%s", pl.Token))
	require.Equal(s.T(), resp.Name, pl.Name)
	require.Equal(s.T(), resp.Endpoints, []string(pl.Endpoints))
}

func (s *PublicPortalLinkIntegrationTestSuite) Test_GetPortalLinkByID_PortalLinkNotFound() {
	portalLinkID := "123"

	// Arrange Request
	url := fmt.Sprintf("/api/v1/projects/%s/portal-links/%s", s.DefaultProject.UID, portalLinkID)
	req := createRequest(http.MethodGet, url, s.APIKey, nil)
	w := httptest.NewRecorder()

	// Act
	s.Router.ServeHTTP(w, req)

	// Assert
	require.Equal(s.T(), http.StatusNotFound, w.Code)
}

func (s *PublicPortalLinkIntegrationTestSuite) Test_GetPortalLinkByID_ValidPortalLink() {
	// Just Before
	endpoint, _ := testdb.SeedEndpoint(s.ConvoyApp.A.DB, s.DefaultProject, "", ulid.Make().String(), "", false, datastore.ActiveEndpointStatus)
	portalLink, _ := testdb.SeedPortalLink(s.ConvoyApp.A.DB, s.DefaultProject, []string{endpoint.UID})

	// Arrange Request
	url := fmt.Sprintf("/api/v1/projects/%s/portal-links/%s", s.DefaultProject.UID, portalLink.UID)
	req := createRequest(http.MethodGet, url, s.APIKey, nil)
	w := httptest.NewRecorder()

	// Act
	s.Router.ServeHTTP(w, req)

	// Assert
	require.Equal(s.T(), http.StatusOK, w.Code)

	// Deep Assert
	var resp models.PortalLinkResponse
	parseResponse(s.T(), w.Result(), &resp)

	portalLinkRepo := postgres.NewPortalLinkRepo(s.ConvoyApp.A.DB, nil)
	pl, err := portalLinkRepo.FindPortalLinkByID(context.Background(), resp.ProjectID, resp.UID)

	require.NoError(s.T(), err)

	require.Equal(s.T(), resp.UID, pl.UID)
	require.Equal(s.T(), resp.URL, fmt.Sprintf("https://app.convoy.io/portal?token=%s", pl.Token))
	require.Equal(s.T(), resp.Name, pl.Name)
	require.Equal(s.T(), resp.Endpoints, []string(pl.Endpoints))
	require.Equal(s.T(), 1, resp.EndpointCount)
}

func (s *PublicPortalLinkIntegrationTestSuite) Test_GetPortalLinks_ValidPortalLinks() {
	r := rand.New(rand.NewSource(time.Now().Unix()))
	totalLinks := r.Intn(5)

	// Just Before
	for i := 0; i < totalLinks; i++ {
		endpoint, err := testdb.SeedEndpoint(s.ConvoyApp.A.DB, s.DefaultProject, ulid.Make().String(), ulid.Make().String(), "", false, datastore.ActiveEndpointStatus)
		require.NoError(s.T(), err)

		_, err = testdb.SeedPortalLink(s.ConvoyApp.A.DB, s.DefaultProject, []string{endpoint.UID})
		require.NoError(s.T(), err)
	}

	// Arrange Request
	url := fmt.Sprintf("/api/v1/projects/%s/portal-links", s.DefaultProject.UID)
	req := createRequest(http.MethodGet, url, s.APIKey, nil)
	w := httptest.NewRecorder()

	// Act
	s.Router.ServeHTTP(w, req)

	// Assert
	require.Equal(s.T(), http.StatusOK, w.Code)

	// Deep Assert
	var resp pagedResponse
	parseResponse(s.T(), w.Result(), &resp)
	require.Equal(s.T(), totalLinks, len(resp.Content.([]interface{})))
}

func (s *PublicPortalLinkIntegrationTestSuite) Test_GetPortalLinks_ValidPortalLinks_FilterByEndpointID() {
	r := rand.New(rand.NewSource(time.Now().Unix()))
	totalLinks := r.Intn(5)

	// Just Before
	for i := 0; i < totalLinks; i++ {
		endpoint, err := testdb.SeedEndpoint(s.ConvoyApp.A.DB, s.DefaultProject, ulid.Make().String(), ulid.Make().String(), "", false, datastore.ActiveEndpointStatus)
		require.NoError(s.T(), err)

		_, err = testdb.SeedPortalLink(s.ConvoyApp.A.DB, s.DefaultProject, []string{endpoint.UID})
		require.NoError(s.T(), err)
	}

	endpoint, _ := testdb.SeedEndpoint(s.ConvoyApp.A.DB, s.DefaultProject, "", ulid.Make().String(), "", false, datastore.ActiveEndpointStatus)
	_, _ = testdb.SeedPortalLink(s.ConvoyApp.A.DB, s.DefaultProject, []string{endpoint.UID})

	// Arrange Request
	url := fmt.Sprintf("/api/v1/projects/%s/portal-links?endpointId=%s", s.DefaultProject.UID, endpoint.UID)
	req := createRequest(http.MethodGet, url, s.APIKey, nil)
	w := httptest.NewRecorder()

	// Act
	s.Router.ServeHTTP(w, req)

	// Assert
	require.Equal(s.T(), http.StatusOK, w.Code)

	// Deep Assert
	var resp pagedResponse
	parseResponse(s.T(), w.Result(), &resp)
	require.Equal(s.T(), 1, len(resp.Content.([]interface{})))
}

func (s *PublicPortalLinkIntegrationTestSuite) Test_UpdatePortalLinks() {
	// Just Before
	endpoint1, _ := testdb.SeedEndpoint(s.ConvoyApp.A.DB, s.DefaultProject, "", ulid.Make().String(), "", false, datastore.ActiveEndpointStatus)
	endpoint2, _ := testdb.SeedEndpoint(s.ConvoyApp.A.DB, s.DefaultProject, "", ulid.Make().String(), "", false, datastore.ActiveEndpointStatus)
	portalLink, _ := testdb.SeedPortalLink(s.ConvoyApp.A.DB, s.DefaultProject, []string{endpoint1.UID})

	url := fmt.Sprintf("/api/v1/projects/%s/portal-links/%s", s.DefaultProject.UID, portalLink.UID)
	bodyStr := fmt.Sprintf(`{
		    "name": "test_portal_link",
			"endpoints": ["%s"]
		}`, endpoint2.UID)

	body := serialize(bodyStr)
	req := createRequest(http.MethodPut, url, s.APIKey, body)
	w := httptest.NewRecorder()

	// Act
	s.Router.ServeHTTP(w, req)

	// Assert
	require.Equal(s.T(), http.StatusAccepted, w.Code)

	// Deep Assert
	var resp models.PortalLinkResponse
	parseResponse(s.T(), w.Result(), &resp)

	portalLinkRepo := postgres.NewPortalLinkRepo(s.ConvoyApp.A.DB, nil)
	pl, err := portalLinkRepo.FindPortalLinkByID(context.Background(), resp.ProjectID, resp.UID)

	require.NoError(s.T(), err)

	require.Equal(s.T(), resp.UID, pl.UID)
	require.Equal(s.T(), resp.URL, fmt.Sprintf("https://app.convoy.io/portal?token=%s", pl.Token))
	require.Equal(s.T(), resp.Name, pl.Name)
	require.Equal(s.T(), resp.Endpoints, []string(pl.Endpoints))
}

func (s *PublicPortalLinkIntegrationTestSuite) Test_RevokePortalLink() {
	// Just Before
	endpoint1, _ := testdb.SeedEndpoint(s.ConvoyApp.A.DB, s.DefaultProject, "", ulid.Make().String(), "", false, datastore.ActiveEndpointStatus)
	portalLink, _ := testdb.SeedPortalLink(s.ConvoyApp.A.DB, s.DefaultProject, []string{endpoint1.UID})

	// Arrange Request.
	url := fmt.Sprintf("/api/v1/projects/%s/portal-links/%s/revoke", s.DefaultProject.UID, portalLink.UID)
	req := createRequest(http.MethodPut, url, s.APIKey, nil)
	w := httptest.NewRecorder()

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), http.StatusOK, w.Code)

	// Deep Assert.
	plRepo := postgres.NewPortalLinkRepo(s.ConvoyApp.A.DB, nil)
	_, err := plRepo.FindPortalLinkByID(context.Background(), s.DefaultProject.UID, portalLink.UID)
	require.ErrorIs(s.T(), err, datastore.ErrPortalLinkNotFound)
}

func TestPublicPortalLinkIntegrationTestSuite(t *testing.T) {
	suite.Run(t, new(PublicPortalLinkIntegrationTestSuite))
}

type PublicProjectIntegrationTestSuite struct {
	suite.Suite
	DB              database.Database
	Router          http.Handler
	ConvoyApp       *ApplicationHandler
	AuthenticatorFn AuthenticatorFn
	DefaultOrg      *datastore.Organisation
	DefaultProject  *datastore.Project
	DefaultUser     *datastore.User
}

func (s *PublicProjectIntegrationTestSuite) SetupSuite() {
	s.DB = getDB()
	s.ConvoyApp = buildServer()
	s.Router = s.ConvoyApp.BuildRoutes()
}

func (s *PublicProjectIntegrationTestSuite) SetupTest() {
	testdb.PurgeDB(s.T(), s.DB)

	user, err := testdb.SeedDefaultUser(s.ConvoyApp.A.DB)
	require.NoError(s.T(), err)
	s.DefaultUser = user

	org, err := testdb.SeedDefaultOrganisation(s.ConvoyApp.A.DB, user)
	require.NoError(s.T(), err)
	s.DefaultOrg = org

	// Setup Default Project.
	s.DefaultProject, err = testdb.SeedDefaultProject(s.ConvoyApp.A.DB, s.DefaultOrg.UID)
	require.NoError(s.T(), err)

	s.AuthenticatorFn = authenticateRequest(&models.LoginUser{
		Username: user.Email,
		Password: testdb.DefaultUserPassword,
	})

	// Setup Config.
	err = config.LoadConfig("./testdata/Auth_Config/full-convoy-with-all-realms.json")
	require.NoError(s.T(), err)

	apiRepo := postgres.NewAPIKeyRepo(s.ConvoyApp.A.DB, nil)
	userRepo := postgres.NewUserRepo(s.ConvoyApp.A.DB, nil)
	portalLinkRepo := postgres.NewPortalLinkRepo(s.ConvoyApp.A.DB, s.ConvoyApp.A.Cache)
	initRealmChain(s.T(), apiRepo, userRepo, portalLinkRepo, s.ConvoyApp.A.Cache)
}

func (s *PublicProjectIntegrationTestSuite) TestGetProjectWithPersonalAPIKey() {
	expectedStatusCode := http.StatusOK

	_, key, err := testdb.SeedAPIKey(s.ConvoyApp.A.DB, auth.Role{}, ulid.Make().String(), "test", string(datastore.PersonalKey), s.DefaultUser.UID)
	require.NoError(s.T(), err)

	url := fmt.Sprintf("/api/v1/projects/%s", s.DefaultProject.UID)
	req := createRequest(http.MethodGet, url, key, nil)

	w := httptest.NewRecorder()

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)

	var respProject datastore.Project
	parseResponse(s.T(), w.Result(), &respProject)

	require.Equal(s.T(), s.DefaultProject.UID, respProject.UID)
	require.Equal(s.T(), s.DefaultProject.Name, respProject.Name)
}

func (s *PublicProjectIntegrationTestSuite) TestGetProjectWithPersonalAPIKey_UnauthorizedRole() {
	expectedStatusCode := http.StatusBadRequest

	user, err := testdb.SeedUser(s.ConvoyApp.A.DB, "test@gmail.com", testdb.DefaultUserPassword)
	require.NoError(s.T(), err)

	_, key, err := testdb.SeedAPIKey(s.ConvoyApp.A.DB, auth.Role{}, ulid.Make().String(), "test", string(datastore.PersonalKey), user.UID)
	require.NoError(s.T(), err)

	orgID := s.DefaultProject.OrganisationID
	url := fmt.Sprintf("/api/v1/projects/%s?orgID=%s", s.DefaultProject.UID, orgID)
	req := createRequest(http.MethodGet, url, key, nil)

	w := httptest.NewRecorder()

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)
}

func (s *PublicProjectIntegrationTestSuite) TestDeleteProjectWithPersonalAPIKey() {
	expectedStatusCode := http.StatusOK
	projectID := ulid.Make().String()

	// Just Before.
	project, err := testdb.SeedProject(s.ConvoyApp.A.DB, projectID, "test", s.DefaultOrg.UID, datastore.OutgoingProject, &datastore.DefaultProjectConfig)
	require.NoError(s.T(), err)

	_, key, err := testdb.SeedAPIKey(s.ConvoyApp.A.DB, auth.Role{}, ulid.Make().String(), "test", string(datastore.PersonalKey), s.DefaultUser.UID)
	require.NoError(s.T(), err)

	url := fmt.Sprintf("/api/v1/projects/%s", project.UID)
	req := createRequest(http.MethodDelete, url, key, nil)

	w := httptest.NewRecorder()

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)

	projectRepo := postgres.NewProjectRepo(s.ConvoyApp.A.DB, nil)
	_, err = projectRepo.FetchProjectByID(context.Background(), projectID)
	require.Equal(s.T(), datastore.ErrProjectNotFound, err)
}

func (s *PublicProjectIntegrationTestSuite) TestDeleteProjectWithPersonalAPIKey_UnauthorizedRole() {
	expectedStatusCode := http.StatusBadRequest

	user, err := testdb.SeedUser(s.ConvoyApp.A.DB, "test@gmail.com", testdb.DefaultUserPassword)
	require.NoError(s.T(), err)

	_, key, err := testdb.SeedAPIKey(s.ConvoyApp.A.DB, auth.Role{}, ulid.Make().String(), "test", string(datastore.PersonalKey), user.UID)
	require.NoError(s.T(), err)

	orgID := s.DefaultProject.OrganisationID
	url := fmt.Sprintf("/api/v1/projects/%s?orgID=%s", s.DefaultProject.UID, orgID)
	req := createRequest(http.MethodDelete, url, key, nil)

	w := httptest.NewRecorder()

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)
}

func (s *PublicProjectIntegrationTestSuite) TestCreateProjectWithPersonalAPIKey() {
	expectedStatusCode := http.StatusCreated

	bodyStr := `{
    "name": "test-project",
	"type": "outgoing",
    "logo_url": "",
    "config": {
        "strategy": {
            "type": "linear",
            "duration": 10,
            "retry_count": 2
        },
        "signature": {
            "header": "X-Convoy-Signature",
            "hash": "SHA512"
        },
        "disable_endpoint": false,
        "replay_attacks": false,
        "ratelimit": {
            "count": 8000,
            "duration": 60
        }
    }
}`

	_, key, err := testdb.SeedAPIKey(s.ConvoyApp.A.DB, auth.Role{}, ulid.Make().String(), "test", string(datastore.PersonalKey), s.DefaultUser.UID)
	require.NoError(s.T(), err)

	url := fmt.Sprintf("/api/v1/projects?orgID=%s", s.DefaultOrg.UID)
	body := serialize(bodyStr)

	req := createRequest(http.MethodPost, url, key, body)

	w := httptest.NewRecorder()

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)

	var respProject models.CreateProjectResponse
	parseResponse(s.T(), w.Result(), &respProject)
	require.NotEmpty(s.T(), respProject.Project.UID)
	require.Equal(s.T(), 8000, respProject.Project.Config.RateLimit.Count)
	require.Equal(s.T(), uint64(60), respProject.Project.Config.RateLimit.Duration)
	require.Equal(s.T(), "test-project", respProject.Project.Name)
	require.Equal(s.T(), "test-project's default key", respProject.APIKey.Name)

	require.Equal(s.T(), auth.RoleAdmin, respProject.APIKey.Role.Type)
	require.Equal(s.T(), respProject.Project.UID, respProject.APIKey.Role.Project)
	require.Equal(s.T(), "test-project's default key", respProject.APIKey.Name)
	require.NotEmpty(s.T(), respProject.APIKey.Key)
}

func (s *PublicProjectIntegrationTestSuite) TestCreateProjectWithPersonalAPIKey_UnauthorizedRole() {
	expectedStatusCode := http.StatusForbidden

	bodyStr := `{
        "name": "test-project",
        "type": "outgoing",
        "logo_url": "",
        "config": {
            "strategy": {
                "type": "linear",
                "duration": 10,
                "retry_count": 2
            },
            "signature": {
                "header": "X-Convoy-Signature",
                "hash": "SHA512"
            },
            "disable_endpoint": false,
            "replay_attacks": false,
            "ratelimit": {
                "count": 8000,
                "duration": 60
            }
        }
    }`

	user, err := testdb.SeedUser(s.ConvoyApp.A.DB, "test@gmail.com", testdb.DefaultUserPassword)
	require.NoError(s.T(), err)

	_, key, err := testdb.SeedAPIKey(s.ConvoyApp.A.DB, auth.Role{}, ulid.Make().String(), "test", string(datastore.PersonalKey), user.UID)
	require.NoError(s.T(), err)

	url := fmt.Sprintf("/api/v1/projects?orgID=%s", s.DefaultOrg.UID)
	body := serialize(bodyStr)
	req := createRequest(http.MethodPost, url, key, body)

	w := httptest.NewRecorder()

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)
}

func (s *PublicProjectIntegrationTestSuite) TestUpdateProjectWithPersonalAPIKey() {
	expectedStatusCode := http.StatusAccepted
	projectID := ulid.Make().String()

	// Just Before.
	project, err := testdb.SeedProject(s.ConvoyApp.A.DB, projectID, "test", s.DefaultOrg.UID, datastore.OutgoingProject, &datastore.DefaultProjectConfig)
	require.NoError(s.T(), err)

	_, key, err := testdb.SeedAPIKey(s.ConvoyApp.A.DB, auth.Role{}, ulid.Make().String(), "test", string(datastore.PersonalKey), s.DefaultUser.UID)
	require.NoError(s.T(), err)

	body := serialize(`{"name":"update_project"}`)
	url := fmt.Sprintf("/api/v1/projects/%s", project.UID)
	req := createRequest(http.MethodPut, url, key, body)

	w := httptest.NewRecorder()

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)

	var respProject datastore.Project
	parseResponse(s.T(), w.Result(), &respProject)

	require.Equal(s.T(), projectID, respProject.UID)
	require.Equal(s.T(), "update_project", respProject.Name)
}

func (s *PublicProjectIntegrationTestSuite) TestUpdateProjectWithPersonalAPIKey_UnauthorizedRole() {
	expectedStatusCode := http.StatusBadRequest

	user, err := testdb.SeedUser(s.ConvoyApp.A.DB, "test@gmail.com", testdb.DefaultUserPassword)
	require.NoError(s.T(), err)

	_, key, err := testdb.SeedAPIKey(s.ConvoyApp.A.DB, auth.Role{}, ulid.Make().String(), "test", string(datastore.PersonalKey), user.UID)
	require.NoError(s.T(), err)

	url := fmt.Sprintf("/api/v1/projects/%s", s.DefaultProject.UID)
	body := serialize(`{"name": "updatedproject"}`)
	req := createRequest(http.MethodPut, url, key, body)

	w := httptest.NewRecorder()

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)
}

func (s *PublicProjectIntegrationTestSuite) TestGetProjectsWithPersonalAPIKey() {
	expectedStatusCode := http.StatusOK

	// Just Before.
	project1, err := testdb.SeedProject(s.ConvoyApp.A.DB, ulid.Make().String(), "vve", s.DefaultOrg.UID, datastore.OutgoingProject, &datastore.DefaultProjectConfig)
	require.NoError(s.T(), err)

	project2, err := testdb.SeedProject(s.ConvoyApp.A.DB, ulid.Make().String(), "bbv", s.DefaultOrg.UID, datastore.OutgoingProject, &datastore.DefaultProjectConfig)
	require.NoError(s.T(), err)

	_, key, err := testdb.SeedAPIKey(s.ConvoyApp.A.DB, auth.Role{}, ulid.Make().String(), "test", string(datastore.PersonalKey), s.DefaultUser.UID)
	require.NoError(s.T(), err)

	url := fmt.Sprintf("/api/v1/projects?orgID=%s", s.DefaultOrg.UID)
	req := createRequest(http.MethodGet, url, key, nil)

	w := httptest.NewRecorder()

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)

	var projects []*datastore.Project
	parseResponse(s.T(), w.Result(), &projects)
	require.Equal(s.T(), 3, len(projects))

	v := []string{projects[0].UID, projects[1].UID, projects[2].UID}
	require.Contains(s.T(), v, project1.UID)
	require.Contains(s.T(), v, project2.UID)
	require.Contains(s.T(), v, s.DefaultProject.UID)
}

func (s *PublicProjectIntegrationTestSuite) TearDownTest() {
	testdb.PurgeDB(s.T(), s.DB)
	metrics.Reset()
}

func TestPublicProjectIntegrationTestSuite(t *testing.T) {
	suite.Run(t, new(PublicProjectIntegrationTestSuite))
}

type PublicSourceIntegrationTestSuite struct {
	suite.Suite
	DB             database.Database
	Router         http.Handler
	ConvoyApp      *ApplicationHandler
	DefaultOrg     *datastore.Organisation
	DefaultProject *datastore.Project
	DefaultUser    *datastore.User
	APIKey         string
}

func (s *PublicSourceIntegrationTestSuite) SetupSuite() {
	s.DB = getDB()
	s.ConvoyApp = buildServer()
	s.Router = s.ConvoyApp.BuildRoutes()
}

func (s *PublicSourceIntegrationTestSuite) SetupTest() {
	testdb.PurgeDB(s.T(), s.DB)

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
		Type:    auth.RoleAdmin,
		Project: s.DefaultProject.UID,
	}

	_, s.APIKey, _ = testdb.SeedAPIKey(s.ConvoyApp.A.DB, role, "", "test", "", "")

	// Setup Config.
	err = config.LoadConfig("./testdata/Auth_Config/full-convoy.json")
	require.NoError(s.T(), err)

	apiRepo := postgres.NewAPIKeyRepo(s.ConvoyApp.A.DB, nil)
	userRepo := postgres.NewUserRepo(s.ConvoyApp.A.DB, nil)
	// orgRepo := postgres.NewOrgRepo(s.ConvoyApp.A.DB, nil)
	portalLinkRepo := postgres.NewPortalLinkRepo(s.ConvoyApp.A.DB, s.ConvoyApp.A.Cache)
	initRealmChain(s.T(), apiRepo, userRepo, portalLinkRepo, s.ConvoyApp.A.Cache)
}

func (s *PublicSourceIntegrationTestSuite) TearDownTest() {
	testdb.PurgeDB(s.T(), s.DB)
	metrics.Reset()
}

func (s *PublicSourceIntegrationTestSuite) Test_GetSourceByID_SourceNotFound() {
	sourceID := "123"

	// Arrange Request
	url := fmt.Sprintf("/api/v1/projects/%s/sources/%s", s.DefaultProject.UID, sourceID)
	req := createRequest(http.MethodGet, url, s.APIKey, nil)
	w := httptest.NewRecorder()

	// Act
	s.Router.ServeHTTP(w, req)

	// Assert
	require.Equal(s.T(), http.StatusNotFound, w.Code)
}

func (s *PublicSourceIntegrationTestSuite) Test_GetSourceBy_ValidSource() {
	sourceID := "123456789"

	// Just Before
	_, _ = testdb.SeedSource(s.ConvoyApp.A.DB, s.DefaultProject, sourceID, "", "", nil, "", "")

	// Arrange Request
	url := fmt.Sprintf("/api/v1/projects/%s/sources/%s", s.DefaultProject.UID, sourceID)
	req := createRequest(http.MethodGet, url, s.APIKey, nil)
	w := httptest.NewRecorder()

	// Act
	s.Router.ServeHTTP(w, req)

	// Assert
	require.Equal(s.T(), http.StatusOK, w.Code)

	// Deep Assert
	var source datastore.Source
	parseResponse(s.T(), w.Result(), &source)

	sourceRepo := postgres.NewSourceRepo(s.ConvoyApp.A.DB, nil)
	dbSource, err := sourceRepo.FindSourceByID(context.Background(), s.DefaultProject.UID, sourceID)
	require.NoError(s.T(), err)
	require.Equal(s.T(), source.UID, dbSource.UID)
	require.Equal(s.T(), source.Name, dbSource.Name)
	require.Equal(s.T(), source.MaskID, dbSource.MaskID)
}

func (s *PublicSourceIntegrationTestSuite) Test_GetSource_ValidSources() {
	r := rand.New(rand.NewSource(time.Now().Unix()))
	totalSources := r.Intn(5)

	// Just Before
	for i := 0; i < totalSources; i++ {
		_, err := testdb.SeedSource(s.ConvoyApp.A.DB, s.DefaultProject, "", "", "", nil, "", "")
		require.NoError(s.T(), err)
	}

	// Arrange Request
	url := fmt.Sprintf("/api/v1/projects/%s/sources", s.DefaultProject.UID)
	req := createRequest(http.MethodGet, url, s.APIKey, nil)
	w := httptest.NewRecorder()

	// Act
	s.Router.ServeHTTP(w, req)

	// Assert
	require.Equal(s.T(), http.StatusOK, w.Code)

	// Deep Assert
	var resp pagedResponse
	parseResponse(s.T(), w.Result(), &resp)
	require.Equal(s.T(), totalSources, len(resp.Content.([]interface{})))
}

func (s *PublicSourceIntegrationTestSuite) Test_CreateSource() {
	bodyStr := `{
		"name": "convoy-prod",
		"type": "http",
		"is_disabled": false,
        "custom_response": {
            "body": "[accepted]",
            "content_type": "text/plain"
        },
		"verifier": {
			"type": "hmac",
			"hmac": {
				"encoding": "base64",
				"header": "X-Convoy-Header",
				"hash": "SHA512",
				"secret": "convoy-secret"
			}
		}
	}`

	url := fmt.Sprintf("/api/v1/projects/%s/sources", s.DefaultProject.UID)
	body := serialize(bodyStr)
	req := createRequest(http.MethodPost, url, s.APIKey, body)
	w := httptest.NewRecorder()

	// Act
	s.Router.ServeHTTP(w, req)

	// Assert
	require.Equal(s.T(), http.StatusCreated, w.Code)

	var source datastore.Source
	parseResponse(s.T(), w.Result(), &source)

	require.NotEmpty(s.T(), source.UID)
	require.Equal(s.T(), "convoy-prod", source.Name)
	require.Equal(s.T(), datastore.SourceType("http"), source.Type)
	require.Equal(s.T(), datastore.VerifierType("hmac"), source.Verifier.Type)
	require.Equal(s.T(), "[accepted]", source.CustomResponse.Body)
	require.Equal(s.T(), "text/plain", source.CustomResponse.ContentType)
}

func (s *PublicSourceIntegrationTestSuite) Test_CreateSource_RedirectToProjects() {
	s.T().Skip("Deprecated Redirects")
	//	bodyStr := `{
	//		"name": "convoy-prod",
	//		"type": "http",
	//		"is_disabled": false,
	//		"verifier": {
	//			"type": "hmac",
	//			"hmac": {
	//				"encoding": "base64",
	//				"header": "X-Convoy-Header",
	//				"hash": "SHA512",
	//				"secret": "convoy-secret"
	//			}
	//		}
	//	}`
	//
	//	url := fmt.Sprintf("/api/v1/sources?groupID=%s", s.DefaultProject.UID)
	//	body := serialize(bodyStr)
	//	req := createRequest(http.MethodPost, url, s.APIKey, body)
	//	w := httptest.NewRecorder()
	//
	//	// Act
	//	s.Router.ServeHTTP(w, req)
	//
	//	// Assert
	//	require.Equal(s.T(), http.StatusTemporaryRedirect, w.Code)
}

func (s *PublicSourceIntegrationTestSuite) Test_CreateSource_NoName() {
	bodyStr := `{
		"type": "http",
		"is_disabled": false,
		"verifier": {
			"type": "hmac",
			"hmac": {
				"encoding": "base64",
				"header": "X-Convoy-Header",
				"hash": "SHA512",
				"secret": "convoy-secret"
			}
		}
	}`

	url := fmt.Sprintf("/api/v1/projects/%s/sources", s.DefaultProject.UID)
	body := serialize(bodyStr)
	req := createRequest(http.MethodPost, url, s.APIKey, body)
	w := httptest.NewRecorder()

	// Act
	s.Router.ServeHTTP(w, req)

	// Assert
	require.Equal(s.T(), http.StatusBadRequest, w.Code)
}

func (s *PublicSourceIntegrationTestSuite) Test_CreateSource_InvalidSourceType() {
	bodyStr := `{
		"name": "convoy-prod",
		"type": "some-random-source-type",
		"is_disabled": false,
		"verifier": {
			"type": "hmac",
			"hmac": {
				"encoding": "base64",
				"header": "X-Convoy-Header",
				"hash": "SHA512",
				"secret": "convoy-secret"
			}
		}
	}`

	url := fmt.Sprintf("/api/v1/projects/%s/sources", s.DefaultProject.UID)

	body := serialize(bodyStr)
	req := createRequest(http.MethodPost, url, s.APIKey, body)
	w := httptest.NewRecorder()

	// Act
	s.Router.ServeHTTP(w, req)

	// Assert
	require.Equal(s.T(), http.StatusBadRequest, w.Code)
}

func (s *PublicSourceIntegrationTestSuite) Test_UpdateSource() {
	name := "updated-convoy-prod"
	isDisabled := randBool()
	sourceID := ulid.Make().String()

	// Just Before
	_, _ = testdb.SeedSource(s.ConvoyApp.A.DB, s.DefaultProject, sourceID, "", "", nil, `{name:"daniel"}`, "application/json")

	// Arrange Request
	url := fmt.Sprintf("/api/v1/projects/%s/sources/%s", s.DefaultProject.UID, sourceID)
	bodyStr := fmt.Sprintf(`{
		"name": "%s",
		"type": "http",
		"is_disabled": %t,
        "custom_response": {
            "body": "[tee]",
            "content_type": "text/plain"
        },
		"verifier": {
			"type": "hmac",
			"hmac": {
				"encoding": "hex",
				"header": "X-Convoy-Header",
				"hash": "SHA512",
				"secret": "convoy-secret"
			}
		}
	}`, name, !isDisabled)

	body := serialize(bodyStr)
	req := createRequest(http.MethodPut, url, s.APIKey, body)
	w := httptest.NewRecorder()

	// Act
	s.Router.ServeHTTP(w, req)

	// Assert
	require.Equal(s.T(), http.StatusAccepted, w.Code)

	// Deep Asset
	var source datastore.Source
	parseResponse(s.T(), w.Result(), &source)

	sourceRepo := postgres.NewSourceRepo(s.ConvoyApp.A.DB, nil)
	dbSource, err := sourceRepo.FindSourceByID(context.Background(), s.DefaultProject.UID, sourceID)
	require.NoError(s.T(), err)
	require.Equal(s.T(), source.UID, dbSource.UID)
	require.Equal(s.T(), name, dbSource.Name)
	require.Equal(s.T(), !isDisabled, dbSource.IsDisabled)
	require.Equal(s.T(), "[tee]", source.CustomResponse.Body)
	require.Equal(s.T(), "text/plain", source.CustomResponse.ContentType)
}

func (s *PublicSourceIntegrationTestSuite) Test_DeleteSource() {
	sourceID := ulid.Make().String()

	// Just Before.
	_, _ = testdb.SeedSource(s.ConvoyApp.A.DB, s.DefaultProject, sourceID, "", "", nil, "", "")

	// Arrange Request.
	url := fmt.Sprintf("/api/v1/projects/%s/sources/%s", s.DefaultProject.UID, sourceID)
	req := createRequest(http.MethodDelete, url, s.APIKey, nil)
	w := httptest.NewRecorder()

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), http.StatusOK, w.Code)

	// Deep Assert.
	sourceRepo := postgres.NewSourceRepo(s.ConvoyApp.A.DB, nil)
	_, err := sourceRepo.FindSourceByID(context.Background(), s.DefaultProject.UID, sourceID)
	require.ErrorIs(s.T(), err, datastore.ErrSourceNotFound)
}

func TestPublicSourceIntegrationTestSuite(t *testing.T) {
	suite.Run(t, new(PublicSourceIntegrationTestSuite))
}

type PublicSubscriptionIntegrationTestSuite struct {
	suite.Suite
	DB             database.Database
	Router         http.Handler
	ConvoyApp      *ApplicationHandler
	DefaultOrg     *datastore.Organisation
	DefaultProject *datastore.Project
	APIKey         string
}

func (s *PublicSubscriptionIntegrationTestSuite) SetupSuite() {
	s.DB = getDB()
	s.ConvoyApp = buildServer()
	s.Router = s.ConvoyApp.BuildRoutes()
}

func (s *PublicSubscriptionIntegrationTestSuite) SetupTest() {
	testdb.PurgeDB(s.T(), s.DB)

	user, err := testdb.SeedDefaultUser(s.ConvoyApp.A.DB)
	require.NoError(s.T(), err)

	org, err := testdb.SeedDefaultOrganisation(s.ConvoyApp.A.DB, user)
	require.NoError(s.T(), err)
	s.DefaultOrg = org

	// Setup Default Project.
	s.DefaultProject, err = testdb.SeedDefaultProject(s.ConvoyApp.A.DB, org.UID)
	require.NoError(s.T(), err)

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

	apiRepo := postgres.NewAPIKeyRepo(s.ConvoyApp.A.DB, nil)
	userRepo := postgres.NewUserRepo(s.ConvoyApp.A.DB, nil)

	portalLinkRepo := postgres.NewPortalLinkRepo(s.ConvoyApp.A.DB, s.ConvoyApp.A.Cache)
	initRealmChain(s.T(), apiRepo, userRepo, portalLinkRepo, s.ConvoyApp.A.Cache)
}

func (s *PublicSubscriptionIntegrationTestSuite) TearDownTest() {
	testdb.PurgeDB(s.T(), s.DB)
	metrics.Reset()
}

func (s *PublicSubscriptionIntegrationTestSuite) Test_CreateSubscription() {
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
	require.Equal(s.T(), http.StatusCreated, w.Code)

	var subscription *datastore.Subscription
	parseResponse(s.T(), w.Result(), &subscription)

	subRepo := postgres.NewSubscriptionRepo(s.ConvoyApp.A.DB, nil)
	dbSub, err := subRepo.FindSubscriptionByID(context.Background(), s.DefaultProject.UID, subscription.UID)
	require.NoError(s.T(), err)

	require.NotEmpty(s.T(), subscription.UID)
	require.Equal(s.T(), dbSub.Name, subscription.Name)
	require.Equal(s.T(), len(dbSub.FilterConfig.EventTypes), len(subscription.FilterConfig.EventTypes))
	require.Equal(s.T(), dbSub.RateLimitConfig.Count, subscription.RateLimitConfig.Count)
}

func (s *PublicSubscriptionIntegrationTestSuite) Test_CreateSubscription_IncomingProject() {
	project, err := testdb.SeedProject(s.ConvoyApp.A.DB, ulid.Make().String(), "test_project", s.DefaultOrg.UID, datastore.IncomingProject, &datastore.DefaultProjectConfig)
	require.NoError(s.T(), err)

	// Seed Auth
	role := auth.Role{
		Type:    auth.RoleAdmin,
		Project: project.UID,
	}

	_, apiKey, _ := testdb.SeedAPIKey(s.ConvoyApp.A.DB, role, "", "test", "", "")

	endpoint, _ := testdb.SeedEndpoint(s.ConvoyApp.A.DB, project, ulid.Make().String(), "", "", false, datastore.ActiveEndpointStatus)
	source, _ := testdb.SeedSource(s.ConvoyApp.A.DB, project, ulid.Make().String(), "", "", nil, "", "")
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

	subRepo := postgres.NewSubscriptionRepo(s.ConvoyApp.A.DB, nil)
	dbSub, err := subRepo.FindSubscriptionByID(context.Background(), project.UID, subscription.UID)
	require.NoError(s.T(), err)

	require.NotEmpty(s.T(), subscription.UID)
	require.Equal(s.T(), dbSub.Name, subscription.Name)
	require.Equal(s.T(), len(dbSub.FilterConfig.EventTypes), len(subscription.FilterConfig.EventTypes))
	require.Equal(s.T(), dbSub.RateLimitConfig.Count, subscription.RateLimitConfig.Count)
}

func (s *PublicSubscriptionIntegrationTestSuite) Test_CreateSubscription_AppNotFound() {
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

func (s *PublicSubscriptionIntegrationTestSuite) Test_CreateSubscription_EndpointNotFound() {
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

func (s *PublicSubscriptionIntegrationTestSuite) Test_CreateSubscription_InvalidBody() {
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

func (s *PublicSubscriptionIntegrationTestSuite) Test_GetOneSubscription_SubscriptionNotFound() {
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

func (s *PublicSubscriptionIntegrationTestSuite) Test_GetOneSubscription_OutgoingProject_ValidSubscription() {
	subscriptionId := ulid.Make().String()

	project := s.DefaultProject

	// Just Before
	endpoint, err := testdb.SeedEndpoint(s.ConvoyApp.A.DB, project, ulid.Make().String(), "", "", false, datastore.ActiveEndpointStatus)
	require.NoError(s.T(), err)

	source, err := testdb.SeedSource(s.ConvoyApp.A.DB, project, ulid.Make().String(), "", "", nil, "", "")
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

	subRepo := postgres.NewSubscriptionRepo(s.ConvoyApp.A.DB, nil)
	dbSub, err := subRepo.FindSubscriptionByID(context.Background(), project.UID, subscriptionId)
	require.NoError(s.T(), err)
	require.Equal(s.T(), subscription.UID, dbSub.UID)
	require.Equal(s.T(), subscription.Endpoint.UID, dbSub.EndpointID)
}

func (s *PublicSubscriptionIntegrationTestSuite) Test_GetOneSubscription_IncomingProject_ValidSubscription() {
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

	source, err := testdb.SeedSource(s.ConvoyApp.A.DB, project, ulid.Make().String(), "", "", nil, "", "")
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

	subRepo := postgres.NewSubscriptionRepo(s.ConvoyApp.A.DB, nil)
	dbSub, err := subRepo.FindSubscriptionByID(context.Background(), project.UID, subscriptionId)
	require.NoError(s.T(), err)
	require.Equal(s.T(), subscription.UID, dbSub.UID)
	require.Equal(s.T(), subscription.Source.UID, dbSub.SourceID)
	require.Equal(s.T(), subscription.Endpoint.UID, dbSub.EndpointID)
}

func (s *PublicSubscriptionIntegrationTestSuite) Test_GetSubscriptions_ValidSubscriptions() {
	r := rand.New(rand.NewSource(time.Now().Unix()))
	totalSubs := r.Intn(10)

	for i := 0; i < totalSubs; i++ {
		// Just Before
		endpoint, err := testdb.SeedEndpoint(s.ConvoyApp.A.DB, s.DefaultProject, ulid.Make().String(), "", "", false, datastore.ActiveEndpointStatus)
		require.NoError(s.T(), err)
		source, err := testdb.SeedSource(s.ConvoyApp.A.DB, s.DefaultProject, ulid.Make().String(), "", "", nil, "", "")
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
	require.Equal(s.T(), totalSubs, len(resp.Content.([]interface{})))
}

func (s *PublicSubscriptionIntegrationTestSuite) Test_DeleteSubscription() {
	subscriptionId := ulid.Make().String()

	// Just Before
	endpoint, err := testdb.SeedEndpoint(s.ConvoyApp.A.DB, s.DefaultProject, ulid.Make().String(), "", "", false, datastore.ActiveEndpointStatus)
	require.NoError(s.T(), err)

	source, err := testdb.SeedSource(s.ConvoyApp.A.DB, s.DefaultProject, ulid.Make().String(), "", "", nil, "", "")
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
	subRepo := postgres.NewSubscriptionRepo(s.ConvoyApp.A.DB, nil)
	_, err = subRepo.FindSubscriptionByID(context.Background(), s.DefaultProject.UID, subscriptionId)
	require.ErrorIs(s.T(), err, datastore.ErrSubscriptionNotFound)
}

func (s *PublicSubscriptionIntegrationTestSuite) Test_UpdateSubscription() {
	subscriptionId := ulid.Make().String()

	// Just Before
	endpoint, err := testdb.SeedEndpoint(s.ConvoyApp.A.DB, s.DefaultProject, ulid.Make().String(), "", "", false, datastore.ActiveEndpointStatus)
	require.NoError(s.T(), err)

	source, err := testdb.SeedSource(s.ConvoyApp.A.DB, s.DefaultProject, ulid.Make().String(), "", "", nil, "", "")
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

	subRepo := postgres.NewSubscriptionRepo(s.ConvoyApp.A.DB, nil)
	dbSub, err := subRepo.FindSubscriptionByID(context.Background(), s.DefaultProject.UID, subscriptionId)
	require.NoError(s.T(), err)
	require.Equal(s.T(), 2, len(dbSub.FilterConfig.EventTypes))
	require.Equal(s.T(), "1h", dbSub.AlertConfig.Threshold)
	require.Equal(s.T(), subscription.RetryConfig.Duration, dbSub.RetryConfig.Duration)
}

func TestPublicSubscriptionIntegrationTestSuite(t *testing.T) {
	suite.Run(t, new(PublicSubscriptionIntegrationTestSuite))
}

type PublicMetaEventIntegrationTestSuite struct {
	suite.Suite
	DB             database.Database
	Router         http.Handler
	ConvoyApp      *ApplicationHandler
	DefaultOrg     *datastore.Organisation
	DefaultProject *datastore.Project
	APIKey         string
}

func (s *PublicMetaEventIntegrationTestSuite) SetupSuite() {
	s.DB = getDB()
	s.ConvoyApp = buildServer()
	s.Router = s.ConvoyApp.BuildRoutes()
}

func (s *PublicMetaEventIntegrationTestSuite) SetupTest() {
	testdb.PurgeDB(s.T(), s.DB)

	user, err := testdb.SeedDefaultUser(s.ConvoyApp.A.DB)
	require.NoError(s.T(), err)

	org, err := testdb.SeedDefaultOrganisation(s.ConvoyApp.A.DB, user)
	require.NoError(s.T(), err)
	s.DefaultOrg = org

	// Setup Default Project.
	s.DefaultProject, err = testdb.SeedDefaultProject(s.ConvoyApp.A.DB, org.UID)
	require.NoError(s.T(), err)

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

	apiRepo := postgres.NewAPIKeyRepo(s.ConvoyApp.A.DB, nil)
	userRepo := postgres.NewUserRepo(s.ConvoyApp.A.DB, nil)

	portalLinkRepo := postgres.NewPortalLinkRepo(s.ConvoyApp.A.DB, s.ConvoyApp.A.Cache)
	initRealmChain(s.T(), apiRepo, userRepo, portalLinkRepo, s.ConvoyApp.A.Cache)
}

func (s *PublicMetaEventIntegrationTestSuite) TearDownTest() {
	testdb.PurgeDB(s.T(), s.DB)
	metrics.Reset()
}

func (s *PublicMetaEventIntegrationTestSuite) Test_GetMetaEventsPaged() {
	expectedStatusCode := http.StatusOK

	// Just Before.
	m1, err := testdb.SeedMetaEvent(s.ConvoyApp.A.DB, s.DefaultProject)
	require.NoError(s.T(), err)

	m2, err := testdb.SeedMetaEvent(s.ConvoyApp.A.DB, s.DefaultProject)
	require.NoError(s.T(), err)

	url := fmt.Sprintf("/api/v1/projects/%s/meta-events", s.DefaultProject.UID)
	req := createRequest(http.MethodGet, url, s.APIKey, nil)
	w := httptest.NewRecorder()

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)

	// Deep Assert.
	var respEvents []datastore.MetaEvent
	resp := pagedResponse{Content: &respEvents}
	parseResponse(s.T(), w.Result(), &resp)
	require.Equal(s.T(), 2, len(respEvents))

	v := []string{m1.UID, m2.UID}
	for i := range respEvents {
		require.Contains(s.T(), v, respEvents[i].UID)
	}
}

func (s *PublicMetaEventIntegrationTestSuite) Test_GetMetaEvent_Valid_MetaEvent() {
	expectedStatusCode := http.StatusOK

	// Just Before.
	metaEvent, err := testdb.SeedMetaEvent(s.ConvoyApp.A.DB, s.DefaultProject)
	require.NoError(s.T(), err)

	url := fmt.Sprintf("/api/v1/projects/%s/meta-events/%s", s.DefaultProject.UID, metaEvent.UID)
	req := createRequest(http.MethodGet, url, s.APIKey, nil)
	w := httptest.NewRecorder()

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)

	// Deep Assert.
	var respMetaEvent datastore.MetaEvent
	parseResponse(s.T(), w.Result(), &respMetaEvent)
	require.Equal(s.T(), metaEvent.UID, respMetaEvent.UID)
}

func TestPublicMetaEventIntegrationTestSuite(t *testing.T) {
	suite.Run(t, new(PublicMetaEventIntegrationTestSuite))
}
