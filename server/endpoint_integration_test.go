//go:build integration
// +build integration

package server

import (
	"context"
	"fmt"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/frain-dev/convoy/internal/pkg/metrics"

	"github.com/frain-dev/convoy/auth"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/datastore"
	cm "github.com/frain-dev/convoy/datastore/mongo"
	"github.com/frain-dev/convoy/server/testdb"
	"github.com/google/uuid"
	"github.com/jaswdr/faker"
	"github.com/stretchr/testify/require"

	"github.com/stretchr/testify/suite"
)

type EndpointIntegrationTestSuite struct {
	suite.Suite
	DB             cm.Client
	Router         http.Handler
	ConvoyApp      *ApplicationHandler
	DefaultOrg     *datastore.Organisation
	DefaultGroup   *datastore.Group
	DefaultUser    *datastore.User
	APIKey         string
	PersonalAPIKey string
}

func (s *EndpointIntegrationTestSuite) SetupSuite() {
	s.DB = getDB()
	s.ConvoyApp = buildServer()
	s.Router = s.ConvoyApp.BuildRoutes()
}

func (s *EndpointIntegrationTestSuite) SetupTest() {
	testdb.PurgeDB(s.T(), s.DB)

	user, err := testdb.SeedDefaultUser(s.ConvoyApp.A.Store)
	require.NoError(s.T(), err)
	s.DefaultUser = user

	org, err := testdb.SeedDefaultOrganisation(s.ConvoyApp.A.Store, user)
	require.NoError(s.T(), err)
	s.DefaultOrg = org

	// Setup Default Group.
	s.DefaultGroup, _ = testdb.SeedDefaultGroup(s.ConvoyApp.A.Store, s.DefaultOrg.UID)

	// Seed Auth
	role := auth.Role{
		Type:  auth.RoleAdmin,
		Group: s.DefaultGroup.UID,
	}

	_, s.APIKey, _ = testdb.SeedAPIKey(s.ConvoyApp.A.Store, role, "", "test", "", "")

	_, s.PersonalAPIKey, _ = testdb.SeedAPIKey(s.ConvoyApp.A.Store, auth.Role{}, uuid.NewString(), "test-personal-key", string(datastore.PersonalKey), s.DefaultUser.UID)

	// Setup Config.
	err = config.LoadConfig("./testdata/Auth_Config/full-convoy.json")
	require.NoError(s.T(), err)

	apiRepo := cm.NewApiKeyRepo(s.ConvoyApp.A.Store)
	userRepo := cm.NewUserRepo(s.ConvoyApp.A.Store)
	initRealmChain(s.T(), apiRepo, userRepo, s.ConvoyApp.A.Cache)
}

func (s *EndpointIntegrationTestSuite) TearDownTest() {
	testdb.PurgeDB(s.T(), s.DB)
	metrics.Reset()
}

func (s *EndpointIntegrationTestSuite) Test_GetEndpoint_EndpointNotFound() {
	appID := "123"
	expectedStatusCode := http.StatusNotFound

	// Arrange Request.
	url := fmt.Sprintf("/api/v1/projects/%s/endpoints/%s", s.DefaultGroup.UID, appID)
	req := createRequest(http.MethodGet, url, s.APIKey, nil)
	w := httptest.NewRecorder()

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)
}

func (s *EndpointIntegrationTestSuite) Test_GetEndpoint_ValidEndpoint() {
	endpointID := "123456789"
	expectedStatusCode := http.StatusOK

	// Just Before.
	_, _ = testdb.SeedEndpoint(s.ConvoyApp.A.Store, s.DefaultGroup, endpointID, "", true)

	// Arrange Request.
	url := fmt.Sprintf("/api/v1/projects/%s/endpoints/%s", s.DefaultGroup.UID, endpointID)
	req := createRequest(http.MethodGet, url, s.APIKey, nil)
	w := httptest.NewRecorder()

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)

	// Deep Assert.
	var endpoint datastore.Endpoint
	parseResponse(s.T(), w.Result(), &endpoint)

	endpointRepo := cm.NewEndpointRepo(s.ConvoyApp.A.Store)
	dbEndpoint, err := endpointRepo.FindEndpointByID(context.Background(), endpointID)
	require.NoError(s.T(), err)
	require.Equal(s.T(), endpoint.UID, dbEndpoint.UID)
	require.Equal(s.T(), endpoint.Title, dbEndpoint.Title)
}

