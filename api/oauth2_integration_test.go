package api

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"gopkg.in/guregu/null.v4"

	"github.com/frain-dev/convoy/api/models"
	"github.com/frain-dev/convoy/api/testdb"
	"github.com/frain-dev/convoy/cache"
	mcache "github.com/frain-dev/convoy/cache/memory"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/database"
	"github.com/frain-dev/convoy/database/postgres"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/internal/pkg/fflag"
	"github.com/frain-dev/convoy/internal/pkg/keys"
	"github.com/frain-dev/convoy/internal/pkg/metrics"
	"github.com/frain-dev/convoy/internal/portal_links"
	"github.com/frain-dev/convoy/pkg/log"
	"github.com/frain-dev/convoy/services"
)

type OAuth2IntegrationTestSuite struct {
	suite.Suite
	DB              database.Database
	Router          http.Handler
	ConvoyApp       *ApplicationHandler
	AuthenticatorFn AuthenticatorFn
	DefaultOrg      *datastore.Organisation
	DefaultProject  *datastore.Project
	DefaultUser     *datastore.User
	OAuth2Server    *httptest.Server
	WebhookServer   *httptest.Server
	MemoryCache     cache.Cache
	// Track requests to servers
	OAuth2TokenRequests  []map[string]string
	WebhookRequests      []http.Request
	OAuth2TokenCallCount int
	WebhookCallCount     int
}

func (s *OAuth2IntegrationTestSuite) SetupSuite() {
	s.ConvoyApp = buildServer(s.T())
	s.DB = s.ConvoyApp.A.DB
	s.Router = s.ConvoyApp.BuildControlPlaneRoutes()
}

func (s *OAuth2IntegrationTestSuite) SetupTest() {
	testdb.PurgeDB(s.T(), s.DB)
	s.DB = s.ConvoyApp.A.DB

	// Reset tracking
	s.OAuth2TokenRequests = []map[string]string{}
	s.WebhookRequests = []http.Request{}
	s.OAuth2TokenCallCount = 0
	s.WebhookCallCount = 0

	user, err := testdb.SeedDefaultUser(s.ConvoyApp.A.DB)
	require.NoError(s.T(), err)
	s.DefaultUser = user

	org, err := testdb.SeedDefaultOrganisation(s.ConvoyApp.A.DB, user)
	require.NoError(s.T(), err)
	s.DefaultOrg = org

	// Setup Default Project.
	s.DefaultProject, err = testdb.SeedDefaultProject(s.ConvoyApp.A.DB, org.UID)
	require.NoError(s.T(), err)

	// Enable OAuth2 feature flag for the organization
	err = enableOAuth2FeatureFlag(s.T(), s.ConvoyApp.A.DB, org.UID)
	require.NoError(s.T(), err)

	s.AuthenticatorFn = authenticateRequest(&models.LoginUser{
		Username: user.Email,
		Password: testdb.DefaultUserPassword,
	})

	// Setup Config.
	err = config.LoadConfig("./testdata/Auth_Config/full-convoy-with-jwt-realm.json")
	require.NoError(s.T(), err)

	_ = config.LoadCaCert("", "")

	apiRepo := postgres.NewAPIKeyRepo(s.ConvoyApp.A.DB)
	userRepo := postgres.NewUserRepo(s.ConvoyApp.A.DB)
	portalLinkRepo := portal_links.New(s.ConvoyApp.A.Logger, s.ConvoyApp.A.DB)
	initRealmChain(s.T(), apiRepo, userRepo, portalLinkRepo, s.ConvoyApp.A.Cache)

	// Setup in-memory cache for OAuth2 token service
	s.MemoryCache = mcache.NewMemoryCache()

	// Setup mock OAuth2 token server
	s.setupOAuth2Server()

	// Setup mock webhook server to receive deliveries
	s.setupWebhookServer()
}

func (s *OAuth2IntegrationTestSuite) TearDownTest() {
	if s.OAuth2Server != nil {
		s.OAuth2Server.Close()
	}
	if s.WebhookServer != nil {
		s.WebhookServer.Close()
	}
	testdb.PurgeDB(s.T(), s.DB)
	metrics.Reset()
}

