package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestShouldAuthRoute_LicenseFeatures(t *testing.T) {
	t.Parallel()

	publicReq := httptest.NewRequest(http.MethodGet, "/ui/license/features", nil)
	require.False(t, shouldAuthRoute(publicReq))

	orgReq := httptest.NewRequest(http.MethodGet, "/ui/license/features?orgID=org-123", nil)
	require.True(t, shouldAuthRoute(orgReq))

	headerReq := httptest.NewRequest(http.MethodGet, "/ui/license/features", nil)
	headerReq.Header.Set("X-Organisation-Id", "org-123")
	require.True(t, shouldAuthRoute(headerReq))
}
