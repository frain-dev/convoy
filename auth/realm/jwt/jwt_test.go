package jwt

import (
	"encoding/base64"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	mcache "github.com/frain-dev/convoy/cache/memory"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/datastore"
)

const base64URLAlphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-_"

// respellSignature returns an equivalent spelling of the token's signature: it
// flips the unused trailing bits of the final base64url character so the string
// differs but decodes to the exact same signature bytes (which golang-jwt
// accepts under non-strict decoding).
func respellSignature(t *testing.T, token string) string {
	parts := strings.Split(token, ".")
	require.Len(t, parts, 3)

	sig := parts[2]
	last := sig[len(sig)-1]
	v := strings.IndexByte(base64URLAlphabet, last)
	require.GreaterOrEqual(t, v, 0)

	base := v &^ 0x03
	alt := base
	if alt == v {
		alt++
	}

	newSig := sig[:len(sig)-1] + string(base64URLAlphabet[alt])
	require.NotEqual(t, sig, newSig)

	orig, err := base64.RawURLEncoding.DecodeString(sig)
	require.NoError(t, err)
	respelled, err := base64.RawURLEncoding.DecodeString(newSig)
	require.NoError(t, err)
	require.Equal(t, orig, respelled, "respelled signature must decode to identical bytes")

	parts[2] = newSig
	return strings.Join(parts, ".")
}

func provideJwt(t *testing.T) *Jwt {
	newCache := mcache.NewMemoryCache()

	jwt := NewJwt(&config.JwtRealmOptions{
		Secret:        "test-access-secret",
		RefreshSecret: "test-refresh-secret",
	}, newCache)
	return jwt
}

func TestNewJwt_DoesNotUseDefaultSecrets(t *testing.T) {
	newCache := mcache.NewMemoryCache()

	jwt := NewJwt(&config.JwtRealmOptions{}, newCache)

	require.Empty(t, jwt.Secret)
	require.Empty(t, jwt.RefreshSecret)
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

// A token blacklisted at logout must stay blacklisted even when presented under
// an alternate base64url spelling of the same signature (GHSA-hpqj-2j2x-p5p2).
func TestJwt_BlacklistToken_AlternateSpellingBypass(t *testing.T) {
	user := &datastore.User{UID: "123456"}
	jwt := provideJwt(t)

	token, err := jwt.GenerateToken(user)
	require.Nil(t, err)

	verified, err := jwt.ValidateAccessToken(token.AccessToken)
	require.Nil(t, err)

	err = jwt.BlacklistToken(verified, token.AccessToken)
	require.Nil(t, err)

	respelled := respellSignature(t, token.AccessToken)

	// The respelled token still verifies (jwt decodes signatures non-strictly)...
	require.Equal(t, canonicalToken(token.AccessToken), canonicalToken(respelled))

	// ...so it must be caught by the blacklist and rejected on validation.
	isBlacklist, err := jwt.isTokenBlacklisted(respelled)
	require.Nil(t, err)
	require.True(t, isBlacklist)

	_, err = jwt.ValidateAccessToken(respelled)
	require.ErrorIs(t, err, ErrInvalidToken)
}
