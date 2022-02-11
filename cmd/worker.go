package main

import (
	"context"
	"fmt"

	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/worker"
	"github.com/frain-dev/convoy/worker/task"
	log "github.com/sirupsen/logrus"
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
			// register tasks.
			handler := task.ProcessEventDelivery(a.applicationRepo, a.eventDeliveryRepo, a.groupRepo)
			if err := task.CreateTasks(a.groupRepo, handler); err != nil {
				log.WithError(err).Error("failed to register tasks")
				return err
			}

			errChan := worker.RegisterNewGroupTask(a.applicationRepo, a.eventDeliveryRepo, a.groupRepo)
			for err := range errChan {
				if err != nil {
					return fmt.Errorf("failed to load groups - %w", err)
				}
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