func (s *EndpointIntegrationTestSuite) Test_GetEndpoint_ValidEndpoint_WithPersonalAPIKey() {
	endpointID := "123456789"
	expectedStatusCode := http.StatusOK

	// Just Before.
	_, _ = testdb.SeedEndpoint(s.ConvoyApp.A.Store, s.DefaultGroup, endpointID, "", true)

	// Arrange Request.
	url := fmt.Sprintf("/api/v1/projects/%s/endpoints/%s", s.DefaultGroup.UID, endpointID)
	req := createRequest(http.MethodGet, url, s.PersonalAPIKey, nil)
	w := httptest.NewRecorder()

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)

	// Deep Assert.
	var endpoint datastore.Endpoint
	parseResponse(s.T(), w.Result(), &endpoint)

	endpointRepo := cm.NewEndpointRepo(s.ConvoyApp.A.Store)
	dbEndpoint, err := endpointRepo.FindEndpointByID(context.Background(), endpointID)
	require.NoError(s.T(), err)
	require.Equal(s.T(), endpoint.UID, dbEndpoint.UID)
	require.Equal(s.T(), endpoint.Title, dbEndpoint.Title)
}

func (s *EndpointIntegrationTestSuite) Test_GetEndpoints_ValidEndpoints() {
	rand.Seed(time.Now().UnixNano())
	totalEndpoints := rand.Intn(5)
	expectedStatusCode := http.StatusOK

	// Just Before.
	_ = testdb.SeedMultipleEndpoints(s.ConvoyApp.A.Store, s.DefaultGroup, totalEndpoints)

	// Arrange.
	url := fmt.Sprintf("/api/v1/projects/%s/endpoints", s.DefaultGroup.UID)
	req := createRequest(http.MethodGet, url, s.APIKey, nil)
	w := httptest.NewRecorder()

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)

	// Deep Assert.
	var resp pagedResponse
	parseResponse(s.T(), w.Result(), &resp)
	require.Equal(s.T(), int64(totalEndpoints), resp.Pagination.Total)
}

func (s *EndpointIntegrationTestSuite) Test_GetEndpoints_ValidEndpoints_WithPersonalAPIKey() {
	rand.Seed(time.Now().UnixNano())
	totalEndpoints := rand.Intn(5)
	expectedStatusCode := http.StatusOK

	// Just Before.
	_ = testdb.SeedMultipleEndpoints(s.ConvoyApp.A.Store, s.DefaultGroup, totalEndpoints)

	// Arrange.
	url := fmt.Sprintf("/api/v1/projects/%s/endpoints", s.DefaultGroup.UID)
	req := createRequest(http.MethodGet, url, s.PersonalAPIKey, nil)
	w := httptest.NewRecorder()

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)

	// Deep Assert.
	var resp pagedResponse
	parseResponse(s.T(), w.Result(), &resp)
	require.Equal(s.T(), int64(totalEndpoints), resp.Pagination.Total)
}

func (s *EndpointIntegrationTestSuite) Test_GetEndpoints_Filters() {
	s.T().Skip("Depends on #637")
}

func (s *EndpointIntegrationTestSuite) Test_CreateEndpoint() {
	endpointTitle := fmt.Sprintf("Test-%s", uuid.New().String())
	endpointURL := faker.New().Internet().URL()
	expectedStatusCode := http.StatusCreated

	// Arrange Request.
	url := fmt.Sprintf("/api/v1/projects/%s/endpoints", s.DefaultGroup.UID)
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

	endpointRepo := cm.NewEndpointRepo(s.ConvoyApp.A.Store)
	dbEndpoint, err := endpointRepo.FindEndpointByID(context.Background(), endpoint.UID)
	require.NoError(s.T(), err)
	require.Equal(s.T(), endpointTitle, dbEndpoint.Title)
	require.Equal(s.T(), endpointURL, dbEndpoint.TargetURL)
}

func (s *EndpointIntegrationTestSuite) Test_CreateEndpointWithPersonalAPIKey() {
	endpointTitle := fmt.Sprintf("Test-%s", uuid.New().String())
	endpointURL := faker.New().Internet().URL()
	expectedStatusCode := http.StatusCreated

	// Arrange Request.
	url := fmt.Sprintf("/api/v1/projects/%s/endpoints", s.DefaultGroup.UID)
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

	endpointRepo := cm.NewEndpointRepo(s.ConvoyApp.A.Store)
	dbApp, err := endpointRepo.FindEndpointByID(context.Background(), endpoint.UID)
	require.NoError(s.T(), err)
	require.Equal(s.T(), endpointTitle, dbApp.Title)
	require.Equal(s.T(), endpointURL, dbApp.TargetURL)
}

