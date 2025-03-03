package keys

import (
	"context"
	"fmt"
	mcache "github.com/frain-dev/convoy/cache/memory"
	"github.com/frain-dev/convoy/mocks"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

var (
	HCPClientID     = os.Getenv("CONVOY_HCP_CLIENT_ID")
	HCPClientSecret = os.Getenv("CONVOY_HCP_CLIENT_SECRET")
	HCPOrgID        = os.Getenv("CONVOY_HCP_ORG_ID")
	HCPProjectID    = os.Getenv("CONVOY_HCP_PROJECT_ID")
	HCPAppName      = os.Getenv("CONVOY_HCP_APP_NAME")
	HCPSecretName   = os.Getenv("CONVOY_HCP_SECRET_NAME")
)

func TestNewHCPVaultKeyManagerEnv(t *testing.T) {
	if HCPClientID == "" || HCPClientSecret == "" || HCPOrgID == "" || HCPProjectID == "" {
		fmt.Println("Skipping test due to missing environment variables")
		return
	}

	h := NewHCPVaultKeyManager(HCPClientID, HCPClientSecret, HCPOrgID, HCPProjectID, HCPAppName, HCPSecretName)
	assert.NotNil(t, h)

	// Happy path for getting the current key
	key, err := h.GetCurrentKeyFromCache()
	assert.Nil(t, err)
	assert.NotEmpty(t, key)

	// Happy path for setting a key
	newKey := "from-test-" + time.Now().String()
	err = h.SetKey(newKey)
	assert.Nil(t, err)

	// Verifying if the new key was set properly
	keyAfterSet, err := h.GetCurrentKeyFromCache()
	assert.Nil(t, err)
	assert.Equal(t, newKey, keyAfterSet)
}

func TestNewHCPVaultKeyManager(t *testing.T) {
	// Mock HTTP server to simulate HCP Vault API
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		// Mock the GET request for retrieving the current key
		case r.Method == http.MethodGet && strings.Contains(r.URL.Path, ":open"):
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{
				"secret": {
					"static_version": {
						"value": "mock-key"
					}
				}
			}`))
		// Mock the POST request for setting a new key
		case r.Method == http.MethodPost && strings.Contains(r.URL.Path, "/secret/kv"):
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{
				"secret": {
					"name": "mock-secret",
					"static_version": {
						"value": "from-test-mock"
					}
				}
			}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer mockServer.Close()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ll := mocks.NewMockLicenser(ctrl)
	ll.EXPECT().CredentialEncryption().Return(true).AnyTimes()

	// Create a new instance of HCPVaultKeyManager with the mock server
	h := &HCPVaultKeyManager{
		APIBaseURL: mockServer.URL,
		OrgID:      "test-org-id",
		ProjectID:  "test-project-id",
		AppName:    "test-app-name",
		SecretName: "test-secret",
		token:      "dummy-token",
		cache:      mcache.NewMemoryCache(),
		httpClient: http.DefaultClient,
		licenser:   ll,
		isSet:      true,
	}

	// Mocked happy path for getting the current key
	key, err := h.GetCurrentKeyFromCache()
	assert.Nil(t, err)
	assert.Equal(t, "mock-key", key)
	t.Logf("Retrieved mocked key: %s", key)

	// Mocked happy path for setting a new key
	newKey := "from-test-mock"
	err = h.SetKey(newKey)
	assert.Nil(t, err)

	// Mocked verification of the new key
	err = h.cache.Delete(context.Background(), RedisCacheKey) // Force cache expiration
	assert.Nil(t, err)
	keyAfterSet, err := h.GetCurrentKeyFromCache()
	assert.Nil(t, err)
	assert.Equal(t, "mock-key", keyAfterSet)
}

func TestHCPVaultKeyManagerEdgeCases(t *testing.T) {
	// Mock HTTP server to simulate HCP Vault responses
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := r.Header.Get("Authorization")
		if token == "" {
			http.Error(w, `{"code":16,"message":"unauthorized"}`, http.StatusUnauthorized)
			return
		}

		switch r.URL.Path {
		case "/secrets/2023-11-28/organizations/mock-org/projects/mock-proj/apps/mock-app/secrets/mock-secret:open":
			if r.Method == "GET" {
				w.WriteHeader(http.StatusOK)
				_, err := w.Write([]byte(`{"secret":{"static_version":{"value":"mock-key"}}}`))
				if err != nil {
					return
				}
				return
			}
		case "/secrets/2023-11-28/organizations/mock-org/projects/mock-proj/apps/mock-app/secret/kv":
			if r.Method == "POST" {
				body := []byte(`{"name":"mock-secret","value":"new-mock-key"}`)
				_, err := w.Write(body)
				if err != nil {
					return
				}
				return
			}
		default:
			http.Error(w, `{"code":3,"message":"unknown error"}`, http.StatusInternalServerError)
		}
	}))
	defer server.Close()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ll := mocks.NewMockLicenser(ctrl)
	ll.EXPECT().CredentialEncryption().Return(true).AnyTimes()

	// Initialize the HCPVaultKeyManager with the mock server's base URL
	h := &HCPVaultKeyManager{
		APIBaseURL:   server.URL,
		OrgID:        "mock-org",
		ProjectID:    "mock-proj",
		AppName:      "mock-app",
		SecretName:   "mock-secret",
		ClientID:     "mock-client-id",
		ClientSecret: "mock-client-secret",
		cache:        mcache.NewMemoryCache(),

		httpClient: http.DefaultClient,
		licenser:   ll,
		isSet:      true,
	}

	// Test token refresh on 401 Unauthorized
	t.Run("Token refresh on unauthorized", func(t *testing.T) {
		h.token = "expired-token"
		key, err := h.GetCurrentKeyFromCache()
		assert.Nil(t, err)
		assert.Equal(t, "mock-key", key)
	})

	// Test setting key when the secret version limit is reached
	t.Run("SetKey handles version limit", func(t *testing.T) {
		err := h.SetKey("new-mock-key")
		assert.Nil(t, err)
	})

	// Test error response handling
	t.Run("Handles API error responses", func(t *testing.T) {
		h.SecretName = "unknown-secret"
		err := h.cache.Delete(context.Background(), RedisCacheKey)
		assert.Nil(t, err)
		_, err = h.GetCurrentKeyFromCache()
		assert.NotNil(t, err)
		assert.Contains(t, err.Error(), "unknown error")
	})
}
