package backup

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/internal/configuration"
	"github.com/frain-dev/convoy/internal/delivery_attempts"
	"github.com/frain-dev/convoy/internal/event_deliveries"
	"github.com/frain-dev/convoy/internal/events"
	blobstore "github.com/frain-dev/convoy/internal/pkg/blob-store"
	"github.com/frain-dev/convoy/internal/pkg/cli"
	"github.com/frain-dev/convoy/internal/pkg/exporter"
)

func AddBackupCommand(a *cli.App) *cobra.Command {
	var startFlag, endFlag string

	cmd := &cobra.Command{
		Use:   "backup",
		Short: "Run a one-time backup of events, deliveries, and delivery attempts",
		Annotations: map[string]string{
			"ShouldBootstrap": "false",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()

			cfg, err := config.Get()
			if err != nil {
				return fmt.Errorf("failed to get config: %w", err)
			}

			// Default time window: last backup interval to now
			end := time.Now()
			start := end.Add(-exporter.ParseBackupInterval(cfg.RetentionPolicy.BackupInterval))

			// Override with flags if provided
			if endFlag != "" {
				end, err = time.Parse(time.RFC3339, endFlag)
				if err != nil {
					return fmt.Errorf("invalid --end value (expected RFC3339): %w", err)
				}
			}
			if startFlag != "" {
				start, err = time.Parse(time.RFC3339, startFlag)
				if err != nil {
					return fmt.Errorf("invalid --start value (expected RFC3339): %w", err)
				}
			}

			if !start.Before(end) {
				return fmt.Errorf("--start (%s) must be before --end (%s)", start.Format(time.RFC3339), end.Format(time.RFC3339))
			}

			fmt.Fprintf(os.Stdout, "Backup window: [%s, %s)\n", start.Format(time.RFC3339), end.Format(time.RFC3339))

			// Create repos
			configRepo := configuration.New(a.Logger, a.DB)
			eventRepo := events.New(a.Logger, a.DB)
			eventDeliveryRepo := event_deliveries.New(a.Logger, a.DB)
			attemptsRepo := delivery_attempts.New(a.Logger, a.DB)

			// Load DB config for storage policy
			dbConfig, err := configRepo.LoadConfiguration(ctx)
			if err != nil {
				return fmt.Errorf("failed to load configuration: %w", err)
			}

			// Create blob store
			store, err := blobstore.NewBlobStoreClient(dbConfig.StoragePolicy, a.Logger)
			if err != nil {
				return fmt.Errorf("failed to create blob store: %w", err)
			}

			// Create exporter with explicit window
			exp, err := exporter.NewExporterWithWindow(
				eventRepo, eventDeliveryRepo, dbConfig, attemptsRepo,
				start, end, a.Logger,
			)
			if err != nil {
				return fmt.Errorf("failed to create exporter: %w", err)
			}

			// Run export
			result, err := exp.StreamExport(ctx, store)
			if err != nil {
				return fmt.Errorf("backup failed: %w", err)
			}

			// Print results
			for table, r := range result {
				fmt.Fprintf(os.Stdout, "%s: %d records exported → %s\n", table, r.NumDocs, r.ExportFile)
			}

			fmt.Fprintln(os.Stdout, "Backup complete.")
			return nil
		},
	}

	cmd.Flags().StringVar(&startFlag, "start", "", "Export window start (RFC3339, e.g. 2026-04-01T00:00:00Z)")
	cmd.Flags().StringVar(&endFlag, "end", "", "Export window end (RFC3339, e.g. 2026-04-02T00:00:00Z)")
	return cmd
}
