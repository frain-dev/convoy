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
	"github.com/frain-dev/convoy/internal/pkg/metrics"

	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/datastore"
	cm "github.com/frain-dev/convoy/datastore/mongo"
	"github.com/frain-dev/convoy/server/models"
	"github.com/frain-dev/convoy/server/testdb"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type SecurityIntegrationTestSuite struct {
	suite.Suite
	DB              cm.Client
	Router          http.Handler
	ConvoyApp       *ApplicationHandler
	AuthenticatorFn AuthenticatorFn
	DefaultOrg      *datastore.Organisation
	DefaultGroup    *datastore.Group
	DefaultUser     *datastore.User
}

func (s *SecurityIntegrationTestSuite) SetupSuite() {
	s.DB = getDB()
	s.ConvoyApp = buildServer()
	s.Router = s.ConvoyApp.BuildRoutes()
}

func (s *SecurityIntegrationTestSuite) SetupTest() {
	testdb.PurgeDB(s.T(), s.DB)

	user, err := testdb.SeedDefaultUser(s.ConvoyApp.A.Store)
	require.NoError(s.T(), err)
	s.DefaultUser = user

	org, err := testdb.SeedDefaultOrganisation(s.ConvoyApp.A.Store, user)
	require.NoError(s.T(), err)
	s.DefaultOrg = org

	// Setup Default Group.
	s.DefaultGroup, _ = testdb.SeedDefaultGroup(s.ConvoyApp.A.Store, s.DefaultOrg.UID)

	s.AuthenticatorFn = authenticateRequest(&models.LoginUser{
		Username: user.Email,
		Password: testdb.DefaultUserPassword,
	})

	// Setup Config.
	err = config.LoadConfig("./testdata/Auth_Config/full-convoy-with-jwt-realm.json")
	require.NoError(s.T(), err)

	apiRepo := cm.NewApiKeyRepo(s.ConvoyApp.A.Store)
	userRepo := cm.NewUserRepo(s.ConvoyApp.A.Store)
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

	apiRepo := cm.NewApiKeyRepo(s.ConvoyApp.A.Store)
	apiKey, err := apiRepo.FindAPIKeyByID(context.Background(), apiKeyResponse.UID)
	require.NoError(s.T(), err)

	require.NotEmpty(s.T(), apiKeyResponse.Key)
	require.Equal(s.T(), datastore.PersonalKey, apiKeyResponse.Type)
	require.Equal(s.T(), s.DefaultUser.UID, apiKeyResponse.UserID)
	require.Equal(s.T(), apiKey.UID, apiKeyResponse.UID)
}

func (s *SecurityIntegrationTestSuite) Test_CreateAppPortalAPIKey() {
	expectedStatusCode := http.StatusCreated

	// Switch to the native realm
	err := config.LoadConfig("./testdata/Auth_Config/full-convoy-with-native-auth-realm.json")
	require.NoError(s.T(), err)

	apiRepo := cm.NewApiKeyRepo(s.ConvoyApp.A.Store)
	userRepo := cm.NewUserRepo(s.ConvoyApp.A.Store)
	initRealmChain(s.T(), apiRepo, userRepo, s.ConvoyApp.A.Cache)

	// Just Before.
	app, _ := testdb.SeedApplication(s.ConvoyApp.A.Store, s.DefaultGroup, uuid.NewString(), "test-app", true)

	role := auth.Role{
		Type:  auth.RoleAdmin,
		Group: s.DefaultGroup.UID,
	}

	// Generate api key for this group, use the key to authenticate for this request later on
	_, keyString, err := testdb.SeedAPIKey(s.ConvoyApp.A.Store, role, uuid.NewString(), "test", "api", "")
	require.NoError(s.T(), err)

	// Arrange Request.
	bodyStr := `{"key_type":"app_portal"}`
	body := serialize(bodyStr)

	url := fmt.Sprintf("/api/v1/projects/%s/security/applications/%s/keys", s.DefaultGroup.UID, app.UID)

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
	require.Equal(s.T(), apiKeyResponse.Url, fmt.Sprintf("https://app.convoy.io/app/%s?groupID=%s&appId=%s", apiKeyResponse.Key, s.DefaultGroup.UID, app.UID))
	require.Equal(s.T(), apiKeyResponse.Type, string(datastore.AppPortalKey))
	require.Equal(s.T(), apiKeyResponse.GroupID, s.DefaultGroup.UID)
	require.Equal(s.T(), apiKeyResponse.AppID, app.UID)
}

