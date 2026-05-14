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

func TestSanitizeURLForLog(t *testing.T) {
	tests := []struct {
		name     string
		inputURL string
		wantURL  string
	}{
		{
			name:     "strips userinfo and query parts",
			inputURL: "https://user:secret@example.com/billing/health?token=abc#frag",
			wantURL:  "https://example.com/billing/health",
		},
		{
			name:     "strips username only userinfo",
			inputURL: "https://alice@example.com/api",
			wantURL:  "https://example.com/api",
		},
		{
			name:     "keeps safe URL unchanged",
			inputURL: "https://example.com/billing",
			wantURL:  "https://example.com/billing",
		},
		{
			name:     "returns empty for empty input",
			inputURL: "   ",
			wantURL:  "",
		},
		{
			name:     "returns placeholder for invalid URL",
			inputURL: "http://[::1",
			wantURL:  invalidURLLog,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotURL := sanitizeURLForLog(tt.inputURL)
			require.Equal(t, tt.wantURL, gotURL)
		})
	}
}
