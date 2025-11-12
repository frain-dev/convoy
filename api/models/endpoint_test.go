package models

import (
	"testing"

	"github.com/frain-dev/convoy/datastore"
	"github.com/stretchr/testify/require"
)

func TestOAuth2_Transform_ECKey(t *testing.T) {
	oauth2 := &OAuth2{
		URL:                "https://oauth.example.com/token",
		ClientID:           "test-client-id",
		GrantType:          "client_credentials",
		Scope:              "read write",
		AuthenticationType: "client_assertion",
		SigningKey: &OAuth2SigningKey{
			Kty: "EC",
			Crv: "P-256",
			X:   "test-x-coordinate",
			Y:   "test-y-coordinate",
			D:   "test-private-key",
			Kid: "test-key-id",
		},
		SigningAlgorithm: "ES256",
		Issuer:           "test-client-id",
		Subject:          "test-client-id",
	}

	result := oauth2.Transform()

	require.NotNil(t, result)
	require.Equal(t, oauth2.URL, result.URL)
	require.Equal(t, oauth2.ClientID, result.ClientID)
	require.Equal(t, oauth2.GrantType, result.GrantType)
	require.Equal(t, oauth2.Scope, result.Scope)
	require.Equal(t, datastore.OAuth2AuthenticationType(oauth2.AuthenticationType), result.AuthenticationType)
	require.Equal(t, oauth2.SigningAlgorithm, result.SigningAlgorithm)
	require.Equal(t, oauth2.Issuer, result.Issuer)
	require.Equal(t, oauth2.Subject, result.Subject)

	require.NotNil(t, result.SigningKey)
	require.Equal(t, "EC", result.SigningKey.Kty)
	require.Equal(t, "P-256", result.SigningKey.Crv)
	require.Equal(t, "test-x-coordinate", result.SigningKey.X)
	require.Equal(t, "test-y-coordinate", result.SigningKey.Y)
	require.Equal(t, "test-private-key", result.SigningKey.D)
	require.Equal(t, "test-key-id", result.SigningKey.Kid)

	// Verify RSA fields are empty for EC keys
	require.Empty(t, result.SigningKey.N)
	require.Empty(t, result.SigningKey.E)
	require.Empty(t, result.SigningKey.P)
	require.Empty(t, result.SigningKey.Q)
	require.Empty(t, result.SigningKey.Dp)
	require.Empty(t, result.SigningKey.Dq)
	require.Empty(t, result.SigningKey.Qi)
}

func TestOAuth2_Transform_RSAKey(t *testing.T) {
	oauth2 := &OAuth2{
		URL:                "https://oauth.example.com/token",
		ClientID:           "test-client-id",
		GrantType:          "client_credentials",
		Scope:              "read write",
		AuthenticationType: "client_assertion",
		SigningKey: &OAuth2SigningKey{
			Kty: "RSA",
			N:   "test-modulus",
			E:   "AQAB",
			D:   "test-private-exponent",
			P:   "test-prime-p",
			Q:   "test-prime-q",
			Dp:  "test-dp",
			Dq:  "test-dq",
			Qi:  "test-qi",
			Kid: "test-rsa-key-id",
		},
		SigningAlgorithm: "RS256",
		Issuer:           "test-client-id",
		Subject:          "test-client-id",
	}

	result := oauth2.Transform()

	require.NotNil(t, result)
	require.Equal(t, oauth2.URL, result.URL)
	require.Equal(t, oauth2.ClientID, result.ClientID)
	require.Equal(t, oauth2.GrantType, result.GrantType)
	require.Equal(t, oauth2.Scope, result.Scope)
	require.Equal(t, datastore.OAuth2AuthenticationType(oauth2.AuthenticationType), result.AuthenticationType)
	require.Equal(t, oauth2.SigningAlgorithm, result.SigningAlgorithm)
	require.Equal(t, oauth2.Issuer, result.Issuer)
	require.Equal(t, oauth2.Subject, result.Subject)

	require.NotNil(t, result.SigningKey)
	require.Equal(t, "RSA", result.SigningKey.Kty)
	require.Equal(t, "test-modulus", result.SigningKey.N)
	require.Equal(t, "AQAB", result.SigningKey.E)
	require.Equal(t, "test-private-exponent", result.SigningKey.D)
	require.Equal(t, "test-prime-p", result.SigningKey.P)
	require.Equal(t, "test-prime-q", result.SigningKey.Q)
	require.Equal(t, "test-dp", result.SigningKey.Dp)
	require.Equal(t, "test-dq", result.SigningKey.Dq)
	require.Equal(t, "test-qi", result.SigningKey.Qi)
	require.Equal(t, "test-rsa-key-id", result.SigningKey.Kid)

	// Verify EC fields are empty for RSA keys
	require.Empty(t, result.SigningKey.Crv)
	require.Empty(t, result.SigningKey.X)
	require.Empty(t, result.SigningKey.Y)
}

func TestOAuth2_Transform_SharedSecret(t *testing.T) {
	oauth2 := &OAuth2{
		URL:                "https://oauth.example.com/token",
		ClientID:           "test-client-id",
		GrantType:          "client_credentials",
		Scope:              "read write",
		AuthenticationType: "shared_secret",
		ClientSecret:       "test-client-secret",
	}

	result := oauth2.Transform()

	require.NotNil(t, result)
	require.Equal(t, oauth2.URL, result.URL)
	require.Equal(t, oauth2.ClientID, result.ClientID)
	require.Equal(t, oauth2.GrantType, result.GrantType)
	require.Equal(t, oauth2.Scope, result.Scope)
	require.Equal(t, datastore.OAuth2AuthenticationType(oauth2.AuthenticationType), result.AuthenticationType)
	require.Equal(t, oauth2.ClientSecret, result.ClientSecret)
	require.Nil(t, result.SigningKey)
}

func TestOAuth2_Transform_FieldMapping(t *testing.T) {
	oauth2 := &OAuth2{
		URL:                "https://oauth.example.com/token",
		ClientID:           "test-client-id",
		AuthenticationType: "shared_secret",
		ClientSecret:       "test-client-secret",
		FieldMapping: &OAuth2FieldMapping{
			AccessToken: "accessToken",
			TokenType:   "tokenType",
			ExpiresIn:   "expiresIn",
		},
		ExpiryTimeUnit: "milliseconds",
	}

	result := oauth2.Transform()

	require.NotNil(t, result)
	require.NotNil(t, result.FieldMapping)
	require.Equal(t, "accessToken", result.FieldMapping.AccessToken)
	require.Equal(t, "tokenType", result.FieldMapping.TokenType)
	require.Equal(t, "expiresIn", result.FieldMapping.ExpiresIn)
	require.Equal(t, datastore.ExpiryTimeUnitMilliseconds, result.ExpiryTimeUnit)
}

func TestOAuth2_Transform_Nil(t *testing.T) {
	var oauth2 *OAuth2
	result := oauth2.Transform()
	require.Nil(t, result)
}