func (s *SecurityIntegrationTestSuite) Test_CreateAppPortalAPIKey_RedirectToProjects() {
	expectedStatusCode := http.StatusTemporaryRedirect

	// Switch to the native realm
	err := config.LoadConfig("./testdata/Auth_Config/full-convoy-with-native-auth-realm.json")
	require.NoError(s.T(), err)

	apiRepo := cm.NewApiKeyRepo(s.ConvoyApp.A.Store)
	userRepo := cm.NewUserRepo(s.ConvoyApp.A.Store)
	initRealmChain(s.T(), apiRepo, userRepo, s.ConvoyApp.A.Cache)

	// Just Before.
	app, _ := testdb.SeedApplication(s.ConvoyApp.A.Store, s.DefaultGroup, uuid.NewString(), "test-app", true)

	role := auth.Role{
		Type:  auth.RoleAdmin,
		Group: s.DefaultGroup.UID,
	}

	// Generate api key for this group, use the key to authenticate for this request later on
	_, keyString, err := testdb.SeedAPIKey(s.ConvoyApp.A.Store, role, uuid.NewString(), "test", "api", "")
	require.NoError(s.T(), err)

	// Arrange Request.
	bodyStr := `{"key_type":"cli"}`
	body := serialize(bodyStr)

	url := fmt.Sprintf("/api/v1/security/applications/%s/keys?groupID=%s", app.UID, s.DefaultGroup.UID)

	req := createRequest(http.MethodPost, url, "", body)
	req.Header.Set("Authorization", fmt.Sprintf("BEARER %s", keyString)) // authenticate with previously generated key
	w := httptest.NewRecorder()

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)
}

func (s *SecurityIntegrationTestSuite) Test_CreateAppCliAPIKey() {
	expectedStatusCode := http.StatusCreated

	// Switch to the native realm
	err := config.LoadConfig("./testdata/Auth_Config/full-convoy-with-native-auth-realm.json")
	require.NoError(s.T(), err)

	apiRepo := cm.NewApiKeyRepo(s.ConvoyApp.A.Store)
	userRepo := cm.NewUserRepo(s.ConvoyApp.A.Store)
	initRealmChain(s.T(), apiRepo, userRepo, s.ConvoyApp.A.Cache)

	// Just Before.
	app, _ := testdb.SeedApplication(s.ConvoyApp.A.Store, s.DefaultGroup, uuid.NewString(), "test-app", true)

	role := auth.Role{
		Type:  auth.RoleAdmin,
		Group: s.DefaultGroup.UID,
	}

	// Generate api key for this group, use the key to authenticate for this request later on
	_, keyString, err := testdb.SeedAPIKey(s.ConvoyApp.A.Store, role, uuid.NewString(), "test", "api", "")
	require.NoError(s.T(), err)

	// Arrange Request.
	bodyStr := `{"key_type":"cli"}"`
	body := serialize(bodyStr, s.DefaultGroup.UID, time.Now().Add(time.Hour))

	url := fmt.Sprintf("/api/v1/projects/%s/security/applications/%s/keys", s.DefaultGroup.UID, app.UID)

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
	require.Equal(s.T(), apiKeyResponse.GroupID, s.DefaultGroup.UID)
	require.Equal(s.T(), apiKeyResponse.AppID, app.UID)
}

func (s *SecurityIntegrationTestSuite) Test_CreateAppPortalAPIKey_AppDoesNotBelongToGroup() {
	expectedStatusCode := http.StatusUnauthorized

	// Switch to the native realm
	err := config.LoadConfig("./testdata/Auth_Config/full-convoy-with-native-auth-realm.json")
	require.NoError(s.T(), err)

	apiRepo := cm.NewApiKeyRepo(s.ConvoyApp.A.Store)
	userRepo := cm.NewUserRepo(s.ConvoyApp.A.Store)
	initRealmChain(s.T(), apiRepo, userRepo, s.ConvoyApp.A.Cache)

	// Just Before.
	app, _ := testdb.SeedApplication(s.ConvoyApp.A.Store, &datastore.Group{UID: uuid.NewString()}, uuid.NewString(), "test-app", true)

	role := auth.Role{
		Type:  auth.RoleAdmin,
		Group: s.DefaultGroup.UID,
	}

	// Generate api key for this group, use the key to authenticate for this request later on
	_, keyString, err := testdb.SeedAPIKey(s.ConvoyApp.A.Store, role, uuid.NewString(), "test", "api", "")
	require.NoError(s.T(), err)

	// Arrange Request.
	bodyStr := `{"name":"default_api_key","role":{"type":"admin","group":"%s"},"key_type":"api_key","expires_at":"%s"}"`
	body := serialize(bodyStr, s.DefaultGroup.UID, time.Now().Add(time.Hour))

	url := fmt.Sprintf("/api/v1/projects/%s/security/applications/%s/keys", s.DefaultGroup.UID, app.UID)

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
	apiKey, _, _ := testdb.SeedAPIKey(s.ConvoyApp.A.Store, auth.Role{}, uuid.NewString(), "test", string(datastore.PersonalKey), s.DefaultUser.UID)

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
	apiRepo := cm.NewApiKeyRepo(s.ConvoyApp.A.Store)
	_, err = apiRepo.FindAPIKeyByID(context.Background(), apiKey.UID)
	require.Equal(s.T(), datastore.ErrAPIKeyNotFound, err)
}

