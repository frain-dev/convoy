package dataplane

import (
	"github.com/frain-dev/convoy/cache"
	"github.com/frain-dev/convoy/database"
	"github.com/frain-dev/convoy/internal/pkg/broker"
	"github.com/frain-dev/convoy/internal/pkg/license"
	"github.com/frain-dev/convoy/internal/pkg/limiter"
	"github.com/frain-dev/convoy/internal/pkg/tracer"
	"github.com/frain-dev/convoy/pkg/logger"
	"github.com/frain-dev/convoy/queue"
)

type RuntimeOpts struct {
	DB            database.Database
	Queue         queue.Queuer
	Logger        logger.Logger
	Cache         cache.Cache
	Rate          limiter.RateLimiter
	Licenser      license.Licenser
	TracerBackend tracer.Backend
	Broker        *broker.Dependencies

	// Optional test hooks used by E2E setup.
	JobTracker            interface{}
	SetSubscriptionLoader func(interface{})
	SetSubscriptionTable  func(interface{})
}

func (d RuntimeOpts) setSubscriptionState(loader, table interface{}) {
	if d.SetSubscriptionLoader != nil {
		d.SetSubscriptionLoader(loader)
	}
	if d.SetSubscriptionTable != nil {
		d.SetSubscriptionTable(table)
	}
}
