package retention

import (
	"context"
	"errors"
	"fmt"
	"time"
)

const (
	// orphanBatchRows is how many orphaned rows one sweep reads at a time. The
	// delete that follows removes every row belonging to the events those rows
	// named, so a batch clears at least this many rows off a backlogged table,
	// and more wherever the limit fell inside a fan-out.
	orphanBatchRows = 1000

	// orphanSweepBudget is how long one run spends on the sweep. The asynq job
	// holds its lock for 30 minutes and cancels the context at that point, and
	// partition maintenance has already run by the time this starts, so the
	// sweep takes a small share and leaves the rest to the work that reclaims
	// whole partitions.
	orphanSweepBudget = 5 * time.Minute
)

// sweepOrphanedEventEndpoints removes convoy.events_endpoints rows whose event
// retention has already dropped.
//
// Nothing else can reclaim them. The table holds event_id and endpoint_id and
// nothing else, so it has no key to partition on and no timestamp to delete by,
// and it carries no foreign key either: sql/1724932863.sql rebuilt it with
// LIKE ... INCLUDING CONSTRAINTS, which copies CHECK and NOT NULL but not
// references. Adding one back is not open to us either, because Postgres
// refuses an inbound key onto a partitioned table that omits the partition
// columns. Reading the parent is what is left.
//
// Failure policy: best effort inside a fixed budget. Every statement is bounded
// and nothing records that the sweep finished, so a run that is cut short
// cannot be mistaken for a complete one. A backlog is cleared by repetition,
// one budget per night, and a failed batch ends the sweep and is logged rather
// than failing the retention job, which has already done the work it exists
// for.
func (r *PartitionRetentionPolicy) sweepOrphanedEventEndpoints(ctx context.Context) {
	ctx, cancel := context.WithTimeout(ctx, orphanSweepBudget)
	defer cancel()

	settled, err := r.eventTablesSettled(ctx)
	if err != nil {
		r.reportSweepStopped(0, err)
		return
	}
	if !settled {
		r.logger.Info("skipped the convoy.events_endpoints sweep: a conversion has not finished draining")
		return
	}

	var removed int64
	for {
		deleted, err := r.deleteOrphanedEventEndpoints(ctx)
		removed += deleted
		if err != nil {
			r.reportSweepStopped(removed, err)
			return
		}

		// A batch that removed nothing ends the sweep. Either the table is
		// clean, or the read and the delete disagreed about an event because
		// the schema moved between them, and neither leaves progress to make.
		if deleted == 0 {
			break
		}
	}

	if removed > 0 {
		r.logger.Info(fmt.Sprintf("removed %d orphaned convoy.events_endpoints rows", removed))
	}
}

// reportSweepStopped separates the clock from a fault. Spending the budget, or
// having the retention job's lock cancel everything under it, is the expected
// end of a run with a backlog: the rows already removed are committed and
// tomorrow's run carries on from there.
func (r *PartitionRetentionPolicy) reportSweepStopped(removed int64, err error) {
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		r.logger.Info(fmt.Sprintf(
			"convoy.events_endpoints sweep stopped on its budget after removing %d rows", removed))
		return
	}
	r.logger.Error(fmt.Sprintf(
		"failed to sweep orphaned convoy.events_endpoints rows after removing %d", removed), "error", err)
}

// deleteOrphanedEventEndpoints resolves one batch of expired events and removes
// the endpoint rows pointing at them, reporting how many rows went.
//
// The delete names events rather than rows so it is driven by
// idx_events_endpoints_event_id_key and takes each event's fan-out whole. Half
// a fan-out is a row set no reader wants and the next batch would have to find
// again.
func (r *PartitionRetentionPolicy) deleteOrphanedEventEndpoints(ctx context.Context) (int64, error) {
	ids, err := r.expiredEvents(ctx)
	if err != nil {
		return 0, err
	}
	if len(ids) == 0 {
		return 0, nil
	}

	// The read above ran under its own snapshot, so the delete proves the
	// events are still gone rather than trusting a list that a conversion could
	// have republished in between. It costs one index probe per event in the
	// batch, and it is what keeps the destructive statement from depending on
	// a read it did not take.
	tag, err := r.db.GetConn().Exec(ctx, `
        DELETE FROM convoy.events_endpoints ee
        WHERE ee.event_id = ANY($1)
          AND NOT EXISTS (SELECT 1 FROM convoy.events e WHERE e.id = ee.event_id)
          AND NOT EXISTS (SELECT 1 FROM convoy.events_search s WHERE s.id = ee.event_id)`, ids)
	if err != nil {
		return 0, fmt.Errorf("removing orphaned convoy.events_endpoints rows: %w", err)
	}
	return tag.RowsAffected(), nil
}

// expiredEvents names the events that convoy.events_endpoints still points at
// and neither event table holds any more.
//
// The limit counts rows and the duplicates a fan-out puts in them are folded
// here rather than by DISTINCT, which plans as a HashAggregate underneath the
// limit. That node has to read the whole table before it returns its first row,
// which would make every batch of a long cleanup pay for a full scan of the
// table the cleanup is shrinking. Limiting rows keeps the scan streaming and it
// stops as soon as the batch is full.
func (r *PartitionRetentionPolicy) expiredEvents(ctx context.Context) ([]string, error) {
	rows, err := r.db.GetConn().Query(ctx, `
        SELECT ee.event_id
        FROM convoy.events_endpoints ee
        WHERE NOT EXISTS (SELECT 1 FROM convoy.events e WHERE e.id = ee.event_id)
          AND NOT EXISTS (SELECT 1 FROM convoy.events_search s WHERE s.id = ee.event_id)
        LIMIT $1`, orphanBatchRows)
	if err != nil {
		return nil, fmt.Errorf("reading expired events from convoy.events_endpoints: %w", err)
	}
	defer rows.Close()

	var ids []string
	seen := make(map[string]struct{}, orphanBatchRows)
	for rows.Next() {
		var id string
		if err = rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("reading expired events from convoy.events_endpoints: %w", err)
		}
		if _, repeated := seen[id]; repeated {
			continue
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("reading expired events from convoy.events_endpoints: %w", err)
	}
	return ids, nil
}

// eventTablesSettled reports that neither event table is mid-conversion.
//
// Detaching renames the partitioned parent to <table>_partitioned and only
// copies the rows back under the live name on a later statement, so for the
// length of that drain convoy.events does not hold every event. A sweep reading
// it then would find live rows whose event it could not see and delete them.
// The same leftover name is what the stand-in foreign key trigger looks in, so
// its absence is the signal this codebase already uses for "the swap is done".
//
// Fail closed: a read that fails is not a verdict that the tables are settled.
func (r *PartitionRetentionPolicy) eventTablesSettled(ctx context.Context) (bool, error) {
	var settled bool
	err := r.db.GetConn().QueryRow(ctx, `
        SELECT to_regclass('convoy.events_partitioned') IS NULL
           AND to_regclass('convoy.events_search_partitioned') IS NULL`).Scan(&settled)
	if err != nil {
		return false, fmt.Errorf("checking for an unfinished conversion: %w", err)
	}
	return settled, nil
}
