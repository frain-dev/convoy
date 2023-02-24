//go:build integration
// +build integration

package server

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/frain-dev/convoy/auth"
	"github.com/frain-dev/convoy/database"
	"github.com/frain-dev/convoy/database/postgres"
	"github.com/frain-dev/convoy/internal/pkg/metrics"

	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/server/models"
	"github.com/frain-dev/convoy/server/testdb"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type SecurityIntegrationTestSuite struct {
	suite.Suite
	DB              database.Database
	Router          http.Handler
	ConvoyApp       *ApplicationHandler
	AuthenticatorFn AuthenticatorFn
	DefaultOrg      *datastore.Organisation
	DefaultProject  *datastore.Project
	DefaultUser     *datastore.User
}

func (s *SecurityIntegrationTestSuite) SetupSuite() {
	s.DB = getDB()
	s.ConvoyApp = buildServer()
	s.Router = s.ConvoyApp.BuildRoutes()
}

func (s *SecurityIntegrationTestSuite) SetupTest() {
	testdb.PurgeDB(s.T(), s.DB)

	user, err := testdb.SeedDefaultUser(s.ConvoyApp.A.DB)
	require.NoError(s.T(), err)
	s.DefaultUser = user

	org, err := testdb.SeedDefaultOrganisation(s.ConvoyApp.A.DB, user)
	require.NoError(s.T(), err)
	s.DefaultOrg = org

	// Setup Default Project.
	s.DefaultProject, _ = testdb.SeedDefaultProject(s.ConvoyApp.A.DB, s.DefaultOrg.UID)

	s.AuthenticatorFn = authenticateRequest(&models.LoginUser{
		Username: user.Email,
		Password: testdb.DefaultUserPassword,
	})

	// Setup Config.
	err = config.LoadConfig("./testdata/Auth_Config/full-convoy-with-jwt-realm.json")
	require.NoError(s.T(), err)

	apiRepo := postgres.NewAPIKeyRepo(s.ConvoyApp.A.DB)
	userRepo := postgres.NewUserRepo(s.ConvoyApp.A.DB)
	initRealmChain(s.T(), apiRepo, userRepo, s.ConvoyApp.A.Cache)
}

func (s *SecurityIntegrationTestSuite) Test_CreatePersonalAPIKey() {
	expectedStatusCode := http.StatusCreated

	// Arrange Request.
	bodyStr := `{"name":"default_api_key","expiration":1}`
	body := serialize(bodyStr)

	url := fmt.Sprintf("/ui/users/%s/security/personal_api_keys", s.DefaultUser.UID)

	req := createRequest(http.MethodPost, url, "", body)
	err := s.AuthenticatorFn(req, s.Router)
	require.NoError(s.T(), err)

	w := httptest.NewRecorder()

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)

	// Deep Assert.
	var apiKeyResponse models.APIKeyResponse
	parseResponse(s.T(), w.Result(), &apiKeyResponse)

	apiRepo := postgres.NewAPIKeyRepo(s.ConvoyApp.A.DB)
	apiKey, err := apiRepo.FindAPIKeyByID(context.Background(), apiKeyResponse.UID)
	require.NoError(s.T(), err)

	require.NotEmpty(s.T(), apiKeyResponse.Key)
	require.Equal(s.T(), datastore.PersonalKey, apiKeyResponse.Type)
	require.Equal(s.T(), s.DefaultUser.UID, apiKeyResponse.UserID)
	require.Equal(s.T(), apiKey.UID, apiKeyResponse.UID)
}

