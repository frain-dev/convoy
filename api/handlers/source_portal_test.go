package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/frain-dev/convoy/api/types"
)

// TestSourceHandlers_RejectPortalLinkToken verifies portal credentials cannot
// list, fetch, create, update, or delete project-wide sources (including
// verifier secrets). The guard runs before any project lookup.
func TestSourceHandlers_RejectPortalLinkToken(t *testing.T) {
	handler := &Handler{A: &types.APIOptions{}}

	cases := []struct {
		name    string
		handler http.HandlerFunc
		method  string
	}{
		{"create", handler.CreateSource, http.MethodPost},
		{"get", handler.GetSource, http.MethodGet},
		{"update", handler.UpdateSource, http.MethodPut},
		{"delete", handler.DeleteSource, http.MethodDelete},
		{"list", handler.LoadSourcesPaged, http.MethodGet},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := withPortalTokenAuth(httptest.NewRequest(tc.method, "/sources", nil))
			w := httptest.NewRecorder()

			tc.handler(w, req)

			require.Equal(t, http.StatusUnauthorized, w.Code)
		})
	}
}
