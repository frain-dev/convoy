package services

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/frain-dev/convoy/cache"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/pkg/log"
)

const (
	defaultAssertionLifetime  = 5 * time.Minute
	minRefreshBuffer          = 1 * time.Minute
	maxTokenCacheTTL          = 1 * time.Hour
	oauth2TokenCacheKeyPrefix = "oauth2_token:"
	defaultTokenType          = "Bearer"
)

// OAuth2TokenResponse represents the response from OAuth2 token endpoint
type OAuth2TokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int    `json:"expires_in"` // Time in seconds
	Scope       string `json:"scope,omitempty"`
}

// CachedToken represents a cached OAuth2 access token
// Exported for use in API handlers
type CachedToken struct {
	AccessToken string    `json:"access_token"`
	TokenType   string    `json:"token_type"`
	ExpiresAt   time.Time `json:"expires_at"`
}

type OAuth2TokenService struct {
	Cache      cache.Cache
	Logger     log.StdLogger
	HTTPClient *http.Client
}

func NewOAuth2TokenService(cache cache.Cache, logger log.StdLogger) *OAuth2TokenService {
	return &OAuth2TokenService{
		Cache:  cache,
		Logger: logger,
		HTTPClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// GetAccessToken retrieves a valid access token for the endpoint.
// Kept for backward compatibility. Internally uses GetAuthorizationHeader.
func (s *OAuth2TokenService) GetAccessToken(ctx context.Context, endpoint *datastore.Endpoint) (string, error) {
	authHeader, err := s.GetAuthorizationHeader(ctx, endpoint)
	if err != nil {
		return "", err
	}

	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) == 2 {
		return parts[1], nil
	}
	return authHeader, nil
}

// GetAuthorizationHeader returns the formatted Authorization header value (e.g., "Bearer token").
// Respects the token type from the OAuth2 response.
func (s *OAuth2TokenService) GetAuthorizationHeader(ctx context.Context, endpoint *datastore.Endpoint) (string, error) {
	if endpoint.Authentication == nil || endpoint.Authentication.Type != datastore.OAuth2Authentication {
		return "", errors.New("endpoint does not have OAuth2 authentication configured")
	}

	oauth2 := endpoint.Authentication.OAuth2
	if oauth2 == nil {
		return "", errors.New("oauth2 configuration is missing")
	}

	cacheKey := oauth2TokenCacheKeyPrefix + endpoint.UID
	var cachedToken CachedToken
	err := s.Cache.Get(ctx, cacheKey, &cachedToken)
	if err == nil && cachedToken.AccessToken != "" {
		refreshTime := s.calculateRefreshTime(cachedToken.ExpiresAt)
		if time.Now().Before(refreshTime) {
			tokenType := cachedToken.TokenType
			if tokenType == "" {
				tokenType = defaultTokenType
			}
			return fmt.Sprintf("%s %s", tokenType, cachedToken.AccessToken), nil
		}
	}

	token, err := s.exchangeToken(ctx, oauth2, endpoint.UID)
	if err != nil {
		return "", fmt.Errorf("failed to exchange token: %w", err)
	}

	expiresIn := time.Duration(token.ExpiresIn) * time.Second
	if expiresIn > maxTokenCacheTTL {
		expiresIn = maxTokenCacheTTL
	}
	if expiresIn <= 0 {
		expiresIn = 1 * time.Hour
	}

	cachedToken = CachedToken{
		AccessToken: token.AccessToken,
		TokenType:   token.TokenType,
		ExpiresAt:   time.Now().Add(expiresIn),
	}

	err = s.Cache.Set(ctx, cacheKey, cachedToken, expiresIn)
	if err != nil {
		s.Logger.WithError(err).Warn("failed to cache OAuth2 token")
	}

	tokenType := token.TokenType
	if tokenType == "" {
		tokenType = defaultTokenType
	}
	return fmt.Sprintf("%s %s", tokenType, token.AccessToken), nil
}

// InvalidateToken removes the cached token for an endpoint.
func (s *OAuth2TokenService) InvalidateToken(ctx context.Context, endpointID string) error {
	cacheKey := oauth2TokenCacheKeyPrefix + endpointID
	return s.Cache.Delete(ctx, cacheKey)
}

// exchangeToken exchanges credentials/assertion for an access token.
func (s *OAuth2TokenService) exchangeToken(ctx context.Context, oauth2 *datastore.OAuth2, _ string) (*OAuth2TokenResponse, error) {
	var reqBody url.Values
	var err error

	switch oauth2.AuthenticationType {
	case datastore.SharedSecretAuth:
		reqBody = s.buildSharedSecretRequest(oauth2)
	case datastore.ClientAssertionAuth:
		reqBody, err = s.buildClientAssertionRequest(ctx, oauth2)
		if err != nil {
			return nil, fmt.Errorf("failed to build client assertion: %w", err)
		}
	default:
		return nil, fmt.Errorf("unsupported authentication type: %s", oauth2.AuthenticationType)
	}

	// Make the token exchange request
	req, err := http.NewRequestWithContext(ctx, "POST", oauth2.URL, strings.NewReader(reqBody.Encode()))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := s.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("token exchange request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("token exchange failed with status %d: failed to read error response body: %w", resp.StatusCode, err)
		}
		return nil, fmt.Errorf("token exchange failed with status %d: %s", resp.StatusCode, string(body))
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	var responseMap map[string]interface{}
	if err := json.Unmarshal(bodyBytes, &responseMap); err != nil {
		return nil, fmt.Errorf("failed to decode token response: %w", err)
	}

	accessTokenField := "access_token"
	tokenTypeField := "token_type"
	expiresInField := "expires_in"

	if oauth2.FieldMapping != nil {
		if oauth2.FieldMapping.AccessToken != "" {
			accessTokenField = oauth2.FieldMapping.AccessToken
		}
		if oauth2.FieldMapping.TokenType != "" {
			tokenTypeField = oauth2.FieldMapping.TokenType
		}
		if oauth2.FieldMapping.ExpiresIn != "" {
			expiresInField = oauth2.FieldMapping.ExpiresIn
		}
	}

	var accessToken string
	if val, ok := responseMap[accessTokenField]; ok {
		accessToken, _ = val.(string)
	}
	if accessToken == "" {
		return nil, fmt.Errorf("access token field '%s' is missing or empty in response", accessTokenField)
	}

	var tokenType string
	if val, ok := responseMap[tokenTypeField]; ok {
		tokenType, _ = val.(string)
	}
	if tokenType == "" {
		tokenType = defaultTokenType
	}

	var expiresIn int
	if val, ok := responseMap[expiresInField]; ok {
		expiresIn = s.extractExpiresIn(val, oauth2.ExpiryTimeUnit)
	}
	if expiresIn == 0 {
		expiresIn = 3600 // Default to 1 hour if not provided
	}

	return &OAuth2TokenResponse{
		AccessToken: accessToken,
		TokenType:   tokenType,
		ExpiresIn:   expiresIn,
	}, nil
}

// extractExpiresIn extracts and converts expiry time to seconds based on the configured unit.
func (s *OAuth2TokenService) extractExpiresIn(val interface{}, unit datastore.OAuth2ExpiryTimeUnit) int {
	var expiresIn float64

	switch v := val.(type) {
	case float64:
		expiresIn = v
	case int:
		expiresIn = float64(v)
	case int64:
		expiresIn = float64(v)
	case string:
		if parsed, err := strconv.ParseFloat(v, 64); err == nil {
			expiresIn = parsed
		} else {
			return 0
		}
	default:
		return 0
	}

	switch unit {
	case datastore.ExpiryTimeUnitMilliseconds:
		return int(expiresIn / 1000)
	case datastore.ExpiryTimeUnitMinutes:
		return int(expiresIn * 60)
	case datastore.ExpiryTimeUnitHours:
		return int(expiresIn * 3600)
	case datastore.ExpiryTimeUnitSeconds:
		return int(expiresIn)
	default:
		return int(expiresIn)
	}
}

// buildSharedSecretRequest builds the request body for shared secret authentication.
func (s *OAuth2TokenService) buildSharedSecretRequest(oauth2 *datastore.OAuth2) url.Values {
	reqBody := url.Values{}
	reqBody.Set("grant_type", s.getGrantType(oauth2))
	reqBody.Set("client_id", oauth2.ClientID)
	reqBody.Set("client_secret", oauth2.ClientSecret)

	if oauth2.Scope != "" {
		reqBody.Set("scope", oauth2.Scope)
	}
	if oauth2.Audience != "" {
		reqBody.Set("audience", oauth2.Audience)
	}

	return reqBody
}

// buildClientAssertionRequest builds the request body for client assertion authentication.
func (s *OAuth2TokenService) buildClientAssertionRequest(ctx context.Context, oauth2 *datastore.OAuth2) (url.Values, error) {
	assertion, err := s.generateClientAssertion(ctx, oauth2)
	if err != nil {
		return nil, fmt.Errorf("failed to generate client assertion: %w", err)
	}

	reqBody := url.Values{}
	reqBody.Set("grant_type", s.getGrantType(oauth2))
	reqBody.Set("client_id", oauth2.ClientID)
	reqBody.Set("client_assertion_type", "urn:ietf:params:oauth:client-assertion-type:jwt-bearer")
	reqBody.Set("client_assertion", assertion)

	if oauth2.Scope != "" {
		reqBody.Set("scope", oauth2.Scope)
	}
	if oauth2.Audience != "" {
		reqBody.Set("audience", oauth2.Audience)
	}

	return reqBody, nil
}

// generateClientAssertion generates a signed JWT assertion for client assertion authentication.
func (s *OAuth2TokenService) generateClientAssertion(_ context.Context, oauth2 *datastore.OAuth2) (string, error) {
	if oauth2.SigningKey == nil {
		return "", errors.New("signing_key is required for client assertion")
	}

	signingMethod, err := s.getSigningMethod(oauth2.SigningAlgorithm)
	if err != nil {
		return "", fmt.Errorf("failed to get signing method: %w", err)
	}

	privateKey, err := s.jwkToPrivateKey(oauth2.SigningKey)
	if err != nil {
		return "", fmt.Errorf("failed to parse signing key: %w", err)
	}

	now := time.Now()
	exp := now.Add(defaultAssertionLifetime)

	claims := jwt.MapClaims{
		"iss": oauth2.Issuer,
		"sub": oauth2.Subject,
		"aud": oauth2.URL,
		"exp": exp.Unix(),
		"iat": now.Unix(),
		"jti": fmt.Sprintf("%s-%d", oauth2.ClientID, now.UnixNano()),
	}

	token := jwt.NewWithClaims(signingMethod, claims)
	token.Header["kid"] = oauth2.SigningKey.Kid

	assertion, err := token.SignedString(privateKey)
	if err != nil {
		return "", fmt.Errorf("failed to sign assertion: %w", err)
	}

	return assertion, nil
}

// getSigningMethod returns the JWT signing method for the given algorithm.
func (s *OAuth2TokenService) getSigningMethod(algorithm string) (jwt.SigningMethod, error) {
	if algorithm == "" {
		algorithm = "ES256"
	}

	switch algorithm {
	case "ES256":
		return jwt.SigningMethodES256, nil
	case "ES384":
		return jwt.SigningMethodES384, nil
	case "ES512":
		return jwt.SigningMethodES512, nil
	case "RS256":
		return jwt.SigningMethodRS256, nil
	case "RS384":
		return jwt.SigningMethodRS384, nil
	case "RS512":
		return jwt.SigningMethodRS512, nil
	case "PS256":
		return jwt.SigningMethodPS256, nil
	case "PS384":
		return jwt.SigningMethodPS384, nil
	case "PS512":
		return jwt.SigningMethodPS512, nil
	default:
		return nil, fmt.Errorf("unsupported signing algorithm: %s", algorithm)
	}
}

// jwkToPrivateKey converts a JWK to a private key (ECDSA or RSA).
func (s *OAuth2TokenService) jwkToPrivateKey(jwk *datastore.OAuth2SigningKey) (interface{}, error) {
	switch jwk.Kty {
	case "EC":
		return s.jwkToECDSAPrivateKey(jwk)
	case "RSA":
		return s.jwkToRSAPrivateKey(jwk)
	default:
		return nil, fmt.Errorf("unsupported key type: %s (only EC and RSA are supported)", jwk.Kty)
	}
}

// jwkToECDSAPrivateKey converts a JWK to an ECDSA private key.
func (s *OAuth2TokenService) jwkToECDSAPrivateKey(jwk *datastore.OAuth2SigningKey) (*ecdsa.PrivateKey, error) {
	if jwk.Kty != "EC" {
		return nil, fmt.Errorf("expected EC key type, got: %s", jwk.Kty)
	}

	var curve elliptic.Curve
	switch jwk.Crv {
	case "P-256":
		curve = elliptic.P256()
	case "P-384":
		curve = elliptic.P384()
	case "P-521":
		curve = elliptic.P521()
	default:
		return nil, fmt.Errorf("unsupported curve: %s (supported: P-256, P-384, P-521)", jwk.Crv)
	}

	// Decode the coordinates
	xBytes, err := base64.RawURLEncoding.DecodeString(jwk.X)
	if err != nil {
		return nil, fmt.Errorf("failed to decode x coordinate: %w", err)
	}

	yBytes, err := base64.RawURLEncoding.DecodeString(jwk.Y)
	if err != nil {
		return nil, fmt.Errorf("failed to decode y coordinate: %w", err)
	}

	// Decode the private key (d)
	dBytes, err := base64.RawURLEncoding.DecodeString(jwk.D)
	if err != nil {
		return nil, fmt.Errorf("failed to decode private key: %w", err)
	}

	// Convert to big integers
	x := new(big.Int).SetBytes(xBytes)
	y := new(big.Int).SetBytes(yBytes)
	d := new(big.Int).SetBytes(dBytes)

	// Create ECDSA private key
	privateKey := &ecdsa.PrivateKey{
		PublicKey: ecdsa.PublicKey{
			Curve: curve,
			X:     x,
			Y:     y,
		},
		D: d,
	}

	// Validate the key
	if !curve.IsOnCurve(x, y) {
		return nil, errors.New("public key point is not on the curve")
	}

	return privateKey, nil
}

// jwkToRSAPrivateKey converts a JWK to an RSA private key.
func (s *OAuth2TokenService) jwkToRSAPrivateKey(jwk *datastore.OAuth2SigningKey) (*rsa.PrivateKey, error) {
	if jwk.Kty != "RSA" {
		return nil, fmt.Errorf("expected RSA key type, got: %s", jwk.Kty)
	}

	// Decode modulus (n)
	nBytes, err := base64.RawURLEncoding.DecodeString(jwk.N)
	if err != nil {
		return nil, fmt.Errorf("failed to decode modulus: %w", err)
	}

	// Decode public exponent (e)
	eBytes, err := base64.RawURLEncoding.DecodeString(jwk.E)
	if err != nil {
		return nil, fmt.Errorf("failed to decode public exponent: %w", err)
	}

	// Decode private exponent (d) - required for private key
	dBytes, err := base64.RawURLEncoding.DecodeString(jwk.D)
	if err != nil {
		return nil, fmt.Errorf("failed to decode private exponent: %w", err)
	}

	// Convert to big integers
	n := new(big.Int).SetBytes(nBytes)
	e := new(big.Int).SetBytes(eBytes)
	d := new(big.Int).SetBytes(dBytes)

	// Check if we have CRT parameters (for optimized private key)
	if jwk.P != "" && jwk.Q != "" {
		// Decode prime factors
		pBytes, err := base64.RawURLEncoding.DecodeString(jwk.P)
		if err != nil {
			return nil, fmt.Errorf("failed to decode prime P: %w", err)
		}

		qBytes, err := base64.RawURLEncoding.DecodeString(jwk.Q)
		if err != nil {
			return nil, fmt.Errorf("failed to decode prime Q: %w", err)
		}

		p := new(big.Int).SetBytes(pBytes)
		q := new(big.Int).SetBytes(qBytes)

		// Decode CRT parameters if available
		var dp, dq, qi *big.Int
		if jwk.Dp != "" {
			dpBytes, err := base64.RawURLEncoding.DecodeString(jwk.Dp)
			if err != nil {
				return nil, fmt.Errorf("failed to decode Dp: %w", err)
			}
			dp = new(big.Int).SetBytes(dpBytes)
		}

		if jwk.Dq != "" {
			dqBytes, err := base64.RawURLEncoding.DecodeString(jwk.Dq)
			if err != nil {
				return nil, fmt.Errorf("failed to decode Dq: %w", err)
			}
			dq = new(big.Int).SetBytes(dqBytes)
		}

		if jwk.Qi != "" {
			qiBytes, err := base64.RawURLEncoding.DecodeString(jwk.Qi)
			if err != nil {
				return nil, fmt.Errorf("failed to decode Qi: %w", err)
			}
			qi = new(big.Int).SetBytes(qiBytes)
		}

		// Create RSA private key with CRT parameters
		privateKey := &rsa.PrivateKey{
			PublicKey: rsa.PublicKey{
				N: n,
				E: int(e.Int64()),
			},
			D:      d,
			Primes: []*big.Int{p, q},
		}

		if dp != nil && dq != nil && qi != nil {
			privateKey.Precomputed = rsa.PrecomputedValues{
				Dp:   dp,
				Dq:   dq,
				Qinv: qi,
			}
		}

		// Validate the key
		if err := privateKey.Validate(); err != nil {
			return nil, fmt.Errorf("invalid RSA private key: %w", err)
		}

		return privateKey, nil
	}

	// RSA private key requires prime factors (p, q) and CRT parameters (dp, dq, qi)
	// Computing primes from n is computationally expensive and not recommended
	return nil, errors.New("RSA private key requires prime factors (p, q) and CRT parameters (dp, dq, qi)")
}

// getGrantType returns the grant type, defaulting to client_credentials
func (s *OAuth2TokenService) getGrantType(oauth2 *datastore.OAuth2) string {
	if oauth2.GrantType != "" {
		return oauth2.GrantType
	}
	return "client_credentials"
}

// calculateRefreshTime calculates when to refresh the token (10% of TTL or 1 minute, whichever is smaller)
func (s *OAuth2TokenService) calculateRefreshTime(expiresAt time.Time) time.Time {
	ttl := time.Until(expiresAt)
	if ttl <= 0 {
		return time.Now() // Already expired
	}

	refreshBuffer := ttl / 10 // 10% of TTL
	if refreshBuffer > minRefreshBuffer {
		refreshBuffer = minRefreshBuffer
	}

	return expiresAt.Add(-refreshBuffer)
}
