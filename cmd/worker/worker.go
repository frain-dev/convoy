package worker

import (
	"context"
	"errors"
	"fmt"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/internal/pkg/fflag"
	"github.com/frain-dev/convoy/internal/pkg/keys"
	"github.com/frain-dev/convoy/internal/pkg/retention"
	"net/http"
	"strings"
	"time"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/database/postgres"
	"github.com/frain-dev/convoy/internal/pkg/cli"
	"github.com/frain-dev/convoy/internal/pkg/limiter"
	"github.com/frain-dev/convoy/internal/pkg/loader"
	"github.com/frain-dev/convoy/internal/pkg/memorystore"
	"github.com/frain-dev/convoy/internal/pkg/metrics"
	"github.com/frain-dev/convoy/internal/pkg/rdb"
	"github.com/frain-dev/convoy/internal/pkg/server"
	"github.com/frain-dev/convoy/internal/pkg/smtp"
	"github.com/frain-dev/convoy/internal/telemetry"
	"github.com/frain-dev/convoy/net"
	cb "github.com/frain-dev/convoy/pkg/circuit_breaker"
	"github.com/frain-dev/convoy/pkg/clock"
	"github.com/frain-dev/convoy/pkg/log"
	"github.com/frain-dev/convoy/queue"
	redisQueue "github.com/frain-dev/convoy/queue/redis"
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
	var executionMode string
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
				a.Logger.WithError(err).Fatal("Failed to load configuration")
			}

			err = StartWorker(ctx, a, cfg, interval)
			if err != nil {
				return err
			}

			router := chi.NewRouter()
			router.Handle("/metrics", promhttp.HandlerFor(metrics.Reg(), promhttp.HandlerOpts{}))
			router.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
				render.JSON(w, r, "Convoy")
			})

			srv := server.NewServer(cfg.Server.HTTP.WorkerPort, func() {})
			srv.SetHandler(router)

			httpConfig := cfg.Server.HTTP
			if httpConfig.SSL {
				a.Logger.Infof("Worker started with SSL: cert_file: %s, key_file: %s", httpConfig.SSLCertFile, httpConfig.SSLKeyFile)

				srv.ListenAndServeTLS(httpConfig.SSLCertFile, httpConfig.SSLKeyFile)
				return nil
			}

			fmt.Printf("Starting Convoy Worker on port %v\n", cfg.Server.HTTP.WorkerPort)
			srv.Listen()

			return nil
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

	cmd.Flags().Uint32Var(&workerPort, "worker-port", 0, "Worker port")
	cmd.Flags().StringVar(&logLevel, "log-level", "", "scheduler log level")
	cmd.Flags().IntVar(&consumerPoolSize, "consumers", -1, "Size of the consumers pool.")
	cmd.Flags().IntVar(&interval, "interval", 10, "the time interval, measured in seconds to update the in-memory store from the database")
	cmd.Flags().StringVar(&executionMode, "mode", "", "Execution Mode (one of events, retry and default)")

	return cmd
}

