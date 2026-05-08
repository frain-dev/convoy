package types

import (
	"github.com/redis/go-redis/v9"
	"go.opentelemetry.io/otel/trace"

	authz "github.com/Subomi/go-authz"
	tracenoop "go.opentelemetry.io/otel/trace/noop"

	"github.com/frain-dev/convoy/cache"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/database"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/internal/pkg/billing"
	"github.com/frain-dev/convoy/internal/pkg/fflag"
	"github.com/frain-dev/convoy/internal/pkg/license"
	"github.com/frain-dev/convoy/internal/pkg/limiter"
	"github.com/frain-dev/convoy/internal/pkg/tracer"
	"github.com/frain-dev/convoy/pkg/logger"
	"github.com/frain-dev/convoy/queue"
)

type APIOptions struct {
	FFlag                      *fflag.FFlag
	FeatureFlagFetcher         fflag.FeatureFlagFetcher
	EarlyAdopterFeatureFetcher fflag.EarlyAdopterFeatureFetcher
	DB                         database.Database
	Redis                      redis.UniversalClient
	Queue                      queue.Queuer
	Logger                     logger.Logger
	Cache                      cache.Cache
	Authz                      *authz.Authz
	Rate                       limiter.RateLimiter
	Licenser                   license.Licenser
	Cfg                        config.Configuration
	BillingClient              billing.Client
	TracerBackend              tracer.Backend
	ConfigRepo                 datastore.ConfigurationRepository
	OrgRepo                    datastore.OrganisationRepository
	OrgMemberRepo              datastore.OrganisationMemberRepository
	ProjectRepo                datastore.ProjectRepository
}

// TracerProvider returns the trace.TracerProvider used to mint span tracers.
// Always non-nil; falls back to a no-op provider when the backend is absent
// (e.g. during early bootstrap or in tests).
func (a *APIOptions) TracerProvider() trace.TracerProvider {
	if a == nil || a.TracerBackend == nil {
		return tracenoop.NewTracerProvider()
	}
	return a.TracerBackend.TracerProvider()
}
