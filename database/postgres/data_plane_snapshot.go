package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/frain-dev/convoy/internal/pkg/dataplanestats"
)

// The snapshot is stored as one row per replica with the sections as JSON. The
// sections are named by the plane that published them, so a column per section
// would be a migration every time a plane renames a stage, and the control plane
// only ever reads the whole row.
const (
	publishDataPlaneSnapshotSQL = `
	INSERT INTO convoy.data_plane_snapshots (replica, mode, running, sampled_at, snapshot, updated_at)
	VALUES ($1, $2, $3, $4, $5, NOW())
	ON CONFLICT (replica) DO UPDATE SET
		mode = EXCLUDED.mode,
		running = EXCLUDED.running,
		sampled_at = EXCLUDED.sampled_at,
		snapshot = EXCLUDED.snapshot,
		updated_at = NOW()`

	// Expiry is by sampled_at, the publisher's own clock, which is the same value
	// staleness is measured against.
	expireDataPlaneSnapshotsSQL = `
	DELETE FROM convoy.data_plane_snapshots
	WHERE sampled_at < $1`

	// Keyed on the primary key, so this removes one named row and never a range.
	// Expiry above deletes by age and takes whatever matches; this one is a
	// replica retracting itself, and the two must not be able to be confused.
	deleteDataPlaneSnapshotSQL = `
	DELETE FROM convoy.data_plane_snapshots
	WHERE replica = $1`

	// Fetch order is by replica so the scan itself is stable. Presentation
	// order (live then stale) is applied after staleness is computed.
	loadDataPlaneSnapshotsSQL = `
	SELECT snapshot FROM convoy.data_plane_snapshots ORDER BY replica`
)

// PublishSnapshot replaces this replica's row. The replica column is the
// conflict target, so a replica can only ever overwrite itself.
func (p *Postgres) PublishSnapshot(ctx context.Context, s dataplanestats.Snapshot) error {
	if s.Replica == "" {
		return fmt.Errorf("data plane snapshot: replica is required")
	}

	payload, err := json.Marshal(s)
	if err != nil {
		return fmt.Errorf("data plane snapshot: encode: %w", err)
	}

	_, err = p.GetDB().ExecContext(ctx, publishDataPlaneSnapshotSQL,
		s.Replica, s.Mode, s.Running, s.SampledAt, payload)
	if err != nil {
		return fmt.Errorf("data plane snapshot: publish: %w", err)
	}

	return nil
}

// DeleteSnapshot removes one replica's row. It is the counterpart to
// PublishSnapshot for a plane that is stopping on purpose: the row goes when the
// replica does, so a rolling restart updates the dashboard at once instead of
// leaving a row for the whole expiry window. What is left behind afterwards is a
// plane that did not get to say goodbye, which is the case worth looking at.
//
// The replica is the primary key, so at most one row goes. An empty one is
// refused rather than run: it would match nothing today, but the guard is what
// keeps a caller that lost its identity from being one column change away from a
// delete with no subject. Expiry is the backstop for a row this did not remove;
// there is none for a row it should not have.
//
// Removing a row that is already gone is not an error. The caller is a shutdown
// path with nothing to do about it either way, and reporting it would only turn
// a repeated stop into a logged failure.
func (p *Postgres) DeleteSnapshot(ctx context.Context, replica string) error {
	if replica == "" {
		return fmt.Errorf("data plane snapshot: replica is required")
	}

	if _, err := p.GetDB().ExecContext(ctx, deleteDataPlaneSnapshotSQL, replica); err != nil {
		return fmt.Errorf("data plane snapshot: delete: %w", err)
	}

	return nil
}

func (p *Postgres) ExpireSnapshots(ctx context.Context, age time.Duration) (int64, error) {
	result, err := p.GetDB().ExecContext(ctx, expireDataPlaneSnapshotsSQL, time.Now().Add(-age))
	if err != nil {
		return 0, fmt.Errorf("data plane snapshot: expire: %w", err)
	}

	// The delete committed; a driver that cannot report the count does not undo
	// it, so report zero removed rather than an error the caller would retry.
	removed, err := result.RowsAffected()
	if err != nil {
		return 0, nil
	}

	return removed, nil
}

func (p *Postgres) DataPlaneStatus(ctx context.Context, staleAfter time.Duration) (dataplanestats.Status, error) {
	status := dataplanestats.Status{
		Replicas:          []dataplanestats.Replica{},
		StaleAfterSeconds: staleAfter.Seconds(),
	}

	// Read the primary, not the replica: staleness here is measured in seconds,
	// and replica lag longer than the publish interval would mark a replica that
	// is still publishing as stopped.
	rows, err := p.GetDB().QueryxContext(ctx, loadDataPlaneSnapshotsSQL)
	if err != nil {
		return status, fmt.Errorf("data plane snapshot: load: %w", err)
	}
	defer closeWithError(rows)

	now := time.Now()
	for rows.Next() {
		var payload []byte
		if err := rows.Scan(&payload); err != nil {
			return status, fmt.Errorf("data plane snapshot: scan: %w", err)
		}

		var snapshot dataplanestats.Snapshot
		if err := json.Unmarshal(payload, &snapshot); err != nil {
			return status, fmt.Errorf("data plane snapshot: decode: %w", err)
		}

		age := now.Sub(snapshot.SampledAt)
		status.Replicas = append(status.Replicas, dataplanestats.Replica{
			Snapshot:   snapshot,
			AgeSeconds: age.Seconds(),
			Stale:      age > staleAfter,
		})
	}

	if err := rows.Err(); err != nil {
		return status, fmt.Errorf("data plane snapshot: read: %w", err)
	}

	dataplanestats.OrderReplicas(status.Replicas)
	return status, nil
}
