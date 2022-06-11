//go:build integration
// +build integration

package server

import (
	"context"
	"fmt"
	"github.com/frain-dev/convoy/auth"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

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
	DB              datastore.DatabaseClient
	Router          http.Handler
	ConvoyApp       *applicationHandler
	AuthenticatorFn AuthenticatorFn
	DefaultOrg      *datastore.Organisation
	DefaultGroup    *datastore.Group
	DefaultUser     *datastore.User
}

func (s *SecurityIntegrationTestSuite) SetupSuite() {
	s.DB = getDB()
	s.ConvoyApp = buildApplication()
	s.Router = buildRoutes(s.ConvoyApp)
}

func (s *SecurityIntegrationTestSuite) SetupTest() {
	testdb.PurgeDB(s.DB)

	user, err := testdb.SeedDefaultUser(s.DB)
	require.NoError(s.T(), err)
	s.DefaultUser = user

	org, err := testdb.SeedDefaultOrganisation(s.DB, user)
	require.NoError(s.T(), err)
	s.DefaultOrg = org

	//Setup Default Group.
	s.DefaultGroup, _ = testdb.SeedDefaultGroup(s.DB, s.DefaultOrg.UID)

	s.AuthenticatorFn = authenticateRequest(&models.LoginUser{
		Username: user.Email,
		Password: testdb.DefaultUserPassword,
	})

	// Setup Config.
	err = config.LoadConfig("./testdata/Auth_Config/full-convoy-with-jwt-realm.json")
	require.NoError(s.T(), err)

	initRealmChain(s.T(), s.DB.APIRepo(), s.DB.UserRepo(), s.ConvoyApp.cache)
}

func (s *SecurityIntegrationTestSuite) Test_CreateAPIKey() {
	expectedStatusCode := http.StatusCreated

	// Arrange Request.
	bodyStr := `{"name":"default_api_key","role":{"type":"ui_admin","group":"%s"},"key_type":"api_key","expires_at":"%s"}`
	body := serialize(bodyStr, s.DefaultGroup.UID, time.Now().Add(time.Hour).Format(time.RFC3339))

	url := fmt.Sprintf("/ui/organisations/%s/security/keys", s.DefaultOrg.UID)

	req := createRequest(http.MethodPost, url, body)
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

	apiKey, err := s.DB.APIRepo().FindAPIKeyByID(context.Background(), apiKeyResponse.UID)
	require.NoError(s.T(), err)

	require.NotEmpty(s.T(), apiKeyResponse.Key)
	require.Equal(s.T(), apiKey.UID, apiKeyResponse.UID)
}

func (s *SecurityIntegrationTestSuite) Test_CreateAppPortalAPIKey() {
	expectedStatusCode := http.StatusCreated

	// Switch to the native realm
	err := config.LoadConfig("./testdata/Auth_Config/full-convoy-with-native-auth-realm.json")
	require.NoError(s.T(), err)

	initRealmChain(s.T(), s.DB.APIRepo(), s.DB.UserRepo(), s.ConvoyApp.cache)

	member, err := testdb.SeedOrganisationMember(s.DB, s.DefaultOrg, s.DefaultUser, &auth.Role{
		Type:   auth.RoleAdmin,
		Groups: []string{s.DefaultGroup.UID},
	})

	newAPIKey := &models.APIKey{
		Name: s.DefaultOrg.Name + "'s default key",
		Role: models.Role{
			Type:  auth.RoleAdmin,
			Group: s.DefaultGroup.UID,
		},
	}

	_, keyString, err := s.ConvoyApp.securityService.CreateAPIKey(context.Background(), member, newAPIKey)
	require.NoError(s.T(), err)

	// Just Before.
	app, _ := testdb.SeedApplication(s.DB, s.DefaultGroup, uuid.NewString(), "test-app", true)

	// Arrange Request.
	bodyStr := `{"name":"default_api_key","role":{"type":"ui_admin","group":"%s"},"key_type":"api_key","expires_at":"%s"}"`
	body := serialize(bodyStr, s.DefaultGroup.UID, time.Now().Add(time.Hour))

	url := fmt.Sprintf("/api/v1/security/applications/%s/keys", app.UID)

	req := createRequest(http.MethodPost, url, body)
	req.Header.Set("Authorization", fmt.Sprintf("BEARER %s", keyString))
	w := httptest.NewRecorder()

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)

	// Deep Assert.
	var apiKeyResponse models.PortalAPIKeyResponse
	parseResponse(s.T(), w.Result(), &apiKeyResponse)
	require.NotEmpty(s.T(), apiKeyResponse.Key)
	require.Equal(s.T(), apiKeyResponse.Url, fmt.Sprintf("https://app.convoy.io/app-portal/%s?groupID=%s&appId=%s", apiKeyResponse.Key, s.DefaultGroup.UID, app.UID))
	require.Equal(s.T(), apiKeyResponse.Type, "app_portal")
	require.Equal(s.T(), apiKeyResponse.GroupID, s.DefaultGroup.UID)
	require.Equal(s.T(), apiKeyResponse.AppID, app.UID)
}