func (s *EndpointIntegrationTestSuite) Test_CreateEndpoint_NoName() {
	endpointTitle := ""
	expectedStatusCode := http.StatusBadRequest

	// Arrange Request.
	url := fmt.Sprintf("/api/v1/projects/%s/endpoints", s.DefaultGroup.UID)
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

func (s *EndpointIntegrationTestSuite) Test_CreateEndpoint_NameNotUnique() {
	endpointTitle := uuid.New().String()
	endpointURL := faker.New().Internet().URL()
	expectedStatusCode := http.StatusBadRequest

	// Just Before.
	_, _ = testdb.SeedEndpoint(s.ConvoyApp.A.Store, s.DefaultGroup, "", endpointTitle, true)

	// Arrange Request.
	url := fmt.Sprintf("/api/v1/projects/%s/endpoints", s.DefaultGroup.UID)
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
}

func (s *EndpointIntegrationTestSuite) Test_UpdateEndpoint_InvalidRequest() {
	endpointID := uuid.New().String()
	expectedStatusCode := http.StatusBadRequest

	// Just Before.
	_, _ = testdb.SeedEndpoint(s.ConvoyApp.A.Store, s.DefaultGroup, endpointID, "", true)

	// Arrange Request.
	url := fmt.Sprintf("/api/v1/projects/%s/endpoints/%s", s.DefaultGroup.UID, endpointID)
	plainBody := ""
	body := strings.NewReader(plainBody)
	req := createRequest(http.MethodPut, url, s.APIKey, body)
	w := httptest.NewRecorder()

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)
}

func (s *EndpointIntegrationTestSuite) Test_UpdateEndpoint_DuplicateNames() {
	endpointID := uuid.New().String()
	endpointTitle := "appTitle"
	endpointURL := faker.New().Internet().URL()
	expectedStatusCode := http.StatusBadRequest

	// Just Before.
	_, _ = testdb.SeedEndpoint(s.ConvoyApp.A.Store, s.DefaultGroup, "", endpointTitle, false)
	_, _ = testdb.SeedEndpoint(s.ConvoyApp.A.Store, s.DefaultGroup, endpointID, "", false)

	// Arrange Request.
	url := fmt.Sprintf("/api/v1/projects/%s/endpoints/%s", s.DefaultGroup.UID, endpointID)
	plainBody := fmt.Sprintf(`{
		"name": "%s",
		"description": "test endpoint",
		"url": "%s",
		"support_email": "%s"
	}`, endpointTitle, endpointURL, "10xengineer@getconvoy.io")
	body := strings.NewReader(plainBody)
	req := createRequest(http.MethodPut, url, s.APIKey, body)
	w := httptest.NewRecorder()

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)
}

func (s *EndpointIntegrationTestSuite) Test_UpdateEndpoint() {
	title := "random-name"
	endpointURL := faker.New().Internet().URL()
	secret := faker.New().Lorem().Text(25)
	supportEmail := "10xengineer@getconvoy.io"
	isDisabled := randBool()
	endpointID := uuid.New().String()
	expectedStatusCode := http.StatusAccepted

	// Just Before.
	_, _ = testdb.SeedEndpoint(s.ConvoyApp.A.Store, s.DefaultGroup, endpointID, "", isDisabled)

	// Arrange Request.
	url := fmt.Sprintf("/api/v1/projects/%s/endpoints/%s", s.DefaultGroup.UID, endpointID)
	plainBody := fmt.Sprintf(`{
		"name": "%s",
		"description": "test endpoint",
		"secret": "%s",
		"url": "%s",
		"support_email": "%s",
		"is_disabled": %t
	}`, title, secret, endpointURL, supportEmail, !isDisabled)
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

	endpointRepo := cm.NewEndpointRepo(s.ConvoyApp.A.Store)
	dbEndpoint, err := endpointRepo.FindEndpointByID(context.Background(), endpointID)
	require.NoError(s.T(), err)
	require.Equal(s.T(), endpoint.UID, dbEndpoint.UID)
	require.Equal(s.T(), title, dbEndpoint.Title)
	require.Equal(s.T(), supportEmail, dbEndpoint.SupportEmail)
	require.Equal(s.T(), !isDisabled, dbEndpoint.IsDisabled)
	require.Equal(s.T(), secret, dbEndpoint.Secret)
	require.Equal(s.T(), endpointURL, dbEndpoint.TargetURL)
}