func (s *SecurityIntegrationTestSuite) Test_RevokePersonalAPIKey_UnauthorizedUser() {
	expectedStatusCode := http.StatusUnauthorized

	// Just Before.
	apiKey, _, _ := testdb.SeedAPIKey(s.ConvoyApp.A.Store, auth.Role{}, uuid.NewString(), "test", string(datastore.PersonalKey), uuid.NewString())

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
	apiRepo := cm.NewApiKeyRepo(s.ConvoyApp.A.Store)
	_, err = apiRepo.FindAPIKeyByID(context.Background(), apiKey.UID)
	require.Nil(s.T(), err)
}

func (s *SecurityIntegrationTestSuite) Test_GetPersonalAPIKeys() {
	expectedStatusCode := http.StatusOK

	// Just Before.
	_, _, _ = testdb.SeedAPIKey(s.ConvoyApp.A.Store, auth.Role{}, uuid.NewString(), "test-1", string(datastore.PersonalKey), s.DefaultUser.UID)
	_, _, _ = testdb.SeedAPIKey(s.ConvoyApp.A.Store, auth.Role{}, uuid.NewString(), "test-2", string(datastore.PersonalKey), s.DefaultUser.UID)
	_, _, _ = testdb.SeedAPIKey(s.ConvoyApp.A.Store, auth.Role{}, uuid.NewString(), "test-3", string(datastore.PersonalKey), uuid.NewString())

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

func (s *SecurityIntegrationTestSuite) Test_GetAppAPIKeys() {
	expectedStatusCode := http.StatusOK

	// Just Before.
	app, _ := testdb.SeedApplication(s.ConvoyApp.A.Store, s.DefaultGroup, uuid.NewString(), "test-app", true)

	role := auth.Role{
		Type:  auth.RoleAdmin,
		Group: s.DefaultGroup.UID,
		App:   app.UID,
	}

	_, _, _ = testdb.SeedAPIKey(s.ConvoyApp.A.Store, role, uuid.NewString(), "test", string(datastore.CLIKey), "")
	_, _, _ = testdb.SeedAPIKey(s.ConvoyApp.A.Store, role, uuid.NewString(), "test", string(datastore.AppPortalKey), "")

	url := fmt.Sprintf("/ui/organisations/%s/projects/%s/apps/%s/keys", s.DefaultOrg.UID, s.DefaultGroup.UID, app.UID)
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

func (s *SecurityIntegrationTestSuite) Test_RevokeAppAPIKey() {
	expectedStatusCode := http.StatusOK

	// Just Before.
	app, _ := testdb.SeedApplication(s.ConvoyApp.A.Store, s.DefaultGroup, uuid.NewString(), "test-app", true)

	role := auth.Role{
		Type:  auth.RoleAdmin,
		Group: s.DefaultGroup.UID,
		App:   app.UID,
	}

	apiKey, _, _ := testdb.SeedAPIKey(s.ConvoyApp.A.Store, role, uuid.NewString(), "test", string(datastore.CLIKey), "")

	url := fmt.Sprintf("/ui/organisations/%s/projects/%s/apps/%s/keys/%s/revoke", s.DefaultOrg.UID, s.DefaultGroup.UID, app.UID, apiKey.UID)
	req := createRequest(http.MethodPut, url, "", nil)
	err := s.AuthenticatorFn(req, s.Router)
	require.NoError(s.T(), err)

	w := httptest.NewRecorder()

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)

	// Deep Assert.
	apiRepo := cm.NewApiKeyRepo(s.ConvoyApp.A.Store)
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
