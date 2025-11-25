package services

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/frain-dev/convoy/cache"
	mcache "github.com/frain-dev/convoy/cache/memory"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/pkg/log"
	"github.com/stretchr/testify/require"
)

func provideOAuth2TokenService() (*OAuth2TokenService, cache.Cache) {
	memCache := mcache.NewMemoryCache()
	logger := log.NewLogger(nil)
	return NewOAuth2TokenService(memCache, logger), memCache
}

func generateTestECDSAPrivateKey(t *testing.T) *datastore.OAuth2SigningKey {
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

	signingKey := &datastore.OAuth2SigningKey{
		Kty: "EC",
		Crv: "P-256",
		X:   base64.RawURLEncoding.EncodeToString(xPadded),
		Y:   base64.RawURLEncoding.EncodeToString(yPadded),
		D:   base64.RawURLEncoding.EncodeToString(dPadded),
		Kid: "test-key-id",
	}

	return signingKey
}

func TestOAuth2TokenService_GetAccessToken_SharedSecret(t *testing.T) {
	ctx := context.Background()
	service, cache := provideOAuth2TokenService()

	// Create a mock OAuth2 token server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "POST", r.Method)
		require.Equal(t, "application/x-www-form-urlencoded", r.Header.Get("Content-Type"))

		err := r.ParseForm()
		require.NoError(t, err)

		require.Equal(t, "client_credentials", r.Form.Get("grant_type"))
		require.Equal(t, "test-client-id", r.Form.Get("client_id"))
		require.Equal(t, "test-client-secret", r.Form.Get("client_secret"))
		require.Equal(t, "test-scope", r.Form.Get("scope"))

		response := map[string]interface{}{
			"access_token": "test-access-token",
			"token_type":   "Bearer",
			"expires_in":   3600,
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	endpoint := &datastore.Endpoint{
		UID: "test-endpoint-id",
		Authentication: &datastore.EndpointAuthentication{
			Type: datastore.OAuth2Authentication,
			OAuth2: &datastore.OAuth2{
				URL:                server.URL,
				ClientID:           "test-client-id",
				GrantType:          "client_credentials",
				Scope:              "test-scope",
				AuthenticationType: datastore.SharedSecretAuth,
				ClientSecret:       "test-client-secret",
			},
		},
	}

	// First call should exchange token
	token, err := service.GetAccessToken(ctx, endpoint)
	require.NoError(t, err)
	require.Equal(t, "test-access-token", token)

	// Second call should use cached token
	token2, err := service.GetAccessToken(ctx, endpoint)
	require.NoError(t, err)
	require.Equal(t, "test-access-token", token2)

	// Verify token is cached
	var cachedToken CachedToken
	err = cache.Get(ctx, "oauth2_token:test-endpoint-id", &cachedToken)
	require.NoError(t, err)
	require.Equal(t, "test-access-token", cachedToken.AccessToken)
	require.Equal(t, "Bearer", cachedToken.TokenType)
}

func TestOAuth2TokenService_GetAccessToken_ClientAssertion(t *testing.T) {
	ctx := context.Background()
	service, cache := provideOAuth2TokenService()

	signingKey := generateTestECDSAPrivateKey(t)

	// Create a mock OAuth2 token server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "POST", r.Method)
		require.Equal(t, "application/x-www-form-urlencoded", r.Header.Get("Content-Type"))

		err := r.ParseForm()
		require.NoError(t, err)

		require.Equal(t, "client_credentials", r.Form.Get("grant_type"))
		require.Equal(t, "test-client-id", r.Form.Get("client_id"))
		require.Equal(t, "urn:ietf:params:oauth:client-assertion-type:jwt-bearer", r.Form.Get("client_assertion_type"))
		require.NotEmpty(t, r.Form.Get("client_assertion"))

		response := map[string]interface{}{
			"access_token": "test-access-token-assertion",
			"token_type":   "Bearer",
			"expires_in":   1800,
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	endpoint := &datastore.Endpoint{
		UID: "test-endpoint-id-assertion",
		Authentication: &datastore.EndpointAuthentication{
			Type: datastore.OAuth2Authentication,
			OAuth2: &datastore.OAuth2{
				URL:                server.URL,
				ClientID:           "test-client-id",
				GrantType:          "client_credentials",
				Scope:              "test-scope",
				AuthenticationType: datastore.ClientAssertionAuth,
				SigningKey:         signingKey,
				SigningAlgorithm:   "ES256",
				Issuer:             "test-client-id",
				Subject:            "test-client-id",
			},
		},
	}

	// First call should exchange token
	token, err := service.GetAccessToken(ctx, endpoint)
	require.NoError(t, err)
	require.Equal(t, "test-access-token-assertion", token)

	// Verify token is cached
	var cachedToken CachedToken
	err = cache.Get(ctx, "oauth2_token:test-endpoint-id-assertion", &cachedToken)
	require.NoError(t, err)
	require.Equal(t, "test-access-token-assertion", cachedToken.AccessToken)
}

func TestOAuth2TokenService_GetAccessToken_TokenRefresh(t *testing.T) {
	ctx := context.Background()
	service, cache := provideOAuth2TokenService()

	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		response := map[string]interface{}{
			"access_token": "test-token-" + string(rune('0'+callCount)),
			"token_type":   "Bearer",
			"expires_in":   60, // 1 minute - will trigger refresh
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	endpoint := &datastore.Endpoint{
		UID: "test-endpoint-refresh",
		Authentication: &datastore.EndpointAuthentication{
			Type: datastore.OAuth2Authentication,
			OAuth2: &datastore.OAuth2{
				URL:                server.URL,
				ClientID:           "test-client-id",
				AuthenticationType: datastore.SharedSecretAuth,
				ClientSecret:       "test-secret",
			},
		},
	}

	// First call
	token1, err := service.GetAccessToken(ctx, endpoint)
	require.NoError(t, err)
	require.Equal(t, "test-token-1", token1)
	require.Equal(t, 1, callCount)

	// Invalidate cache to simulate expiration/refresh needed
	err = cache.Delete(ctx, "oauth2_token:test-endpoint-refresh")
	require.NoError(t, err)

	// Second call should trigger refresh
	token2, err := service.GetAccessToken(ctx, endpoint)
	require.NoError(t, err)
	require.Equal(t, "test-token-2", token2)
	require.Equal(t, 2, callCount)
}

func TestOAuth2TokenService_InvalidateToken(t *testing.T) {
	ctx := context.Background()
	service, cache := provideOAuth2TokenService()

	// Cache a token
	cacheKey := "oauth2_token:test-endpoint"
	cachedToken := CachedToken{
		AccessToken: "cached-token",
		TokenType:   "Bearer",
		ExpiresAt:   time.Now().Add(1 * time.Hour),
	}
	err := cache.Set(ctx, cacheKey, cachedToken, 1*time.Hour)
	require.NoError(t, err)

	// Verify it's cached
	var retrieved CachedToken
	err = cache.Get(ctx, cacheKey, &retrieved)
	require.NoError(t, err)
	require.Equal(t, "cached-token", retrieved.AccessToken)

	// Invalidate
	err = service.InvalidateToken(ctx, "test-endpoint")
	require.NoError(t, err)

	// Verify it's gone
	var afterInvalidate CachedToken
	err = cache.Get(ctx, cacheKey, &afterInvalidate)
	require.NoError(t, err)
	require.Empty(t, afterInvalidate.AccessToken)
}

func TestOAuth2TokenService_GenerateClientAssertion(t *testing.T) {
	ctx := context.Background()
	service, _ := provideOAuth2TokenService()

	signingKey := generateTestECDSAPrivateKey(t)

	oauth2 := &datastore.OAuth2{
		URL:                "https://oauth.example.com/token",
		ClientID:           "test-client",
		AuthenticationType: datastore.ClientAssertionAuth,
		SigningKey:         signingKey,
		SigningAlgorithm:   "ES256",
		Issuer:             "test-client",
		Subject:            "test-client",
	}

	assertion, err := service.generateClientAssertion(ctx, oauth2)
	require.NoError(t, err)
	require.NotEmpty(t, assertion)

	// Verify it's a valid JWT format (3 parts separated by dots)
	// We can't easily verify the signature without the public key, but we can check format
	require.Contains(t, assertion, ".")
	dotCount := 0
	for _, r := range assertion {
		if r == '.' {
			dotCount++
		}
	}
	require.Equal(t, 2, dotCount, "JWT should have 2 dots separating 3 parts")
}

func TestOAuth2TokenService_JWKToECDSAPrivateKey(t *testing.T) {
	service, _ := provideOAuth2TokenService()

	signingKey := generateTestECDSAPrivateKey(t)

	privateKey, err := service.jwkToECDSAPrivateKey(signingKey)
	require.NoError(t, err)
	require.NotNil(t, privateKey)
	require.Equal(t, elliptic.P256(), privateKey.Curve)
	require.NotNil(t, privateKey.D)
	require.NotNil(t, privateKey.PublicKey.X)
	require.NotNil(t, privateKey.PublicKey.Y)
}

func TestOAuth2TokenService_JWKToECDSAPrivateKey_InvalidKeyType(t *testing.T) {
	service, _ := provideOAuth2TokenService()

	signingKey := &datastore.OAuth2SigningKey{
		Kty: "RSA", // Wrong key type
		Crv: "P-256",
		D:   "test",
	}

	_, err := service.jwkToECDSAPrivateKey(signingKey)
	require.Error(t, err)
	require.Contains(t, err.Error(), "expected EC key type")
}

func TestOAuth2TokenService_JWKToECDSAPrivateKey_InvalidCurve(t *testing.T) {
	service, _ := provideOAuth2TokenService()

	signingKey := &datastore.OAuth2SigningKey{
		Kty: "EC",
		Crv: "P-128", // Invalid/unsupported curve
		D:   base64.RawURLEncoding.EncodeToString([]byte("test")),
		X:   base64.RawURLEncoding.EncodeToString([]byte("test")),
		Y:   base64.RawURLEncoding.EncodeToString([]byte("test")),
	}

	_, err := service.jwkToECDSAPrivateKey(signingKey)
	require.Error(t, err)
	require.Contains(t, err.Error(), "unsupported curve")
}

func TestOAuth2TokenService_ExchangeToken_ErrorHandling(t *testing.T) {
	ctx := context.Background()
	service, _ := provideOAuth2TokenService()

	// Test with server returning error
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error": "invalid_client"}`))
	}))
	defer server.Close()

	endpoint := &datastore.Endpoint{
		UID: "test-endpoint-error",
		Authentication: &datastore.EndpointAuthentication{
			Type: datastore.OAuth2Authentication,
			OAuth2: &datastore.OAuth2{
				URL:                server.URL,
				ClientID:           "test-client-id",
				AuthenticationType: datastore.SharedSecretAuth,
				ClientSecret:       "test-secret",
			},
		},
	}

	_, err := service.GetAccessToken(ctx, endpoint)
	require.Error(t, err)
	require.Contains(t, err.Error(), "token exchange failed")
}