func (s *EndpointIntegrationTestSuite) Test_UpdateEndpoint_WithPersonalAPIKey() {
	title := "random-name"
	supportEmail := "10xengineer@getconvoy.io"
	isDisabled := randBool()
	endpointID := uuid.New().String()
	expectedStatusCode := http.StatusAccepted
	endpointURL := faker.New().Internet().URL()

	// Just Before.
	_, _ = testdb.SeedEndpoint(s.ConvoyApp.A.Store, s.DefaultGroup, endpointID, "", isDisabled)

	// Arrange Request.
	url := fmt.Sprintf("/api/v1/projects/%s/endpoints/%s", s.DefaultGroup.UID, endpointID)
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

	fmt.Println(w.Body.String())

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)

	// Deep Assert.
	var endpoint datastore.Endpoint
	parseResponse(s.T(), w.Result(), &endpoint)

	endpointRepo := cm.NewEndpointRepo(s.ConvoyApp.A.Store)
	dbEndpoint, err := endpointRepo.FindEndpointByID(context.Background(), endpointID)
	require.NoError(s.T(), err)
	require.Equal(s.T(), endpoint.UID, dbEndpoint.UID)
	require.Equal(s.T(), title, dbEndpoint.Title)
	require.Equal(s.T(), supportEmail, dbEndpoint.SupportEmail)
	require.Equal(s.T(), !isDisabled, dbEndpoint.IsDisabled)
	require.Equal(s.T(), endpointURL, dbEndpoint.TargetURL)
}

func (s *EndpointIntegrationTestSuite) Test_DeleteEndpoint() {
	endpointID := uuid.New().String()
	expectedStatusCode := http.StatusOK

	// Just Before.
	_, _ = testdb.SeedEndpoint(s.ConvoyApp.A.Store, s.DefaultGroup, endpointID, "", true)

	// Arrange Request.
	url := fmt.Sprintf("/api/v1/projects/%s/endpoints/%s", s.DefaultGroup.UID, endpointID)
	req := createRequest(http.MethodDelete, url, s.APIKey, nil)
	w := httptest.NewRecorder()

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)

	// Deep Assert.
	endpointRepo := cm.NewEndpointRepo(s.ConvoyApp.A.Store)
	_, err := endpointRepo.FindEndpointByID(context.Background(), endpointID)
	require.Error(s.T(), err, datastore.ErrEndpointNotFound)
}

func (s *EndpointIntegrationTestSuite) Test_DeleteEndpoint_WithPersonalAPIKey() {
	endpointID := uuid.New().String()
	expectedStatusCode := http.StatusOK

	// Just Before.
	_, _ = testdb.SeedEndpoint(s.ConvoyApp.A.Store, s.DefaultGroup, endpointID, "", true)

	// Arrange Request.
	url := fmt.Sprintf("/api/v1/projects/%s/endpoints/%s", s.DefaultGroup.UID, endpointID)
	req := createRequest(http.MethodDelete, url, s.PersonalAPIKey, nil)
	w := httptest.NewRecorder()

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)

	// Deep Assert.
	endpointRepo := cm.NewEndpointRepo(s.ConvoyApp.A.Store)
	_, err := endpointRepo.FindEndpointByID(context.Background(), endpointID)
	require.Error(s.T(), err, datastore.ErrEndpointNotFound)
}

func (s *EndpointIntegrationTestSuite) Test_CreateEndpoint_With_Custom_Authentication() {
	title := "random-name"
	f := faker.New()
	endpointURL := f.Internet().URL()
	secret := f.Lorem().Text(25)
	expectedStatusCode := http.StatusCreated

	// Arrange Request
	url := fmt.Sprintf("/api/v1/projects/%s/endpoints", s.DefaultGroup.UID)
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

func (s *EndpointIntegrationTestSuite) Test_ExpireEndpointSecret() {
	endpointID := uuid.New().String()
	f := faker.New()
	secret := f.Lorem().Text(25)
	expiration := 7
	expectedStatusCode := http.StatusOK

	// Just Before.
	e, _ := testdb.SeedEndpoint(s.ConvoyApp.A.Store, s.DefaultGroup, endpointID, "", true)
	_, _ = testdb.SeedEndpointSecret(s.ConvoyApp.A.Store, e, secret)

	// Arrange Request
	url := fmt.Sprintf("/api/v1/projects/%s/endpoints/%s/expire_secret", s.DefaultGroup.UID, endpointID)
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

	endpointRepo := cm.NewEndpointRepo(s.ConvoyApp.A.Store)
	endpoint2, err := endpointRepo.FindEndpointByID(context.Background(), endpointID)
	require.NoError(s.T(), err)
	require.NotEmpty(s.T(), endpoint2.Secrets[0].ExpiresAt)
}

func TestEndpointIntegrationTestSuite(t *testing.T) {
	suite.Run(t, new(EndpointIntegrationTestSuite))
}
