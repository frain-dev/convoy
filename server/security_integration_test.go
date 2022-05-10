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
	DB           datastore.DatabaseClient
	Router       http.Handler
	ConvoyApp    *applicationHandler
	DefaultGroup *datastore.Group
}

func (s *SecurityIntegrationTestSuite) SetupSuite() {
	s.DB = getDB()
	s.ConvoyApp = buildApplication()
	s.Router = buildRoutes(s.ConvoyApp)
}

func (s *SecurityIntegrationTestSuite) SetupTest() {
	testdb.PurgeDB(s.DB)

	// Setup Default Group.
	s.DefaultGroup, _ = testdb.SeedDefaultGroup(s.DB)

	// Setup Config.
	err := config.LoadConfig("./testdata/Auth_Config/full-convoy-with-native-auth-realm.json")
	require.NoError(s.T(), err)

	initRealmChain(s.T(), s.DB.APIRepo())
}

func (s *SecurityIntegrationTestSuite) Test_CreateAPIKey() {
	expectedStatusCode := http.StatusCreated

	// Arrange Request.
	bodyStr := `{"name":"default_api_key","role":{"type":"ui_admin","groups":["%s"]},"key_type":"api_key","expires_at":"%s"}"`
	body := serialize(bodyStr, s.DefaultGroup.UID, time.Now().Add(time.Hour))

	req := createRequest(http.MethodPost, "/api/v1/security/keys", body)
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

	// Just Before.
	app, _ := testdb.SeedApplication(s.DB, s.DefaultGroup, uuid.NewString(), true)

	// Arrange Request.
	bodyStr := `{"name":"default_api_key","role":{"type":"ui_admin","groups":["%s"]},"key_type":"api_key","expires_at":"%s"}"`
	body := serialize(bodyStr, s.DefaultGroup.UID, time.Now().Add(time.Hour))

	url := fmt.Sprintf("/api/v1/security/applications/%s/keys", app.UID)

	req := createRequest(http.MethodPost, url, body)
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
	require.Equal(s.T(), apiKeyResponse.AppID, app.UID)
}

func (s *SecurityIntegrationTestSuite) Test_RevokeAPIKey() {
	expectedStatusCode := http.StatusOK

	// Just Before.
	apiKey, _ := testdb.SeedAPIKey(s.DB, s.DefaultGroup, uuid.NewString(), "test", "api")

	url := fmt.Sprintf("/api/v1/security/keys/%s/revoke", apiKey.UID)

	req := createRequest(http.MethodPut, url, nil)
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

	url := fmt.Sprintf("/api/v1/security/keys/%s", apiKey.UID)
	req := createRequest(http.MethodGet, url, nil)
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

func (s *SecurityIntegrationTestSuite) Test_GetAPIKeyByID_APIKey_not_found() {
	expectedStatusCode := http.StatusBadRequest

	url := fmt.Sprintf("/api/v1/security/keys/%s", uuid.NewString())
	req := createRequest(http.MethodGet, url, nil)
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

	url := fmt.Sprintf("/api/v1/security/keys/%s", apiKey.UID)
	req := createRequest(http.MethodPut, url, body)
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

func (s *SecurityIntegrationTestSuite) Test_UpdateAPIKey_APIKey_not_found() {
	expectedStatusCode := http.StatusBadRequest

	bodyStr := `{"role":{"type":"api","groups":["%s"]}}`
	body := serialize(bodyStr, s.DefaultGroup.UID)

	url := fmt.Sprintf("/api/v1/security/keys/%s", uuid.NewString())

	req := createRequest(http.MethodPut, url, body)
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

	req := createRequest(http.MethodGet, "/api/v1/security/keys", body)
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