func (s *SecurityIntegrationTestSuite) Test_CreateEndpointPortalAPIKey() {
	expectedStatusCode := http.StatusCreated

	// Switch to the native realm
	err := config.LoadConfig("./testdata/Auth_Config/full-convoy-with-native-auth-realm.json")
	require.NoError(s.T(), err)

	apiRepo := postgres.NewAPIKeyRepo(s.ConvoyApp.A.DB)
	userRepo := postgres.NewUserRepo(s.ConvoyApp.A.DB)
	initRealmChain(s.T(), apiRepo, userRepo, s.ConvoyApp.A.Cache)

	// Just Before.
	endpoint, _ := testdb.SeedEndpoint(s.ConvoyApp.A.DB, s.DefaultProject, uuid.NewString(), "test-app", "", true, datastore.ActiveEndpointStatus)

	role := auth.Role{
		Type:    auth.RoleAdmin,
		Project: s.DefaultProject.UID,
	}

	// Generate api key for this Project, use the key to authenticate for this request later on
	_, keyString, err := testdb.SeedAPIKey(s.ConvoyApp.A.DB, role, uuid.NewString(), "test", "api", "")
	require.NoError(s.T(), err)

	// Arrange Request.
	bodyStr := `{"key_type":"app_portal"}`
	body := serialize(bodyStr)

	url := fmt.Sprintf("/api/v1/projects/%s/security/endpoints/%s/keys", s.DefaultProject.UID, endpoint.UID)

	req := createRequest(http.MethodPost, url, "", body)
	req.Header.Set("Authorization", fmt.Sprintf("BEARER %s", keyString)) // authenticate with previously generated key
	w := httptest.NewRecorder()

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)

	// Deep Assert.
	var apiKeyResponse models.PortalAPIKeyResponse
	parseResponse(s.T(), w.Result(), &apiKeyResponse)
	require.NotEmpty(s.T(), apiKeyResponse.Key)
	require.Equal(s.T(), apiKeyResponse.Url, fmt.Sprintf("https://app.convoy.io/endpoint/%s?projectID=%s&endpointId=%s", apiKeyResponse.Key, s.DefaultProject.UID, endpoint.UID))
	require.Equal(s.T(), apiKeyResponse.Type, string(datastore.AppPortalKey))
	require.Equal(s.T(), apiKeyResponse.ProjectID, s.DefaultProject.UID)
	require.Equal(s.T(), apiKeyResponse.EndpointID, endpoint.UID)
}

func (s *SecurityIntegrationTestSuite) Test_RegenerateProjectAPIKey() {
	expectedStatusCode := http.StatusOK

	// Switch to the native realm
	err := config.LoadConfig("./testdata/Auth_Config/full-convoy-with-jwt-realm.json")
	require.NoError(s.T(), err)

	apiRepo := postgres.NewAPIKeyRepo(s.ConvoyApp.A.DB)
	userRepo := postgres.NewUserRepo(s.ConvoyApp.A.DB)
	initRealmChain(s.T(), apiRepo, userRepo, s.ConvoyApp.A.Cache)

	role := auth.Role{
		Type:    auth.RoleAdmin,
		Project: s.DefaultProject.UID,
	}

	// Generate api key for this Project
	_, keyString, err := testdb.SeedAPIKey(s.ConvoyApp.A.DB, role, uuid.NewString(), "test", "api", "")
	require.NoError(s.T(), err)

	url := fmt.Sprintf("/ui/organisations/%s/projects/%s/security/keys/regenerate", s.DefaultOrg.UID, s.DefaultProject.UID)

	req := createRequest(http.MethodPut, url, "", nil)
	err = s.AuthenticatorFn(req, s.Router)
	require.NoError(s.T(), err)

	w := httptest.NewRecorder()

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)

	// Deep Assert.
	var apiKeyResponse models.APIKeyResponse
	parseResponse(s.T(), w.Result(), &apiKeyResponse)
	require.NotEmpty(s.T(), apiKeyResponse.Key)
	require.NotEqual(s.T(), apiKeyResponse.Key, keyString)

	dbAPIKey, err := apiRepo.FindAPIKeyByProjectID(context.Background(), s.DefaultProject.UID)
	require.NoError(s.T(), err)
	require.Equal(s.T(), apiKeyResponse.UID, dbAPIKey.UID)
}

