// Package dataplanestats is the shape a data plane reports itself in, and the
// contracts for publishing and reading it.
//
// Nothing here names a component of any particular data plane. A plane reports
// named stages, named writers, named totals and named durable backlogs; the
// store keeps whatever it published and the dashboard renders it. That is what
// lets a plane change its own vocabulary without a migration, a handler change
// or a UI change.
package dataplanestats

import (
	"context"
	"time"
)

// Snapshot is one replica's report on the plane it is running. Producers fill
// only the sections they have: an absent section is absent, and a consumer must
// render it as unknown rather than as zero. That distinction is the whole point
// of this contract, because a backlog reported as an empty queue is the failure
// it exists to prevent.
type Snapshot struct {
	// Replica identifies the process that published this. It is the primary key
	// of the stored snapshot, so a replica overwrites its own row and never
	// another's.
	Replica string `json:"replica"`

	// Mode is what the publisher calls the plane it is running.
	Mode string `json:"mode"`

	// Running is false when the process is up but the plane is not accepting.
	// A false here means every gauge below is meaningless, not zero.
	Running bool `json:"running"`

	// SampledAt is when the numbers were read, not when they were stored. It is
	// what staleness is measured against.
	SampledAt time.Time `json:"sampled_at"`

	// Stages are the admission points work passes through, in flow order.
	Stages []Stage `json:"stages,omitempty"`

	// Writers are the batching writers holding work in front of a store.
	Writers []Writer `json:"writers,omitempty"`

	// Counters are monotonic totals for the life of the publishing process, so a
	// consumer may rate them. They do not reset when the plane restarts.
	Counters []Metric `json:"counters,omitempty"`

	// Gauges are point-in-time values that are not stage or writer depth.
	//
	// This list and Counters above are read name by name, so absence applies to a
	// name and not only to the list holding it. A present list is not a complete
	// one: a publisher omits a name it has nothing to say about, so a consumer
	// that defaults a missing name to zero reports a measurement the plane never
	// made. The list being here says only that the plane published something,
	// never that it published this.
	//
	// Where several names are one compound value, the publisher omits or sends
	// them together, and a consumer missing any one of them must treat the whole
	// value as unknown rather than fill the gap. A group every plane shares is
	// named in this package and documents itself there; for a name belonging to
	// one plane's own vocabulary the grouping cannot be told from here, and that
	// publisher documents it.
	//
	// A name that is present with a value of 0 is the opposite of an absent one.
	// It is a real reading of a plane that did none of that work, which is an
	// answer an operator needs, so it renders as the number rather than as
	// unknown.
	Gauges []Metric `json:"gauges,omitempty"`

	// Outstanding are the durable backlogs, which is the number an operator
	// compares against a queue depth. Each carries its own known flag because
	// these are the counts most likely to be unavailable.
	Outstanding []Backlog `json:"outstanding,omitempty"`

	// Throughput has no section of its own. A plane that measures an interval
	// publishes it through Gauges, under names carrying the concept rather than
	// its own vocabulary, which is what lets a queue-based plane publish the same
	// fact. A plane with no previous reading to difference omits those gauges
	// entirely: absence is the only encoding of "not measured", so there is no
	// flag beside it that can disagree, and a consumer must render the absence as
	// unknown rather than as a plane that moved nothing.
	//
	// The monotonic totals are Counters, which is what they already were.
}

// The gauge names a plane publishes its measured interval under. Named for the
// concept rather than for any plane's own components, so a queue-based plane can
// publish the same four and be read by the same consumer.
//
// A publisher emits all four or none. A consumer that finds only some of them
// has no rate: a count without the window it covers is not a rate, and a window
// without its counts is not a measurement.
const (
	GaugeThroughputWindowMS  = "throughput_window_ms"
	GaugeThroughputAccepted  = "throughput_accepted"
	GaugeThroughputDelivered = "throughput_delivered"
	GaugeThroughputFailed    = "throughput_failed"
)

// Stage is one admission point. Queued is work already admitted; Waiting is
// callers the stage is currently blocking, which is the saturation signal that
// tells backpressure apart from an idle plane.
type Stage struct {
	Name    string `json:"name"`
	Queued  int    `json:"queued"`
	Waiting int    `json:"waiting"`

	// Workers is 0 when the stage runs one goroutine per item rather than a
	// fixed pool, which is a mode and not an absence.
	Workers int `json:"workers"`

	// Partitions is how many independent lanes the queued work is spread over,
	// Capacity is what one lane holds before its senders block, and Deepest is
	// the fullest single lane. Queued alone cannot say whether one tenant is
	// blocking, which is what these three answer.
	Partitions int `json:"partitions"`
	Capacity   int `json:"partition_capacity"`
	Deepest    int `json:"deepest_partition"`
}

