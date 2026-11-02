package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/auth"
	"github.com/frain-dev/convoy/util"
)

func TestRejectPortalIdempotencyValue(t *testing.T) {
	request := func(credential auth.CredentialType) *http.Request {
		r := httptest.NewRequest(http.MethodGet, "/eventdeliveries", nil)
		user := &auth.AuthenticatedUser{Credential: auth.Credential{Type: credential}}
		return r.WithContext(context.WithValue(r.Context(), convoy.AuthUserCtx, user))
	}

	h := &Handler{}

	err := h.rejectPortalIdempotencyValue(request(auth.CredentialTypeToken), "abc")
	var serviceErr *util.ServiceError
	require.ErrorAs(t, err, &serviceErr)
	require.Equal(t, http.StatusBadRequest, serviceErr.ErrCode())

	require.NoError(t, h.rejectPortalIdempotencyValue(request(auth.CredentialTypeAPIKey), "abc"))
	require.NoError(t, h.rejectPortalIdempotencyValue(request(auth.CredentialTypeToken), ""))
}
