package e2e

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/require"
	"gopkg.in/guregu/null.v4"

	convoy "github.com/frain-dev/convoy-go/v2"
	"github.com/frain-dev/convoy/api/models"
	"github.com/frain-dev/convoy/database"
	"github.com/frain-dev/convoy/database/postgres"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/internal/pkg/fflag"
	"github.com/frain-dev/convoy/util"
)

// OAuth2TestEnv holds OAuth2-specific test infrastructure
type OAuth2TestEnv struct {
	OAuth2Server     *httptest.Server
	WebhookServer    *httptest.Server
	TokenCallCount   atomic.Int64
	WebhookCallCount atomic.Int64
	TokenRequests    []map[string]string
	WebhookRequests  []http.Request
	AuthHeaders      []string
}

func TestE2E_OAuth2_SharedSecret(t *testing.T) {
	// Setup E2E environment
	env := SetupE2E(t)

	// Enable OAuth2 feature flag for the organization
	err := enableOAuth2FeatureFlag(t, env.App.DB, env.Organisation.UID)
	require.NoError(t, err)

	// Setup OAuth2 test infrastructure
	oauth2Env := setupOAuth2TestEnv(t)

	// Create Convoy SDK client
	c := convoy.New(env.ServerURL+"/api/v1", env.APIKey, env.Project.UID)

	// Setup test infrastructure
	manifest := NewEventManifest()
	done := make(chan bool, 1)
	var counter atomic.Int64
	counter.Store(1) // Expecting 1 webhook

	// Start mock webhook server that verifies OAuth2 Authorization header
	port := 19920
	StartMockWebhookServerWithOAuth2(t, manifest, done, &counter, port, oauth2Env)

	ownerID := env.Organisation.OwnerID + "_e2e_oauth2_0"

	// Create endpoint with OAuth2 shared secret authentication
	endpoint := CreateOAuth2EndpointViaHTTP(t, env, oauth2Env.OAuth2Server.URL, port, ownerID, "shared_secret", "", nil)
	t.Logf("Created OAuth2 endpoint: %s at http://localhost:%d/webhook", endpoint.UID, port)

	// Create subscription with wildcard filter
	subscription := CreateSubscriptionViaSDK(t, c, endpoint.UID, []string{"*"})
	t.Logf("Created subscription: %s with wildcard filter", subscription.UID)

	// Send event
	traceId := "e2e-oauth2-shared-secret-" + ulid.Make().String()
	SendEventViaSDK(t, c, endpoint.UID, "test.event", traceId)
	t.Logf("Sent event with traceId: %s", traceId)

	// Wait for webhook to be delivered
	WaitForWebhooks(t, done, 30*time.Second)

	// Verify OAuth2 token server was called
	require.Greater(t, oauth2Env.TokenCallCount.Load(), int64(0), "OAuth2 token server should have been called")
	require.Len(t, oauth2Env.TokenRequests, 1, "Should have received 1 token request")
	require.Equal(t, "client_credentials", oauth2Env.TokenRequests[0]["grant_type"])
	require.NotEmpty(t, oauth2Env.TokenRequests[0]["client_secret"], "Client secret should be present")

	// Verify webhook was received with OAuth2 Authorization header
	require.Greater(t, oauth2Env.WebhookCallCount.Load(), int64(0), "Webhook server should have been called")
	require.Greater(t, len(oauth2Env.AuthHeaders), 0, "Should have received Authorization header")
	require.Contains(t, oauth2Env.AuthHeaders[0], "Bearer", "Authorization header should contain Bearer token")

	// Verify webhook was received
	webhookURL := fmt.Sprintf("http://localhost:%d/webhook", port)
	hits := manifest.ReadEndpoint(webhookURL)
	require.Equal(t, 1, hits, "Should have received 1 webhook")

	t.Log("✅ E2E test passed: OAuth2 shared secret authentication works end-to-end")
}

