package api

import (
	"net/http"

	authz "github.com/Subomi/go-authz"
	"github.com/frain-dev/convoy/api"
	"github.com/frain-dev/convoy/api/types"
	"github.com/frain-dev/convoy/database/postgres"
	"github.com/frain-dev/convoy/ee/api/policies"
	"github.com/frain-dev/convoy/internal/pkg/middleware"
)

type EHandler struct {
	*api.ApplicationHandler
	opts *types.APIOptions
}

func NewEHandler(opts *types.APIOptions) (*EHandler, error) {
	az, err := authz.NewAuthz(&authz.AuthzOpts{
		AuthCtxKey: authz.AuthCtxType(middleware.AuthUserCtx),
	})

	if err != nil {
		return &EHandler{}, err
	}

	opts.Authz = az
	eeh := &EHandler{
		opts:               opts,
		ApplicationHandler: &api.ApplicationHandler{A: opts},
	}

	return eeh, nil
}

func (eh *EHandler) BuildRoutes() http.Handler {
	// register community routes
	router := eh.ApplicationHandler.BuildControlPlaneRoutes()

	// apply overrides

	return router
}

func (eh *EHandler) RegisterPolicy() error {
	var err error

	err = eh.opts.Authz.RegisterPolicy(func() authz.Policy {
		po := &policies.ProjectPolicy{
			BasePolicy:             authz.NewBasePolicy(),
			OrganisationRepo:       postgres.NewOrgRepo(eh.opts.DB, eh.opts.Cache),
			OrganisationMemberRepo: postgres.NewOrgMemberRepo(eh.opts.DB, eh.opts.Cache),
		}

		po.SetRule("manage", authz.RuleFunc(po.Manage))

		return po
	}())

	if err != nil {
		return err
	}

	err = eh.opts.Authz.RegisterPolicy(func() authz.Policy {
		po := &policies.OrganisationPolicy{
			BasePolicy:             authz.NewBasePolicy(),
			OrganisationMemberRepo: postgres.NewOrgMemberRepo(eh.opts.DB, eh.opts.Cache),
		}

		po.SetRule("manage", authz.RuleFunc(po.Manage))

		return po
	}())

	return err
}
