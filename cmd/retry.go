package main

import (
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/worker/task"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

func addRetryCommand(a *app) *cobra.Command {
	var status string
	var timeInterval string
	var queueName string
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
			task.RetryEventDeliveries(statuses, timeInterval, a.eventDeliveryRepo, a.groupRepo, queueName, a.queue)
		},
	}

	cmd.Flags().StringVar(&status, "status", "", "Status of event deliveries to requeue")
	cmd.Flags().StringVar(&timeInterval, "time", "", "Time interval")
	cmd.Flags().StringVar(&queueName, "queuename", "EventQueue", "queue name")
	return cmd
}
