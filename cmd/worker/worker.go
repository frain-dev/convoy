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
	batch_retries "github.com/frain-dev/convoy/internal/batch_retries"
	"github.com/frain-dev/convoy/internal/configuration"
	"github.com/frain-dev/convoy/internal/delivery_attempts"
	"github.com/frain-dev/convoy/internal/filters"
	"github.com/frain-dev/convoy/internal/meta_events"
	"github.com/frain-dev/convoy/internal/organisations"
	"github.com/frain-dev/convoy/internal/pkg/billing"
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
	"github.com/frain-dev/convoy/internal/projects"
	"github.com/frain-dev/convoy/internal/subscriptions"
	"github.com/frain-dev/convoy/internal/telemetry"
	"github.com/frain-dev/convoy/internal/users"
	"github.com/frain-dev/convoy/net"
	cb "github.com/frain-dev/convoy/pkg/circuit_breaker"
	"github.com/frain-dev/convoy/pkg/clock"
	"github.com/frain-dev/convoy/pkg/log"
	"github.com/frain-dev/convoy/queue"
	redisQueue "github.com/frain-dev/convoy/queue/redis"
	"github.com/frain-dev/convoy/services"
	"github.com/frain-dev/convoy/worker"
	"github.com/frain-dev/convoy/worker/task"
)

type Worker struct {
	consumer *worker.Consumer
	logger   *log.Logger
}