func (s *SecurityIntegrationTestSuite) Test_CreateEndpointPortalAPIKey_RedirectToProjects() {
	expectedStatusCode := http.StatusTemporaryRedirect

	// Switch to the native realm
	err := config.LoadConfig("./testdata/Auth_Config/full-convoy-with-native-auth-realm.json")
	require.NoError(s.T(), err)

	apiRepo := postgres.NewAPIKeyRepo(s.ConvoyApp.A.DB)
	userRepo := postgres.NewUserRepo(s.ConvoyApp.A.DB)
	initRealmChain(s.T(), apiRepo, userRepo, s.ConvoyApp.A.Cache)

	// Just Before.
	endpoint, _ := testdb.SeedEndpoint(s.ConvoyApp.A.DB, s.DefaultProject, uuid.NewString(), "test-app", "", true, datastore.ActiveEndpointStatus)

	role := auth.Role{
		Type:    auth.RoleAdmin,
		Project: s.DefaultProject.UID,
	}

	// Generate api key for this Project, use the key to authenticate for this request later on
	_, keyString, err := testdb.SeedAPIKey(s.ConvoyApp.A.DB, role, uuid.NewString(), "test", "api", "")
	require.NoError(s.T(), err)

	// Arrange Request.
	bodyStr := `{"key_type":"cli"}`
	body := serialize(bodyStr)

	url := fmt.Sprintf("/api/v1/security/endpoints/%s/keys?projectID=%s", endpoint.UID, s.DefaultProject.UID)

	req := createRequest(http.MethodPost, url, "", body)
	req.Header.Set("Authorization", fmt.Sprintf("BEARER %s", keyString)) // authenticate with previously generated key
	w := httptest.NewRecorder()

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)
}

func (s *SecurityIntegrationTestSuite) Test_CreateEndpointCliAPIKey() {
	expectedStatusCode := http.StatusCreated

	// Switch to the native realm
	err := config.LoadConfig("./testdata/Auth_Config/full-convoy-with-native-auth-realm.json")
	require.NoError(s.T(), err)

	apiRepo := postgres.NewAPIKeyRepo(s.ConvoyApp.A.DB)
	userRepo := postgres.NewUserRepo(s.ConvoyApp.A.DB)
	initRealmChain(s.T(), apiRepo, userRepo, s.ConvoyApp.A.Cache)

	// Just Before.
	endpoint, _ := testdb.SeedEndpoint(s.ConvoyApp.A.DB, s.DefaultProject, uuid.NewString(), "test-app", "", true, datastore.ActiveEndpointStatus)

	role := auth.Role{
		Type:    auth.RoleAdmin,
		Project: s.DefaultProject.UID,
	}

	// Generate api key for this Project, use the key to authenticate for this request later on
	_, keyString, err := testdb.SeedAPIKey(s.ConvoyApp.A.DB, role, uuid.NewString(), "test", "api", "")
	require.NoError(s.T(), err)

	// Arrange Request.
	bodyStr := `{"key_type":"cli"}`
	body := serialize(bodyStr)

	url := fmt.Sprintf("/api/v1/projects/%s/security/endpoints/%s/keys", s.DefaultProject.UID, endpoint.UID)

	req := createRequest(http.MethodPost, url, "", body)
	req.Header.Set("Authorization", fmt.Sprintf("BEARER %s", keyString)) // authenticate with previously generated key
	w := httptest.NewRecorder()

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)

	// Deep Assert.
	var apiKeyResponse models.PortalAPIKeyResponse
	parseResponse(s.T(), w.Result(), &apiKeyResponse)
	require.NotEmpty(s.T(), apiKeyResponse.Key)
	require.Equal(s.T(), apiKeyResponse.Type, string(datastore.CLIKey))
	require.Equal(s.T(), apiKeyResponse.ProjectID, s.DefaultProject.UID)
	require.Equal(s.T(), apiKeyResponse.EndpointID, endpoint.UID)
}

