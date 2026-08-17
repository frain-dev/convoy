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
