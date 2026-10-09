package handlers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// Both index endpoints are instance admin only, and the gate has to be the first
// thing either one does.
//
// The handler here has no database, so a request that got past the gate would
// panic reaching for one. That is the assertion: the 403 proves nothing ran
// before the check. A caller without an authenticated user in context fails the
// check closed, which is the unauthenticated case.
func TestIndexEndpointsRejectANonInstanceAdminBeforeDoingAnything(t *testing.T) {
	h := &Handler{}

	report := httptest.NewRecorder()
	h.ListIndexes(report, httptest.NewRequest(http.MethodGet, "/ui/admin/indexes", nil))
	require.Equal(t, http.StatusForbidden, report.Code)

	// A well-formed body must not buy a decode, a lookup, or a run. The rebuild
	// is hours of work on a production table, so this is the ordering that
	// matters most.
	rebuild := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/ui/admin/indexes/rebuild",
		strings.NewReader(`{"index":"idx_event_deliveries_usage"}`))
	request.Header.Set("Content-Type", "application/json")
	h.StartIndexRebuild(rebuild, request)
	require.Equal(t, http.StatusForbidden, rebuild.Code)
}
