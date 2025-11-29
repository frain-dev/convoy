package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/frain-dev/convoy/api/testdb"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/database"
	"github.com/frain-dev/convoy/database/postgres"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/internal/pkg/metrics"
)

type IngestIntegrationTestSuite struct {
	suite.Suite
	DB             database.Database
	Router         http.Handler
	ConvoyApp      *ApplicationHandler
	DefaultProject *datastore.Project
}

func (i *IngestIntegrationTestSuite) SetupSuite() {
	i.ConvoyApp = buildServer(i.T())
	i.Router = i.ConvoyApp.BuildControlPlaneRoutes()
}

func (i *IngestIntegrationTestSuite) SetupTest() {
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
	maskID := ulid.Make().String()
	sourceID := ulid.Make().String()
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
	maskID := ulid.Make().String()
	sourceID := ulid.Make().String()
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

	body := serialize(`{ "name": "convoy" }`)
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
	maskID := ulid.Make().String()
	sourceID := ulid.Make().String()
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

	body := serialize(`{ "name": "convoy" }`)
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
	maskID := ulid.Make().String()
	sourceID := ulid.Make().String()
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

	body := serialize(`{ "name": "convoy" }`)
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
	maskID := ulid.Make().String()
	sourceID := ulid.Make().String()
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

	body := serialize(`{ "name": "convoy" }`)
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
	maskID := ulid.Make().String()
	sourceID := ulid.Make().String()
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

	body := serialize(`{ "name": "convoy" }`)

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
	maskID := ulid.Make().String()
	sourceID := ulid.Make().String()
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

	body := serialize(`{ "name": "convoy" }`)

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
	maskID := ulid.Make().String()
	sourceID := ulid.Make().String()
	expectedStatusCode := http.StatusOK
	resp := "[accepted]"
	// Just Before
	v := &datastore.VerifierConfig{
		Type: datastore.NoopVerifier,
	}
	_, _ = testdb.SeedSource(i.ConvoyApp.A.DB, i.DefaultProject, sourceID, maskID, "", v, resp, "")

	body := serialize(`{ "name": "convoy" }`)

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
	maskID := ulid.Make().String()
	sourceID := ulid.Make().String()
	expectedStatusCode := http.StatusOK

	// Just Before
	v := &datastore.VerifierConfig{
		Type: datastore.NoopVerifier,
	}
	_, _ = testdb.SeedSource(i.ConvoyApp.A.DB, i.DefaultProject, sourceID, maskID, "", v, "", "")

	body := serialize(``)

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

	// Check the length of the request body
	require.Equal(i.T(), float64(2), response["data"].(float64))
}

func (i *IngestIntegrationTestSuite) Test_IngestEvent_WriteToQueueFailed() {
	i.T().Skip("Depends on mocking")
}

func (i *IngestIntegrationTestSuite) Test_IngestEvent_PayloadExceedsConfiguredPayloadSize() {
	maskID := ulid.Make().String()
	sourceID := ulid.Make().String()

	// Just Before
	v := &datastore.VerifierConfig{
		Type: datastore.NoopVerifier,
	}
	_, _ = testdb.SeedSource(i.ConvoyApp.A.DB, i.DefaultProject, sourceID, maskID, "", v, "", "")

	url := fmt.Sprintf("/ingest/%s", maskID)
	body := serialize(`{ "payload": %s }`, strings.Repeat("abcdef", 100))
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
