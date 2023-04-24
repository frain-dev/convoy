package api

import (
	"net/http"

	authz "github.com/Subomi/go-authz"
	"github.com/frain-dev/convoy/api"
	"github.com/frain-dev/convoy/api/types"
	"github.com/frain-dev/convoy/ee/api/dashboard"
	"github.com/frain-dev/convoy/internal/pkg/middleware"
	"github.com/go-chi/chi/v5"
)

type EHandler struct {
	*api.ApplicationHandler
	opts *types.APIOptions
}

func NewEHandler(opts *types.APIOptions) (*EHandler, error) {
	eeh := &EHandler{
		opts:               opts,
		ApplicationHandler: &api.ApplicationHandler{A: opts},
	}

	az, err := authz.NewAuthz(&authz.AuthzOpts{
		AuthCtxKey: authz.AuthCtxType(middleware.AuthUserCtx),
	})
	eeh.opts.Authz = az

	if err != nil {
		return &EHandler{}, err
	}

	return eeh, nil
}

func (eh *EHandler) BuildRoutes() http.Handler {
	// register community routes
	router := eh.ApplicationHandler.BuildRoutes()

	// apply overrides
	eh.RegisterEnterpriseDashboardHandler(router)

	return router
}

func (eh *EHandler) RegisterEnterpriseDashboardHandler(r *chi.Mux) {
	edh := &dashboard.DashboardHandler{Opts: eh.opts}

	r.Method(api.POST, "/ui/organisations/{orgID}/invites", http.HandlerFunc(edh.InviteUserToOrganisation))
}
