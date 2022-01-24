package main

import (
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/worker"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

func addNodeCommand(a *app) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "node",
		Short: "Start a server/worker node",
	}

	cmd.AddCommand(nodeServerCommand(a))
	cmd.AddCommand(nodeWorkerCommand(a))

	return cmd
}

func nodeServerCommand(a *app) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "server",
		Short: "Create a server node",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Get()
			if err != nil {
				return err
			}
			err = StartConvoyServer(a, cfg, false)
			if err != nil {
				log.Printf("Error starting convoy server: %v", err)
			}
			return nil
		},
	}
	return cmd
}

func nodeWorkerCommand(a *app) *cobra.Command {

	cmd := &cobra.Command{
		Use:   "worker",
		Short: "Create a worker node",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Get()
			if err != nil {
				return err
			}
			// register workers.
			producer := worker.NewProducer(a.eventQueue)

			cleaner := worker.NewCleaner(a.deadLetterQueue)

			if cfg.Queue.Type != config.InMemoryQueueProvider {
				producer.Start()
				cleaner.Start()
			}
			return nil
		},
	}
	return cmd
}
