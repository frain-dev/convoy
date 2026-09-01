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
	Gauges []Metric `json:"gauges,omitempty"`

	// Outstanding are the durable backlogs, which is the number an operator
	// compares against a queue depth. Each carries its own known flag because
	// these are the counts most likely to be unavailable.
	Outstanding []Backlog `json:"outstanding,omitempty"`
}

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

// Metric is one named number.
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
