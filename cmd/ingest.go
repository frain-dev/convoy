package main

import (
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/database/postgres"
	"github.com/frain-dev/convoy/internal/pkg/pubsub"
	"github.com/frain-dev/convoy/pkg/log"
	"github.com/spf13/cobra"
)

func addIngestCommand(a *app) *cobra.Command {
	var interval int
	cmd := &cobra.Command{
		Use:   "ingest",
		Short: "Ingest webhook events from Pub/Sub streams",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Get()
			if err != nil {
				a.logger.Errorf("Failed to retrieve config: %v", err)
				return err
			}

			sourceRepo := postgres.NewSourceRepo(a.db)
			endpointRepo := postgres.NewEndpointRepo(a.db)

			lo := a.logger.(*log.Logger)
			lo.SetPrefix("ingester")

			lvl, err := log.ParseLevel(cfg.Logger.Level)
			if err != nil {
				return err
			}

			lo.SetLevel(lvl)

			sourcePool := pubsub.NewSourcePool(lo)
			sourceLoader := pubsub.NewSourceLoader(endpointRepo, sourceRepo, a.queue, sourcePool, lo)

			sourceLoader.Run(interval)

			return nil
		},
	}

	cmd.Flags().IntVar(&interval, "interval", 300, "the time interval, measured in seconds, at which the database should be polled for new pub sub sources")
	return cmd
}
