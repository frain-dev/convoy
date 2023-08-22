package worker

import (
	"context"
	"fmt"
	"github.com/frain-dev/convoy/internal/pkg/rdb"
	"net/http"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/analytics"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/database/postgres"
	"github.com/frain-dev/convoy/internal/pkg/cli"
	"github.com/frain-dev/convoy/internal/pkg/metrics"
	"github.com/frain-dev/convoy/internal/pkg/searcher"
	"github.com/frain-dev/convoy/internal/pkg/smtp"
	"github.com/frain-dev/convoy/pkg/log"
	"github.com/frain-dev/convoy/util"
	"github.com/frain-dev/convoy/worker"
	"github.com/frain-dev/convoy/worker/task"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/spf13/cobra"
)

func AddWorkerCommand(a *cli.App) *cobra.Command {
	var workerPort uint32
	var logLevel string

	var newRelicApp string
	var newRelicKey string
	var newRelicTracerEnabled bool
	var newRelicConfigEnabled bool

	cmd := &cobra.Command{
		Use:   "worker",
		Short: "Start worker instance",
		Annotations: map[string]string{
			"ShouldBootstrap": "false",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			// override config with cli Flags
			cliConfig, err := buildWorkerCliConfiguration(cmd)
			if err != nil {
				return err
			}

			if err = config.Override(cliConfig); err != nil {
				return err
			}

			cfg, err := config.Get()
			if err != nil {
				a.Logger.Errorf("Failed to retrieve config: %v", err)
				return err
			}

			lo := a.Logger.(*log.Logger)
			lo.SetPrefix("worker")

			lvl, err := log.ParseLevel(cfg.Logger.Level)
			if err != nil {
				return err
			}
			lo.SetLevel(lvl)

			sc, err := smtp.NewClient(&cfg.SMTP)
			if err != nil {
				a.Logger.WithError(err).Error("Failed to create smtp client")
				return err
			}

			ctx := context.Background()

			// register worker.
			consumer := worker.NewConsumer(a.Queue, lo)
			projectRepo := postgres.NewProjectRepo(a.DB)
			metaEventRepo := postgres.NewMetaEventRepo(a.DB)
			endpointRepo := postgres.NewEndpointRepo(a.DB)
			eventRepo := postgres.NewEventRepo(a.DB)
			eventDeliveryRepo := postgres.NewEventDeliveryRepo(a.DB)
			subRepo := postgres.NewSubscriptionRepo(a.DB)
			deviceRepo := postgres.NewDeviceRepo(a.DB)
			configRepo := postgres.NewConfigRepo(a.DB)
			searchBackend, err := searcher.NewSearchClient(cfg)
			if err != nil {
				a.Logger.Debug("Failed to initialise search backend")
			}

			rd, err := rdb.NewClient(cfg.Redis.BuildDsn())
			if err != nil {
				return err
			}

			consumer.RegisterHandlers(convoy.EventProcessor, task.ProcessEventDelivery(
				endpointRepo,
				eventDeliveryRepo,
				projectRepo,
				subRepo,
				a.Queue))

			consumer.RegisterHandlers(convoy.CreateEventProcessor, task.ProcessEventCreation(
				endpointRepo,
				eventRepo,
				projectRepo,
				eventDeliveryRepo,
				a.Cache,
				a.Queue,
				subRepo,
				deviceRepo))

			consumer.RegisterHandlers(convoy.CreateDynamicEventProcessor, task.ProcessDynamicEventCreation(
				endpointRepo,
				eventRepo,
				projectRepo,
				eventDeliveryRepo,
				a.Cache,
				a.Queue,
				subRepo,
				deviceRepo))

			consumer.RegisterHandlers(convoy.RetentionPolicies, task.RetentionPolicies(
				configRepo,
				projectRepo,
				eventRepo,
				eventDeliveryRepo,
				postgres.NewExportRepo(a.DB),
				searchBackend,
			))

			consumer.RegisterHandlers(convoy.MonitorTwitterSources, task.MonitorTwitterSources(a.DB, a.Queue))

			consumer.RegisterHandlers(convoy.ExpireSecretsProcessor, task.ExpireSecret(endpointRepo))

			consumer.RegisterHandlers(convoy.DailyAnalytics, analytics.TrackDailyAnalytics(a.DB, cfg, rd))
			consumer.RegisterHandlers(convoy.EmailProcessor, task.ProcessEmails(sc))

			consumer.RegisterHandlers(convoy.TokenizeSearch, task.SearchTokenizer(projectRepo))
			consumer.RegisterHandlers(convoy.NotificationProcessor, task.ProcessNotifications(sc))
			consumer.RegisterHandlers(convoy.MetaEventProcessor, task.ProcessMetaEvent(projectRepo, metaEventRepo))
			consumer.RegisterHandlers(convoy.DeleteArchivedTasksProcessor, task.DeleteArchivedTasks(a.Queue, rd))

			// start worker
			lo.Infof("Starting Convoy workers...")
			consumer.Start()

			metrics.RegisterQueueMetrics(a.Queue)

			router := chi.NewRouter()
			router.Handle("/metrics", promhttp.HandlerFor(metrics.Reg(), promhttp.HandlerOpts{}))
			router.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
				render.JSON(w, r, "Convoy")
			})

			srv := &http.Server{
				Handler: router,
				Addr:    fmt.Sprintf(":%d", workerPort),
			}

			a.Logger.Infof("Worker running on port %v", workerPort)

			e := srv.ListenAndServe()
			if e != nil {
				return e
			}

			<-ctx.Done()
			return ctx.Err()
		},
	}

	cmd.Flags().Uint32Var(&workerPort, "worker-port", 5006, "Worker port")
	cmd.Flags().StringVar(&logLevel, "log-level", "", "scheduler log level")
	cmd.Flags().BoolVar(&newRelicConfigEnabled, "new-relic-config-enabled", false, "Enable new-relic config")
	cmd.Flags().BoolVar(&newRelicTracerEnabled, "new-relic-tracer-enabled", false, "Enable new-relic distributed tracer")
	cmd.Flags().StringVar(&newRelicApp, "new-relic-app", "", "NewRelic application name")
	cmd.Flags().StringVar(&newRelicKey, "new-relic-key", "", "NewRelic application license key")

	return cmd
}

