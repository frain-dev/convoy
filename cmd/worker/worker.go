package worker

import (
	"context"
	"net/http"
	"time"

	"github.com/frain-dev/convoy/net"

	"github.com/frain-dev/convoy/internal/pkg/limiter"
	"github.com/frain-dev/convoy/internal/pkg/rdb"
	"github.com/frain-dev/convoy/internal/pkg/server"
	"github.com/frain-dev/convoy/internal/telemetry"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/database/postgres"
	"github.com/frain-dev/convoy/internal/pkg/cli"
	"github.com/frain-dev/convoy/internal/pkg/metrics"
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
	var consumerPoolSize int
	var interval int

	var smtpSSL bool
	var smtpUsername string
	var smtpPassword string
	var smtpReplyTo string
	var smtpFrom string
	var smtpProvider string
	var smtpUrl string
	var smtpPort uint32

	cmd := &cobra.Command{
		Use:   "worker",
		Short: "Start worker instance",
		Annotations: map[string]string{
			"ShouldBootstrap": "false",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := context.WithCancel(cmd.Context())
			defer cancel()

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

			// register worker.
			consumer := worker.NewConsumer(ctx, cfg.ConsumerPoolSize, a.Queue, lo)
			projectRepo := postgres.NewProjectRepo(a.DB, a.Cache)
			metaEventRepo := postgres.NewMetaEventRepo(a.DB, a.Cache)
			endpointRepo := postgres.NewEndpointRepo(a.DB, a.Cache)
			eventRepo := postgres.NewEventRepo(a.DB, a.Cache)
			jobRepo := postgres.NewJobRepo(a.DB, a.Cache)
			eventDeliveryRepo := postgres.NewEventDeliveryRepo(a.DB, a.Cache)
			subRepo := postgres.NewSubscriptionRepo(a.DB, a.Cache)
			deviceRepo := postgres.NewDeviceRepo(a.DB, a.Cache)
			configRepo := postgres.NewConfigRepo(a.DB)

			rd, err := rdb.NewClient(cfg.Redis.BuildDsn())
			if err != nil {
				return err
			}

			rateLimiter, err := limiter.NewLimiter([]string{}, cfg, true)
			if err != nil {
				return err
			}

			counter := &telemetry.EventsCounter{}

			pb := telemetry.NewposthogBackend()
			mb := telemetry.NewmixpanelBackend()

			configuration, err := configRepo.LoadConfiguration(context.Background())
			if err != nil {
				a.Logger.WithError(err).Fatal("Failed to instance configuration")
				return err
			}

			newTelemetry := telemetry.NewTelemetry(a.Logger.(*log.Logger), configuration,
				telemetry.OptionTracker(counter),
				telemetry.OptionBackend(pb),
				telemetry.OptionBackend(mb))

			dispatcher, err := net.NewDispatcher(10*time.Second, cfg.Server.HTTP.HttpProxy, false)
			if err != nil {
				a.Logger.WithError(err).Fatal("Failed to create new net dispatcher")
				return err
			}

			consumer.RegisterHandlers(convoy.EventProcessor, task.ProcessEventDelivery(
				endpointRepo,
				eventDeliveryRepo,
				projectRepo,
				a.Queue, rateLimiter, dispatcher,
			), newTelemetry)

			consumer.RegisterHandlers(convoy.CreateEventProcessor, task.ProcessEventCreation(
				endpointRepo,
				eventRepo,
				projectRepo,
				eventDeliveryRepo,
				a.Queue,
				subRepo,
				deviceRepo), newTelemetry)

			consumer.RegisterHandlers(convoy.CreateBroadcastEventProcessor, task.ProcessBroadcastEventCreation(
				endpointRepo,
				eventRepo,
				projectRepo,
				eventDeliveryRepo,
				a.Queue,
				subRepo,
				deviceRepo), newTelemetry)

			consumer.RegisterHandlers(convoy.CreateDynamicEventProcessor, task.ProcessDynamicEventCreation(
				endpointRepo,
				eventRepo,
				projectRepo,
				eventDeliveryRepo,
				a.Queue,
				subRepo,
				deviceRepo), newTelemetry)

			consumer.RegisterHandlers(convoy.RetentionPolicies, task.RetentionPolicies(
				configRepo,
				projectRepo,
				eventRepo,
				eventDeliveryRepo,
				rd,
			), nil)

			consumer.RegisterHandlers(convoy.MonitorTwitterSources, task.MonitorTwitterSources(a.DB, a.Cache, a.Queue, rd), nil)

			consumer.RegisterHandlers(convoy.ExpireSecretsProcessor, task.ExpireSecret(endpointRepo), nil)

			consumer.RegisterHandlers(convoy.DailyAnalytics, task.PushDailyTelemetry(lo, a.DB, a.Cache, rd), nil)
			consumer.RegisterHandlers(convoy.EmailProcessor, task.ProcessEmails(sc), nil)

			consumer.RegisterHandlers(convoy.TokenizeSearch, task.GeneralTokenizerHandler(projectRepo, eventRepo, jobRepo, rd), nil)
			consumer.RegisterHandlers(convoy.TokenizeSearchForProject, task.TokenizerHandler(eventRepo, jobRepo), nil)

			consumer.RegisterHandlers(convoy.NotificationProcessor, task.ProcessNotifications(sc), nil)
			consumer.RegisterHandlers(convoy.MetaEventProcessor, task.ProcessMetaEvent(projectRepo, metaEventRepo), nil)
			consumer.RegisterHandlers(convoy.DeleteArchivedTasksProcessor, task.DeleteArchivedTasks(a.Queue, rd), nil)

			// start worker
			lo.Infof("Starting Convoy workers...")
			consumer.Start()

			metrics.RegisterQueueMetrics(a.Queue, a.DB)

			router := chi.NewRouter()
			router.Handle("/metrics", promhttp.HandlerFor(metrics.Reg(), promhttp.HandlerOpts{}))
			router.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
				render.JSON(w, r, "Convoy")
			})

			srv := server.NewServer(workerPort, func() {})
			srv.SetHandler(router)

			httpConfig := cfg.Server.HTTP
			if httpConfig.SSL {
				a.Logger.Infof("Worker started with SSL: cert_file: %s, key_file: %s", httpConfig.SSLCertFile, httpConfig.SSLKeyFile)

				srv.ListenAndServeTLS(httpConfig.SSLCertFile, httpConfig.SSLKeyFile)
				return nil
			}

			a.Logger.Infof("Worker running on port %v", workerPort)
			srv.Listen()

			return ctx.Err()
		},
	}

	cmd.Flags().BoolVar(&smtpSSL, "smtp-ssl", false, "Enable SMTP SSL")
	cmd.Flags().StringVar(&smtpUsername, "smtp-username", "", "SMTP authentication username")
	cmd.Flags().StringVar(&smtpPassword, "smtp-password", "", "SMTP authentication password")
	cmd.Flags().StringVar(&smtpFrom, "smtp-from", "", "Sender email address")
	cmd.Flags().StringVar(&smtpReplyTo, "smtp-reply-to", "", "Email address to reply to")
	cmd.Flags().StringVar(&smtpProvider, "smtp-provider", "", "SMTP provider")
	cmd.Flags().StringVar(&smtpUrl, "smtp-url", "", "SMTP provider URL")
	cmd.Flags().Uint32Var(&smtpPort, "smtp-port", 0, "SMTP Port")

	cmd.Flags().Uint32Var(&workerPort, "worker-port", 5006, "Worker port")
	cmd.Flags().StringVar(&logLevel, "log-level", "", "scheduler log level")
	cmd.Flags().IntVar(&consumerPoolSize, "consumers", -1, "Size of the consumers pool.")
	cmd.Flags().IntVar(&interval, "interval", 10, "the time interval, measured in seconds to update the in-memory store from the database")

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

	// CONVOY_WORKER_POOL_SIZE
	consumerPoolSize, err := cmd.Flags().GetInt("consumers")
	if err != nil {
		return nil, err
	}

	workerPort, err := cmd.Flags().GetUint32("worker-port")
	if err != nil {
		return nil, err
	}

	if workerPort != 0 {
		c.Server.HTTP.WorkerPort = workerPort
	}

	c.Server.HTTP.WorkerPort = workerPort

	if consumerPoolSize >= 0 {
		c.ConsumerPoolSize = consumerPoolSize
	}

	// CONVOY_SMTP_PROVIDER
	smtpProvider, err := cmd.Flags().GetString("smtp-provider")
	if err != nil {
		return nil, err
	}

	if !util.IsStringEmpty(smtpProvider) {
		c.SMTP.Provider = smtpProvider
	}

	// CONVOY_SMTP_URL
	smtpUrl, err := cmd.Flags().GetString("smtp-url")
	if err != nil {
		return nil, err
	}

	if !util.IsStringEmpty(smtpUrl) {
		c.SMTP.URL = smtpUrl
	}

	// CONVOY_SMTP_USERNAME
	smtpUsername, err := cmd.Flags().GetString("smtp-username")
	if err != nil {
		return nil, err
	}

	if !util.IsStringEmpty(smtpUsername) {
		c.SMTP.Username = smtpUsername
	}

	// CONVOY_SMTP_PASSWORD
	smtpPassword, err := cmd.Flags().GetString("smtp-password")
	if err != nil {
		return nil, err
	}

	if !util.IsStringEmpty(smtpPassword) {
		c.SMTP.Password = smtpPassword
	}

	// CONVOY_SMTP_FROM
	smtpFrom, err := cmd.Flags().GetString("smtp-from")
	if err != nil {
		return nil, err
	}

	if !util.IsStringEmpty(smtpFrom) {
		c.SMTP.From = smtpFrom
	}

	// CONVOY_SMTP_REPLY_TO
	smtpReplyTo, err := cmd.Flags().GetString("smtp-reply-to")
	if err != nil {
		return nil, err
	}

	if !util.IsStringEmpty(smtpReplyTo) {
		c.SMTP.ReplyTo = smtpReplyTo
	}

	// CONVOY_SMTP_PORT
	smtpPort, err := cmd.Flags().GetUint32("smtp-port")
	if err != nil {
		return nil, err
	}

	if smtpPort != 0 {
		c.SMTP.Port = smtpPort
	}

	return c, nil
}