func TestOAuth2TokenService_GetAccessToken_InvalidEndpoint(t *testing.T) {
	ctx := context.Background()
	service, _ := provideOAuth2TokenService()

	// Endpoint without OAuth2
	endpoint := &datastore.Endpoint{
		UID: "test-endpoint",
		Authentication: &datastore.EndpointAuthentication{
			Type: datastore.APIKeyAuthentication,
		},
	}

	_, err := service.GetAccessToken(ctx, endpoint)
	require.Error(t, err)
	require.Contains(t, err.Error(), "does not have OAuth2 authentication")
}

func TestOAuth2TokenService_CalculateRefreshTime(t *testing.T) {
	service, _ := provideOAuth2TokenService()

	// Test with long TTL (should use 1 minute buffer - min of 10% or 1 minute)
	expiresAt := time.Now().Add(10 * time.Hour)
	refreshTime := service.calculateRefreshTime(expiresAt)
	// 10% of 10 hours = 1 hour, but should cap at 1 minute
	expectedBuffer := 1 * time.Minute
	actualBuffer := expiresAt.Sub(refreshTime)
	require.InDelta(t, float64(expectedBuffer), float64(actualBuffer), float64(10*time.Second))

	// Test with short TTL (should use 10% buffer if less than 1 minute)
	expiresAt2 := time.Now().Add(5 * time.Minute)
	refreshTime2 := service.calculateRefreshTime(expiresAt2)
	// 10% of 5 minutes = 30 seconds, which is less than 1 minute, so use 30 seconds
	expectedBuffer2 := 30 * time.Second
	actualBuffer2 := expiresAt2.Sub(refreshTime2)
	require.InDelta(t, float64(expectedBuffer2), float64(actualBuffer2), float64(5*time.Second))

	// Test with very long TTL (10% would be > 1 minute, so use 1 minute)
	expiresAt3 := time.Now().Add(2 * time.Hour)
	refreshTime3 := service.calculateRefreshTime(expiresAt3)
	// 10% of 2 hours = 12 minutes, but should cap at 1 minute
	expectedBuffer3 := 1 * time.Minute
	actualBuffer3 := expiresAt3.Sub(refreshTime3)
	require.InDelta(t, float64(expectedBuffer3), float64(actualBuffer3), float64(10*time.Second))
}