func (s *OAuth2IntegrationTestSuite) setupOAuth2Server() {
	s.OAuth2Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.OAuth2TokenCallCount++

		require.Equal(s.T(), "POST", r.Method)
		require.Equal(s.T(), "application/x-www-form-urlencoded", r.Header.Get("Content-Type"))

		err := r.ParseForm()
		require.NoError(s.T(), err)

		require.Equal(s.T(), "client_credentials", r.Form.Get("grant_type"))

		// Track the request
		requestData := make(map[string]string)
		requestData["grant_type"] = r.Form.Get("grant_type")
		requestData["client_id"] = r.Form.Get("client_id")
		requestData["client_secret"] = r.Form.Get("client_secret")
		requestData["client_assertion"] = r.Form.Get("client_assertion")
		requestData["client_assertion_type"] = r.Form.Get("client_assertion_type")
		s.OAuth2TokenRequests = append(s.OAuth2TokenRequests, requestData)

		// Check for either client_secret or client_assertion
		clientID := r.Form.Get("client_id")
		require.NotEmpty(s.T(), clientID)

		// Validate authentication
		hasSecret := r.Form.Get("client_secret") != ""
		hasAssertion := r.Form.Get("client_assertion") != ""
		require.True(s.T(), hasSecret || hasAssertion, "Either client_secret or client_assertion must be provided")

		if hasAssertion {
			require.Equal(s.T(), "urn:ietf:params:oauth:client-assertion-type:jwt-bearer", r.Form.Get("client_assertion_type"))
			require.NotEmpty(s.T(), r.Form.Get("client_assertion"))
		}

		response := map[string]interface{}{
			"access_token": fmt.Sprintf("test-access-token-%s", clientID),
			"token_type":   "Bearer",
			"expires_in":   3600,
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(response)
	}))
}

func (s *OAuth2IntegrationTestSuite) setupWebhookServer() {
	s.WebhookServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.WebhookCallCount++

		// Track the request (clone headers and body for inspection)
		reqCopy := *r
		reqCopy.Header = r.Header.Clone()
		s.WebhookRequests = append(s.WebhookRequests, reqCopy)

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"received": true,
		})
	}))
}

func (s *OAuth2IntegrationTestSuite) generateTestJWK() *datastore.OAuth2SigningKey {
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(s.T(), err)

	// Convert to JWK format
	xBytes := privateKey.PublicKey.X.Bytes()
	yBytes := privateKey.PublicKey.Y.Bytes()
	dBytes := privateKey.D.Bytes()

	// Pad to 32 bytes for P-256
	xPadded := make([]byte, 32)
	yPadded := make([]byte, 32)
	dPadded := make([]byte, 32)

	copy(xPadded[32-len(xBytes):], xBytes)
	copy(yPadded[32-len(yBytes):], yBytes)
	copy(dPadded[32-len(dBytes):], dBytes)

	return &datastore.OAuth2SigningKey{
		Kty: "EC",
		Crv: "P-256",
		X:   base64.RawURLEncoding.EncodeToString(xPadded),
		Y:   base64.RawURLEncoding.EncodeToString(yPadded),
		D:   base64.RawURLEncoding.EncodeToString(dPadded),
		Kid: "test-key-id",
	}
}

func (s *OAuth2IntegrationTestSuite) Test_CreateEndpoint_WithOAuth2SharedSecret() {
	endpointTitle := fmt.Sprintf("OAuth2-Endpoint-%s", ulid.Make().String())
	expectedStatusCode := http.StatusCreated

	// Arrange Request.
	url := fmt.Sprintf("/ui/organisations/%s/projects/%s/endpoints", s.DefaultProject.OrganisationID, s.DefaultProject.UID)
	plainBody := fmt.Sprintf(`{
		"name": "%s",
		"description": "test endpoint with oauth2",
		"url": "%s",
		"authentication": {
			"type": "oauth2",
			"oauth2": {
				"url": "%s",
				"client_id": "test-client-id",
				"authentication_type": "shared_secret",
				"client_secret": "test-client-secret",
				"grant_type": "client_credentials"
			}
		}
	}`, endpointTitle, s.WebhookServer.URL, s.OAuth2Server.URL)
	body := strings.NewReader(plainBody)
	req := createRequest(http.MethodPost, url, "", body)
	err := s.AuthenticatorFn(req, s.Router)
	require.NoError(s.T(), err)

	w := httptest.NewRecorder()

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)

	// Deep Assert.
	var endpoint datastore.Endpoint
	parseResponse(s.T(), w.Result(), &endpoint)
	require.NotEmpty(s.T(), endpoint.UID)
	require.Equal(s.T(), endpointTitle, endpoint.Name)
	require.NotNil(s.T(), endpoint.Authentication)
	require.Equal(s.T(), datastore.OAuth2Authentication, endpoint.Authentication.Type)
	require.NotNil(s.T(), endpoint.Authentication.OAuth2)
	require.Equal(s.T(), s.OAuth2Server.URL, endpoint.Authentication.OAuth2.URL)
	require.Equal(s.T(), "test-client-id", endpoint.Authentication.OAuth2.ClientID)
	require.Equal(s.T(), datastore.SharedSecretAuth, endpoint.Authentication.OAuth2.AuthenticationType)
}