func buildWorkerCliConfiguration(cmd *cobra.Command) (*config.Configuration, error) {
	c := &config.Configuration{}

	logLevel, err := cmd.Flags().GetString("log-level")
	if err != nil {
		return nil, err
	}

	if !util.IsStringEmpty(logLevel) {
		c.Logger.Level = logLevel
	}

	workerPort, err := cmd.Flags().GetUint32("worker-port")
	if err != nil {
		return nil, err
	}

	if workerPort != 0 {
		c.Server.HTTP.WorkerPort = workerPort
	}

	c.Server.HTTP.WorkerPort = workerPort

	// CONVOY_NEWRELIC_APP_NAME
	newReplicApp, err := cmd.Flags().GetString("new-relic-app")
	if err != nil {
		return nil, err
	}

	if !util.IsStringEmpty(newReplicApp) {
		c.Tracer.NewRelic.AppName = newReplicApp
	}

	// CONVOY_NEWRELIC_LICENSE_KEY
	newReplicKey, err := cmd.Flags().GetString("new-relic-key")
	if err != nil {
		return nil, err
	}

	if !util.IsStringEmpty(newReplicKey) {
		c.Tracer.NewRelic.LicenseKey = newReplicKey
	}

	// CONVOY_NEWRELIC_CONFIG_ENABLED
	isNRCESet := cmd.Flags().Changed("new-relic-config-enabled")
	if isNRCESet {
		newReplicConfigEnabled, err := cmd.Flags().GetBool("new-relic-config-enabled")
		if err != nil {
			return nil, err
		}

		c.Tracer.NewRelic.ConfigEnabled = newReplicConfigEnabled
	}

	// CONVOY_NEWRELIC_DISTRIBUTED_TRACER_ENABLED
	isNRTESet := cmd.Flags().Changed("new-relic-tracer-enabled")
	if isNRTESet {
		newReplicTracerEnabled, err := cmd.Flags().GetBool("new-relic-tracer-enabled")
		if err != nil {
			return nil, err
		}

		c.Tracer.NewRelic.DistributedTracerEnabled = newReplicTracerEnabled
	}

	return c, nil
}