// Test helper functions for different curves and algorithms

func generateTestECDSAPrivateKeyForCurve(t *testing.T, curve elliptic.Curve, curveName string) *datastore.OAuth2SigningKey {
	t.Helper()

	privateKey, err := ecdsa.GenerateKey(curve, rand.Reader)
	require.NoError(t, err)

	// Get curve size in bytes
	curveSize := (curve.Params().BitSize + 7) / 8

	// Convert to JWK format
	xBytes := privateKey.PublicKey.X.Bytes()
	yBytes := privateKey.PublicKey.Y.Bytes()
	dBytes := privateKey.D.Bytes()

	// Pad to curve size
	xPadded := make([]byte, curveSize)
	yPadded := make([]byte, curveSize)
	dPadded := make([]byte, curveSize)

	copy(xPadded[curveSize-len(xBytes):], xBytes)
	copy(yPadded[curveSize-len(yBytes):], yBytes)
	copy(dPadded[curveSize-len(dBytes):], dBytes)

	signingKey := &datastore.OAuth2SigningKey{
		Kty: "EC",
		Crv: curveName,
		X:   base64.RawURLEncoding.EncodeToString(xPadded),
		Y:   base64.RawURLEncoding.EncodeToString(yPadded),
		D:   base64.RawURLEncoding.EncodeToString(dPadded),
		Kid: "test-key-id-" + curveName,
	}

	return signingKey
}