func (s *OAuth2IntegrationTestSuite) Test_CreateEndpoint_WithOAuth2ClientAssertion() {
	endpointTitle := fmt.Sprintf("OAuth2-Assertion-Endpoint-%s", ulid.Make().String())
	expectedStatusCode := http.StatusCreated

	signingKey := s.generateTestJWK()

	// Arrange Request.
	url := fmt.Sprintf("/ui/organisations/%s/projects/%s/endpoints", s.DefaultProject.OrganisationID, s.DefaultProject.UID)
	plainBody := fmt.Sprintf(`{
		"name": "%s",
		"description": "test endpoint with oauth2 client assertion",
		"url": "%s",
		"authentication": {
			"type": "oauth2",
			"oauth2": {
				"url": "%s",
				"client_id": "test-client-id-assertion",
				"authentication_type": "client_assertion",
				"signing_key": {
					"kty": "%s",
					"crv": "%s",
					"kid": "%s",
					"x": "%s",
					"y": "%s",
					"d": "%s"
				},
				"signing_algorithm": "ES256",
				"issuer": "test-client-id-assertion",
				"subject": "test-client-id-assertion",
				"grant_type": "client_credentials"
			}
		}
	}`, endpointTitle, s.WebhookServer.URL, s.OAuth2Server.URL,
		signingKey.Kty, signingKey.Crv, signingKey.Kid,
		signingKey.X, signingKey.Y, signingKey.D)
	body := strings.NewReader(plainBody)
	req := createRequest(http.MethodPost, url, "", body)
	err := s.AuthenticatorFn(req, s.Router)
	require.NoError(s.T(), err)

	w := httptest.NewRecorder()

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)

	// Deep Assert.
	var endpoint datastore.Endpoint
	parseResponse(s.T(), w.Result(), &endpoint)
	require.NotEmpty(s.T(), endpoint.UID)
	require.Equal(s.T(), endpointTitle, endpoint.Name)
	require.NotNil(s.T(), endpoint.Authentication)
	require.Equal(s.T(), datastore.OAuth2Authentication, endpoint.Authentication.Type)
	require.NotNil(s.T(), endpoint.Authentication.OAuth2)
	require.Equal(s.T(), datastore.ClientAssertionAuth, endpoint.Authentication.OAuth2.AuthenticationType)
	require.NotNil(s.T(), endpoint.Authentication.OAuth2.SigningKey)
}