func (s *SecurityIntegrationTestSuite) Test_RevokeAPIKey() {
	expectedStatusCode := http.StatusOK

	// Just Before.
	apiKey, _ := testdb.SeedAPIKey(s.DB, s.DefaultGroup, uuid.NewString(), "test", "api")

	url := fmt.Sprintf("/ui/organisations/%s/security/keys/%s/revoke", s.DefaultOrg.UID, apiKey.UID)

	req := createRequest(http.MethodPut, url, nil)
	err := s.AuthenticatorFn(req, s.Router)
	require.NoError(s.T(), err)

	w := httptest.NewRecorder()

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)

	// Deep assert
	a, err := s.DB.APIRepo().FindAPIKeyByID(context.Background(), apiKey.UID)
	require.NoError(s.T(), err)
	require.Equal(s.T(), datastore.DeletedDocumentStatus, a.DocumentStatus)
	require.True(s.T(), a.DeletedAt > 0)
}

func (s *SecurityIntegrationTestSuite) Test_GetAPIKeyByID() {
	expectedStatusCode := http.StatusOK

	// Just Before.
	apiKey, _ := testdb.SeedAPIKey(s.DB, s.DefaultGroup, uuid.NewString(), "test", "api")

	url := fmt.Sprintf("/ui/organisations/%s/security/keys/%s", s.DefaultOrg.UID, apiKey.UID)

	req := createRequest(http.MethodGet, url, nil)
	err := s.AuthenticatorFn(req, s.Router)
	require.NoError(s.T(), err)

	w := httptest.NewRecorder()

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)

	// Deep Assert.
	var apiKeyResponse models.APIKeyByIDResponse
	parseResponse(s.T(), w.Result(), &apiKeyResponse)
	require.Equal(s.T(), apiKeyResponse.UID, apiKey.UID)
}

func (s *SecurityIntegrationTestSuite) Test_GetAPIKeyByID_APIKeyNotFound() {
	expectedStatusCode := http.StatusBadRequest

	url := fmt.Sprintf("/ui/organisations/%s/security/keys/%s", s.DefaultOrg.UID, uuid.NewString())

	req := createRequest(http.MethodGet, url, nil)
	err := s.AuthenticatorFn(req, s.Router)
	require.NoError(s.T(), err)

	w := httptest.NewRecorder()

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)
}

func (s *SecurityIntegrationTestSuite) Test_UpdateAPIKey() {
	expectedStatusCode := http.StatusOK

	// Just Before.
	apiKey, _ := testdb.SeedAPIKey(s.DB, s.DefaultGroup, uuid.NewString(), "test", "api")

	bodyStr := `{"role":{"type":"api","groups":["%s"]}}`
	body := serialize(bodyStr, s.DefaultGroup.UID)

	url := fmt.Sprintf("/ui/organisations/%s/security/keys/%s", s.DefaultOrg.UID, apiKey.UID)
	req := createRequest(http.MethodPut, url, body)
	err := s.AuthenticatorFn(req, s.Router)
	require.NoError(s.T(), err)

	w := httptest.NewRecorder()

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)

	// Deep Assert.
	a, err := s.DB.APIRepo().FindAPIKeyByID(context.Background(), apiKey.UID)
	require.NoError(s.T(), err)

	var apiKeyResponse models.APIKeyByIDResponse
	parseResponse(s.T(), w.Result(), &apiKeyResponse)
	require.Equal(s.T(), a.Role, apiKeyResponse.Role)
	require.Equal(s.T(), apiKeyResponse.UID, apiKey.UID)
}

func (s *SecurityIntegrationTestSuite) Test_UpdateAPIKey_APIKeyNotFound() {
	expectedStatusCode := http.StatusBadRequest

	bodyStr := `{"role":{"type":"api","groups":["%s"]}}`
	body := serialize(bodyStr, s.DefaultGroup.UID)

	url := fmt.Sprintf("/ui/organisations/%s/security/keys/%s", s.DefaultOrg.UID, uuid.NewString())

	req := createRequest(http.MethodPut, url, body)
	err := s.AuthenticatorFn(req, s.Router)
	require.NoError(s.T(), err)

	w := httptest.NewRecorder()

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)
}

func (s *SecurityIntegrationTestSuite) Test_GetAPIKeys() {
	expectedStatusCode := http.StatusOK

	// Just Before.
	_, _ = testdb.SeedAPIKey(s.DB, s.DefaultGroup, uuid.NewString(), "test", "api")
	_, _ = testdb.SeedAPIKey(s.DB, s.DefaultGroup, uuid.NewString(), "test", "api")
	_, _ = testdb.SeedAPIKey(s.DB, s.DefaultGroup, uuid.NewString(), "test", "api")

	bodyStr := `{"role":{"type":"api","groups":["%s"]}}`
	body := serialize(bodyStr, uuid.NewString())

	url := fmt.Sprintf("/ui/organisations/%s/security/keys", s.DefaultOrg.UID)
	req := createRequest(http.MethodGet, url, body)
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
	require.Equal(s.T(), 3, len(apiKeyResponse))
}

func (s *SecurityIntegrationTestSuite) TearDownTest() {
	testdb.PurgeDB(s.DB)
}

func TestSecurityIntegrationTestSuiteTest(t *testing.T) {
	suite.Run(t, new(SecurityIntegrationTestSuite))
}
