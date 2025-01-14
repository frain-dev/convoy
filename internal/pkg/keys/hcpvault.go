package keys

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/frain-dev/convoy/cache"
	mcache "github.com/frain-dev/convoy/cache/memory"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/internal/pkg/license"
	"github.com/frain-dev/convoy/pkg/log"
	"io"
	"net/http"
	"time"
)

const RedisCacheKey = "HCPVaultRedisKey"

var (
	HCPAPIBaseURL                                            = "https://api.cloud.hashicorp.com"
	ErrCredentialEncryptionFeatureUnavailable                = errors.New("credential encryption feature unavailable, please upgrade")
	ErrCredentialEncryptionFeatureUnavailableUpgradeOrRevert = errors.New("credential encryption feature unavailable, please upgrade or revert encryption")
)

// HCPVaultKeyManager manages interaction with HCP Vault secrets.
type HCPVaultKeyManager struct {
	ClientID     string
	ClientSecret string
	OrgID        string
	ProjectID    string
	AppName      string
	SecretName   string
	APIBaseURL   string

	httpClient *http.Client
	token      string
	expiryTime time.Time

	licenser license.Licenser
	cache    cache.Cache

	isSet bool
}

type SecretResponse struct {
	Secret struct {
		Name          string                 `json:"name"`
		Type          string                 `json:"type"`
		LatestVersion int                    `json:"latest_version"`
		CreatedAt     string                 `json:"created_at"`
		CreatedByID   string                 `json:"created_by_id"`
		SyncStatus    map[string]interface{} `json:"sync_status"`
		StaticVersion struct {
			Version     int    `json:"version"`
			Value       string `json:"value"`
			CreatedAt   string `json:"created_at"`
			CreatedByID string `json:"created_by_id"`
		} `json:"static_version"`
	} `json:"secret"`
}

// APIError represents an error returned by the HCP Vault API.
type APIError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Details []any  `json:"details"`
}

// Error implements the error interface for APIError.
func (e *APIError) Error() string {
	return fmt.Sprintf("API Error: Code %d, Message: %s", e.Code, e.Message)
}

// fetchNewToken fetches a new OAuth token from HCP Vault.
func (k *HCPVaultKeyManager) fetchNewToken() error {
	url := "https://auth.idp.hashicorp.com/oauth/token"
	resp, err := http.PostForm(url, map[string][]string{
		"grant_type":    {"client_credentials"},
		"client_id":     {k.ClientID},
		"client_secret": {k.ClientSecret},
		"audience":      {"https://api.hashicorp.cloud"},
	})
	if err != nil {
		return fmt.Errorf("error requesting token: %w", err)
	}
	defer resp.Body.Close()

	var response struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"` // Expiration in seconds
	}
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return fmt.Errorf("error decoding token response: %w", err)
	}

	// Update token and expiration time
	k.token = response.AccessToken
	k.expiryTime = time.Now().Add(time.Duration(response.ExpiresIn) * time.Second)
	return nil
}

// ensureValidToken ensures the token is valid, refreshing it if necessary.
func (k *HCPVaultKeyManager) ensureValidToken() error {
	if k.token == "" || time.Now().After(k.expiryTime) {
		return k.fetchNewToken()
	}
	return nil
}

func (k *HCPVaultKeyManager) IsSet() bool {
	return k.isSet
}

// GetCurrentKeyFromCache retrieves the current key from the Cache.
func (k *HCPVaultKeyManager) GetCurrentKeyFromCache() (string, error) {
	if !k.isSet {
		return "", nil
	}

	if !k.licenser.CredentialEncryption() {
		return "", ErrCredentialEncryptionFeatureUnavailable
	}

	var currentKey string
	err := k.cache.Get(context.Background(), RedisCacheKey, &currentKey)
	if err != nil {
		return "", err
	}

	if currentKey != "" {
		return currentKey, nil
	}

	return k.GetCurrentKey()
}

// GetCurrentKey retrieves the current key from HCP Vault.
func (k *HCPVaultKeyManager) GetCurrentKey() (string, error) {
	if !k.licenser.CredentialEncryption() {
		return "", ErrCredentialEncryptionFeatureUnavailable
	}

	retryCount := 1
	for {
		if err := k.ensureValidToken(); err != nil {
			return "", fmt.Errorf("failed to ensure valid token: %w", err)
		}

		currentKey, err := k.fetchSecretKey()
		if err != nil {
			// Retry if unauthorized and retries are still available
			if isUnauthorizedError(err) && retryCount > 0 {
				if fetchErr := k.fetchNewToken(); fetchErr != nil {
					return "", fmt.Errorf("failed to refresh token: %w", fetchErr)
				}
				retryCount--
				continue
			}
			return "", err
		}

		return currentKey, k.cache.Set(context.Background(), RedisCacheKey, currentKey, -1)
	}
}

// fetchSecretKey makes the actual API call to fetch the current key.
func (k *HCPVaultKeyManager) fetchSecretKey() (string, error) {
	url := fmt.Sprintf("%s/secrets/2023-11-28/organizations/%s/projects/%s/apps/%s/secrets/%s:open",
		k.APIBaseURL, k.OrgID, k.ProjectID, k.AppName, k.SecretName)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", fmt.Errorf("error creating request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+k.token)

	resp, err := k.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("error sending request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", parseErrorResponse(resp)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("error reading response: %w", err)
	}
	var response SecretResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return "", fmt.Errorf("error decoding secret response: %w", err)
	}

	return response.Secret.StaticVersion.Value, nil
}