func (s *OAuth2IntegrationTestSuite) Test_OAuth2TokenService_Integration() {
	// Create endpoint with OAuth2 authentication via repository
	endpointID := ulid.Make().String()
	clientID := "test-client-id"
	endpoint := &datastore.Endpoint{
		UID:       endpointID,
		Name:      "OAuth2 Test Endpoint",
		ProjectID: s.DefaultProject.UID,
		Url:       s.WebhookServer.URL,
		Status:    datastore.ActiveEndpointStatus,
		Secrets: []datastore.Secret{
			{Value: "1234"},
		},
		Authentication: &datastore.EndpointAuthentication{
			Type: datastore.OAuth2Authentication,
			OAuth2: &datastore.OAuth2{
				URL:                s.OAuth2Server.URL,
				ClientID:           clientID,
				GrantType:          "client_credentials",
				AuthenticationType: datastore.SharedSecretAuth,
				ClientSecret:       "test-client-secret",
			},
		},
	}

	endpointRepo := postgres.NewEndpointRepo(s.ConvoyApp.A.DB)
	err := endpointRepo.CreateEndpoint(context.Background(), endpoint, s.DefaultProject.UID)
	require.NoError(s.T(), err)

	// Test OAuth2 token service with the endpoint (using in-memory cache)
	oauth2TokenService := services.NewOAuth2TokenService(s.MemoryCache, log.NewLogger(nil))

	// Verify no token requests yet
	require.Equal(s.T(), 0, s.OAuth2TokenCallCount)

	// Fetch token - should trigger OAuth2 token exchange
	token, err := oauth2TokenService.GetAccessToken(context.Background(), endpoint)
	require.NoError(s.T(), err)
	require.NotEmpty(s.T(), token)
	require.Contains(s.T(), token, "test-access-token")

	// Verify OAuth2 server was called
	require.Equal(s.T(), 1, s.OAuth2TokenCallCount)
	require.Len(s.T(), s.OAuth2TokenRequests, 1)
	require.Equal(s.T(), "client_credentials", s.OAuth2TokenRequests[0]["grant_type"])
	require.Equal(s.T(), clientID, s.OAuth2TokenRequests[0]["client_id"])
	require.Equal(s.T(), "test-client-secret", s.OAuth2TokenRequests[0]["client_secret"])

	// Verify token is cached
	cacheKey := fmt.Sprintf("oauth2_token:%s", endpointID)
	var cachedToken services.CachedToken
	err = s.MemoryCache.Get(context.Background(), cacheKey, &cachedToken)
	require.NoError(s.T(), err)
	require.NotEmpty(s.T(), cachedToken.AccessToken)
	require.Equal(s.T(), "Bearer", cachedToken.TokenType)
	require.Equal(s.T(), token, cachedToken.AccessToken)

	// Fetch again - should use cached token (no new OAuth2 request)
	token2, err := oauth2TokenService.GetAccessToken(context.Background(), endpoint)
	require.NoError(s.T(), err)
	require.Equal(s.T(), token, token2)
	require.Equal(s.T(), 1, s.OAuth2TokenCallCount, "Should not make another OAuth2 request when using cached token")
}

func (s *OAuth2IntegrationTestSuite) Test_OAuth2TokenService_ClientAssertion_Integration() {
	// Create endpoint with OAuth2 client assertion authentication
	endpointID := ulid.Make().String()
	clientID := "test-client-id-assertion"
	signingKey := s.generateTestJWK()

	endpoint := &datastore.Endpoint{
		UID:       endpointID,
		Name:      "OAuth2 Assertion Test Endpoint",
		ProjectID: s.DefaultProject.UID,
		Url:       s.WebhookServer.URL,
		Status:    datastore.ActiveEndpointStatus,
		Secrets: []datastore.Secret{
			{Value: "1234"},
		},
		Authentication: &datastore.EndpointAuthentication{
			Type: datastore.OAuth2Authentication,
			OAuth2: &datastore.OAuth2{
				URL:                s.OAuth2Server.URL,
				ClientID:           clientID,
				GrantType:          "client_credentials",
				AuthenticationType: datastore.ClientAssertionAuth,
				SigningKey:         signingKey,
				SigningAlgorithm:   "ES256",
				Issuer:             clientID,
				Subject:            clientID,
			},
		},
	}

	endpointRepo := postgres.NewEndpointRepo(s.ConvoyApp.A.DB)
	err := endpointRepo.CreateEndpoint(context.Background(), endpoint, s.DefaultProject.UID)
	require.NoError(s.T(), err)

	// Test OAuth2 token service with client assertion (using in-memory cache)
	oauth2TokenService := services.NewOAuth2TokenService(s.MemoryCache, log.NewLogger(nil))

	// Fetch token - should trigger OAuth2 token exchange with client assertion
	token, err := oauth2TokenService.GetAccessToken(context.Background(), endpoint)
	require.NoError(s.T(), err)
	require.NotEmpty(s.T(), token)
	require.Contains(s.T(), token, "test-access-token")

	// Verify OAuth2 server was called with client assertion
	require.Equal(s.T(), 1, s.OAuth2TokenCallCount)
	require.Len(s.T(), s.OAuth2TokenRequests, 1)
	require.Equal(s.T(), "client_credentials", s.OAuth2TokenRequests[0]["grant_type"])
	require.Equal(s.T(), clientID, s.OAuth2TokenRequests[0]["client_id"])
	require.Equal(s.T(), "urn:ietf:params:oauth:client-assertion-type:jwt-bearer", s.OAuth2TokenRequests[0]["client_assertion_type"])
	require.NotEmpty(s.T(), s.OAuth2TokenRequests[0]["client_assertion"], "Client assertion JWT should be present")
	require.Empty(s.T(), s.OAuth2TokenRequests[0]["client_secret"], "Client secret should not be present for assertion flow")
}

