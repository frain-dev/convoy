package main

import (
	"context"
	"fmt"
	"net/http"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/analytics"
	"github.com/frain-dev/convoy/config"
	cm "github.com/frain-dev/convoy/datastore/mongo"
	"github.com/frain-dev/convoy/internal/pkg/metrics"
	"github.com/frain-dev/convoy/internal/pkg/smtp"
	"github.com/frain-dev/convoy/pkg/log"
	"github.com/frain-dev/convoy/worker"
	"github.com/frain-dev/convoy/worker/task"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/spf13/cobra"
)

func addWorkerCommand(a *app) *cobra.Command {
	var workerPort uint32
	var logLevel string

	cmd := &cobra.Command{
		Use:   "worker",
		Short: "Start worker instance",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Get()
			if err != nil {
				a.logger.Errorf("Failed to retrieve config: %v", err)
				return err
			}

			lo := a.logger.(*log.Logger)
			lo.SetPrefix("worker")

			lvl, err := log.ParseLevel(cfg.Logger.Level)
			if err != nil {
				return err
			}
			lo.SetLevel(lvl)

			sc, err := smtp.NewClient(&cfg.SMTP)
			if err != nil {
				a.logger.WithError(err).Error("Failed to create smtp client")
				return err
			}

			ctx := context.Background()

			// register worker.
			consumer, err := worker.NewConsumer(a.queue, lo)
			if err != nil {
				a.logger.WithError(err).Error("failed to create worker")
			}

			appRepo := cm.NewApplicationRepo(a.store)
			eventRepo := cm.NewEventRepository(a.store)
			eventDeliveryRepo := cm.NewEventDeliveryRepository(a.store)
			groupRepo := cm.NewGroupRepo(a.store)
			subRepo := cm.NewSubscriptionRepo(a.store)
			deviceRepo := cm.NewDeviceRepository(a.store)
			configRepo := cm.NewConfigRepo(a.store)

			consumer.RegisterHandlers(convoy.EventProcessor, task.ProcessEventDelivery(
				appRepo,
				eventDeliveryRepo,
				groupRepo,
				a.limiter,
				subRepo,
				a.queue))

			consumer.RegisterHandlers(convoy.CreateEventProcessor, task.ProcessEventCreation(
				appRepo,
				eventRepo,
				groupRepo,
				eventDeliveryRepo,
				a.cache,
				a.queue,
				subRepo,
				a.searcher,
				deviceRepo))

			consumer.RegisterHandlers(convoy.RetentionPolicies, task.RententionPolicies(
				cfg,
				configRepo,
				groupRepo,
				eventRepo,
				eventDeliveryRepo,
				a.searcher))

			consumer.RegisterHandlers(convoy.MonitorTwitterSources, task.MonitorTwitterSources(
				a.store,
				a.queue))

			consumer.RegisterHandlers(convoy.ExpireSecretsProcessor, task.ExpireSecret(
				appRepo))

			consumer.RegisterHandlers(convoy.DailyAnalytics, analytics.TrackDailyAnalytics(a.store, cfg))
			consumer.RegisterHandlers(convoy.EmailProcessor, task.ProcessEmails(sc))
			consumer.RegisterHandlers(convoy.IndexDocument, task.SearchIndex(a.searcher))
			consumer.RegisterHandlers(convoy.NotificationProcessor, task.ProcessNotifications(sc))

			//start worker
			lo.Infof("Starting Convoy workers...")
			consumer.Start()

			metrics.RegisterQueueMetrics(a.queue)

			router := chi.NewRouter()
			router.Handle("/metrics", promhttp.HandlerFor(metrics.Reg(), promhttp.HandlerOpts{}))
			router.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
				render.JSON(w, r, "Convoy")
			})

			srv := &http.Server{
				Handler: router,
				Addr:    fmt.Sprintf(":%d", workerPort),
			}

			a.logger.Infof("Worker running on port %v", workerPort)

			e := srv.ListenAndServe()
			if e != nil {
				return e
			}

			<-ctx.Done()
			return ctx.Err()
		},
	}

	cmd.Flags().Uint32Var(&workerPort, "worker-port", 5006, "Worker port")
	cmd.Flags().StringVar(&logLevel, "log-level", "error", "scheduler log level")

	return cmd
}
