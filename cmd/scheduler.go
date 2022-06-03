package main

import (
	"github.com/frain-dev/convoy/analytics"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/worker"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

func addSchedulerCommand(a *app) *cobra.Command {
	var timeInterval string
	var timer string
	cmd := &cobra.Command{
		Use:   "scheduler",
		Short: "requeue event deliveries in the background with a scheduler.",
		Run: func(cmd *cobra.Command, args []string) {
			cfg, err := config.Get()
			if err != nil {
				log.Fatalf("Error getting config: %v", err)
			}
			if cfg.Queue.Type != config.RedisQueueProvider {
				log.WithError(err).Fatalf("Queue type error: Command is available for redis queue only.")
			}

			s := worker.NewScheduler(&a.eventQueue)

			// Register tasks.
			// s.AddTask("retry events", 30, func() {
			// 	  task.RetryEventDeliveries(nil, "", a.eventDeliveryRepo, a.groupRepo, a.eventQueue)
			// })

			s.AddTask("track analytics", 30, func() {
				analytics.TrackDailyAnalytics(
					&analytics.Repo{
						ConfigRepo: a.configRepo,
						EventRepo:  a.eventRepo,
						GroupRepo:  a.groupRepo,
						OrgRepo:    a.orgRepo,
						UserRepo:   a.userRepo,
					})
			})

			// Start Processing
			s.Start()
		},
	}

	cmd.Flags().StringVar(&timeInterval, "time", "", "eventdelivery time interval")
	cmd.Flags().StringVar(&timer, "timer", "", "schedule timer")
	return cmd
}