func (s *SecurityIntegrationTestSuite) Test_CreateEndpointPortalAPIKey_EndpointDoesNotBelongToProject() {
	expectedStatusCode := http.StatusUnauthorized

	// Switch to the native realm
	err := config.LoadConfig("./testdata/Auth_Config/full-convoy-with-native-auth-realm.json")
	require.NoError(s.T(), err)

	apiRepo := postgres.NewAPIKeyRepo(s.ConvoyApp.A.DB)
	userRepo := postgres.NewUserRepo(s.ConvoyApp.A.DB)
	initRealmChain(s.T(), apiRepo, userRepo, s.ConvoyApp.A.Cache)

	// Just Before.
	endpoint, _ := testdb.SeedEndpoint(s.ConvoyApp.A.DB, &datastore.Project{UID: uuid.NewString()}, uuid.NewString(), "test-app", "", true, datastore.ActiveEndpointStatus)

	role := auth.Role{
		Type:    auth.RoleAdmin,
		Project: s.DefaultProject.UID,
	}

	// Generate api key for this Project, use the key to authenticate for this request later on
	_, keyString, err := testdb.SeedAPIKey(s.ConvoyApp.A.DB, role, uuid.NewString(), "test", "api", "")
	require.NoError(s.T(), err)

	// Arrange Request.
	bodyStr := `{"key_type":"cli"}"`
	body := serialize(bodyStr, s.DefaultProject.UID, time.Now().Add(time.Hour))

	url := fmt.Sprintf("/api/v1/projects/%s/security/endpoints/%s/keys", s.DefaultProject.UID, endpoint.UID)

	req := createRequest(http.MethodPost, url, "", body)
	req.Header.Set("Authorization", fmt.Sprintf("BEARER %s", keyString)) // authenticate with previously generated key
	w := httptest.NewRecorder()

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)
}

func (s *SecurityIntegrationTestSuite) Test_RevokePersonalAPIKey() {
	expectedStatusCode := http.StatusOK

	// Just Before.
	apiKey, _, _ := testdb.SeedAPIKey(s.ConvoyApp.A.DB, auth.Role{}, uuid.NewString(), "test", string(datastore.PersonalKey), s.DefaultUser.UID)

	url := fmt.Sprintf("/ui/users/%s/security/personal_api_keys/%s/revoke", s.DefaultUser.UID, apiKey.UID)

	req := createRequest(http.MethodPut, url, "", nil)
	err := s.AuthenticatorFn(req, s.Router)
	require.NoError(s.T(), err)

	w := httptest.NewRecorder()

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)

	// Deep assert
	apiRepo := postgres.NewAPIKeyRepo(s.ConvoyApp.A.DB)
	_, err = apiRepo.FindAPIKeyByID(context.Background(), apiKey.UID)
	require.Equal(s.T(), datastore.ErrAPIKeyNotFound, err)
}

func (s *SecurityIntegrationTestSuite) Test_RevokePersonalAPIKey_UnauthorizedUser() {
	expectedStatusCode := http.StatusUnauthorized

	// Just Before.
	apiKey, _, _ := testdb.SeedAPIKey(s.ConvoyApp.A.DB, auth.Role{}, uuid.NewString(), "test", string(datastore.PersonalKey), uuid.NewString())

	url := fmt.Sprintf("/ui/users/%s/security/personal_api_keys/%s/revoke", s.DefaultUser.UID, apiKey.UID)

	req := createRequest(http.MethodPut, url, "", nil)
	err := s.AuthenticatorFn(req, s.Router)
	require.NoError(s.T(), err)

	w := httptest.NewRecorder()

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)

	// Deep assert
	apiRepo := postgres.NewAPIKeyRepo(s.ConvoyApp.A.DB)
	_, err = apiRepo.FindAPIKeyByID(context.Background(), apiKey.UID)
	require.Nil(s.T(), err)
}