func (s *OAuth2IntegrationTestSuite) initEncryption() {
	km, err := keys.Get()
	require.NoError(s.T(), err)
	err = keys.InitEncryption(log.FromContext(context.Background()), s.DB, km, "test-key", 120)
	require.NoError(s.T(), err)
}

func (s *OAuth2IntegrationTestSuite) Test_CreateEndpoint_WithOAuth2SharedSecret_Encrypted() {
	// Initialize encryption
	s.initEncryption()

	endpointTitle := fmt.Sprintf("OAuth2-Encrypted-Endpoint-%s", ulid.Make().String())
	expectedStatusCode := http.StatusCreated

	// Arrange Request.
	url := fmt.Sprintf("/ui/organisations/%s/projects/%s/endpoints", s.DefaultProject.OrganisationID, s.DefaultProject.UID)
	plainBody := fmt.Sprintf(`{
		"name": "%s",
		"description": "test endpoint with oauth2 encrypted",
		"url": "%s",
		"authentication": {
			"type": "oauth2",
			"oauth2": {
				"url": "%s",
				"client_id": "test-client-id-encrypted",
				"authentication_type": "shared_secret",
				"client_secret": "test-client-secret-encrypted-12345",
				"grant_type": "client_credentials"
			}
		}
	}`, endpointTitle, s.WebhookServer.URL, s.OAuth2Server.URL)
	body := strings.NewReader(plainBody)
	req := createRequest(http.MethodPost, url, "", body)
	err := s.AuthenticatorFn(req, s.Router)
	require.NoError(s.T(), err)

	w := httptest.NewRecorder()

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)

	// Deep Assert.
	var endpoint datastore.Endpoint
	parseResponse(s.T(), w.Result(), &endpoint)
	require.NotEmpty(s.T(), endpoint.UID)
	require.Equal(s.T(), endpointTitle, endpoint.Name)
	require.NotNil(s.T(), endpoint.Authentication)
	require.Equal(s.T(), datastore.OAuth2Authentication, endpoint.Authentication.Type)
	require.NotNil(s.T(), endpoint.Authentication.OAuth2)
	require.Equal(s.T(), s.OAuth2Server.URL, endpoint.Authentication.OAuth2.URL)
	require.Equal(s.T(), "test-client-id-encrypted", endpoint.Authentication.OAuth2.ClientID)
	require.Equal(s.T(), "test-client-secret-encrypted-12345", endpoint.Authentication.OAuth2.ClientSecret)
	require.Equal(s.T(), datastore.SharedSecretAuth, endpoint.Authentication.OAuth2.AuthenticationType)

	// Verify we can fetch the endpoint again and it's still decrypted correctly
	endpointRepo := postgres.NewEndpointRepo(s.ConvoyApp.A.DB)
	fetchedEndpoint, err := endpointRepo.FindEndpointByID(context.Background(), endpoint.UID, s.DefaultProject.UID)
	require.NoError(s.T(), err)
	require.NotNil(s.T(), fetchedEndpoint.Authentication.OAuth2)
	require.Equal(s.T(), "test-client-secret-encrypted-12345", fetchedEndpoint.Authentication.OAuth2.ClientSecret)
}

