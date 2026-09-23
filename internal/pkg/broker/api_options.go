package broker

import (
	"github.com/frain-dev/convoy/api/types"
	"github.com/frain-dev/convoy/queue/inventory"
)

// ApplyToAPIOptions copies every broker-owned dependency the HTTP layer uses
// into o. Worker-only fields (Scheduler, ConsumerBackend, TaskErrors) stay on
// Dependencies. Call after filling non-broker fields on o (DB, Licenser, Cfg, …).
func (d *Dependencies) ApplyToAPIOptions(o *types.APIOptions) {
	if d == nil || o == nil {
		return
	}
	o.Queue = d.Queue
	o.QueueDrain = d.Drain
	if d.Source != nil {
		o.PreviousQueueDrain = d.Source.Drain
	}
	if d.Drain != nil {
		stores := []inventory.Registration{{ID: d.Drain.Target.StoreID, Provider: d.Drain.Target.Provider, Role: "active", Reader: d.Drain.Participants[0].Reader}}
		if d.Source != nil {
			stores = append(stores, inventory.Registration{ID: d.Source.Drain.Target.StoreID, Provider: d.Source.Drain.Target.Provider, Role: "previous", Reader: d.Source.Drain.Reader})
		}
		o.QueueInventory, _ = inventory.New(stores)
	}
	o.QueueAdmission = d.Admission
	o.QueueMonitor = d.QueueMonitor
	o.QueueInspector = d.QueueInspector
	o.Cache = d.Cache
	o.QueueSessionStore = d.Cache
	o.Rate = d.RateLimiter
	o.CircuitBreakerStore = d.CircuitBreakerStore
	o.TrialEvents = d.TrialEvents
	o.Acker = d.Acker
	o.ResendClaims = d.ResendClaims
	o.UsageLocker = d.JobLocker
	o.BatchTracker = d.BatchTracker
}
