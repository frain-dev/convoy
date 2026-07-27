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

func TestValidateLicenseSendsUsageWhenLoaderPresent(t *testing.T) {
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
		UsageLoader: staticUsageLoader{&UsageSnapshot{
			EndpointCount: 12,
			EventCount:    100,
			ProjectCount:  3,
			OrgCount:      1,
			UserCount:     2,
			AsOf:          "2026-07-13T00:00:00Z",
		}},
	})

	_, err := c.ValidateLicense(context.Background(), "some-key")
	require.NoError(t, err)

	usage, ok := got["usage"].(map[string]any)
	require.True(t, ok)
	require.EqualValues(t, 12, usage["endpoint_count"])
	require.EqualValues(t, 100, usage["event_count"])
	require.EqualValues(t, 3, usage["project_count"])
	require.EqualValues(t, 1, usage["org_count"])
	require.EqualValues(t, 2, usage["user_count"])
}

func TestValidateLicenseOmitsUsageWhenLoaderMissing(t *testing.T) {
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
	})

	_, err := c.ValidateLicense(context.Background(), "some-key")
	require.NoError(t, err)
	_, hasUsage := got["usage"]
	require.False(t, hasUsage)
}

type staticUsageLoader struct {
	snap *UsageSnapshot
}

func (s staticUsageLoader) LoadCached(ctx context.Context) (*UsageSnapshot, error) {
	return s.snap, nil
}