func TestE2E_OAuth2_ClientAssertion(t *testing.T) {
	// Setup E2E environment
	env := SetupE2E(t)

	// Enable OAuth2 feature flag for the organization
	err := enableOAuth2FeatureFlag(t, env.App.DB, env.Organisation.UID)
	require.NoError(t, err)

	// Setup OAuth2 test infrastructure
	oauth2Env := setupOAuth2TestEnv(t)

	// Generate test JWK for client assertion
	signingKey := generateTestJWK(t)

	// Create Convoy SDK client
	c := convoy.New(env.ServerURL+"/api/v1", env.APIKey, env.Project.UID)

	// Setup test infrastructure
	manifest := NewEventManifest()
	done := make(chan bool, 1)
	var counter atomic.Int64
	counter.Store(1) // Expecting 1 webhook

	// Start mock webhook server that verifies OAuth2 Authorization header
	port := 19921
	StartMockWebhookServerWithOAuth2(t, manifest, done, &counter, port, oauth2Env)

	ownerID := env.Organisation.OwnerID + "_e2e_oauth2_1"

	// Create endpoint with OAuth2 client assertion authentication
	endpoint := CreateOAuth2EndpointViaHTTP(t, env, oauth2Env.OAuth2Server.URL, port, ownerID, "client_assertion", "ES256", signingKey)
	t.Logf("Created OAuth2 endpoint: %s at http://localhost:%d/webhook", endpoint.UID, port)

	// Create subscription with wildcard filter
	subscription := CreateSubscriptionViaSDK(t, c, endpoint.UID, []string{"*"})
	t.Logf("Created subscription: %s with wildcard filter", subscription.UID)

	// Send event
	traceId := "e2e-oauth2-client-assertion-" + ulid.Make().String()
	SendEventViaSDK(t, c, endpoint.UID, "test.event", traceId)
	t.Logf("Sent event with traceId: %s", traceId)

	// Wait for webhook to be delivered
	WaitForWebhooks(t, done, 30*time.Second)

	// Verify OAuth2 token server was called with client assertion
	require.Greater(t, oauth2Env.TokenCallCount.Load(), int64(0), "OAuth2 token server should have been called")
	require.Len(t, oauth2Env.TokenRequests, 1, "Should have received 1 token request")
	require.Equal(t, "client_credentials", oauth2Env.TokenRequests[0]["grant_type"])
	require.Equal(t, "urn:ietf:params:oauth:client-assertion-type:jwt-bearer", oauth2Env.TokenRequests[0]["client_assertion_type"])
	require.NotEmpty(t, oauth2Env.TokenRequests[0]["client_assertion"], "Client assertion JWT should be present")
	require.Empty(t, oauth2Env.TokenRequests[0]["client_secret"], "Client secret should not be present for assertion flow")

	// Verify webhook was received with OAuth2 Authorization header
	require.Greater(t, oauth2Env.WebhookCallCount.Load(), int64(0), "Webhook server should have been called")
	require.Greater(t, len(oauth2Env.AuthHeaders), 0, "Should have received Authorization header")
	require.Contains(t, oauth2Env.AuthHeaders[0], "Bearer", "Authorization header should contain Bearer token")

	// Verify webhook was received
	webhookURL := fmt.Sprintf("http://localhost:%d/webhook", port)
	hits := manifest.ReadEndpoint(webhookURL)
	require.Equal(t, 1, hits, "Should have received 1 webhook")

	t.Log("✅ E2E test passed: OAuth2 client assertion authentication works end-to-end")
}

// setupOAuth2TestEnv sets up mock OAuth2 token server and tracking
func setupOAuth2TestEnv(t *testing.T) *OAuth2TestEnv {
	t.Helper()

	env := &OAuth2TestEnv{
		TokenRequests:   []map[string]string{},
		WebhookRequests: []http.Request{},
		AuthHeaders:     []string{},
	}

	// Setup mock OAuth2 token server
	env.OAuth2Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		env.TokenCallCount.Add(1)

		require.Equal(t, "POST", r.Method)
		require.Equal(t, "application/x-www-form-urlencoded", r.Header.Get("Content-Type"))

		err := r.ParseForm()
		require.NoError(t, err)

		require.Equal(t, "client_credentials", r.Form.Get("grant_type"))

		// Track the request
		requestData := make(map[string]string)
		requestData["grant_type"] = r.Form.Get("grant_type")
		requestData["client_id"] = r.Form.Get("client_id")
		requestData["client_secret"] = r.Form.Get("client_secret")
		requestData["client_assertion"] = r.Form.Get("client_assertion")
		requestData["client_assertion_type"] = r.Form.Get("client_assertion_type")
		env.TokenRequests = append(env.TokenRequests, requestData)

		// Check for either client_secret or client_assertion
		clientID := r.Form.Get("client_id")
		require.NotEmpty(t, clientID)

		// Validate authentication
		hasSecret := r.Form.Get("client_secret") != ""
		hasAssertion := r.Form.Get("client_assertion") != ""
		require.True(t, hasSecret || hasAssertion, "Either client_secret or client_assertion must be provided")

		if hasAssertion {
			require.Equal(t, "urn:ietf:params:oauth:client-assertion-type:jwt-bearer", r.Form.Get("client_assertion_type"))
			require.NotEmpty(t, r.Form.Get("client_assertion"))
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

	t.Cleanup(func() {
		env.OAuth2Server.Close()
	})

	return env
}

// StartMockWebhookServerWithOAuth2 starts a mock webhook server that verifies OAuth2 Authorization headers
func StartMockWebhookServerWithOAuth2(t *testing.T, manifest *EventManifest, done chan bool, counter *atomic.Int64, port int, oauth2Env *OAuth2TestEnv) {
	t.Helper()

	mux := http.NewServeMux()
	mux.HandleFunc("/webhook", func(w http.ResponseWriter, r *http.Request) {
		oauth2Env.WebhookCallCount.Add(1)
		endpoint := fmt.Sprintf("http://localhost:%d/webhook", port)
		manifest.IncEndpoint(endpoint)

		// Track Authorization header
		authHeader := r.Header.Get("Authorization")
		if authHeader != "" {
			oauth2Env.AuthHeaders = append(oauth2Env.AuthHeaders, authHeader)
		}

		// Verify OAuth2 Authorization header is present
		require.NotEmpty(t, authHeader, "Authorization header should be present")
		require.Contains(t, authHeader, "Bearer", "Authorization header should contain Bearer token")

		if r.Method != "POST" {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		reqBody, err := io.ReadAll(r.Body)
		if err != nil {
			t.Logf("Error reading webhook body: %v", err)
			http.Error(w, "Bad request", http.StatusBadRequest)
			return
		}

		// Track the request
		reqCopy := *r
		reqCopy.Header = r.Header.Clone()
		oauth2Env.WebhookRequests = append(oauth2Env.WebhookRequests, reqCopy)

		// Parse the webhook payload
		contentType := r.Header.Get("Content-Type")
		var payload map[string]interface{}

		if contentType == "application/x-www-form-urlencoded" {
			manifest.IncEvent(string(reqBody))
			t.Logf("Received form-encoded webhook on %s with OAuth2: %s", endpoint, string(reqBody))
		} else {
			if err := json.Unmarshal(reqBody, &payload); err != nil {
				t.Logf("Error parsing webhook JSON: %v", err)
				http.Error(w, "Bad request", http.StatusBadRequest)
				return
			}
			eventJSON, _ := json.Marshal(payload)
			manifest.IncEvent(string(eventJSON))
			t.Logf("Received JSON webhook on %s with OAuth2: %s", endpoint, string(reqBody))
		}

		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))

		// Decrement counter
		current := counter.Add(-1)
		if current <= 0 {
			select {
			case done <- true:
			default:
			}
		}
	})

	server := &http.Server{
		ReadHeaderTimeout: 10 * time.Second,
		Addr:              fmt.Sprintf(":%d", port),
		Handler:           mux,
	}

	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			t.Logf("Mock webhook server error on port %d: %v", port, err)
		}
	}()

	t.Cleanup(func() {
		server.Close()
	})

	// Give server time to start
	time.Sleep(100 * time.Millisecond)
}