func generateTestRSAPrivateKey(t *testing.T) (*rsa.PrivateKey, *datastore.OAuth2SigningKey) {
	t.Helper()

	// Generate RSA key (2048 bits for testing)
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	// Convert to JWK format (base64url encoding without padding)
	// RSA modulus (n) - raw bytes
	nBytes := privateKey.N.Bytes()

	// Public exponent (e) - typically 65537, raw bytes
	eBytes := big.NewInt(int64(privateKey.E)).Bytes()

	// Private exponent (d) - raw bytes
	dBytes := privateKey.D.Bytes()

	// Prime factors (p, q) - raw bytes
	pBytes := privateKey.Primes[0].Bytes()
	qBytes := privateKey.Primes[1].Bytes()

	// CRT parameters - raw bytes
	dpBytes := privateKey.Precomputed.Dp.Bytes()
	dqBytes := privateKey.Precomputed.Dq.Bytes()
	qiBytes := privateKey.Precomputed.Qinv.Bytes()

	signingKey := &datastore.OAuth2SigningKey{
		Kty: "RSA",
		N:   base64.RawURLEncoding.EncodeToString(nBytes),
		E:   base64.RawURLEncoding.EncodeToString(eBytes),
		D:   base64.RawURLEncoding.EncodeToString(dBytes),
		P:   base64.RawURLEncoding.EncodeToString(pBytes),
		Q:   base64.RawURLEncoding.EncodeToString(qBytes),
		Dp:  base64.RawURLEncoding.EncodeToString(dpBytes),
		Dq:  base64.RawURLEncoding.EncodeToString(dqBytes),
		Qi:  base64.RawURLEncoding.EncodeToString(qiBytes),
		Kid: "test-rsa-key-id",
	}

	return privateKey, signingKey
}

