package services

import (
	"context"
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"strings"
	"time"

	"github.com/oklog/ulid/v2"

	"github.com/frain-dev/convoy/api/models"
	"github.com/frain-dev/convoy/api/types"
	"github.com/frain-dev/convoy/auth/realm/jwt"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/internal/pkg/license"
	"github.com/frain-dev/convoy/pkg/log"
	"github.com/frain-dev/convoy/util"
)

type GoogleOAuthService struct {
	UserRepo      datastore.UserRepository
	OrgRepo       datastore.OrganisationRepository
	OrgMemberRepo datastore.OrganisationMemberRepository
	JWT           *jwt.Jwt
	ConfigRepo    datastore.ConfigurationRepository
	Licenser      license.Licenser
}

func NewGoogleOAuthService(
	userRepo datastore.UserRepository,
	orgRepo datastore.OrganisationRepository,
	orgMemberRepo datastore.OrganisationMemberRepository,
	jwt *jwt.Jwt,
	configRepo datastore.ConfigurationRepository,
	licenser license.Licenser,
) *GoogleOAuthService {
	return &GoogleOAuthService{
		UserRepo:      userRepo,
		OrgRepo:       orgRepo,
		OrgMemberRepo: orgMemberRepo,
		JWT:           jwt,
		ConfigRepo:    configRepo,
		Licenser:      licenser,
	}
}

func (g *GoogleOAuthService) HandleIDToken(ctx context.Context, idToken string, a *types.APIOptions) (*datastore.User, *jwt.Token, error) {
	log.FromContext(ctx).Info("HandleIDToken called - processing Google ID token")

	// Verify the Google ID token and extract user info
	userInfo, err := g.verifyGoogleIDToken(idToken)
	if err != nil {
		return nil, nil, err
	}

	// Check if user already exists
	user, err := g.UserRepo.FindUserByEmail(ctx, userInfo.Email)
	if err != nil && !errors.Is(err, datastore.ErrUserNotFound) {
		return nil, nil, &ServiceError{ErrMsg: "failed to check user existence", Err: err}
	}

	if user != nil {
		// User exists, authenticate them
		token, err := g.JWT.GenerateToken(user)
		if err != nil {
			return nil, nil, &ServiceError{ErrMsg: "failed to generate token", Err: err}
		}

		return user, &token, nil
	}

	// User doesn't exist, return user info for setup flow
	// Don't create the user yet - wait for setup completion
	return &datastore.User{
		FirstName:     userInfo.GivenName,
		LastName:      userInfo.FamilyName,
		Email:         userInfo.Email,
		EmailVerified: true,
		AuthType:      string(datastore.GoogleOAuthUserType),
	}, nil, nil
}

