package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/api/types"
	"github.com/frain-dev/convoy/auth"
)

func withPortalTokenAuth(req *http.Request) *http.Request {
	authUser := &auth.AuthenticatedUser{Credential: auth.Credential{Type: auth.CredentialTypeToken}}
	return req.WithContext(context.WithValue(req.Context(), convoy.AuthUserCtx, authUser))
}

// TestEventTypeMutations_RejectPortalLinkToken verifies portal-link credentials
// cannot create, update, or deprecate project-wide event types. The guard runs
// before any project lookup, so no datastore is required.
func TestEventTypeMutations_RejectPortalLinkToken(t *testing.T) {
	handler := &Handler{A: &types.APIOptions{}}

	cases := []struct {
		name    string
		handler http.HandlerFunc
		method  string
	}{
		{"create", handler.CreateEventType, http.MethodPost},
		{"update", handler.UpdateEventType, http.MethodPut},
		{"deprecate", handler.DeprecateEventType, http.MethodPost},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := withPortalTokenAuth(httptest.NewRequest(tc.method, "/event-types", nil))
			w := httptest.NewRecorder()

			tc.handler(w, req)

			require.Equal(t, http.StatusUnauthorized, w.Code)
		})
	}
}

// TestEventTypeMutations_AllowNonPortalCallers confirms the new portal guard does
// not block JWT, PAT, or project-API-key callers (they fall through to normal
// project authorization).
func TestEventTypeMutations_AllowNonPortalCallers(t *testing.T) {
	handler := &Handler{A: &types.APIOptions{}}

	for _, credType := range []auth.CredentialType{auth.CredentialTypeJWT, auth.CredentialTypeAPIKey} {
		t.Run(string(credType), func(t *testing.T) {
			authUser := &auth.AuthenticatedUser{Credential: auth.Credential{Type: credType}}
			req := httptest.NewRequest(http.MethodPost, "/event-types", nil)
			req = req.WithContext(context.WithValue(req.Context(), convoy.AuthUserCtx, authUser))
			w := httptest.NewRecorder()

			require.False(t, handler.rejectPortalLinkToken(w, req))
		})
	}
}