// Test ECDSA curves: P-256, P-384, P-521
func TestOAuth2TokenService_ECDSACurves(t *testing.T) {
	ctx := context.Background()
	service, _ := provideOAuth2TokenService()

	curves := []struct {
		name      string
		curve     elliptic.Curve
		curveStr  string
		algorithm string
	}{
		{"P-256", elliptic.P256(), "P-256", "ES256"},
		{"P-384", elliptic.P384(), "P-384", "ES384"},
		{"P-521", elliptic.P521(), "P-521", "ES512"},
	}

	for _, tc := range curves {
		t.Run(tc.name, func(t *testing.T) {
			signingKey := generateTestECDSAPrivateKeyForCurve(t, tc.curve, tc.curveStr)

			// Create a mock OAuth2 token server
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				require.Equal(t, "POST", r.Method)
				err := r.ParseForm()
				require.NoError(t, err)
				require.NotEmpty(t, r.Form.Get("client_assertion"))

				response := map[string]interface{}{
					"access_token": "test-token-" + tc.name,
					"token_type":   "Bearer",
					"expires_in":   3600,
				}

				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				_ = json.NewEncoder(w).Encode(response)
			}))
			defer server.Close()

			endpoint := &datastore.Endpoint{
				UID: "test-endpoint-" + tc.name,
				Authentication: &datastore.EndpointAuthentication{
					Type: datastore.OAuth2Authentication,
					OAuth2: &datastore.OAuth2{
						URL:                server.URL,
						ClientID:           "test-client-id",
						GrantType:          "client_credentials",
						AuthenticationType: datastore.ClientAssertionAuth,
						SigningKey:         signingKey,
						SigningAlgorithm:   tc.algorithm,
						Issuer:             "test-client-id",
						Subject:            "test-client-id",
					},
				},
			}

			// Should successfully get token with this curve
			token, err := service.GetAccessToken(ctx, endpoint)
			require.NoError(t, err, "Failed for curve %s", tc.name)
			require.Equal(t, "test-token-"+tc.name, token)
		})
	}
}

// Test ECDSA algorithms: ES256, ES384, ES512
func TestOAuth2TokenService_ECDSAAlgorithms(t *testing.T) {
	ctx := context.Background()
	service, _ := provideOAuth2TokenService()

	algorithms := []struct {
		name     string
		curve    elliptic.Curve
		curveStr string
	}{
		{"ES256", elliptic.P256(), "P-256"},
		{"ES384", elliptic.P384(), "P-384"},
		{"ES512", elliptic.P521(), "P-521"},
	}

	for _, tc := range algorithms {
		t.Run(tc.name, func(t *testing.T) {
			signingKey := generateTestECDSAPrivateKeyForCurve(t, tc.curve, tc.curveStr)
			// Create a mock OAuth2 token server
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				require.Equal(t, "POST", r.Method)
				err := r.ParseForm()
				require.NoError(t, err)
				require.NotEmpty(t, r.Form.Get("client_assertion"))

				response := map[string]interface{}{
					"access_token": "test-token-" + tc.name,
					"token_type":   "Bearer",
					"expires_in":   3600,
				}

				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				_ = json.NewEncoder(w).Encode(response)
			}))
			defer server.Close()

			endpoint := &datastore.Endpoint{
				UID: "test-endpoint-" + tc.name,
				Authentication: &datastore.EndpointAuthentication{
					Type: datastore.OAuth2Authentication,
					OAuth2: &datastore.OAuth2{
						URL:                server.URL,
						ClientID:           "test-client-id",
						GrantType:          "client_credentials",
						AuthenticationType: datastore.ClientAssertionAuth,
						SigningKey:         signingKey,
						SigningAlgorithm:   tc.name,
						Issuer:             "test-client-id",
						Subject:            "test-client-id",
					},
				},
			}

			// Should successfully get token with this algorithm
			token, err := service.GetAccessToken(ctx, endpoint)
			require.NoError(t, err, "Failed for algorithm %s", tc.name)
			require.Equal(t, "test-token-"+tc.name, token)
		})
	}
}

// Test RSA algorithms: RS256, RS384, RS512, PS256, PS384, PS512
func TestOAuth2TokenService_RSAAlgorithms(t *testing.T) {
	ctx := context.Background()
	service, _ := provideOAuth2TokenService()

	algorithms := []string{"RS256", "RS384", "RS512", "PS256", "PS384", "PS512"}

	for _, alg := range algorithms {
		t.Run(alg, func(t *testing.T) {
			_, signingKey := generateTestRSAPrivateKey(t)

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				require.Equal(t, "POST", r.Method)
				err := r.ParseForm()
				require.NoError(t, err)
				require.NotEmpty(t, r.Form.Get("client_assertion"))

				response := map[string]interface{}{
					"access_token": "test-token-" + alg,
					"token_type":   "Bearer",
					"expires_in":   3600,
				}

				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				_ = json.NewEncoder(w).Encode(response)
			}))
			defer server.Close()

			endpoint := &datastore.Endpoint{
				UID: "test-endpoint-rsa-" + alg,
				Authentication: &datastore.EndpointAuthentication{
					Type: datastore.OAuth2Authentication,
					OAuth2: &datastore.OAuth2{
						URL:                server.URL,
						ClientID:           "test-client-id",
						GrantType:          "client_credentials",
						AuthenticationType: datastore.ClientAssertionAuth,
						SigningKey:         signingKey,
						SigningAlgorithm:   alg,
						Issuer:             "test-client-id",
						Subject:            "test-client-id",
					},
				},
			}

			// Should successfully get token with this RSA algorithm
			token, err := service.GetAccessToken(ctx, endpoint)
			require.NoError(t, err, "Failed for algorithm %s", alg)
			require.Equal(t, "test-token-"+alg, token)
		})
	}
}

