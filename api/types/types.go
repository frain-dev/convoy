package types

import (
	"context"
	"time"

	"go.opentelemetry.io/otel/trace"

	authz "github.com/Subomi/go-authz"
	tracenoop "go.opentelemetry.io/otel/trace/noop"

	"github.com/frain-dev/convoy/cache"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/database"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/internal/pkg/batch_tracker"
	"github.com/frain-dev/convoy/internal/pkg/billing"
	"github.com/frain-dev/convoy/internal/pkg/dynamiceventack"
	"github.com/frain-dev/convoy/internal/pkg/fflag"
	"github.com/frain-dev/convoy/internal/pkg/license"
	"github.com/frain-dev/convoy/internal/pkg/limiter"
	"github.com/frain-dev/convoy/internal/pkg/tracer"
	"github.com/frain-dev/convoy/pkg/circuit_breaker"
	"github.com/frain-dev/convoy/pkg/logger"
	"github.com/frain-dev/convoy/queue"
)

type ResendClaimStore interface {
	TryClaim(ctx context.Context, userUID string) (ok bool, token string, err error)
	Release(ctx context.Context, userUID, token string) error
}

// Locker mirrors task.JobLocker for API-side singleton work. maxRuntime bounds
// how long the critical section may run: fn's context is cancelled once it
// elapses and the lock is held until fn returns.
type Locker interface {
	WithLock(ctx context.Context, name string, maxRuntime time.Duration, fn func(context.Context) error) error
}

type APIOptions struct {
	FFlag                      *fflag.FFlag
	FeatureFlagFetcher         fflag.FeatureFlagFetcher
	EarlyAdopterFeatureFetcher fflag.EarlyAdopterFeatureFetcher
	DB                         database.Database
	CircuitBreakerStore        circuit_breaker.CircuitBreakerStore
	Queue                      queue.Queuer
	QueueMonitor               queue.Monitor
	QueueInspector             queue.Inspector
	Logger                     logger.Logger
	Cache                      cache.Cache
	QueueSessionStore          cache.AuthoritativeCache
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
	EventRepo                  datastore.EventRepository
	// TrialEvents enforces the cloud-trial daily event cap at ingestion. It is
	// wired to the active broker at boot in NewApplicationHandler.
	TrialEvents  *license.TrialEventLimiter
	Acker        dynamiceventack.Acker
	ResendClaims ResendClaimStore
	UsageLocker  Locker
	BatchTracker batch_tracker.Tracker
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
