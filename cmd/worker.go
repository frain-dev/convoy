package main

import (
	"context"

	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/worker"
	"github.com/spf13/cobra"
)

func addWorkerCommand(a *app) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "worker",
		Short: "Start worker instance",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Get()
			if err != nil {
				return err
			}
			// register workers.
			ctx := context.Background()
			producer := worker.NewProducer(a.eventQueue)
			if cfg.Queue.Type != config.InMemoryQueueProvider {
				producer.Start(ctx)
			}

			<-ctx.Done()
			return ctx.Err()
		},
	}
	return cmd
}