// Writer is one batching writer. Failures counts batches whose write returned an
// error, each of which answered every row in it as failed.
type Writer struct {
	Name     string `json:"name"`
	Pending  int    `json:"pending"`
	Failures int64  `json:"failures"`
}

// Metric is one named number. The number carries no unit, so the name has to:
// a publisher measuring a duration or an interval says so in the name
// (`..._ms`), because a consumer cannot recover a unit it was never told and a
// number whose unit is only knowable by reading the producer is how a reader
// comes to divide milliseconds by seconds.
type Metric struct {
	Name  string `json:"name"`
	Value int64  `json:"value"`
}

// Backlog is durable work the plane has not finished. Known is false when the
// count could not be read, which a consumer must show as unknown: a backlog
// rendered as 0 because the query timed out reads as an empty queue.
type Backlog struct {
	Name  string    `json:"name"`
	Count int64     `json:"count"`
	Known bool      `json:"known"`
	AsOf  time.Time `json:"as_of"`
}

// Replica is a stored snapshot as the control plane serves it. Age and
// staleness are computed once by the server so every client agrees on when a
// publisher stopped reporting.
type Replica struct {
	Snapshot

	// AgeSeconds is how long ago the snapshot was sampled.
	AgeSeconds float64 `json:"age_seconds"`

	// Stale marks a replica whose last sample is older than the threshold. A
	// stale replica's numbers describe a moment that has passed, so a consumer
	// must not present them as current.
	Stale bool `json:"stale"`
}

// Status is every replica the store holds, with the threshold the server
// applied, so a client does not have to guess it.
type Status struct {
	Replicas          []Replica `json:"replicas"`
	StaleAfterSeconds float64   `json:"stale_after_seconds"`
}

// Reporter is a plane running in this process that can describe itself. Nil
// unless the process runs one, which is what makes an absent report mean "this
// process has no such plane" rather than "the plane is empty".
type Reporter interface {
	Snapshot(ctx context.Context) (Snapshot, error)
}

// Store keeps the snapshots replicas publish. The publishing process and the
// process serving the dashboard are not the same one, which is why this crosses
// the database rather than staying in memory.
//
// Deleting a named row is deliberately not here. api/types hands this interface
// to the dashboard's read path, so a method on it is a method that surface
// holds: putting the delete here is what would let a read one day remove the
// evidence of a replica that stopped. A publisher that retracts its own row asks
// its store for that capability directly, and a store without it is not broken,
// because expiry removes the row anyway.
type Store interface {
	// PublishSnapshot replaces this replica's row.
	PublishSnapshot(ctx context.Context, s Snapshot) error

	// ExpireSnapshots removes rows sampled longer than age ago, so a replica
	// that was scaled away stops claiming depth it no longer holds.
	ExpireSnapshots(ctx context.Context, age time.Duration) (int64, error)

	// DataPlaneStatus reads every stored snapshot, marking as stale any sampled
	// longer than staleAfter ago.
	DataPlaneStatus(ctx context.Context, staleAfter time.Duration) (Status, error)
}

const (
	// DefaultInterval applies when the configured sample time is unset.
	DefaultInterval = 5 * time.Second

	// ExpireAfter is when a replica's row is deleted rather than shown stale. It
	// is long enough that a rolling restart does not drop a replica that is
	// coming back, and short enough that a scaled-down one leaves the page.
	ExpireAfter = 15 * time.Minute
)

// PublishInterval turns the configured metrics sample time into a duration.
// There is one interval owner: publishing reuses the sample time rather than
// adding a knob of its own, so both halves cannot be configured apart.
func PublishInterval(seconds uint64) time.Duration {
	if seconds == 0 {
		return DefaultInterval
	}

	return time.Duration(seconds) * time.Second
}

// StoreFrom returns db as a snapshot store when it can be one, and nil when it
// cannot. It takes any rather than a database interface so this package stays a
// leaf that both a data plane and the API can import.
func StoreFrom(db any) Store {
	if db == nil {
		return nil
	}

	store, ok := db.(Store)
	if !ok {
		return nil
	}

	return store
}

// StaleAfter is how long a snapshot stays current. It is a multiple of the
// publish interval rather than the interval itself, so one skipped sample does
// not blank the page.
func StaleAfter(interval time.Duration) time.Duration {
	if interval <= 0 {
		interval = DefaultInterval
	}

	return 3 * interval
}