// Test algorithm validation - should reject unsupported algorithms
func TestOAuth2TokenService_UnsupportedAlgorithm(t *testing.T) {
	ctx := context.Background()
	service, _ := provideOAuth2TokenService()

	signingKey := generateTestECDSAPrivateKey(t)

	endpoint := &datastore.Endpoint{
		UID: "test-endpoint-unsupported",
		Authentication: &datastore.EndpointAuthentication{
			Type: datastore.OAuth2Authentication,
			OAuth2: &datastore.OAuth2{
				URL:                "https://oauth.example.com/token",
				ClientID:           "test-client-id",
				GrantType:          "client_credentials",
				AuthenticationType: datastore.ClientAssertionAuth,
				SigningKey:         signingKey,
				SigningAlgorithm:   "HS256", // HMAC not supported for client assertion
				Issuer:             "test-client-id",
				Subject:            "test-client-id",
			},
		},
	}

	// Should fail with unsupported algorithm
	_, err := service.GetAccessToken(ctx, endpoint)
	require.Error(t, err)
	require.Contains(t, err.Error(), "unsupported")
}

// Test curve validation - should reject unsupported curves
func TestOAuth2TokenService_UnsupportedCurve(t *testing.T) {
	ctx := context.Background()
	service, _ := provideOAuth2TokenService()

	// Create a signing key with unsupported curve
	signingKey := &datastore.OAuth2SigningKey{
		Kty: "EC",
		Crv: "P-224", // Not supported
		X:   "test-x",
		Y:   "test-y",
		D:   "test-d",
		Kid: "test-key-id",
	}

	endpoint := &datastore.Endpoint{
		UID: "test-endpoint-unsupported-curve",
		Authentication: &datastore.EndpointAuthentication{
			Type: datastore.OAuth2Authentication,
			OAuth2: &datastore.OAuth2{
				URL:                "https://oauth.example.com/token",
				ClientID:           "test-client-id",
				GrantType:          "client_credentials",
				AuthenticationType: datastore.ClientAssertionAuth,
				SigningKey:         signingKey,
				SigningAlgorithm:   "ES256",
				Issuer:             "test-client-id",
				Subject:            "test-client-id",
			},
		},
	}

	// Should fail with unsupported curve
	_, err := service.GetAccessToken(ctx, endpoint)
	require.Error(t, err)
	require.Contains(t, err.Error(), "unsupported")
}

// Test default algorithm when not specified
func TestOAuth2TokenService_DefaultAlgorithm(t *testing.T) {
	ctx := context.Background()
	service, _ := provideOAuth2TokenService()

	signingKey := generateTestECDSAPrivateKey(t)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		err := r.ParseForm()
		require.NoError(t, err)
		require.NotEmpty(t, r.Form.Get("client_assertion"))

		response := map[string]interface{}{
			"access_token": "test-token-default",
			"token_type":   "Bearer",
			"expires_in":   3600,
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	endpoint := &datastore.Endpoint{
		UID: "test-endpoint-default-alg",
		Authentication: &datastore.EndpointAuthentication{
			Type: datastore.OAuth2Authentication,
			OAuth2: &datastore.OAuth2{
				URL:                server.URL,
				ClientID:           "test-client-id",
				GrantType:          "client_credentials",
				AuthenticationType: datastore.ClientAssertionAuth,
				SigningKey:         signingKey,
				SigningAlgorithm:   "", // Empty - should default to ES256
				Issuer:             "test-client-id",
				Subject:            "test-client-id",
			},
		},
	}

	// Should use default algorithm (ES256) when not specified
	token, err := service.GetAccessToken(ctx, endpoint)
	require.NoError(t, err)
	require.Equal(t, "test-token-default", token)
}
