package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/frain-dev/convoy/config"
)

func TestShouldAuthRoute_LicenseFeatures(t *testing.T) {
	t.Parallel()

	publicReq := httptest.NewRequest(http.MethodGet, "/ui/license/features", nil)
	require.False(t, shouldAuthRoute(publicReq, config.Configuration{}))

	orgReq := httptest.NewRequest(http.MethodGet, "/ui/license/features?orgID=org-123", nil)
	require.False(t, shouldAuthRoute(orgReq, config.Configuration{}))
	require.True(t, shouldAuthRoute(orgReq, config.Configuration{LicenseKey: "lk_test"}))
	require.True(t, shouldAuthRoute(orgReq, config.Configuration{Billing: config.BillingConfiguration{APIKey: "sk_test"}}))

	headerReq := httptest.NewRequest(http.MethodGet, "/ui/license/features", nil)
	headerReq.Header.Set("X-Organisation-Id", "org-123")
	require.False(t, shouldAuthRoute(headerReq, config.Configuration{}))
	require.True(t, shouldAuthRoute(headerReq, config.Configuration{LicenseKey: "lk_test"}))
}
