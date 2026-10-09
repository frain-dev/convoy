package utils

import (
	"context"
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/frain-dev/convoy/internal/pkg/cli"
	"github.com/frain-dev/convoy/internal/pkg/partitions"
)

// triggeredByCLI marks runs started from a shell, to tell them apart from the
// dashboard's, which record the user who asked.
const triggeredByCLI = "cli"

func AddPartitionCommand(a *cli.App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "partition",
		Short: "partition tables",
		Long:  "partition tables that are deleted by convoy during retention, valid tables are events, event_deliveries, delivery_attempts and events_search",
		Args:  cobra.MaximumNArgs(1),
		Annotations: map[string]string{
			"CheckMigration":  "true",
			"ShouldBootstrap": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return convert(cmd.Context(), a, args, partitions.OperationPartition)
		},
	}

	return cmd
}

func AddUnPartitionCommand(a *cli.App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "unpartition",
		Short: "unpartitions tables",
		Long:  "unpartition tables that are deleted by convoy during retention, valid tables are events, event_deliveries, delivery_attempts and events_search",
		Args:  cobra.MaximumNArgs(1),
		Annotations: map[string]string{
			"CheckMigration":  "true",
			"ShouldBootstrap": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return convert(cmd.Context(), a, args, partitions.OperationUnpartition)
		},
	}

	return cmd
}

// convert runs the conversion through the same service the dashboard uses.
//
// Going through it, rather than calling the repositories directly, is what puts
// a CLI conversion under the instance-wide single-active lock: two conversions
// rewriting tables at once, one from a shell and one from the dashboard, would
// otherwise hold two sets of locks and saturate the same disk. It also records
// the run, so a conversion started here shows its progress and its outcome in
// the same history.
func convert(ctx context.Context, a *cli.App, args []string, op partitions.Operation) error {
	if !a.Licenser.RetentionPolicy() {
		return fmt.Errorf("partitioning is only available with a license key")
	}

	tables := partitions.Tables()
	if len(args) > 0 {
		table, err := partitions.ParseTable(args[0])
		if err != nil {
			return err
		}
		tables = []partitions.Table{table}
	}

	service := partitions.New(a.DB, a.Logger)
	for _, table := range tables {
		if err := service.Run(ctx, table, op, triggeredByCLI); err != nil {
			if errors.Is(err, partitions.ErrRunInProgress) {
				return fmt.Errorf("%w. If it was left behind by a killed process, close it with "+
					"UPDATE convoy.partition_runs SET status = 'failed', completed_at = NOW() WHERE status = 'running'", err)
			}
			return err
		}
	}

	return nil
}
