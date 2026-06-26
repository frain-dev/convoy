package service

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValidateLicenseSendsVersionAndDeploymentID(t *testing.T) {
	var got map[string]any

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		require.NoError(t, json.Unmarshal(body, &got))
		_, _ = w.Write([]byte(`{"status":true,"data":{"valid":true,"status":"active","entitlements":[]}}`))
	}))
	defer srv.Close()

	c := NewClient(Config{
		Host:         srv.URL,
		ValidatePath: "/validate",
		Version:      "9.9.9",
		DeploymentID: "depl_test",
	})

	_, err := c.ValidateLicense(context.Background(), "some-key")
	require.NoError(t, err)

	require.Equal(t, "some-key", got["license_key"])
	require.Equal(t, "9.9.9", got["version"])
	require.Equal(t, "depl_test", got["deployment_id"])
}

func TestValidateLicenseOmitsEmptyDeploymentID(t *testing.T) {
	var got map[string]any

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		require.NoError(t, json.Unmarshal(body, &got))
		_, _ = w.Write([]byte(`{"status":true,"data":{"valid":true,"status":"active","entitlements":[]}}`))
	}))
	defer srv.Close()

	// No DeploymentID set: it must be omitted from the payload. Version defaults
	// to convoy.GetVersion() and is always present.
	c := NewClient(Config{
		Host:         srv.URL,
		ValidatePath: "/validate",
	})

	_, err := c.ValidateLicense(context.Background(), "some-key")
	require.NoError(t, err)

	_, hasDeployment := got["deployment_id"]
	require.False(t, hasDeployment)
	require.NotEmpty(t, got["version"])
}
