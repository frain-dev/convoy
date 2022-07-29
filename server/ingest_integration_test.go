//go:build integration
// +build integration

package server

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/frain-dev/convoy/internal/pkg/metrics"

	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/datastore"
	convoyMongo "github.com/frain-dev/convoy/datastore/mongo"
	"github.com/frain-dev/convoy/server/testdb"
	"github.com/stretchr/testify/require"

	"github.com/stretchr/testify/suite"
)

type IngestIntegrationTestSuite struct {
	suite.Suite
	DB           convoyMongo.Client
	Router       http.Handler
	ConvoyApp    *ApplicationHandler
	DefaultGroup *datastore.Group
}

func (s *IngestIntegrationTestSuite) SetupSuite() {
	s.DB = getDB()
	s.ConvoyApp = buildServer()
	s.Router = s.ConvoyApp.BuildRoutes()
}

func (i *IngestIntegrationTestSuite) SetupTest() {
	testdb.PurgeDB(i.DB)

	// Setup Default Group.
	i.DefaultGroup, _ = testdb.SeedDefaultGroup(i.DB, "")

	// Setup Config.
	err := config.LoadConfig("./testdata/Auth_Config/full-convoy.json")
	require.NoError(i.T(), err)

	initRealmChain(i.T(), i.DB.APIRepo(), i.DB.UserRepo(), i.ConvoyApp.S.Cache)
}

func (i *IngestIntegrationTestSuite) TearDownTest() {
	testdb.PurgeDB(i.DB)
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
	_, _ = testdb.SeedSource(i.DB, i.DefaultGroup, sourceID, maskID, "non-http", v)

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
	_, _ = testdb.SeedSource(i.DB, i.DefaultGroup, sourceID, maskID, "", v)

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
	_, _ = testdb.SeedSource(i.DB, i.DefaultGroup, sourceID, maskID, "", v)

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
	_, _ = testdb.SeedSource(i.DB, i.DefaultGroup, sourceID, maskID, "", v)

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
	_, _ = testdb.SeedSource(i.DB, i.DefaultGroup, sourceID, maskID, "", v)

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
	_, _ = testdb.SeedSource(i.DB, i.DefaultGroup, sourceID, maskID, "", v)

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
	_, _ = testdb.SeedSource(i.DB, i.DefaultGroup, sourceID, maskID, "", v)

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

func TestIngestIntegrationTestSuite(t *testing.T) {
	suite.Run(t, new(IngestIntegrationTestSuite))
}
