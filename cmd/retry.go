package main

import (
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/pkg/log"
	"github.com/frain-dev/convoy/worker/task"

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
				log.WithError(err).Fatalf("Queue type error: Command is available for redis queue only.")
			}

			statuses := []datastore.EventDeliveryStatus{datastore.EventDeliveryStatus(status)}
			task.RetryEventDeliveries(statuses, timeInterval, a.store, a.queue)
		},
	}

	cmd.Flags().StringVar(&status, "status", "", "Status of event deliveries to requeue")
	cmd.Flags().StringVar(&timeInterval, "time", "", "Time interval")
	return cmd
}
