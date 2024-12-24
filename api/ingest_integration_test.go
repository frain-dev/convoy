//go:build integration
// +build integration

package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/frain-dev/convoy/database"
	"github.com/frain-dev/convoy/database/postgres"
	"github.com/frain-dev/convoy/internal/pkg/metrics"

	"github.com/frain-dev/convoy/api/testdb"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/datastore"
	"github.com/stretchr/testify/require"

	"github.com/stretchr/testify/suite"
)

type IngestIntegrationTestSuite struct {
	suite.Suite
	DB             database.Database
	Router         http.Handler
	ConvoyApp      *ApplicationHandler
	DefaultProject *datastore.Project
}

func (i *IngestIntegrationTestSuite) SetupSuite() {
	i.DB = getDB()
	i.ConvoyApp = buildServer()
	i.Router = i.ConvoyApp.BuildControlPlaneRoutes()
}

func (i *IngestIntegrationTestSuite) SetupTest() {
	testdb.PurgeDB(i.T(), i.DB)

	user, err := testdb.SeedDefaultUser(i.ConvoyApp.A.DB)
	require.NoError(i.T(), err)

	org, err := testdb.SeedDefaultOrganisation(i.ConvoyApp.A.DB, user)
	require.NoError(i.T(), err)

	// Setup Default Project.
	i.DefaultProject, err = testdb.SeedDefaultProject(i.ConvoyApp.A.DB, org.UID)
	require.NoError(i.T(), err)

	// Setup Config.
	err = config.LoadConfig("./testdata/Auth_Config/full-convoy.json")
	require.NoError(i.T(), err)

	apiRepo := postgres.NewAPIKeyRepo(i.ConvoyApp.A.DB)
	userRepo := postgres.NewUserRepo(i.ConvoyApp.A.DB)
	portalLinkRepo := postgres.NewPortalLinkRepo(i.ConvoyApp.A.DB)
	initRealmChain(i.T(), apiRepo, userRepo, portalLinkRepo, i.ConvoyApp.A.Cache)
}

func (i *IngestIntegrationTestSuite) TearDownTest() {
	testdb.PurgeDB(i.T(), i.DB)
	metrics.Reset()
}

func (i *IngestIntegrationTestSuite) Test_IngestEvent_BadMaskID() {
	maskID := "12345"
	expectedStatusCode := http.StatusNotFound

	// Arrange Request.
	url := fmt.Sprintf("/ingest/%s", maskID)
	req := createRequest(http.MethodPost, url, "", nil)
	w := httptest.NewRecorder()

	// Act.
	i.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(i.T(), expectedStatusCode, w.Code)
}

func (i *IngestIntegrationTestSuite) Test_IngestEvent_NotHTTPSource() {
	maskID := "123456"
	sourceID := "123456789"
	expectedStatusCode := http.StatusBadRequest

	// Just Before
	v := &datastore.VerifierConfig{
		Type: datastore.HMacVerifier,
		HMac: &datastore.HMac{
			Header:   "X-Convoy-Signature",
			Hash:     "SHA512",
			Secret:   "Convoy",
			Encoding: "hex",
		},
	}
	_, _ = testdb.SeedSource(i.ConvoyApp.A.DB, i.DefaultProject, sourceID, maskID, "non-http", v, "", "")

	// Arrange Request.
	url := fmt.Sprintf("/ingest/%s", maskID)
	req := createRequest(http.MethodPost, url, "", nil)
	w := httptest.NewRecorder()

	// Act.
	i.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(i.T(), expectedStatusCode, w.Code)
}

func (i *IngestIntegrationTestSuite) Test_IngestEvent_GoodHmac() {
	maskID := "123456"
	sourceID := "123456789"
	expectedStatusCode := http.StatusOK

	// Just Before
	v := &datastore.VerifierConfig{
		Type: datastore.HMacVerifier,
		HMac: &datastore.HMac{
			Header:   "X-Convoy-Signature",
			Hash:     "SHA512",
			Secret:   "Convoy",
			Encoding: "hex",
		},
	}
	_, _ = testdb.SeedSource(i.ConvoyApp.A.DB, i.DefaultProject, sourceID, maskID, "", v, "", "")

	bodyStr := `{ "name": "convoy" }`
	body := serialize(bodyStr)
	auth := "4471d491560781f633f3e53fb68574084adf5b803de16e12c88d49e74e" +
		"13bcafa5ddad1247dffa71479ebd7a800c8af16f6f90a1be5a946140308bac4bd60260"

	// Arrange Request.
	url := fmt.Sprintf("/ingest/%s", maskID)
	req := createRequest(http.MethodPost, url, "", body)
	req.Header.Add("X-Convoy-Signature", auth)

	w := httptest.NewRecorder()

	// Act.
	i.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(i.T(), expectedStatusCode, w.Code)
}

