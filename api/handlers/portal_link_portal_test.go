package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/frain-dev/convoy/api/types"
)

// TestPortalLinkHandlers_RejectPortalLinkToken verifies portal credentials
// cannot list, fetch, create, update, or revoke project-wide portal links.
// The guard runs before any project lookup, so no datastore is required.
func TestPortalLinkHandlers_RejectPortalLinkToken(t *testing.T) {
	handler := &Handler{A: &types.APIOptions{}}

	cases := []struct {
		name    string
		handler http.HandlerFunc
		method  string
	}{
		{"create", handler.CreatePortalLink, http.MethodPost},
		{"get", handler.GetPortalLink, http.MethodGet},
		{"update", handler.UpdatePortalLink, http.MethodPut},
		{"revoke", handler.RevokePortalLink, http.MethodPut},
		{"list", handler.LoadPortalLinksPaged, http.MethodGet},
		{"refresh", handler.RefreshPortalLinkAuthToken, http.MethodGet},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := withPortalTokenAuth(httptest.NewRequest(tc.method, "/portal-links", nil))
			w := httptest.NewRecorder()

			tc.handler(w, req)

			require.Equal(t, http.StatusUnauthorized, w.Code)
		})
	}
}