// NewWorker initializes all worker components and returns a Worker instance.
func NewWorker(ctx context.Context, a *cli.App, cfg config.Configuration) (*Worker, error) {
	lo := a.Logger.(*log.Logger)
	lo.SetPrefix("worker")

	km := keys.NewHCPVaultKeyManagerFromConfig(cfg.HCPVault, a.Licenser, a.Cache)
	if km.IsSet() {
		if _, err := km.GetCurrentKeyFromCache(); err != nil {
			if !errors.Is(err, keys.ErrCredentialEncryptionFeatureUnavailable) {
				return nil, err
			}
			km.Unset()
		}
	}

	if err := keys.Set(km); err != nil {
		return nil, err
	}

	sc, err := smtp.NewClient(&cfg.SMTP)
	if err != nil {
		lo.WithError(err).Error("Failed to create smtp client")
		return nil, err
	}

	redis, err := rdb.NewClientFromConfig(
		cfg.Redis.BuildDsn(),
		cfg.Redis.TLSSkipVerify,
		cfg.Redis.TLSCACertFile,
		cfg.Redis.TLSCertFile,
		cfg.Redis.TLSKeyFile,
	)
	if err != nil {
		return nil, err
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
		return nil, err
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
		return nil, fmt.Errorf("unknown execution mode: %s", cfg.WorkerExecutionMode)
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
		return nil, err
	}

	// register worker.
	consumer := worker.NewConsumer(ctx, cfg.ConsumerPoolSize, q, lo, lvl)

	// Inject job tracker if set (for E2E tests)
	if a.JobTracker != nil {
		if tracker, ok := a.JobTracker.(worker.JobTracker); ok {
			consumer.SetJobTracker(tracker)
			lo.Info("Job tracker injected into worker consumer")
		}
	}

	projectRepo := projects.New(a.Logger, a.DB)
	metaEventRepo := meta_events.New(a.Logger, a.DB)
	endpointRepo := postgres.NewEndpointRepo(a.DB)
	eventRepo := postgres.NewEventRepo(a.DB)
	jobRepo := postgres.NewJobRepo(a.DB)
	eventDeliveryRepo := postgres.NewEventDeliveryRepo(a.DB)
	subRepo := subscriptions.New(a.Logger, a.DB)
	configRepo := configuration.New(a.Logger, a.DB)
	attemptRepo := delivery_attempts.New(a.Logger, a.DB)
	filterRepo := filters.New(a.Logger, a.DB)
	batchRetryRepo := batch_retries.New(lo, a.DB)

	rd, err := rdb.NewClientFromConfig(
		cfg.Redis.BuildDsn(),
		cfg.Redis.TLSSkipVerify,
		cfg.Redis.TLSCACertFile,
		cfg.Redis.TLSCertFile,
		cfg.Redis.TLSKeyFile,
	)
	if err != nil {
		return nil, err
	}

	rateLimiter, err := limiter.NewLimiter(cfg)
	if err != nil {
		return nil, err
	}

	counter := &telemetry.EventsCounter{}

	pb := telemetry.NewposthogBackend()
	mb := telemetry.NewmixpanelBackend()

	configuration, err := configRepo.LoadConfiguration(context.Background())
	if err != nil {
		return nil, fmt.Errorf("failed to initialize configuration: %w", err)
	}

	subscriptionsLoader := loader.NewSubscriptionLoader(subRepo, projectRepo, lo, 0)
	subscriptionsTable := memorystore.NewTable(memorystore.OptionSyncer(subscriptionsLoader))

	// Store subscription loader and table in App for E2E test access
	a.SubscriptionLoader = subscriptionsLoader
	a.SubscriptionTable = subscriptionsTable

	err = memorystore.DefaultStore.Register("subscriptions", subscriptionsTable)
	if err != nil {
		return nil, err
	}

	// initial sync.
	err = subscriptionsLoader.SyncChanges(ctx, subscriptionsTable)
	if err != nil {
		return nil, err
	}

	featureFlag := fflag.NewFFlag(cfg.EnableFeatureFlag)
	newTelemetry := telemetry.NewTelemetry(lo, configuration,
		telemetry.OptionTracker(counter),
		telemetry.OptionBackend(pb),
		telemetry.OptionBackend(mb))

	caCertTLSCfg, err := config.GetCaCert()
	if err != nil {
		return nil, err
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
		return nil, fmt.Errorf("failed to create new net dispatcher: %w", err)
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
					orgRepo := organisations.New(lo, a.DB)
					ownerEmail := ""
					if org, err := orgRepo.FetchOrganisationByID(ctx, project.OrganisationID); err == nil {
						if owner, err := users.New(a.Logger, a.DB).FindUserByID(ctx, org.OwnerID); err == nil {
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
			return nil, fmt.Errorf("failed to create circuit breaker manager: %w", err)
		}

		go circuitBreakerManager.Start(ctx, attemptRepo.GetFailureAndSuccessCounts)
	} else {
		lo.Warn(fflag.ErrCircuitBreakerNotEnabled)
	}

	var ret retention.Retentioner
	if featureFlag.CanAccessFeature(fflag.RetentionPolicy) && a.Licenser.RetentionPolicy() {
		policy, _err := time.ParseDuration(cfg.RetentionPolicy.Policy)
		if _err != nil {
			return nil, fmt.Errorf("failed to parse retention policy: %w", _err)
		}

		ret, err = retention.NewPartitionRetentionPolicy(a.DB, lo, policy)
		if err != nil {
			return nil, fmt.Errorf("failed to create retention policy: %w", err)
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

	// Initialize OAuth2 token service
	oauth2TokenService := services.NewOAuth2TokenService(a.Cache, lo)

	eventDeliveryProcessorDeps := task.EventDeliveryProcessorDeps{
		EndpointRepo:               endpointRepo,
		EventDeliveryRepo:          eventDeliveryRepo,
		Licenser:                   a.Licenser,
		ProjectRepo:                projectRepo,
		Queue:                      a.Queue,
		RateLimiter:                rateLimiter,
		Dispatcher:                 dispatcher,
		AttemptsRepo:               attemptRepo,
		CircuitBreakerManager:      circuitBreakerManager,
		FeatureFlag:                featureFlag,
		FeatureFlagFetcher:         postgres.NewFeatureFlagFetcher(a.DB),
		EarlyAdopterFeatureFetcher: postgres.NewEarlyAdopterFeatureFetcher(a.DB),
		TracerBackend:              a.TracerBackend,
		OAuth2TokenService:         oauth2TokenService,
	}

	consumer.RegisterHandlers(convoy.EventProcessor, task.ProcessEventDelivery(eventDeliveryProcessorDeps), newTelemetry)

	eventProcessorDeps := task.EventProcessorDeps{
		EndpointRepo:       endpointRepo,
		EventRepo:          eventRepo,
		ProjectRepo:        projectRepo,
		EventQueue:         a.Queue,
		SubRepo:            subRepo,
		FilterRepo:         filterRepo,
		Licenser:           a.Licenser,
		TracerBackend:      a.TracerBackend,
		OAuth2TokenService: oauth2TokenService,
		FeatureFlag:        featureFlag,
		FeatureFlagFetcher: postgres.NewFeatureFlagFetcher(a.DB),
	}

	consumer.RegisterHandlers(convoy.CreateEventProcessor, task.ProcessEventCreation(eventProcessorDeps), newTelemetry)

	consumer.RegisterHandlers(convoy.RetryEventProcessor, task.ProcessRetryEventDelivery(eventDeliveryProcessorDeps), newTelemetry)

	consumer.RegisterHandlers(convoy.CreateBroadcastEventProcessor, task.ProcessBroadcastEventCreation(broadcastCh, eventProcessorDeps), newTelemetry)

	consumer.RegisterHandlers(convoy.CreateDynamicEventProcessor, task.ProcessDynamicEventCreation(eventProcessorDeps), newTelemetry)

	if a.Licenser.RetentionPolicy() {
		consumer.RegisterHandlers(convoy.RetentionPolicies, task.RetentionPolicies(rd.Client(), ret), nil)
		consumer.RegisterHandlers(convoy.BackupProjectData, task.BackupProjectData(configRepo, projectRepo, eventRepo, eventDeliveryRepo, attemptRepo, rd.Client()), nil)
	}

	matchSubscriptionsDeps := task.MatchSubscriptionsDeps{
		Channels:                   channels,
		EndpointRepo:               endpointRepo,
		EventRepo:                  eventRepo,
		ProjectRepo:                projectRepo,
		EventDeliveryRepo:          eventDeliveryRepo,
		EventQueue:                 a.Queue,
		SubRepo:                    subRepo,
		FilterRepo:                 filterRepo,
		Licenser:                   a.Licenser,
		TracerBackend:              a.TracerBackend,
		OAuth2TokenService:         oauth2TokenService,
		FeatureFlag:                featureFlag,
		FeatureFlagFetcher:         postgres.NewFeatureFlagFetcher(a.DB),
		EarlyAdopterFeatureFetcher: postgres.NewEarlyAdopterFeatureFetcher(a.DB),
	}
	consumer.RegisterHandlers(convoy.MatchEventSubscriptionsProcessor, task.MatchSubscriptionsAndCreateEventDeliveries(matchSubscriptionsDeps), newTelemetry)

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

	//nolint:gocritic
	// consumer.RegisterHandlers(convoy.RefreshMetricsMaterializedViews, task.RefreshMetricsMaterializedViews(a.DB, rd), nil)

	consumer.RegisterHandlers(convoy.BatchRetryProcessor, task.ProcessBatchRetry(batchRetryRepo, eventDeliveryRepo, a.Queue, lo), nil)

	var billingClient billing.Client
	if cfg.Billing.Enabled {
		billingClient = billing.NewClient(cfg.Billing)
		consumer.RegisterHandlers(convoy.UpdateOrganisationStatus, task.UpdateOrganisationStatus(a.DB, billingClient, rd, lo), nil)
	}

	err = metrics.RegisterQueueMetrics(a.Queue, a.DB, circuitBreakerManager)
	if err != nil {
		return nil, fmt.Errorf("failed to register queue metrics: %w", err)
	}

	return &Worker{
		consumer: consumer,
		logger:   lo,
	}, nil
}

func (w *Worker) Run(ctx context.Context, workerReady chan struct{}) error {
	if err := w.consumer.Start(); err != nil {
		return fmt.Errorf("failed to start consumer: %w", err)
	}
	w.logger.Printf("Starting Convoy Consumer Pool")

	if workerReady != nil {
		close(workerReady)
	}

	// Wait for context to be canceled before returning
	<-ctx.Done()
	w.logger.Printf("Context canceled, stopping Convoy Consumer Pool...")
	w.consumer.Stop()
	w.logger.Printf("Convoy Consumer Pool stopped")

	return ctx.Err()
}
