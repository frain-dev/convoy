package utils

import (
	"errors"
	"fmt"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/frain-dev/convoy/internal/pkg/cli"
	"github.com/frain-dev/convoy/internal/pkg/indexes"
	"github.com/frain-dev/convoy/internal/pkg/partitions"
)

func AddIndexesCommand(a *cli.App) *cobra.Command {
	var rebuild bool

	cmd := &cobra.Command{
		Use:   "indexes",
		Short: "Report indexes Postgres left invalid, and rebuild the ones that were dropped",
		Long: `Reports two things: indexes that are invalid right now, and indexes a
migration dropped because they were invalid and has not rebuilt.

An index build that is interrupted, by a pod restart during an upgrade for
example, leaves the index behind marked invalid. The planner ignores it from
then on, so the table performs as if the index had never been created, and no
query fails to say so. Migrations drop what they find, which is instant, and
record the definition. Server and agent rebuild those in the background after
boot, unique indexes first, one at a time.

--rebuild does the same work from a shell. On a large table this takes hours
per index and can be interrupted and run again: it resumes rather than
starting over. It is cheapest after retention has dropped the largest
partition of a converted table.`,
		Annotations: map[string]string{
			"CheckMigration":  "true",
			"ShouldBootstrap": "false",
		},
		RunE: func(cmd *cobra.Command, _ []string) error {
			if rebuild {
				return rebuildIndexes(cmd, a)
			}
			return reportIndexes(cmd, a)
		},
	}

	cmd.Flags().BoolVar(&rebuild, "rebuild", false, "Rebuild the indexes that were dropped, concurrently")

	return cmd
}

func reportIndexes(cmd *cobra.Command, a *cli.App) error {
	ctx := cmd.Context()
	db := a.DB.GetConn()

	invalid, err := indexes.ListInvalid(ctx, db)
	if err != nil {
		return err
	}

	dropped, err := indexes.ListDropped(ctx, db)
	if err != nil {
		return err
	}

	out := cmd.OutOrStdout()
	if len(invalid) == 0 && len(dropped) == 0 {
		fmt.Fprintln(out, "No invalid indexes, and none waiting to be rebuilt.")
		return nil
	}

	if len(invalid) > 0 {
		fmt.Fprintf(out, "%d invalid index(es). The planner is ignoring these:\n\n", len(invalid))
		w := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "TABLE\tINDEX\tSTATE")
		for _, i := range invalid {
			// A build in progress is marked invalid until it finishes, so it is
			// reported and left alone rather than treated as a failure.
			state := "abandoned by a failed build"
			if i.Busy {
				state = "being built now, leave it"
			}
			fmt.Fprintf(w, "%s\t%s\t%s\n", i.Table, i.Name, state)
		}
		if err := w.Flush(); err != nil {
			return err
		}
		fmt.Fprintln(out)
	}

	if len(dropped) > 0 {
		fmt.Fprintf(out, "%d index(es) dropped and not rebuilt. Rebuild them with --rebuild:\n\n", len(dropped))
		w := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "TABLE\tINDEX\tDROPPED\tCOSTS\tBLOCKED")
		unique := 0
		for _, d := range dropped {
			// A missing index costs speed. A missing unique index also costs the
			// uniqueness it enforced, which is worth saying out loud.
			costs := "slower queries"
			if d.Unique() {
				costs = "slower queries, and its key is no longer unique"
				unique++
			}
			blocked := ""
			if d.Blocked() {
				blocked = d.BlockedReason
			}
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", d.Table, d.Name, d.DroppedAt.Format("2006-01-02 15:04 MST"), costs, blocked)
		}
		if err := w.Flush(); err != nil {
			return err
		}
		if unique > 0 {
			fmt.Fprintf(out, "\nNothing stops a duplicate row on %d of these until it is rebuilt, so --rebuild starts there.\n", unique)
		}
	}

	return nil
}

// rebuildIndexes builds the dropped indexes through the same service the
// dashboard uses.
//
// Going through it is what puts a rebuild started from a shell under the
// instance-wide single-active lock, for the reason a CLI conversion goes through
// it: the index being rebuilt lives on a table a conversion may be rewriting, and
// the two would contend for locks on the same relation. The slot is taken and
// released per index, so a conversion waiting on the instance gets its turn
// between them rather than after all of them.
func rebuildIndexes(cmd *cobra.Command, a *cli.App) error {
	ctx := cmd.Context()
	db := a.DB.GetConn()
	out := cmd.OutOrStdout()

	dropped, err := indexes.ListDropped(ctx, db)
	if err != nil {
		return err
	}
	if len(dropped) == 0 {
		fmt.Fprintln(out, "Nothing to rebuild.")
		return nil
	}

	service := partitions.New(a.DB, a.Logger)
	for _, d := range dropped {
		fmt.Fprintf(out, "Building %s on convoy.%s. This holds no lock against traffic but can take hours.\n", d.Name, d.Table)

		err = service.RunIndexRebuild(ctx, d.Name, triggeredByCLI)
		if err == nil {
			fmt.Fprintf(out, "  done: %s\n", d.Name)
			continue
		}

		// The list was read before the loop, so another operator's rebuild can
		// have cleared this one since. Nothing is owed on it, which is the
		// outcome asked for, not a failure.
		if errors.Is(err, indexes.ErrNotDropped) {
			fmt.Fprintf(out, "  already rebuilt: %s\n", d.Name)
			continue
		}

		// Something else holds the instance, so the next index would fail the
		// same way. Stop rather than reporting a failure per index.
		if errors.Is(err, partitions.ErrRunInProgress) {
			return fmt.Errorf("%w. If it was left behind by a killed process, close it with "+
				"UPDATE convoy.partition_runs SET status = 'failed', completed_at = NOW() WHERE status = 'running'", err)
		}

		// One failure does not stop the rest: the indexes are independent, and a
		// table that cannot take one now should not keep the others missing.
		fmt.Fprintf(out, "  failed: %v\n", err)
	}

	remaining, err := indexes.ListDropped(ctx, db)
	if err != nil {
		return err
	}
	if len(remaining) > 0 {
		return fmt.Errorf("%d of %d index(es) were not rebuilt, see the failures above", len(remaining), len(dropped))
	}

	fmt.Fprintln(out, "All dropped indexes rebuilt.")
	return nil
}