func (i *IngestIntegrationTestSuite) Test_IngestEvent_BadHmac() {
	maskID := "123456"
	sourceID := "123456789"
	expectedStatusCode := http.StatusBadRequest

	// Just Before
	v := &datastore.VerifierConfig{
		Type: datastore.HMacVerifier,
		HMac: &datastore.HMac{
			Header:   "X-Convoy-Signature",
			Hash:     "SHA512",
			Secret:   "Convoy",
			Encoding: "hex",
		},
	}
	_, _ = testdb.SeedSource(i.ConvoyApp.A.DB, i.DefaultProject, sourceID, maskID, "", v, "", "")

	bodyStr := `{ "name": "convoy" }`
	body := serialize(bodyStr)
	auth := "hash with characters outside the hex range"

	// Arrange Request.
	url := fmt.Sprintf("/ingest/%s", maskID)
	req := createRequest(http.MethodPost, url, "", body)
	req.Header.Add("X-Convoy-Signature", auth)

	w := httptest.NewRecorder()

	// Act.
	i.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(i.T(), expectedStatusCode, w.Code)
}

func (i *IngestIntegrationTestSuite) Test_IngestEvent_GoodAPIKey() {
	maskID := "123456"
	sourceID := "123456789"
	expectedStatusCode := http.StatusOK

	// Just Before
	v := &datastore.VerifierConfig{
		Type: datastore.APIKeyVerifier,
		ApiKey: &datastore.ApiKey{
			HeaderName:  "X-Convoy-Signature",
			HeaderValue: "Convoy",
		},
	}
	_, _ = testdb.SeedSource(i.ConvoyApp.A.DB, i.DefaultProject, sourceID, maskID, "", v, "", "")

	bodyStr := `{ "name": "convoy" }`
	body := serialize(bodyStr)

	// Arrange Request.
	url := fmt.Sprintf("/ingest/%s", maskID)
	req := createRequest(http.MethodPost, url, "", body)
	req.Header.Add("X-Convoy-Signature", "Convoy")

	w := httptest.NewRecorder()

	// Act.
	i.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(i.T(), expectedStatusCode, w.Code)
}

func (i *IngestIntegrationTestSuite) Test_IngestEvent_BadAPIKey() {
	maskID := "123456"
	sourceID := "123456789"
	expectedStatusCode := http.StatusBadRequest

	// Just Before
	v := &datastore.VerifierConfig{
		Type: datastore.APIKeyVerifier,
		ApiKey: &datastore.ApiKey{
			HeaderName:  "X-Convoy-Signature",
			HeaderValue: "Convoy",
		},
	}
	_, _ = testdb.SeedSource(i.ConvoyApp.A.DB, i.DefaultProject, sourceID, maskID, "", v, "", "")

	bodyStr := `{ "name": "convoy" }`
	body := serialize(bodyStr)

	// Arrange Request.
	url := fmt.Sprintf("/ingest/%s", maskID)
	req := createRequest(http.MethodPost, url, "", body)
	req.Header.Add("X-Convoy-Signature", "Convoy X")

	w := httptest.NewRecorder()

	// Act.
	i.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(i.T(), expectedStatusCode, w.Code)
}

func (i *IngestIntegrationTestSuite) Test_IngestEvent_GoodBasicAuth() {
	maskID := "123456"
	sourceID := "123456789"
	expectedStatusCode := http.StatusOK

	// Just Before
	v := &datastore.VerifierConfig{
		Type: datastore.BasicAuthVerifier,
		BasicAuth: &datastore.BasicAuth{
			UserName: "Convoy",
			Password: "Convoy",
		},
	}
	_, _ = testdb.SeedSource(i.ConvoyApp.A.DB, i.DefaultProject, sourceID, maskID, "", v, "", "")

	bodyStr := `{ "name": "convoy" }`
	body := serialize(bodyStr)

	// Arrange Request.
	url := fmt.Sprintf("/ingest/%s", maskID)
	req := createRequest(http.MethodPost, url, "", body)
	req.SetBasicAuth("Convoy", "Convoy")

	w := httptest.NewRecorder()

	// Act.
	i.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(i.T(), expectedStatusCode, w.Code)
}

