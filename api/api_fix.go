package api

import (
	"net/http"

	"github.com/frain-dev/convoy/api/handlers"
	"github.com/frain-dev/convoy/internal/pkg/middleware"
	"github.com/go-chi/chi/v5"
)

// GetPortalAPIMiddleware returns the middleware stack for portal API routes
func GetPortalAPIMiddleware(handler *handlers.Handler) []func(http.Handler) http.Handler {
	return []func(http.Handler) http.Handler{
		middleware.JsonResponse,
		middleware.SetupCORS,
		middleware.RequireValidPortalLinksLicense(handler.A.Licenser),
		middleware.RequireAuth(),
		middleware.PortalLinkOwnerIDMiddleware,
	}
}

// UpdatePortalAPIRoutes is deprecated and should not be used
// It was causing a panic by trying to register the same route multiple times
func UpdatePortalAPIRoutes(router *chi.Mux, handler *handlers.Handler) {
	// This function is intentionally left empty to avoid the panic
	// Use GetPortalAPIMiddleware instead
}
