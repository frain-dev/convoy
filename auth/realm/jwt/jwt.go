package jwt

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/cache"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/datastore"
)

var (
	ErrInvalidToken = errors.New("invalid token")
	ErrTokenExpired = errors.New("expired token")
)

type Token struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

type VerifiedToken struct {
	UserID string
	Expiry int64
}

const (
	JwtDefaultExpiry        int = 1800  // seconds
	JwtDefaultRefreshExpiry int = 86400 // seconds
)

type Jwt struct {
	Secret        string
	Expiry        int
	RefreshSecret string
	RefreshExpiry int
	cache         cache.Cache
}

func NewJwt(opts *config.JwtRealmOptions, cache cache.Cache) *Jwt {
	j := &Jwt{
		Secret:        opts.Secret,
		Expiry:        opts.Expiry,
		RefreshSecret: opts.RefreshSecret,
		RefreshExpiry: opts.RefreshExpiry,
		cache:         cache,
	}

	if j.Expiry == 0 {
		j.Expiry = JwtDefaultExpiry
	}

	if j.RefreshExpiry == 0 {
		j.RefreshExpiry = JwtDefaultRefreshExpiry
	}

	return j
}

func (j *Jwt) GenerateToken(user *datastore.User) (Token, error) {
	claims := jwt.MapClaims{
		"sub": user.UID,
		"exp": time.Now().Add(time.Second * time.Duration(j.Expiry)).Unix(),
		"iat": time.Now().Unix(),
	}

	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	token := Token{}

	accessToken, err := tok.SignedString([]byte(j.Secret))
	if err != nil {
		return token, err
	}

	refreshToken, err := j.generateRefreshToken(user)
	if err != nil {
		return token, err
	}

	token.AccessToken = accessToken
	token.RefreshToken = refreshToken

	return token, nil
}

func (j *Jwt) ValidateAccessToken(accessToken string) (*VerifiedToken, error) {
	return j.validateToken(accessToken, j.Secret)
}

func (j *Jwt) ValidateRefreshToken(refreshToken string) (*VerifiedToken, error) {
	return j.validateToken(refreshToken, j.RefreshSecret)
}

// A token is considered blacklisted if the base64 encoding
// of the token exists as a key within the cache
func (j *Jwt) isTokenBlacklisted(token string) (bool, error) {
	var exists *string

	key := convoy.TokenCacheKey.Get(j.EncodeToken(token)).String()
	err := j.cache.Get(context.Background(), key, &exists)

	if err != nil {
		return false, err
	}

	if exists == nil {
		return false, nil
	}

	return true, nil
}

func (j *Jwt) BlacklistToken(verified *VerifiedToken, token string) error {
	// Calculate the remaining valid time for the token
	ttl := time.Until(time.Unix(verified.Expiry, 0))
	key := convoy.TokenCacheKey.Get(j.EncodeToken(token)).String()
	err := j.cache.Set(context.Background(), key, &verified.UserID, ttl)

	if err != nil {
		return err
	}

	return nil
}

func (j *Jwt) EncodeToken(token string) string {
	return base64.StdEncoding.EncodeToString([]byte(canonicalToken(token)))
}

// canonicalToken normalizes a JWT so alternate base64url spellings of the same
// token collapse to a single blacklist key. golang-jwt decodes the signature
// non-strictly, so a token and an equivalent spelling of its signature verify
// identically; keying the blacklist on the raw string would let a logged-out
// token survive under a re-spelled signature (see GHSA-hpqj-2j2x-p5p2). Only the
// signature segment is malleable, since the header and payload are covered by
// the signature, so we re-encode just that segment canonically. Anything that
// is not a well-formed three-segment token is returned unchanged.
func canonicalToken(token string) string {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return token
	}

	sig, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return token
	}

	parts[2] = base64.RawURLEncoding.EncodeToString(sig)
	return strings.Join(parts, ".")
}

func (j *Jwt) generateRefreshToken(user *datastore.User) (string, error) {
	claims := jwt.MapClaims{
		"sub": user.UID,
		"exp": time.Now().Add(time.Second * time.Duration(j.RefreshExpiry)).Unix(),
		"iat": time.Now().Unix(),
	}

	refreshToken := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return refreshToken.SignedString([]byte(j.RefreshSecret))
}

func (j *Jwt) validateToken(accessToken, secret string) (*VerifiedToken, error) {
	var userId string
	var expiry float64

	isBlacklisted, err := j.isTokenBlacklisted(accessToken)
	if err != nil {
		return nil, err
	}

	if isBlacklisted {
		return nil, ErrInvalidToken
	}

	token, err := jwt.Parse(accessToken, func(token *jwt.Token) (interface{}, error) {
		_, ok := token.Method.(*jwt.SigningMethodHMAC)
		if !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}

		return []byte(secret), nil
	}, jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}), jwt.WithIssuedAt())

	if err != nil {
		switch {
		case errors.Is(err, jwt.ErrTokenExpired):
			if payload, ok := token.Claims.(jwt.MapClaims); ok {
				expiry = payload["exp"].(float64)
			}
			return &VerifiedToken{Expiry: int64(expiry)}, ErrTokenExpired
		case errors.Is(err, jwt.ErrTokenMalformed):
			return nil, err
		case errors.Is(err, jwt.ErrTokenSignatureInvalid):
			return nil, err
		default:
			return nil, err
		}
	}

	payload, ok := token.Claims.(jwt.MapClaims)
	if ok && token.Valid {
		userId = payload["sub"].(string)
		expiry = payload["exp"].(float64)

		v := &VerifiedToken{UserID: userId, Expiry: int64(expiry)}
		return v, nil
	}

	return nil, err
}