func (i *IngestIntegrationTestSuite) Test_IngestEvent_BadBasicAuth() {
	maskID := "123456"
	sourceID := "123456789"
	expectedStatusCode := http.StatusBadRequest

	// Just Before
	v := &datastore.VerifierConfig{
		Type: datastore.BasicAuthVerifier,
		BasicAuth: &datastore.BasicAuth{
			UserName: "Convoy",
			Password: "Convoy",
		},
	}
	_, _ = testdb.SeedSource(i.ConvoyApp.A.DB, i.DefaultProject, sourceID, maskID, "", v, "", "")

	bodyStr := `{ "name": "convoy" }`
	body := serialize(bodyStr)

	// Arrange Request.
	url := fmt.Sprintf("/ingest/%s", maskID)
	req := createRequest(http.MethodPost, url, "", body)
	req.SetBasicAuth("Convoy X", "Convoy X")

	w := httptest.NewRecorder()

	// Act.
	i.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(i.T(), expectedStatusCode, w.Code)
}

func (i *IngestIntegrationTestSuite) Test_IngestEvent_NoopVerifier() {
	maskID := "123456"
	sourceID := "123456789"
	expectedStatusCode := http.StatusOK
	resp := "[accepted]"
	// Just Before
	v := &datastore.VerifierConfig{
		Type: datastore.NoopVerifier,
	}
	_, _ = testdb.SeedSource(i.ConvoyApp.A.DB, i.DefaultProject, sourceID, maskID, "", v, resp, "")

	bodyStr := `{ "name": "convoy" }`
	body := serialize(bodyStr)

	// Arrange Request.
	url := fmt.Sprintf("/ingest/%s", maskID)
	req := createRequest(http.MethodPost, url, "", body)

	w := httptest.NewRecorder()

	// Act.
	i.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(i.T(), expectedStatusCode, w.Code)
	require.Equal(i.T(), w.Body.String(), resp)
}

func (i *IngestIntegrationTestSuite) Test_IngestEvent_NoopVerifier_EmptyRequestBody() {
	maskID := "123456"
	sourceID := "123456789"
	expectedStatusCode := http.StatusOK

	// Just Before
	v := &datastore.VerifierConfig{
		Type: datastore.NoopVerifier,
	}
	_, _ = testdb.SeedSource(i.ConvoyApp.A.DB, i.DefaultProject, sourceID, maskID, "", v, "", "")

	bodyStr := ``
	body := serialize(bodyStr)

	// Arrange Request.
	url := fmt.Sprintf("/ingest/%s", maskID)
	req := createRequest(http.MethodPost, url, "", body)

	w := httptest.NewRecorder()

	// Act.
	i.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(i.T(), expectedStatusCode, w.Code)

	var response map[string]interface{}
	err := json.NewDecoder(w.Body).Decode(&response)
	require.NoError(i.T(), err)

	// Check the lenght of the request body
	require.Equal(i.T(), float64(2), response["data"].(float64))
}

func (i *IngestIntegrationTestSuite) Test_IngestEvent_WriteToQueueFailed() {
	i.T().Skip("Depends on mocking")
}

func (i *IngestIntegrationTestSuite) Test_IngestEvent_PayloadExceedsConfiguredPayloadSize() {
	maskID := "123456"
	sourceID := "123456789"

	// Just Before
	v := &datastore.VerifierConfig{
		Type: datastore.NoopVerifier,
	}
	_, _ = testdb.SeedSource(i.ConvoyApp.A.DB, i.DefaultProject, sourceID, maskID, "", v, "", "")

	url := fmt.Sprintf("/ingest/%s", maskID)
	bodyStr := fmt.Sprintf(`{ "payload": %s }`, strings.Repeat("abcdef", 100))
	body := serialize(bodyStr)
	req := createRequest(http.MethodPost, url, "", body)

	w := httptest.NewRecorder()

	// Act.
	i.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(i.T(), http.StatusRequestEntityTooLarge, w.Code)
}

func TestIngestIntegrationTestSuite(t *testing.T) {
	suite.Run(t, new(IngestIntegrationTestSuite))
}
