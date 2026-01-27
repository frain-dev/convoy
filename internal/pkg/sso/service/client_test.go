package service

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewClient(t *testing.T) {
	client := NewClient(Config{})

	assert.NotNil(t, client)
	assert.Equal(t, DefaultOverwatchHost, client.host)
	assert.Equal(t, DefaultRedirectPath, client.redirectPath)
	assert.Equal(t, DefaultTokenPath, client.tokenPath)
	assert.Equal(t, DefaultTimeout, client.timeout)
	assert.Equal(t, DefaultRetryCount, client.retryCount)
	assert.NotNil(t, client.httpClient)
}

func TestClient_GetRedirectURL_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/sso/redirect", r.URL.Path)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		assert.Equal(t, "test-license-key", r.Header.Get("X-License-Key"))

		var req RedirectURLRequest
		err := json.NewDecoder(r.Body).Decode(&req)
		require.NoError(t, err)
		assert.Equal(t, "https://convoy.example.com/sso/callback", req.CallbackURL)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(RedirectURLResponse{
			Status:  true,
			Message: "Success",
			Data:    RedirectURLData{RedirectURL: "https://workos.com/authorize?client_id=test"},
		})
	}))
	defer server.Close()

	client := NewClient(Config{
		Host:         server.URL,
		RedirectPath: "/sso/redirect",
		TokenPath:    "/sso/token",
		Timeout:      5 * time.Second,
		RetryCount:   1,
	})

	resp, err := client.GetRedirectURL(context.Background(), "test-license-key", "https://convoy.example.com", "https://convoy.example.com/sso/callback")
	require.NoError(t, err)
	assert.True(t, resp.Status)
	assert.Equal(t, "https://workos.com/authorize?client_id=test", resp.Data.RedirectURL)
}

func TestClient_GetRedirectURL_MissingLicenseKey(t *testing.T) {
	client := NewClient(Config{})

	resp, err := client.GetRedirectURL(context.Background(), "", "https://convoy.example.com", "https://convoy.example.com/sso/callback")
	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "license key is required")
}

func TestClient_GetRedirectURL_MissingRedirectURI(t *testing.T) {
	client := NewClient(Config{})

	resp, err := client.GetRedirectURL(context.Background(), "test-license-key", "https://convoy.example.com", "")
	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "redirect URI is required")
}

func TestClient_GetRedirectURL_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	client := NewClient(Config{
		Host:         server.URL,
		RedirectPath: "/sso/redirect",
		TokenPath:    "/sso/token",
		Timeout:      5 * time.Second,
		RetryCount:   1,
	})

	resp, err := client.GetRedirectURL(context.Background(), "test-license-key", "https://convoy.example.com", "https://convoy.example.com/sso/callback")
	assert.Error(t, err)
	assert.Nil(t, resp)
}

func TestClient_GetRedirectURL_InvalidResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(RedirectURLResponse{
			Status:  false,
			Message: "SSO not included in license",
		})
	}))
	defer server.Close()

	client := NewClient(Config{
		Host:         server.URL,
		RedirectPath: "/sso/redirect",
		TokenPath:    "/sso/token",
		Timeout:      5 * time.Second,
		RetryCount:   1,
	})

	resp, err := client.GetRedirectURL(context.Background(), "test-license-key", "https://convoy.example.com", "https://convoy.example.com/sso/callback")
	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "SSO redirect failed")
}

func TestClient_ValidateToken_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/sso/token", r.URL.Path)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		var req TokenValidationRequest
		err := json.NewDecoder(r.Body).Decode(&req)
		require.NoError(t, err)
		assert.Equal(t, "test-token-123", req.Token)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(TokenValidationResponse{
			Status:  true,
			Message: "Success",
			Data: TokenValidationData{
				Payload: UserProfile{
					Email:                  "user@example.com",
					FirstName:              "John",
					LastName:               "Doe",
					OrganizationID:         "org-123",
					OrganizationExternalID: "acme-corp",
					ID:                     "user-123",
				},
			},
		})
	}))
	defer server.Close()

	client := NewClient(Config{
		Host:         server.URL,
		RedirectPath: "/sso/redirect",
		TokenPath:    "/sso/token",
		Timeout:      5 * time.Second,
		RetryCount:   1,
	})

	resp, err := client.ValidateToken(context.Background(), "test-token-123")
	require.NoError(t, err)
	assert.True(t, resp.Status)
	assert.Equal(t, "user@example.com", resp.Data.Payload.Email)
	assert.Equal(t, "John", resp.Data.Payload.FirstName)
	assert.Equal(t, "Doe", resp.Data.Payload.LastName)
	assert.Equal(t, "acme-corp", resp.Data.Payload.OrganizationExternalID)
}

func TestClient_ValidateToken_MissingToken(t *testing.T) {
	client := NewClient(Config{})

	resp, err := client.ValidateToken(context.Background(), "")
	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "token is required")
}

func TestClient_ValidateToken_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	client := NewClient(Config{
		Host:         server.URL,
		RedirectPath: "/sso/redirect",
		TokenPath:    "/sso/token",
		Timeout:      5 * time.Second,
		RetryCount:   1,
	})

	resp, err := client.ValidateToken(context.Background(), "test-token")
	assert.Error(t, err)
	assert.Nil(t, resp)
}

func TestClient_ValidateToken_InvalidResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(TokenValidationResponse{
			Status:  false,
			Message: "Token not found or expired",
		})
	}))
	defer server.Close()

	client := NewClient(Config{
		Host:         server.URL,
		RedirectPath: "/sso/redirect",
		TokenPath:    "/sso/token",
		Timeout:      5 * time.Second,
		RetryCount:   1,
	})

	resp, err := client.ValidateToken(context.Background(), "test-token")
	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "SSO token validation failed")
}

func TestClient_ValidateToken_MissingEmail(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(TokenValidationResponse{
			Status:  true,
			Message: "Success",
			Data:    TokenValidationData{Payload: UserProfile{}},
		})
	}))
	defer server.Close()

	client := NewClient(Config{
		Host:         server.URL,
		RedirectPath: "/sso/redirect",
		TokenPath:    "/sso/token",
		Timeout:      5 * time.Second,
		RetryCount:   1,
	})

	resp, err := client.ValidateToken(context.Background(), "test-token")
	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "email is missing from profile")
}