// CreateOAuth2EndpointViaHTTP creates an endpoint with OAuth2 authentication via HTTP API
func CreateOAuth2EndpointViaHTTP(t *testing.T, env *E2ETestEnv, oauth2URL string, webhookPort int, ownerID, authType, signingAlgorithm string, signingKey *datastore.OAuth2SigningKey) *datastore.Endpoint {
	t.Helper()

	webhookURL := fmt.Sprintf("http://localhost:%d/webhook", webhookPort)
	endpointName := fmt.Sprintf("oauth2-endpoint-%s", ulid.Make().String())

	// Build OAuth2 configuration
	oauth2Config := map[string]interface{}{
		"url":                 oauth2URL,
		"client_id":           "test-client-id",
		"authentication_type": authType,
		"grant_type":          "client_credentials",
	}

	if authType == "shared_secret" {
		oauth2Config["client_secret"] = "test-client-secret"
	} else if authType == "client_assertion" {
		if signingKey != nil {
			oauth2Config["signing_key"] = map[string]interface{}{
				"kty": signingKey.Kty,
				"crv": signingKey.Crv,
				"kid": signingKey.Kid,
				"x":   signingKey.X,
				"y":   signingKey.Y,
				"d":   signingKey.D,
			}
		}
		if signingAlgorithm != "" {
			oauth2Config["signing_algorithm"] = signingAlgorithm
		}
		oauth2Config["issuer"] = "test-client-id"
		oauth2Config["subject"] = "test-client-id"
	}

	// Build request body
	requestBody := map[string]interface{}{
		"name":        endpointName,
		"description": "test endpoint with oauth2",
		"url":         webhookURL,
		"owner_id":    ownerID,
		"authentication": map[string]interface{}{
			"type":   "oauth2",
			"oauth2": oauth2Config,
		},
	}

	bodyBytes, err := json.Marshal(requestBody)
	require.NoError(t, err)

	// Make HTTP request to create endpoint
	url := fmt.Sprintf("%s/ui/organisations/%s/projects/%s/endpoints", env.ServerURL, env.Organisation.UID, env.Project.UID)
	req, err := http.NewRequest("POST", url, bytes.NewReader(bodyBytes))
	require.NoError(t, err)

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", env.APIKey))

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	require.Equal(t, http.StatusCreated, resp.StatusCode, "Endpoint creation should succeed")

	// Parse ServerResponse
	var serverResponse util.ServerResponse
	err = json.NewDecoder(resp.Body).Decode(&serverResponse)
	require.NoError(t, err)
	require.True(t, serverResponse.Status, "Server response should indicate success")

	// Parse EndpointResponse from Data
	var endpointResp models.EndpointResponse
	err = json.Unmarshal(serverResponse.Data, &endpointResp)
	require.NoError(t, err)
	require.NotNil(t, endpointResp.Endpoint, "Endpoint should not be nil")
	require.NotEmpty(t, endpointResp.Endpoint.UID, "Endpoint UID should not be empty")

	return endpointResp.Endpoint
}

// generateTestJWK generates a test ECDSA JWK for client assertion
func generateTestJWK(t *testing.T) *datastore.OAuth2SigningKey {
	t.Helper()

	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

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