func (s *OAuth2IntegrationTestSuite) Test_CreateEndpoint_WithOAuth2ClientAssertion_Encrypted() {
	// Initialize encryption
	s.initEncryption()

	endpointTitle := fmt.Sprintf("OAuth2-Assertion-Encrypted-Endpoint-%s", ulid.Make().String())
	expectedStatusCode := http.StatusCreated

	signingKey := s.generateTestJWK()

	// Arrange Request.
	url := fmt.Sprintf("/ui/organisations/%s/projects/%s/endpoints", s.DefaultProject.OrganisationID, s.DefaultProject.UID)
	plainBody := fmt.Sprintf(`{
		"name": "%s",
		"description": "test endpoint with oauth2 client assertion encrypted",
		"url": "%s",
		"authentication": {
			"type": "oauth2",
			"oauth2": {
				"url": "%s",
				"client_id": "test-client-id-assertion-encrypted",
				"authentication_type": "client_assertion",
				"signing_key": {
					"kty": "%s",
					"crv": "%s",
					"kid": "%s",
					"x": "%s",
					"y": "%s",
					"d": "%s"
				},
				"signing_algorithm": "ES256",
				"issuer": "test-client-id-assertion-encrypted",
				"subject": "test-client-id-assertion-encrypted",
				"grant_type": "client_credentials"
			}
		}
	}`, endpointTitle, s.WebhookServer.URL, s.OAuth2Server.URL,
		signingKey.Kty, signingKey.Crv, signingKey.Kid,
		signingKey.X, signingKey.Y, signingKey.D)
	body := strings.NewReader(plainBody)
	req := createRequest(http.MethodPost, url, "", body)
	err := s.AuthenticatorFn(req, s.Router)
	require.NoError(s.T(), err)

	w := httptest.NewRecorder()

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)

	// Deep Assert.
	var endpoint datastore.Endpoint
	parseResponse(s.T(), w.Result(), &endpoint)
	require.NotEmpty(s.T(), endpoint.UID)
	require.Equal(s.T(), endpointTitle, endpoint.Name)
	require.NotNil(s.T(), endpoint.Authentication)
	require.Equal(s.T(), datastore.OAuth2Authentication, endpoint.Authentication.Type)
	require.NotNil(s.T(), endpoint.Authentication.OAuth2)
	require.Equal(s.T(), datastore.ClientAssertionAuth, endpoint.Authentication.OAuth2.AuthenticationType)
	require.NotNil(s.T(), endpoint.Authentication.OAuth2.SigningKey)
	require.Equal(s.T(), "EC", endpoint.Authentication.OAuth2.SigningKey.Kty)
	require.Equal(s.T(), "P-256", endpoint.Authentication.OAuth2.SigningKey.Crv)
	require.Equal(s.T(), signingKey.X, endpoint.Authentication.OAuth2.SigningKey.X)
	require.Equal(s.T(), signingKey.Y, endpoint.Authentication.OAuth2.SigningKey.Y)
	require.Equal(s.T(), signingKey.D, endpoint.Authentication.OAuth2.SigningKey.D)
	require.Equal(s.T(), signingKey.Kid, endpoint.Authentication.OAuth2.SigningKey.Kid)

	// Verify we can fetch the endpoint again and it's still decrypted correctly
	endpointRepo := postgres.NewEndpointRepo(s.ConvoyApp.A.DB)
	fetchedEndpoint, err := endpointRepo.FindEndpointByID(context.Background(), endpoint.UID, s.DefaultProject.UID)
	require.NoError(s.T(), err)
	require.NotNil(s.T(), fetchedEndpoint.Authentication.OAuth2)
	require.NotNil(s.T(), fetchedEndpoint.Authentication.OAuth2.SigningKey)
	require.Equal(s.T(), signingKey.X, fetchedEndpoint.Authentication.OAuth2.SigningKey.X)
	require.Equal(s.T(), signingKey.Y, fetchedEndpoint.Authentication.OAuth2.SigningKey.Y)
	require.Equal(s.T(), signingKey.D, fetchedEndpoint.Authentication.OAuth2.SigningKey.D)
}

// enableOAuth2FeatureFlag enables the OAuth2 feature flag for an organization
func enableOAuth2FeatureFlag(t *testing.T, db database.Database, orgID string) error {
	t.Helper()

	// Fetch feature flag
	featureFlag, err := postgres.FetchFeatureFlagByKey(context.Background(), db, string(fflag.OAuthTokenExchange))
	if err != nil {
		return fmt.Errorf("failed to fetch feature flag: %w", err)
	}

	// Create or update override
	override := &datastore.FeatureFlagOverride{
		FeatureFlagID: featureFlag.UID,
		OwnerType:     "organisation",
		OwnerID:       orgID,
		Enabled:       true,
		EnabledAt:     null.TimeFrom(time.Now()),
	}

	return postgres.UpsertFeatureFlagOverride(context.Background(), db, override)
}

func TestOAuth2IntegrationTestSuite(t *testing.T) {
	suite.Run(t, new(OAuth2IntegrationTestSuite))
}
