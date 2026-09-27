package broker

import "github.com/frain-dev/convoy/api/types"

// ApplyToAPIOptions copies every broker-owned dependency the HTTP layer uses
// into o. Worker-only fields (Scheduler, ConsumerBackend, TaskErrors) stay on
// Dependencies. Call after filling non-broker fields on o (DB, Licenser, Cfg, …).
func (d *Dependencies) ApplyToAPIOptions(o *types.APIOptions) {
	if d == nil || o == nil {
		return
	}
	o.Queue = d.Queue
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
