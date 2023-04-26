package api

import (
	"net/http"

	authz "github.com/Subomi/go-authz"
	"github.com/frain-dev/convoy/api"
	base "github.com/frain-dev/convoy/api/dashboard"
	"github.com/frain-dev/convoy/api/types"
	"github.com/frain-dev/convoy/database/postgres"
	"github.com/frain-dev/convoy/ee/api/dashboard"
	"github.com/frain-dev/convoy/ee/api/policies"
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
	edh := &dashboard.DashboardHandler{
		DashboardHandler: base.NewDashboardHandler(eh.opts),
		Opts:             eh.opts,
	}

	r.Method(api.POST,
		"/ui/organisations/{orgID}/invites", http.HandlerFunc(edh.InviteUserToOrganisation))
}

func (eh *EHandler) RegisterPolicy() error {
	var err error

	err = eh.opts.Authz.RegisterPolicy(func() authz.Policy {
		po := &policies.ProjectPolicy{
			BasePolicy:             authz.NewBasePolicy(),
			OrganisationMemberRepo: postgres.NewOrgMemberRepo(eh.opts.DB),
		}

		po.SetRule("manage", authz.RuleFunc(po.Manage))

		return po
	}())

	return err
}
