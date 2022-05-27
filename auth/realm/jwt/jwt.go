package jwt

import (
	"encoding/base64"
	"errors"
	"fmt"
	"time"

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
	JwtDefaultExpiry        int    = 2
	JwtDefaultRefreshExpiry int    = 86400
)

type Jwt struct {
	Secret        string
	Expiry        int
	RefreshSecret string
	RefreshExpiry int
}

func NewJwt(opts *config.JwtRealmOptions) *Jwt {

	j := &Jwt{
		Secret:        opts.Secret,
		Expiry:        opts.Expiry,
		RefreshSecret: opts.RefreshSecret,
		RefreshExpiry: opts.RefreshExpiry,
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
	token, err := jwt.Parse(accessToken, func(token *jwt.Token) (interface{}, error) {
		_, ok := token.Method.(*jwt.SigningMethodHMAC)
		if !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}

		return []byte(secret), nil
	})

	payload, ok := token.Claims.(jwt.MapClaims)

	if ok && token.Valid {
		userId = payload["sub"].(string)
		expiry = payload["exp"].(float64)

		v := &VerifiedToken{UserID: userId, Expiry: int64(expiry)}
		return v, nil
	}

	v, _ := err.(*jwt.ValidationError)
	if ok && v.Errors == jwt.ValidationErrorExpired {
		expiry = payload["exp"].(float64)

		return &VerifiedToken{Expiry: int64(expiry)}, ErrTokenExpired
	}

	return nil, err
}
