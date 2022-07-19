package main

import (
	"context"
	"fmt"
	"net/http"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/internal/pkg/metrics"
	"github.com/frain-dev/convoy/worker"
	"github.com/frain-dev/convoy/worker/task"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

func addWorkerCommand(a *app) *cobra.Command {
	var workerPort uint32

	cmd := &cobra.Command{
		Use:   "worker",
		Short: "Start worker instance",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Get()
			if err != nil {
				return err
			}
			ctx := context.Background()

			// register worker.
			consumer, err := worker.NewConsumer(a.queue)
			if err != nil {
				log.WithError(err).Error("failed to create worker")
			}

			handler := task.ProcessEventDelivery(a.applicationRepo, a.eventDeliveryRepo, a.groupRepo, a.limiter, a.subRepo)
			consumer.RegisterHandlers(convoy.EventProcessor, handler)

			eventCreatedhandler := task.ProcessEventCreated(a.applicationRepo, a.eventRepo, a.groupRepo, a.eventDeliveryRepo, a.cache, a.queue, a.subRepo)
			consumer.RegisterHandlers(convoy.CreateEventProcessor, eventCreatedhandler)

			notificationHandler := task.SendNotification(a.emailNotificationSender)
			consumer.RegisterHandlers(convoy.NotificationProcessor, notificationHandler)

			//register scheduler
			scheduler := worker.NewScheduler(a.queue)

			//register scheduler tasks
			scheduler.RegisterTask("@every 24h", convoy.ScheduleQueue, convoy.TaskName("retention_policies"))

			//register worker tasks
			consumer.RegisterHandlers(convoy.TaskName("retention_policies"), task.RententionPolicies(
				cfg,
				a.configRepo,
				a.groupRepo,
				a.eventRepo,
				a.eventDeliveryRepo,
				a.searcher,
			))
			//start scheduler
			scheduler.Start()

			//start worker
			log.Infof("Starting Convoy workers...")
			consumer.Start()

			metrics.RegisterQueueMetrics(a.queue, cfg)

			router := chi.NewRouter()
			router.Handle("/metrics", promhttp.HandlerFor(metrics.Reg(), promhttp.HandlerOpts{}))
			router.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
				render.JSON(w, r, "Convoy")
			})

			srv := &http.Server{
				Handler: router,
				Addr:    fmt.Sprintf(":%d", workerPort),
			}

			log.Infof("Worker running on port %v", workerPort)

			e := srv.ListenAndServe()
			if e != nil {
				return e
			}

			<-ctx.Done()
			return ctx.Err()
		},
	}

	cmd.Flags().Uint32Var(&workerPort, "worker-port", 5006, "Worker port")
	return cmd
}
