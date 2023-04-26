package types

import (
	authz "github.com/Subomi/go-authz"
	"github.com/frain-dev/convoy/api/policies"
	"github.com/frain-dev/convoy/cache"
	"github.com/frain-dev/convoy/database"
	"github.com/frain-dev/convoy/database/postgres"
	"github.com/frain-dev/convoy/internal/pkg/searcher"
	"github.com/frain-dev/convoy/limiter"
	"github.com/frain-dev/convoy/pkg/log"
	"github.com/frain-dev/convoy/queue"
	"github.com/frain-dev/convoy/tracer"
)

type APIOptions struct {
	DB       database.Database
	Queue    queue.Queuer
	Logger   log.StdLogger
	Tracer   tracer.Tracer
	Cache    cache.Cache
	Limiter  limiter.RateLimiter
	Searcher searcher.Searcher
	Authz    *authz.Authz
}

func (a *APIOptions) RegisterPolicy() error {
	var err error

	err = a.Authz.RegisterPolicy(func() authz.Policy {
		po := &policies.OrganisationPolicy{
			BasePolicy:             authz.NewBasePolicy(),
			OrganisationMemberRepo: postgres.NewOrgMemberRepo(a.DB),
		}

		po.SetRule("get", authz.RuleFunc(po.Get))
		po.SetRule("update", authz.RuleFunc(po.Update))
		po.SetRule("delete", authz.RuleFunc(po.Delete))

		return po
	}())

	if err != nil {
		return err
	}

	err = a.Authz.RegisterPolicy(func() authz.Policy {
		po := &policies.ProjectPolicy{
			BasePolicy:             authz.NewBasePolicy(),
			OrganisationRepo:       postgres.NewOrgRepo(a.DB),
			OrganisationMemberRepo: postgres.NewOrgMemberRepo(a.DB),
		}

		po.SetRule("manage", authz.RuleFunc(po.Manage))

		return po
	}())

	return err
}
