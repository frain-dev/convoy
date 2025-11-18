package worker

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/database/postgres"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/internal/pkg/cli"
	"github.com/frain-dev/convoy/internal/pkg/fflag"
	"github.com/frain-dev/convoy/internal/pkg/keys"
	"github.com/frain-dev/convoy/internal/pkg/limiter"
	"github.com/frain-dev/convoy/internal/pkg/loader"
	"github.com/frain-dev/convoy/internal/pkg/memorystore"
	"github.com/frain-dev/convoy/internal/pkg/metrics"
	"github.com/frain-dev/convoy/internal/pkg/rdb"
	"github.com/frain-dev/convoy/internal/pkg/retention"
	"github.com/frain-dev/convoy/internal/pkg/smtp"
	"github.com/frain-dev/convoy/internal/telemetry"
	"github.com/frain-dev/convoy/net"
	cb "github.com/frain-dev/convoy/pkg/circuit_breaker"
	"github.com/frain-dev/convoy/pkg/clock"
	"github.com/frain-dev/convoy/pkg/log"
	"github.com/frain-dev/convoy/queue"
	redisQueue "github.com/frain-dev/convoy/queue/redis"
	"github.com/frain-dev/convoy/worker"
	"github.com/frain-dev/convoy/worker/task"
)

func StartWorker(ctx context.Context, a *cli.App, cfg config.Configuration, interval int) error {
	lo := a.Logger.(*log.Logger)
	lo.SetPrefix("worker")

	km := keys.NewHCPVaultKeyManagerFromConfig(cfg.HCPVault, a.Licenser, a.Cache)
	if km.IsSet() {
		if _, err := km.GetCurrentKeyFromCache(); err != nil {
			if !errors.Is(err, keys.ErrCredentialEncryptionFeatureUnavailable) {
				return err
			}
			km.Unset()
		}
	}

	if err := keys.Set(km); err != nil {
		return err
	}

	sc, err := smtp.NewClient(&cfg.SMTP)
	if err != nil {
		lo.WithError(err).Error("Failed to create smtp client")
		return err
	}

	redis, err := rdb.NewClientFromConfig(
		cfg.Redis.BuildDsn(),
		cfg.Redis.TLSSkipVerify,
		cfg.Redis.TLSCACertFile,
		cfg.Redis.TLSCertFile,
		cfg.Redis.TLSKeyFile,
	)
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
		string(convoy.BatchRetryQueue):    5,
		string(convoy.EventWorkflowQueue): 4,
	}

	both := map[string]int{
		string(convoy.EventQueue):         4,
		string(convoy.CreateEventQueue):   4,
		string(convoy.EventWorkflowQueue): 3,
		string(convoy.RetryEventQueue):    1,
		string(convoy.ScheduleQueue):      1,
		string(convoy.DefaultQueue):       1,
		string(convoy.MetaEventQueue):     1,
		string(convoy.BatchRetryQueue):    1,
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

	ctx = log.NewContext(ctx, lo, log.Fields{})
	lvl, err := log.ParseLevel(cfg.Logger.Level)
	if err != nil {
		return err
	}

	// register worker.
	consumer := worker.NewConsumer(ctx, cfg.ConsumerPoolSize, q, lo, lvl)
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
	filterRepo := postgres.NewFilterRepo(a.DB)
	batchRetryRepo := postgres.NewBatchRetryRepo(a.DB)

	rd, err := rdb.NewClientFromConfig(
		cfg.Redis.BuildDsn(),
		cfg.Redis.TLSSkipVerify,
		cfg.Redis.TLSCACertFile,
		cfg.Redis.TLSCertFile,
		cfg.Redis.TLSKeyFile,
	)
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

	featureFlag := fflag.NewFFlag(cfg.EnableFeatureFlag)
	newTelemetry := telemetry.NewTelemetry(lo, configuration,
		telemetry.OptionTracker(counter),
		telemetry.OptionBackend(pb),
		telemetry.OptionBackend(mb))

	caCertTLSCfg, err := config.GetCaCert()
	if err != nil {
		return err
	}

	dispatcher, err := net.NewDispatcher(
		a.Licenser,
		featureFlag,
		net.LoggerOption(lo),
		net.TracerOption(a.TracerBackend),
		net.DetailedTraceOption(true),
		net.ProxyOption(cfg.Server.HTTP.HttpProxy),
		net.AllowListOption(cfg.Dispatcher.AllowList),
		net.BlockListOption(cfg.Dispatcher.BlockList),
		net.TLSConfigOption(cfg.Dispatcher.InsecureSkipVerify, a.Licenser, caCertTLSCfg),
	)
	if err != nil {
		lo.WithError(err).Fatal("Failed to create new net dispatcher")
		return err
	}

	var circuitBreakerManager *cb.CircuitBreakerManager

	if featureFlag.CanAccessFeature(fflag.CircuitBreaker) {
		// Use circuit breaker config from application configuration
		masterDefaults := cb.CircuitBreakerConfig{
			SampleRate:                  cfg.CircuitBreaker.SampleRate,
			BreakerTimeout:              cfg.CircuitBreaker.ErrorTimeout,
			FailureThreshold:            cfg.CircuitBreaker.FailureThreshold,
			SuccessThreshold:            cfg.CircuitBreaker.SuccessThreshold,
			ObservabilityWindow:         cfg.CircuitBreaker.ObservabilityWindow,
			MinimumRequestCount:         cfg.CircuitBreaker.MinimumRequestCount,
			ConsecutiveFailureThreshold: cfg.CircuitBreaker.ConsecutiveFailureThreshold,
			SkipSleep:                   cfg.CircuitBreaker.SkipSleep,
		}

		circuitBreakerManager, err = cb.NewCircuitBreakerManager(
			cb.SkipSleepOption(masterDefaults.SkipSleep),
			cb.MasterConfigOption(masterDefaults),
			cb.ConfigProviderOption(func(projectID string) *cb.CircuitBreakerConfig {
				project, err := projectRepo.FetchProjectByID(ctx, projectID)
				if err != nil {
					lo.WithError(err).Warnf("Failed to fetch project %s for circuit breaker config, using default", projectID)
					return &masterDefaults
				}
				if project.Config.CircuitBreaker == nil {
					lo.Warnf("Project %s has no circuit breaker config, using default", projectID)
					return &masterDefaults
				}
				// Convert config.CircuitBreakerConfiguration to cb.CircuitBreakerConfig
				return &cb.CircuitBreakerConfig{
					SampleRate:                  project.Config.CircuitBreaker.SampleRate,
					BreakerTimeout:              project.Config.CircuitBreaker.ErrorTimeout,
					FailureThreshold:            project.Config.CircuitBreaker.FailureThreshold,
					SuccessThreshold:            project.Config.CircuitBreaker.SuccessThreshold,
					MinimumRequestCount:         project.Config.CircuitBreaker.MinimumRequestCount,
					ObservabilityWindow:         project.Config.CircuitBreaker.ObservabilityWindow,
					ConsecutiveFailureThreshold: project.Config.CircuitBreaker.ConsecutiveFailureThreshold,
				}
			}),
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
					// Disable the endpoint
					breakerErr := endpointRepo.UpdateEndpointStatus(ctx, project.UID, endpoint.UID, datastore.InactiveEndpointStatus)
					if breakerErr != nil {
						return breakerErr
					}

					// Send notification emails (support + owner)
					orgRepo := postgres.NewOrgRepo(a.DB)
					ownerEmail := ""
					if org, err := orgRepo.FetchOrganisationByID(ctx, project.OrganisationID); err == nil {
						if owner, err := postgres.NewUserRepo(a.DB).FindUserByID(ctx, org.OwnerID); err == nil {
							ownerEmail = owner.Email
						}
					}
					_ = EnqueueCircuitBreakerEmails(a.Queue, lo, project, endpoint, ownerEmail)

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

		ret, err = retention.NewPartitionRetentionPolicy(a.DB, lo, policy)
		if err != nil {
			lo.WithError(err).Fatal("Failed to create retention policy")
		}

		ret.Start(ctx, time.Minute)
	} else {
		lo.Warn(fflag.ErrRetentionPolicyNotEnabled)

		ret = retention.NewDeleteRetentionPolicy(a.DB, lo)
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
		a.TracerBackend),
		newTelemetry)

	consumer.RegisterHandlers(convoy.CreateEventProcessor, task.ProcessEventCreation(
		endpointRepo,
		eventRepo,
		projectRepo,
		a.Queue,
		subRepo,
		filterRepo,
		a.Licenser,
		a.TracerBackend),
		newTelemetry)

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
		a.TracerBackend),
		newTelemetry)

	consumer.RegisterHandlers(convoy.CreateBroadcastEventProcessor, task.ProcessBroadcastEventCreation(
		broadcastCh,
		endpointRepo,
		eventRepo,
		projectRepo,
		a.Queue,
		subRepo,
		filterRepo,
		a.Licenser,
		a.TracerBackend),
		newTelemetry)

	consumer.RegisterHandlers(convoy.CreateDynamicEventProcessor, task.ProcessDynamicEventCreation(
		endpointRepo,
		eventRepo,
		projectRepo,
		a.Queue,
		subRepo,
		filterRepo,
		a.Licenser,
		a.TracerBackend),
		newTelemetry)

	if a.Licenser.RetentionPolicy() {
		consumer.RegisterHandlers(convoy.RetentionPolicies, task.RetentionPolicies(rd, ret), nil)
		consumer.RegisterHandlers(convoy.BackupProjectData, task.BackupProjectData(configRepo, projectRepo, eventRepo, eventDeliveryRepo, attemptRepo, rd), nil)
	}

	consumer.RegisterHandlers(convoy.MatchEventSubscriptionsProcessor, task.MatchSubscriptionsAndCreateEventDeliveries(
		channels,
		endpointRepo,
		eventRepo,
		projectRepo,
		eventDeliveryRepo,
		a.Queue,
		subRepo,
		filterRepo,
		deviceRepo,
		a.Licenser,
		a.TracerBackend),
		newTelemetry)

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

	consumer.RegisterHandlers(convoy.BatchRetryProcessor, task.ProcessBatchRetry(batchRetryRepo, eventDeliveryRepo, a.Queue, lo), nil)

	metrics.RegisterQueueMetrics(a.Queue, a.DB, circuitBreakerManager)

	// start worker
	consumer.Start()
	lo.Printf("Starting Convoy Consumer Pool")

	return ctx.Err()
}
