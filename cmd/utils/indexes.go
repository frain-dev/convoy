package utils

import (
	"fmt"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/frain-dev/convoy/internal/pkg/cli"
	"github.com/frain-dev/convoy/internal/pkg/indexes"
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
record the definition so the build can happen here instead of stalling a boot.

--rebuild builds those indexes again, concurrently, one at a time. On a large
table this takes hours per index and can be interrupted and run again: it
resumes rather than starting over. It is cheapest after retention has dropped
the largest partition of a converted table.`,
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
		fmt.Fprintln(w, "TABLE\tINDEX\tDROPPED\tCOSTS")
		unique := 0
		for _, d := range dropped {
			// A missing index costs speed. A missing unique index also costs the
			// uniqueness it enforced, which is worth saying out loud.
			costs := "slower queries"
			if d.Unique() {
				costs = "slower queries, and its key is no longer unique"
				unique++
			}
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", d.Table, d.Name, d.DroppedAt.Format("2006-01-02 15:04 MST"), costs)
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

	for _, d := range dropped {
		fmt.Fprintf(out, "Building %s on convoy.%s. This holds no lock against traffic but can take hours.\n", d.Name, d.Table)

		// One failure does not stop the rest: the indexes are independent, and a
		// table that cannot take one now should not keep the others missing.
		if err = indexes.Rebuild(ctx, db, d); err != nil {
			fmt.Fprintf(out, "  failed: %v\n", err)
			continue
		}
		fmt.Fprintf(out, "  done: %s\n", d.Name)
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