func (s *SecurityIntegrationTestSuite) Test_GetPersonalAPIKeys() {
	expectedStatusCode := http.StatusOK

	// Just Before.
	_, _, _ = testdb.SeedAPIKey(s.ConvoyApp.A.DB, auth.Role{}, uuid.NewString(), "test-1", string(datastore.PersonalKey), s.DefaultUser.UID)
	_, _, _ = testdb.SeedAPIKey(s.ConvoyApp.A.DB, auth.Role{}, uuid.NewString(), "test-2", string(datastore.PersonalKey), s.DefaultUser.UID)
	_, _, _ = testdb.SeedAPIKey(s.ConvoyApp.A.DB, auth.Role{}, uuid.NewString(), "test-3", string(datastore.PersonalKey), uuid.NewString())

	url := fmt.Sprintf("/ui/users/%s/security/personal_api_keys?keyType=personal_key", s.DefaultUser.UID)
	req := createRequest(http.MethodGet, url, "", nil)
	err := s.AuthenticatorFn(req, s.Router)
	require.NoError(s.T(), err)

	w := httptest.NewRecorder()

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)

	var apiKeyResponse []models.APIKeyByIDResponse
	pagedResp := &pagedResponse{Content: &apiKeyResponse}
	parseResponse(s.T(), w.Result(), pagedResp)
	require.Equal(s.T(), 2, len(apiKeyResponse))
}

func (s *SecurityIntegrationTestSuite) Test_GetEndpointAPIKeys() {
	expectedStatusCode := http.StatusOK

	// Just Before.
	endpoint, _ := testdb.SeedEndpoint(s.ConvoyApp.A.DB, s.DefaultProject, uuid.NewString(), "test-app", "", true, datastore.ActiveEndpointStatus)

	role := auth.Role{
		Type:     auth.RoleAdmin,
		Project:  s.DefaultProject.UID,
		Endpoint: endpoint.UID,
	}

	_, _, _ = testdb.SeedAPIKey(s.ConvoyApp.A.DB, role, uuid.NewString(), "test", string(datastore.CLIKey), "")
	_, _, _ = testdb.SeedAPIKey(s.ConvoyApp.A.DB, role, uuid.NewString(), "test", string(datastore.AppPortalKey), "")

	url := fmt.Sprintf("/ui/organisations/%s/projects/%s/endpoints/%s/keys", s.DefaultOrg.UID, s.DefaultProject.UID, endpoint.UID)
	req := createRequest(http.MethodGet, url, "", nil)
	err := s.AuthenticatorFn(req, s.Router)
	require.NoError(s.T(), err)

	w := httptest.NewRecorder()

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)

	var apiKeyResponse []models.APIKeyByIDResponse
	pagedResp := &pagedResponse{Content: &apiKeyResponse}
	parseResponse(s.T(), w.Result(), pagedResp)
	require.Equal(s.T(), 1, len(apiKeyResponse))
}

func (s *SecurityIntegrationTestSuite) Test_RevokeEndpointAPIKey() {
	expectedStatusCode := http.StatusOK

	// Just Before.
	endpoint, _ := testdb.SeedEndpoint(s.ConvoyApp.A.DB, s.DefaultProject, uuid.NewString(), "test-app", "", true, datastore.ActiveEndpointStatus)

	role := auth.Role{
		Type:     auth.RoleAdmin,
		Project:  s.DefaultProject.UID,
		Endpoint: endpoint.UID,
	}

	apiKey, _, _ := testdb.SeedAPIKey(s.ConvoyApp.A.DB, role, uuid.NewString(), "test", string(datastore.CLIKey), "")

	url := fmt.Sprintf("/ui/organisations/%s/projects/%s/endpoints/%s/keys/%s/revoke", s.DefaultOrg.UID, s.DefaultProject.UID, endpoint.UID, apiKey.UID)
	req := createRequest(http.MethodPut, url, "", nil)
	err := s.AuthenticatorFn(req, s.Router)
	require.NoError(s.T(), err)

	w := httptest.NewRecorder()

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)

	// Deep Assert.
	apiRepo := postgres.NewAPIKeyRepo(s.ConvoyApp.A.DB)
	_, err = apiRepo.FindAPIKeyByID(context.Background(), apiKey.UID)
	require.Error(s.T(), err)
}

func (s *SecurityIntegrationTestSuite) TearDownTest() {
	testdb.PurgeDB(s.T(), s.DB)
	metrics.Reset()
}

func TestSecurityIntegrationTestSuiteTest(t *testing.T) {
	suite.Run(t, new(SecurityIntegrationTestSuite))
}
