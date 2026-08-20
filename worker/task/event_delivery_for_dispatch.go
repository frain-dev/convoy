package task

import (
	"context"

	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/util"
)

// eventDeliveryForDispatch returns the delivery row ProcessEventDelivery should use.
//
// Failure policy: only at-least-once deliveries may trust the enqueue snapshot.
// A zero task retry count is not proof the delivery was never dispatched: lease
// reclaim and release return a job to pending without incrementing retry_count,
// so a worker that died mid-dispatch is redelivered with count 0. At-most-once
// must read live status, so the Processing guard in ProcessEventDelivery can
// refuse the second send; at-least-once tolerates that duplicate by contract.
// An absent or unrecognised mode is unknown, not at-least-once, and reloads too.
// Retries and missing or stale snapshots reload so status and metadata reflect
// partial dispatch work.
func eventDeliveryForDispatch(
	ctx context.Context,
	repo datastore.EventDeliveryRepository,
	data EventDelivery,
	taskRetryCount int,
) (*datastore.EventDelivery, error) {
	if taskRetryCount == 0 && data.Snapshot != nil &&
		data.Snapshot.DeliveryMode == datastore.AtLeastOnceDeliveryMode &&
		data.Snapshot.UID == data.EventDeliveryID &&
		data.Snapshot.ProjectID == data.ProjectID &&
		!util.IsStringEmpty(data.Snapshot.UID) {
		return data.Snapshot, nil
	}
	return repo.FindEventDeliveryByIDSlim(ctx, data.ProjectID, data.EventDeliveryID)
}

// processingBlocksSend reports whether finding a delivery already marked
// Processing must abandon this dispatch.
//
// Every delivery writes the Processing marker before dispatch, so a worker that
// dies mid-dispatch leaves the row in Processing and the queue hands the job
// back once its lease expires.
//
// Failure policy: at-most-once refuses. It cannot tell "died before the request
// went out" from "died after the endpoint received it", and it must never send
// twice, so it leaves the delivery stranded in Processing until an operator
// retries it. That stranding is the cost of the guarantee. At-least-once
// resends: its contract permits a duplicate but not a silent loss, and losing
// the delivery is exactly what refusing would do here. An absent or
// unrecognised mode is unknown, not at-least-once, and refuses.
//
// The resend window is the queue's lease timeout, so a live-but-slow endpoint
// whose lease expires mid-dispatch can be sent twice. That is inherent to
// lease-based reclaim rather than to this check, and lease timeout is the knob.
func processingBlocksSend(mode datastore.DeliveryMode) bool {
	return mode != datastore.AtLeastOnceDeliveryMode
}
