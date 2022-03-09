package main

import (
	"context"
	"time"

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
			d, err := time.ParseDuration(timer)
			if err != nil {
				log.WithError(err).Fatalf("failed to parse time duration")
			}

			ticker := time.NewTicker(d)
			ctx := context.Background()

			for {
				select {
				case <-ticker.C:
					go func() {
						err := worker.RequeueEventDeliveries("Processing", timeInterval, a.eventDeliveryRepo, a.groupRepo, a.eventQueue)
						if err != nil {
							log.WithError(err).Errorf("Error requeuing status processing: %v", err)
						}
					}()
					go func() {
						err := worker.RequeueEventDeliveries("Scheduled", timeInterval, a.eventDeliveryRepo, a.groupRepo, a.eventQueue)
						if err != nil {
							log.WithError(err).Errorf("Error requeuing status Scheduled: %v", err)
						}
					}()
					go func() {
						err := worker.RequeueEventDeliveries("Retry", timeInterval, a.eventDeliveryRepo, a.groupRepo, a.eventQueue)
						if err != nil {
							log.WithError(err).Errorf("Error requeuing status Retry: %v", err)
						}
					}()
				case <-ctx.Done():
					ticker.Stop()
					return
				}
			}
		},
	}

	cmd.Flags().StringVar(&timeInterval, "time", "", "eventdelivery time interval")
	cmd.Flags().StringVar(&timer, "timer", "", "schedule timer")
	return cmd
}
