package jwt

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"time"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/cache"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/util"
	"github.com/golang-jwt/jwt"
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
	JwtDefaultSecret        string = "convoy-jwt"
	JwtDefaultRefreshSecret string = "convoy-refresh-jwt"
	JwtDefaultExpiry        int    = 1800  //seconds
	JwtDefaultRefreshExpiry int    = 86400 //seconds
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

	if util.IsStringEmpty(j.Secret) {
		j.Secret = JwtDefaultSecret
	}

	if util.IsStringEmpty(j.RefreshSecret) {
		j.RefreshSecret = JwtDefaultRefreshSecret
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
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub": user.UID,
		"exp": time.Now().Add(time.Second * time.Duration(j.Expiry)).Unix(),
	})

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
	return base64.StdEncoding.EncodeToString([]byte(token))
}

func (j *Jwt) generateRefreshToken(user *datastore.User) (string, error) {
	refreshToken := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub": user.UID,
		"exp": time.Now().Add(time.Second * time.Duration(j.RefreshExpiry)).Unix(),
	})

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
	})

	if err != nil {
		v, ok := err.(*jwt.ValidationError)
		if ok && v.Errors == jwt.ValidationErrorExpired {
			if payload, ok := token.Claims.(jwt.MapClaims); ok {
				expiry = payload["exp"].(float64)
			}

			return &VerifiedToken{Expiry: int64(expiry)}, ErrTokenExpired
		}

		return nil, err
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