func (g *GoogleOAuthService) CompleteGoogleOAuthSetup(ctx context.Context, idToken, businessName string, a *types.APIOptions) (*datastore.User, *jwt.Token, error) {
	// Validate business name
	if businessName == "" || strings.TrimSpace(businessName) == "" {
		return nil, nil, &ServiceError{ErrMsg: "business name is required"}
	}

	if len(strings.TrimSpace(businessName)) < 2 {
		return nil, nil, &ServiceError{ErrMsg: "business name must be at least 2 characters"}
	}

	// Check license limits
	ok, err := a.Licenser.CreateUser(ctx)
	if err != nil {
		return nil, nil, &ServiceError{ErrMsg: err.Error()}
	}
	if !ok {
		return nil, nil, &ServiceError{ErrMsg: ErrUserLimit.Error()}
	}

	// Check if signup is enabled
	cfg, err := g.ConfigRepo.LoadConfiguration(ctx)
	if err != nil && !errors.Is(err, datastore.ErrConfigNotFound) {
		return nil, nil, &ServiceError{ErrMsg: "failed to load configuration", Err: err}
	}
	if cfg != nil && !cfg.IsSignupEnabled {
		return nil, nil, &ServiceError{ErrMsg: datastore.ErrSignupDisabled.Error(), Err: datastore.ErrSignupDisabled}
	}

	// Generate a random password for OAuth users (they won't use it)
	p := datastore.Password{Plaintext: ulid.Make().String()}
	err = p.GenerateHash()
	if err != nil {
		log.FromContext(ctx).WithError(err).Error("failed to generate hash")
		return nil, nil, &ServiceError{ErrMsg: "failed to generate hash", Err: err}
	}

	// Verify the ID token and extract user info
	userInfo, err := g.verifyGoogleIDToken(idToken)
	if err != nil {
		return nil, nil, &ServiceError{ErrMsg: "invalid ID token", Err: err}
	}

	// Extract names from user info or generate from email
	firstName := userInfo.GivenName
	lastName := userInfo.FamilyName
	if firstName == "" || lastName == "" {
		firstName, lastName = util.ExtractOrGenerateNamesFromEmail(userInfo.Email)
	}

	// Create the actual user with all required fields
	user := &datastore.User{
		UID:                    ulid.Make().String(),
		FirstName:              firstName,
		LastName:               lastName,
		Email:                  userInfo.Email,
		Password:               string(p.Hash),
		EmailVerificationToken: ulid.Make().String(),
		CreatedAt:              time.Now(),
		UpdatedAt:              time.Now(),
		EmailVerified:          true, // Google OAuth users are pre-verified
		AuthType:               string(datastore.GoogleOAuthUserType),
	}

	err = g.UserRepo.CreateUser(ctx, user)
	if err != nil {
		if errors.Is(err, datastore.ErrDuplicateEmail) {
			return nil, nil, &ServiceError{ErrMsg: "this email is taken"}
		}
		log.FromContext(ctx).WithError(err).Error("failed to create user")
		return nil, nil, &ServiceError{ErrMsg: "failed to create user", Err: err}
	}

	// Create organization for the user
	co := CreateOrganisationService{
		OrgRepo:       g.OrgRepo,
		OrgMemberRepo: g.OrgMemberRepo,
		Licenser:      g.Licenser,
		NewOrg:        &models.Organisation{Name: businessName},
		User:          user,
	}

	_, err = co.Run(ctx)
	if err != nil && !errors.Is(err, ErrOrgLimit) && !errors.Is(err, ErrUserLimit) {
		return nil, nil, err
	}

	// Generate JWT token
	token, err := g.JWT.GenerateToken(user)
	if err != nil {
		log.FromContext(ctx).WithError(err).Error("failed to generate token")
		return nil, nil, &ServiceError{ErrMsg: "failed to generate token", Err: err}
	}

	return user, &token, nil
}

func (g *GoogleOAuthService) verifyGoogleIDToken(idToken string) (*GoogleUserInfo, error) {
	// Split the JWT token into parts
	parts := strings.Split(idToken, ".")
	if len(parts) != 3 {
		return nil, errors.New("invalid JWT token format")
	}

	// Decode the header to get the key ID (kid)
	headerBytes, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return nil, fmt.Errorf("failed to decode JWT header: %w", err)
	}

	var header struct {
		Kid string `json:"kid"`
		Alg string `json:"alg"`
	}
	if err := json.Unmarshal(headerBytes, &header); err != nil {
		return nil, fmt.Errorf("failed to parse JWT header: %w", err)
	}

	// Fetch Google's public keys
	jwks, err := g.fetchGoogleJWKS()
	if err != nil {
		return nil, fmt.Errorf("failed to fetch Google JWKS: %w", err)
	}

	// Find the matching key
	var publicKey *rsa.PublicKey
	for _, key := range jwks.Keys {
		if key.Kid == header.Kid {
			publicKey, err = g.jwkToRSAPublicKey(key)
			if err != nil {
				return nil, fmt.Errorf("failed to convert JWK to RSA public key: %w", err)
			}
			break
		}
	}

	if publicKey == nil {
		return nil, errors.New("no matching public key found for token")
	}

	// Verify the token signature
	payload := parts[0] + "." + parts[1]
	signature, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return nil, fmt.Errorf("failed to decode signature: %w", err)
	}

	// Hash the payload
	hasher := sha256.New()
	hasher.Write([]byte(payload))
	hashedPayload := hasher.Sum(nil)

	// Verify the signature
	err = rsa.VerifyPKCS1v15(publicKey, crypto.SHA256, hashedPayload, signature)
	if err != nil {
		return nil, fmt.Errorf("invalid token signature: %w", err)
	}

	// Decode the payload
	payloadBytes, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, fmt.Errorf("failed to decode JWT payload: %w", err)
	}

	var claims GoogleIDTokenClaims
	if err := json.Unmarshal(payloadBytes, &claims); err != nil {
		return nil, fmt.Errorf("failed to parse JWT claims: %w", err)
	}

	// Validate the claims
	if err := g.validateIDTokenClaims(claims); err != nil {
		return nil, fmt.Errorf("invalid token claims: %w", err)
	}

	// Extract user info from claims
	userInfo := &GoogleUserInfo{
		ID:            claims.Sub,
		Email:         claims.Email,
		VerifiedEmail: claims.EmailVerified,
		Name:          claims.Name,
		GivenName:     claims.GivenName,
		FamilyName:    claims.FamilyName,
		Picture:       claims.Picture,
		Locale:        claims.Locale,
	}

	return userInfo, nil
}

