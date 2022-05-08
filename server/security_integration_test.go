//go:build integration
// +build integration

package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/frain-dev/convoy/auth"
	"github.com/frain-dev/convoy/server/models"
	"github.com/google/uuid"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/server/testdb"
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
	body := &models.APIKey{
		Name: "default_api_key",
		Role: auth.Role{
			Type:   auth.RoleUIAdmin,
			Groups: []string{s.DefaultGroup.UID},
		},
		Type:      "api_key",
		ExpiresAt: time.Now().Add(time.Hour),
	}

	req, w := newRequestAndResponder(http.MethodPost, "/api/v1/security/keys", serialize(s.T(), body))
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
	require.Equal(s.T(), apiKey.Name, apiKeyResponse.Name)
	require.Equal(s.T(), apiKey.Role, apiKeyResponse.Role)
	require.Equal(s.T(), apiKey.Type, apiKeyResponse.Type)

	// for some reason, comparing this directly will always fail, this difference is in nanoseconds
	// and i believe it comes from our use of primitive.Datetime
	require.True(s.T(), apiKey.ExpiresAt.Time().Sub(body.ExpiresAt) < time.Minute)

	require.NotEmpty(s.T(), apiKey.MaskID)
	require.NotEmpty(s.T(), apiKey.Salt)
	require.NotEmpty(s.T(), apiKey.Hash)
}

func (s *SecurityIntegrationTestSuite) Test_CreateAppPortalAPIKey() {
	expectedStatusCode := http.StatusCreated

	// Just Before.
	app, _ := testdb.SeedApplication(s.DB, s.DefaultGroup, uuid.NewString(), true)

	// Arrange Request.
	body := &models.APIKey{
		Name: "default_api_key",
		Role: auth.Role{
			Type:   auth.RoleUIAdmin,
			Groups: []string{s.DefaultGroup.UID},
		},
		Type:      "api_key",
		ExpiresAt: time.Now().Add(time.Hour),
	}

	url := fmt.Sprintf("/api/v1/security/applications/%s/keys", app.UID)

	req, w := newRequestAndResponder(http.MethodPost, url, serialize(s.T(), body))

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)

	// Deep Assert.
	var apiKeyResponse models.PortalAPIKeyResponse
	parseResponse(s.T(), w.Result(), &apiKeyResponse)
	require.NotEmpty(s.T(), apiKeyResponse.Key)
	require.NotEmpty(s.T(), apiKeyResponse.Key)
	require.Equal(s.T(), apiKeyResponse.Url, fmt.Sprintf("https://app.convoy.io/app-portal/%s?groupID=%s&appId=%s", apiKeyResponse.Key, s.DefaultGroup.UID, app.UID))
	require.Equal(s.T(), body.Role.Groups, apiKeyResponse.Role.Groups)
	require.Equal(s.T(), []string{app.UID}, apiKeyResponse.Role.Apps)
	require.Equal(s.T(), body.Role.Type, apiKeyResponse.Role.Type)
	require.Equal(s.T(), apiKeyResponse.Type, "app_portal")
	require.Equal(s.T(), apiKeyResponse.GroupID, s.DefaultGroup.UID)
	require.Equal(s.T(), apiKeyResponse.AppID, app.UID)
}

func (s *SecurityIntegrationTestSuite) Test_RevokeAPIKey() {
	expectedStatusCode := http.StatusOK

	// Just Before.
	apiKey, _ := testdb.SeedAPIKey(s.DB, s.DefaultGroup, uuid.NewString(), "test", "api")

	url := fmt.Sprintf("/api/v1/security/keys/%s/revoke", apiKey.UID)

	req, w := newRequestAndResponder(http.MethodPut, url, nil)

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
	req, w := newRequestAndResponder(http.MethodGet, url, nil)

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)

	// Deep Assert.
	var apiKeyResponse models.APIKeyByIDResponse
	parseResponse(s.T(), w.Result(), &apiKeyResponse)
	require.Equal(s.T(), apiKeyResponse.Type, apiKey.Type)
	require.Equal(s.T(), apiKeyResponse.Role, apiKey.Role)
	require.Equal(s.T(), apiKeyResponse.UID, apiKey.UID)
	require.Equal(s.T(), apiKeyResponse.ExpiresAt, apiKey.ExpiresAt)
	require.Equal(s.T(), apiKeyResponse.CreatedAt, apiKey.CreatedAt)
	require.Equal(s.T(), apiKeyResponse.UpdatedAt, apiKey.UpdatedAt)
}

func (s *SecurityIntegrationTestSuite) Test_GetAPIKeyByID_APIKey_not_found() {
	expectedStatusCode := http.StatusBadRequest

	url := fmt.Sprintf("/api/v1/security/keys/%s", uuid.NewString())
	req, w := newRequestAndResponder(http.MethodGet, url, nil)

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)
}

func (s *SecurityIntegrationTestSuite) Test_UpdateAPIKey() {
	expectedStatusCode := http.StatusOK

	// Just Before.
	apiKey, _ := testdb.SeedAPIKey(s.DB, s.DefaultGroup, uuid.NewString(), "test", "api")

	body := map[string]auth.Role{
		"role": {
			Type:   auth.RoleAPI,
			Groups: []string{s.DefaultGroup.UID},
		},
	}

	url := fmt.Sprintf("/api/v1/security/keys/%s", apiKey.UID)
	req, w := newRequestAndResponder(http.MethodPut, url, serialize(s.T(), body))

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)

	// Deep Assert.

	a, err := s.DB.APIRepo().FindAPIKeyByID(context.Background(), apiKey.UID)
	require.NoError(s.T(), err)
	require.Equal(s.T(), a.Role, body["role"])
}

func (s *SecurityIntegrationTestSuite) Test_UpdateAPIKey_APIKey_not_found() {
	expectedStatusCode := http.StatusBadRequest

	body := map[string]auth.Role{
		"role": {
			Type:   auth.RoleAPI,
			Groups: []string{s.DefaultGroup.UID},
		},
	}

	url := fmt.Sprintf("/api/v1/security/keys/%s", uuid.NewString())

	req, w := newRequestAndResponder(http.MethodPut, url, serialize(s.T(), body))

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

	body := &auth.Role{
		Type:   auth.RoleAPI,
		Groups: []string{uuid.NewString()},
	}

	req, w := newRequestAndResponder(http.MethodGet, "/api/v1/security/keys", serialize(s.T(), body))

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)

	var apiKeyResponse []models.APIKeyByIDResponse
	pagedResp := &pagedResponse{Content: &apiKeyResponse}
	parseResponse(s.T(), w.Result(), pagedResp)
	require.Equal(s.T(), 3, len(apiKeyResponse))
}

func newRequestAndResponder(method string, url string, body io.Reader) (*http.Request, *httptest.ResponseRecorder) {
	req := httptest.NewRequest(method, url, body)
	req.SetBasicAuth("test", "test")
	req.Header.Add("Content-Type", "application/json")

	return req, httptest.NewRecorder()
}

func serialize(t *testing.T, obj interface{}) io.Reader {
	r, err := json.Marshal(obj)
	require.NoError(t, err)
	return bytes.NewBuffer(r)
}

func (s *SecurityIntegrationTestSuite) TearDownTest() {
	testdb.PurgeDB(s.DB)
}

func TestSecurityIntegrationTestSuiteTest(t *testing.T) {
	suite.Run(t, new(SecurityIntegrationTestSuite))
}
