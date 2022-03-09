package main

import (
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/worker"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

func addRetryCommand(a *app) *cobra.Command {
	var status string
	var timeInterval string

	cmd := &cobra.Command{
		Use:   "retry",
		Short: "retry event deliveries with a particular status in a timeframe",
		Run: func(cmd *cobra.Command, args []string) {
			cfg, err := config.Get()
			if err != nil {
				log.Fatalf("Error getting config: %v", err)
			}
			if cfg.Queue.Type != config.RedisQueueProvider {
				log.Fatalf("Queue type error: Command is available for redis queue only.")
			}

			err = worker.RequeueEventDeliveries(status, timeInterval, a.eventDeliveryRepo, a.groupRepo, a.eventQueue)
			if err != nil {
				log.Errorf("Error requeue event deliveries: %v", err)
			}
		},
	}

	cmd.Flags().StringVar(&status, "status", "", "Status of event deliveries to requeue")
	cmd.Flags().StringVar(&timeInterval, "time", "", " time interval")
	return cmd
}
