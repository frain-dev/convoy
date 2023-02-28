package main

import (
	"context"
	"fmt"
	"net/http"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/analytics"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/database/postgres"
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
			consumer := worker.NewConsumer(a.queue, lo)

			endpointRepo := postgres.NewEndpointRepo(a.db)
			eventRepo := postgres.NewEventRepo(a.db)
			eventDeliveryRepo := postgres.NewEventDeliveryRepo(a.db)
			projectRepo := postgres.NewProjectRepo(a.db)
			subRepo := postgres.NewSubscriptionRepo(a.db)
			deviceRepo := postgres.NewDeviceRepo(a.db)
			configRepo := postgres.NewConfigRepo(a.db)

			consumer.RegisterHandlers(convoy.EventProcessor, task.ProcessEventDelivery(
				endpointRepo,
				eventDeliveryRepo,
				projectRepo,
				a.limiter,
				subRepo,
				a.queue))

			consumer.RegisterHandlers(convoy.CreateEventProcessor, task.ProcessEventCreation(
				endpointRepo,
				eventRepo,
				projectRepo,
				eventDeliveryRepo,
				a.cache,
				a.queue,
				subRepo,
				a.searcher,
				deviceRepo))

			consumer.RegisterHandlers(convoy.RetentionPolicies, task.RetentionPolicies(
				configRepo,
				projectRepo,
				eventRepo,
				eventDeliveryRepo,
				postgres.NewExportRepo(a.db),
				a.searcher,
			))

			consumer.RegisterHandlers(convoy.MonitorTwitterSources, task.MonitorTwitterSources(
				a.db,
				a.queue))

			consumer.RegisterHandlers(convoy.ExpireSecretsProcessor, task.ExpireSecret(
				endpointRepo))

			consumer.RegisterHandlers(convoy.DailyAnalytics, analytics.TrackDailyAnalytics(a.db, cfg))
			consumer.RegisterHandlers(convoy.EmailProcessor, task.ProcessEmails(sc))
			consumer.RegisterHandlers(convoy.IndexDocument, task.SearchIndex(a.searcher))
			consumer.RegisterHandlers(convoy.NotificationProcessor, task.ProcessNotifications(sc))

			// start worker
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