// SetKey sets a new key in HCP Vault.
func (k *HCPVaultKeyManager) SetKey(newKey string) error {
	if !k.isSet {
		return nil
	}
	if !k.licenser.CredentialEncryption() {
		return ErrCredentialEncryptionFeatureUnavailable
	}
	// Retry configuration
	maxRetries := 1
	retryCount := 0

	// Retry loop
	for {
		if err := k.ensureValidToken(); err != nil {
			return fmt.Errorf("failed to ensure valid token: %w", err)
		}

		err := k.createOrUpdateSecret(newKey)
		if err == nil {
			// Success
			return nil
		}

		// Handle errors
		if isUnauthorizedError(err) && retryCount < maxRetries {
			// Refresh token and retry
			if err := k.fetchNewToken(); err != nil {
				return fmt.Errorf("failed to refresh token: %w", err)
			}
			retryCount++
			continue
		}

		// Handle maximum version error by deleting the secret and retrying
		if isMaxVersionError(err) {
			if delErr := k.deleteSecret(); delErr != nil {
				return fmt.Errorf("failed to delete secret after max version error: %w", delErr)
			}
			continue
		}

		return err
	}
}

// createOrUpdateSecret sends a request to create or update the secret.
func (k *HCPVaultKeyManager) createOrUpdateSecret(newKey string) error {
	secretURL := fmt.Sprintf("%s/secrets/2023-11-28/organizations/%s/projects/%s/apps/%s/secret/kv",
		k.APIBaseURL, k.OrgID, k.ProjectID, k.AppName)

	reqBody := map[string]string{
		"name":  k.SecretName,
		"value": newKey,
	}
	body, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("error marshaling request body: %w", err)
	}

	req, err := http.NewRequest("POST", secretURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("error creating request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+k.token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := k.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("error sending request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return parseErrorResponse(resp)
	}

	return k.cache.Set(context.Background(), RedisCacheKey, newKey, -1)
}

// deleteSecret deletes the existing secret to reset the versioning.
func (k *HCPVaultKeyManager) deleteSecret() error {
	deleteURL := fmt.Sprintf("%s/secrets/2023-11-28/organizations/%s/projects/%s/apps/%s/secrets/%s",
		k.APIBaseURL, k.OrgID, k.ProjectID, k.AppName, k.SecretName)

	req, err := http.NewRequest("DELETE", deleteURL, nil)
	if err != nil {
		return fmt.Errorf("error creating delete request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+k.token)

	resp, err := k.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("error sending delete request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return parseErrorResponse(resp)
	}

	return nil
}

func (k *HCPVaultKeyManager) Unset() {
	k.isSet = false
}

// isUnauthorizedError checks if the error is due to an expired or invalid token.
func isUnauthorizedError(err error) bool {
	var apiErr *APIError
	if errors.As(err, &apiErr) && apiErr.Code == 16 {
		return true
	}
	return false
}

// isMaxVersionError checks if the error is due to reaching the maximum version limit.
func isMaxVersionError(err error) bool {
	var apiErr *APIError
	if errors.As(err, &apiErr) && (apiErr.Code == 8 || apiErr.Message == "maximum number of secret versions reached") {
		return true
	}
	return false
}

// parseErrorResponse parses the error response from the HCP Vault API.
func parseErrorResponse(resp *http.Response) error {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("error reading error response body: %w", err)
	}

	var apiErr APIError
	if err := json.Unmarshal(body, &apiErr); err != nil {
		return fmt.Errorf("error decoding error response: %w", err)
	}

	return &apiErr
}

// NewHCPVaultKeyManager initializes a new HCPVaultKeyManager instance.
func NewHCPVaultKeyManager(clientID, clientSecret, orgID, projectID, appName, secretName string) *HCPVaultKeyManager {
	return &HCPVaultKeyManager{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		OrgID:        orgID,
		ProjectID:    projectID,
		AppName:      appName,
		SecretName:   secretName,
		APIBaseURL:   HCPAPIBaseURL,
		httpClient:   http.DefaultClient,
		isSet:        true,
		cache:        mcache.NewMemoryCache(),
	}
}

func NewHCPVaultKeyManagerFromConfig(cfg config.HCPVaultConfig, licenser license.Licenser, cache cache.Cache) *HCPVaultKeyManager {
	if cfg.ClientID == "" || cfg.ClientSecret == "" || cfg.OrgID == "" || cfg.ProjectID == "" || cfg.AppName == "" || cfg.SecretName == "" {
		log.Warn("missing required HCP Vault configuration")
		return &HCPVaultKeyManager{
			cache:    cache,
			licenser: licenser,
			isSet:    false,
		}
	}

	return &HCPVaultKeyManager{
		ClientID:     cfg.ClientID,
		ClientSecret: cfg.ClientSecret,
		OrgID:        cfg.OrgID,
		ProjectID:    cfg.ProjectID,
		AppName:      cfg.AppName,
		SecretName:   cfg.SecretName,
		APIBaseURL:   HCPAPIBaseURL,
		httpClient:   http.DefaultClient,
		cache:        cache,
		licenser:     licenser,
		isSet:        true,
	}
}
