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

	url := fmt.Sprintf("security/applications/%s/keys", app.UID)

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
	require.Equal(s.T(), apiKeyResponse.Url, fmt.Sprintf("https://app.convo.io/app-portal/%s?groupID=%s&appId=%s", apiKeyResponse.Key, s.DefaultGroup.UID, app.UID))
	require.Equal(s.T(), body.Role, apiKeyResponse.Role)
	require.Equal(s.T(), apiKeyResponse.Type, "app_portal")
	require.Equal(s.T(), apiKeyResponse.GroupID, s.DefaultGroup.UID)
	require.Equal(s.T(), apiKeyResponse.AppID, app.UID)
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