func StartWorker(ctx context.Context, a *cli.App, cfg config.Configuration, interval int) error {
	lo := a.Logger.(*log.Logger)
	lo.SetPrefix("worker")

	lvl, err := log.ParseLevel(cfg.Logger.Level)
	if err != nil {
		return err
	}
	lo.SetLevel(lvl)

	km := keys.NewHCPVaultKeyManagerFromConfig(cfg.HCPVault, a.Licenser, a.Cache)
	if km.IsSet() {
		if _, err = km.GetCurrentKeyFromCache(); err != nil {
			if !errors.Is(err, keys.ErrCredentialEncryptionFeatureUnavailable) {
				return err
			}
			km.Unset()
		}
	}
	if err = keys.Set(km); err != nil {
		return err
	}

	sc, err := smtp.NewClient(&cfg.SMTP)
	if err != nil {
		lo.WithError(err).Error("Failed to create smtp client")
		return err
	}

	redis, err := rdb.NewClient(cfg.Redis.BuildDsn())
	if err != nil {
		return err
	}

	events := map[string]int{
		string(convoy.EventQueue):         5,
		string(convoy.CreateEventQueue):   5,
		string(convoy.EventWorkflowQueue): 5,
	}

	retry := map[string]int{
		string(convoy.RetryEventQueue):    7,
		string(convoy.ScheduleQueue):      1,
		string(convoy.DefaultQueue):       1,
		string(convoy.MetaEventQueue):     1,
		string(convoy.EventWorkflowQueue): 4,
	}

	both := map[string]int{
		string(convoy.EventQueue):         4,
		string(convoy.CreateEventQueue):   3,
		string(convoy.RetryEventQueue):    2,
		string(convoy.ScheduleQueue):      1,
		string(convoy.DefaultQueue):       1,
		string(convoy.MetaEventQueue):     1,
		string(convoy.EventWorkflowQueue): 3,
	}

	if !a.Licenser.AgentExecutionMode() {
		cfg.WorkerExecutionMode = config.DefaultExecutionMode
	}

	err = config.Override(&cfg)
	if err != nil {
		return err
	}

	var queueNames map[string]int
	switch cfg.WorkerExecutionMode {
	case config.RetryExecutionMode:
		queueNames = retry
	case config.EventsExecutionMode:
		queueNames = events
	case config.DefaultExecutionMode:
		queueNames = both
	default:
		return fmt.Errorf("unknown execution mode: %s", cfg.WorkerExecutionMode)
	}

	opts := queue.QueueOptions{
		Names:             queueNames,
		RedisClient:       redis,
		RedisAddress:      cfg.Redis.BuildDsn(),
		Type:              string(config.RedisQueueProvider),
		PrometheusAddress: cfg.Prometheus.Dsn,
	}

	q := redisQueue.NewQueue(opts)

	// register worker.
	consumer := worker.NewConsumer(ctx, cfg.ConsumerPoolSize, q, lo)
	projectRepo := postgres.NewProjectRepo(a.DB)
	metaEventRepo := postgres.NewMetaEventRepo(a.DB)
	endpointRepo := postgres.NewEndpointRepo(a.DB)
	eventRepo := postgres.NewEventRepo(a.DB)
	jobRepo := postgres.NewJobRepo(a.DB)
	eventDeliveryRepo := postgres.NewEventDeliveryRepo(a.DB)
	subRepo := postgres.NewSubscriptionRepo(a.DB)
	deviceRepo := postgres.NewDeviceRepo(a.DB)
	configRepo := postgres.NewConfigRepo(a.DB)
	attemptRepo := postgres.NewDeliveryAttemptRepo(a.DB)

	rd, err := rdb.NewClient(cfg.Redis.BuildDsn())
	if err != nil {
		return err
	}

	rateLimiter, err := limiter.NewLimiter(cfg)
	if err != nil {
		return err
	}

	counter := &telemetry.EventsCounter{}

	pb := telemetry.NewposthogBackend()
	mb := telemetry.NewmixpanelBackend()

	configuration, err := configRepo.LoadConfiguration(context.Background())
	if err != nil {
		lo.WithError(err).Fatal("Failed to instance configuration")
		return err
	}

	subscriptionsLoader := loader.NewSubscriptionLoader(subRepo, projectRepo, lo, 0)
	subscriptionsTable := memorystore.NewTable(memorystore.OptionSyncer(subscriptionsLoader))

	err = memorystore.DefaultStore.Register("subscriptions", subscriptionsTable)
	if err != nil {
		return err
	}

	// initial sync.
	err = subscriptionsLoader.SyncChanges(ctx, subscriptionsTable)
	if err != nil {
		return err
	}

	go memorystore.DefaultStore.Sync(ctx, interval)

	featureFlag := fflag.NewFFlag(cfg.EnableFeatureFlag)
	newTelemetry := telemetry.NewTelemetry(lo, configuration,
		telemetry.OptionTracker(counter),
		telemetry.OptionBackend(pb),
		telemetry.OptionBackend(mb))

	dispatcher, err := net.NewDispatcher(
		a.Licenser,
		featureFlag,
		net.LoggerOption(lo),
		net.ProxyOption(cfg.Server.HTTP.HttpProxy),
		net.AllowListOption(cfg.Dispatcher.AllowList),
		net.BlockListOption(cfg.Dispatcher.BlockList),
		net.InsecureSkipVerifyOption(cfg.Dispatcher.InsecureSkipVerify),
	)
	if err != nil {
		lo.WithError(err).Fatal("Failed to create new net dispatcher")
		return err
	}

	var circuitBreakerManager *cb.CircuitBreakerManager

	if featureFlag.CanAccessFeature(fflag.CircuitBreaker) {
		circuitBreakerManager, err = cb.NewCircuitBreakerManager(
			cb.ConfigOption(configuration.ToCircuitBreakerConfig()),
			cb.StoreOption(cb.NewRedisStore(rd.Client(), clock.NewRealClock())),
			cb.ClockOption(clock.NewRealClock()),
			cb.LoggerOption(lo),
			cb.NotificationFunctionOption(func(n cb.NotificationType, c cb.CircuitBreakerConfig, b cb.CircuitBreaker) error {
				endpointId := strings.Split(b.Key, ":")[1]
				project, funcErr := projectRepo.FetchProjectByID(ctx, b.TenantId)
				if funcErr != nil {
					return funcErr
				}

				endpoint, funcErr := endpointRepo.FindEndpointByID(ctx, endpointId, b.TenantId)
				if funcErr != nil {
					return funcErr
				}

				switch n {
				case cb.TypeDisableResource:
					breakerErr := endpointRepo.UpdateEndpointStatus(ctx, project.UID, endpoint.UID, datastore.InactiveEndpointStatus)
					if breakerErr != nil {
						return breakerErr
					}
				default:
					return fmt.Errorf("unsupported circuit breaker notification type: %s", n)
				}
				return nil
			}),
		)
		if err != nil {
			lo.WithError(err).Fatal("Failed to create circuit breaker manager")
		}

		go circuitBreakerManager.Start(ctx, attemptRepo.GetFailureAndSuccessCounts)
	} else {
		lo.Warn(fflag.ErrCircuitBreakerNotEnabled)
	}

	var ret retention.Retentioner
	if featureFlag.CanAccessFeature(fflag.RetentionPolicy) && a.Licenser.RetentionPolicy() {
		policy, _err := time.ParseDuration(cfg.RetentionPolicy.Policy)
		if _err != nil {
			lo.WithError(_err).Fatal("Failed to parse retention policy")
			return _err
		}

		ret, err = retention.NewRetentionPolicy(a.DB, lo, policy)
		if err != nil {
			lo.WithError(err).Fatal("Failed to create retention policy")
		}

		ret.Start(ctx, time.Minute)
	} else {
		lo.Warn(fflag.ErrRetentionPolicyNotEnabled)
	}

	channels := make(map[string]task.EventChannel)
	defaultCh, broadcastCh, dynamicCh := task.NewDefaultEventChannel(), task.NewBroadcastEventChannel(subscriptionsTable), task.NewDynamicEventChannel()
	channels["default"] = defaultCh
	channels["broadcast"] = broadcastCh
	channels["dynamic"] = dynamicCh

	consumer.RegisterHandlers(convoy.EventProcessor, task.ProcessEventDelivery(
		endpointRepo,
		eventDeliveryRepo,
		a.Licenser,
		projectRepo,
		a.Queue,
		rateLimiter,
		dispatcher,
		attemptRepo,
		circuitBreakerManager,
		featureFlag,
		a.TracerBackend,
	), newTelemetry)

	consumer.RegisterHandlers(convoy.CreateEventProcessor, task.ProcessEventCreation(
		defaultCh,
		endpointRepo,
		eventRepo,
		projectRepo,
		eventDeliveryRepo,
		a.Queue,
		subRepo,
		deviceRepo, a.Licenser), newTelemetry)

	consumer.RegisterHandlers(convoy.RetryEventProcessor, task.ProcessRetryEventDelivery(
		endpointRepo,
		eventDeliveryRepo,
		a.Licenser,
		projectRepo,
		a.Queue,
		rateLimiter,
		dispatcher,
		attemptRepo,
		circuitBreakerManager,
		featureFlag,
		a.TracerBackend,
	), newTelemetry)

	consumer.RegisterHandlers(convoy.CreateBroadcastEventProcessor, task.ProcessBroadcastEventCreation(
		broadcastCh,
		endpointRepo,
		eventRepo,
		projectRepo,
		eventDeliveryRepo,
		a.Queue,
		subRepo,
		deviceRepo,
		a.Licenser), newTelemetry)

	consumer.RegisterHandlers(convoy.CreateDynamicEventProcessor, task.ProcessDynamicEventCreation(
		dynamicCh,
		endpointRepo,
		eventRepo,
		projectRepo,
		eventDeliveryRepo,
		a.Queue,
		subRepo,
		deviceRepo, a.Licenser), newTelemetry)

	consumer.RegisterHandlers(convoy.RetentionPolicies, task.RetentionPolicies(rd, ret), nil)

	consumer.RegisterHandlers(convoy.MatchEventSubscriptionsProcessor, task.MatchSubscriptionsAndCreateEventDeliveries(
		channels,
		endpointRepo,
		eventRepo,
		projectRepo,
		eventDeliveryRepo,
		a.Queue,
		subRepo,
		deviceRepo, a.Licenser), newTelemetry)

	consumer.RegisterHandlers(convoy.MonitorTwitterSources, task.MonitorTwitterSources(a.DB, a.Queue, rd), nil)

	consumer.RegisterHandlers(convoy.ExpireSecretsProcessor, task.ExpireSecret(endpointRepo), nil)

	consumer.RegisterHandlers(convoy.DailyAnalytics, task.PushDailyTelemetry(lo, a.DB, rd), nil)
	consumer.RegisterHandlers(convoy.EmailProcessor, task.ProcessEmails(sc), nil)

	if featureFlag.CanAccessFeature(fflag.FullTextSearch) && a.Licenser.AdvancedWebhookFiltering() {
		consumer.RegisterHandlers(convoy.TokenizeSearch, task.GeneralTokenizerHandler(projectRepo, eventRepo, jobRepo, rd), nil)
		consumer.RegisterHandlers(convoy.TokenizeSearchForProject, task.TokenizerHandler(eventRepo, jobRepo), nil)
	}

	consumer.RegisterHandlers(convoy.NotificationProcessor, task.ProcessNotifications(sc), nil)
	consumer.RegisterHandlers(convoy.MetaEventProcessor, task.ProcessMetaEvent(projectRepo, metaEventRepo, dispatcher, a.TracerBackend), nil)
	consumer.RegisterHandlers(convoy.DeleteArchivedTasksProcessor, task.DeleteArchivedTasks(a.Queue, rd), nil)

	metrics.RegisterQueueMetrics(a.Queue, a.DB, circuitBreakerManager)

	// start worker
	consumer.Start()
	lo.Println("Starting Convoy Consumer Pool")

	return ctx.Err()
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

	if consumerPoolSize >= 0 {
		c.ConsumerPoolSize = consumerPoolSize
	}

	// CONVOY_WORKER_PORT
	workerPort, err := cmd.Flags().GetUint32("worker-port")
	if err != nil {
		return nil, err
	}

	if workerPort != 0 {
		c.Server.HTTP.WorkerPort = workerPort
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

	// CONVOY_WORKER_EXECUTION_MODE
	executionMode, err := cmd.Flags().GetString("mode")
	if err != nil {
		return nil, err
	}

	if !util.IsStringEmpty(executionMode) {
		c.WorkerExecutionMode = config.ExecutionMode(executionMode)
	}

	return c, nil
}
