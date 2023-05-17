package jwt

import (
	"testing"

	"github.com/frain-dev/convoy/cache"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/datastore"
	"github.com/stretchr/testify/require"
)

func provideJwt(t *testing.T) *Jwt {
	newCache, err := cache.NewCache(config.DefaultConfiguration.Redis)

	require.Nil(t, err)

	jwt := NewJwt(&config.JwtRealmOptions{}, newCache)
	return jwt
}

func TestJwt_GenerateToken(t *testing.T) {
	user := &datastore.User{UID: "123456"}
	jwt := provideJwt(t)

	token, err := jwt.GenerateToken(user)
	require.Nil(t, err)

	require.NotEmpty(t, token.AccessToken)
	require.NotEmpty(t, token.RefreshToken)
}

func TestJwt_ValidateToken(t *testing.T) {
	user := &datastore.User{UID: "123456"}
	jwt := provideJwt(t)

	token, err := jwt.GenerateToken(user)
	require.Nil(t, err)

	require.NotEmpty(t, token.AccessToken)
	require.NotEmpty(t, token.RefreshToken)

	verified, err := jwt.ValidateAccessToken(token.AccessToken)
	require.Nil(t, err)

	require.Equal(t, user.UID, verified.UserID)
}

func TestJwt_ValidateRefreshToken(t *testing.T) {
	user := &datastore.User{UID: "123456"}
	jwt := provideJwt(t)

	token, err := jwt.GenerateToken(user)
	require.Nil(t, err)

	require.NotEmpty(t, token.AccessToken)
	require.NotEmpty(t, token.RefreshToken)

	verified, err := jwt.ValidateRefreshToken(token.RefreshToken)
	require.Nil(t, err)

	require.Equal(t, user.UID, verified.UserID)
}

func TestJwt_BlacklistToken(t *testing.T) {
	user := &datastore.User{UID: "123456"}
	jwt := provideJwt(t)

	token, err := jwt.GenerateToken(user)
	require.Nil(t, err)

	verified, err := jwt.ValidateAccessToken(token.AccessToken)
	require.Nil(t, err)

	err = jwt.BlacklistToken(verified, token.AccessToken)
	require.Nil(t, err)

	isBlacklist, err := jwt.isTokenBlacklisted(token.AccessToken)
	require.Nil(t, err)
	require.True(t, isBlacklist)
}