func (g *GoogleOAuthService) fetchGoogleJWKS() (*GoogleJWKS, error) {
	// Google's JWKS endpoint
	jwksURL := "https://www.googleapis.com/oauth2/v3/certs"

	resp, err := http.Get(jwksURL)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch JWKS: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("JWKS request failed with status: %d", resp.StatusCode)
	}

	var jwks GoogleJWKS
	if err := json.NewDecoder(resp.Body).Decode(&jwks); err != nil {
		return nil, fmt.Errorf("failed to decode JWKS response: %w", err)
	}

	return &jwks, nil
}

func (g *GoogleOAuthService) jwkToRSAPublicKey(jwk GoogleJWK) (*rsa.PublicKey, error) {
	// Decode the modulus (N)
	nBytes, err := base64.RawURLEncoding.DecodeString(jwk.N)
	if err != nil {
		return nil, fmt.Errorf("failed to decode modulus: %w", err)
	}

	// Decode the exponent (E)
	eBytes, err := base64.RawURLEncoding.DecodeString(jwk.E)
	if err != nil {
		return nil, fmt.Errorf("failed to decode exponent: %w", err)
	}

	// Convert to big integers
	n := new(big.Int).SetBytes(nBytes)
	e := new(big.Int).SetBytes(eBytes)

	// Create RSA public key
	publicKey := &rsa.PublicKey{
		N: n,
		E: int(e.Int64()),
	}

	return publicKey, nil
}

func (g *GoogleOAuthService) validateIDTokenClaims(claims GoogleIDTokenClaims) error {
	// Check if token is expired
	now := time.Now().Unix()
	if claims.Exp < now {
		return errors.New("token has expired")
	}

	// Check if token was issued in the future (with some tolerance)
	if claims.Iat > now+60 { // 1 minute tolerance
		return errors.New("token issued in the future")
	}

	// Verify issuer
	if claims.Iss != "https://accounts.google.com" && claims.Iss != "accounts.google.com" {
		return errors.New("invalid issuer")
	}

	// Verify audience matches configured Google OAuth client ID
	if claims.Aud == "" {
		return errors.New("missing audience")
	}
	cfg, err := config.Get()
	if err != nil {
		return err
	}
	if claims.Aud != cfg.Auth.GoogleOAuth.ClientID {
		return errors.New("invalid audience")
	}

	// Check if email is verified
	if !claims.EmailVerified {
		return errors.New("email not verified")
	}

	// Check if email is present
	if claims.Email == "" {
		return errors.New("missing email")
	}

	return nil
}

// GoogleUserInfo represents the user information returned by Google OAuth
type GoogleUserInfo struct {
	ID            string `json:"id"`
	Email         string `json:"email"`
	VerifiedEmail bool   `json:"verified_email"`
	Name          string `json:"name"`
	GivenName     string `json:"given_name"`
	FamilyName    string `json:"family_name"`
	Picture       string `json:"picture"`
	Locale        string `json:"locale"`
}

// GoogleJWK represents a JSON Web Key from Google's JWKS endpoint
type GoogleJWK struct {
	Kid string `json:"kid"`
	Kty string `json:"kty"`
	Alg string `json:"alg"`
	Use string `json:"use"`
	N   string `json:"n"`
	E   string `json:"e"`
}

// GoogleJWKS represents the JSON Web Key Set response
type GoogleJWKS struct {
	Keys []GoogleJWK `json:"keys"`
}

// GoogleIDTokenClaims represents the claims in a Google ID token
type GoogleIDTokenClaims struct {
	Iss           string `json:"iss"`
	Sub           string `json:"sub"`
	Aud           string `json:"aud"`
	Iat           int64  `json:"iat"`
	Exp           int64  `json:"exp"`
	Email         string `json:"email"`
	EmailVerified bool   `json:"email_verified"`
	Name          string `json:"name"`
	GivenName     string `json:"given_name"`
	FamilyName    string `json:"family_name"`
	Picture       string `json:"picture"`
	Locale        string `json:"locale"`
}
