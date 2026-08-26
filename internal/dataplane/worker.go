package dataplane

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/datastore/cached"
	"github.com/frain-dev/convoy/internal/backup_jobs"
	"github.com/frain-dev/convoy/internal/batch_retries"
	"github.com/frain-dev/convoy/internal/configuration"
	"github.com/frain-dev/convoy/internal/delivery_attempts"
	"github.com/frain-dev/convoy/internal/endpoints"
	"github.com/frain-dev/convoy/internal/endpoints/disable"
	"github.com/frain-dev/convoy/internal/event_deliveries"
	"github.com/frain-dev/convoy/internal/events"
	"github.com/frain-dev/convoy/internal/feature_flags"
	"github.com/frain-dev/convoy/internal/filters"
	"github.com/frain-dev/convoy/internal/meta_events"
	"github.com/frain-dev/convoy/internal/organisations"
	"github.com/frain-dev/convoy/internal/pkg/backup_collector"
	"github.com/frain-dev/convoy/internal/pkg/billing"
	blobstore "github.com/frain-dev/convoy/internal/pkg/blob-store"
	"github.com/frain-dev/convoy/internal/pkg/broker"
	"github.com/frain-dev/convoy/internal/pkg/cbenablement"
	"github.com/frain-dev/convoy/internal/pkg/exporter"
	"github.com/frain-dev/convoy/internal/pkg/fflag"
	"github.com/frain-dev/convoy/internal/pkg/keys"
	"github.com/frain-dev/convoy/internal/pkg/loader"
	"github.com/frain-dev/convoy/internal/pkg/memorystore"
	"github.com/frain-dev/convoy/internal/pkg/metrics"
	"github.com/frain-dev/convoy/internal/pkg/retention"
	"github.com/frain-dev/convoy/internal/pkg/smtp"
	"github.com/frain-dev/convoy/internal/projects"
	"github.com/frain-dev/convoy/internal/subscriptions"
	"github.com/frain-dev/convoy/internal/telemetry"
	"github.com/frain-dev/convoy/internal/users"
	"github.com/frain-dev/convoy/net"
	cb "github.com/frain-dev/convoy/pkg/circuit_breaker"
	"github.com/frain-dev/convoy/pkg/clock"
	log "github.com/frain-dev/convoy/pkg/logger"
	"github.com/frain-dev/convoy/services"
	"github.com/frain-dev/convoy/worker"
	"github.com/frain-dev/convoy/worker/task"
)

type Worker struct {
	consumer        *worker.Consumer
	backupCollector *backup_collector.BackupCollector // nil if CDC backup disabled
	logger          log.Logger
}

// NewWorker initializes all worker components and returns a Worker instance.
func NewWorker(ctx context.Context, opts RuntimeOpts, cfg config.Configuration) (*Worker, error) {
	lo := opts.Logger

	km := keys.NewHCPVaultKeyManagerFromConfig(cfg.HCPVault, opts.Licenser, opts.Cache)
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

	sc, err := smtp.NewClient(&cfg.SMTP, lo)
	if err != nil {
		lo.Error("Failed to create smtp client", "error", err)
		return nil, err
	}

	if opts.Broker == nil {
		return nil, fmt.Errorf("broker dependencies are required")
	}
	dynamicEventAcker := opts.Broker.Acker

	if !opts.Licenser.AgentExecutionMode() {
		cfg.WorkerExecutionMode = config.DefaultExecutionMode
	}

	err = config.Override(&cfg)
	if err != nil {
		return nil, err
	}

	lvl, err := log.ParseLevel(cfg.Logger.Level)
	if err != nil {
		return nil, err
	}
	queueNames, err := broker.QueueNames(cfg.WorkerExecutionMode)
	if err != nil {
		return nil, err
	}

	if cfg.WorkerPoolUndersized() {
		lo.Warnf(
			"database.max_open_conn (%d) is below consumer_pool_size (%d): consumers will block acquiring a connection instead of draining the queue. Raise max_open_conn to at least consumer_pool_size.",
			cfg.Database.EffectiveMaxOpenConnections(), cfg.ConsumerPoolSize,
		)
	}

	consumer, err := worker.NewConsumer(ctx, cfg.ConsumerPoolSize, queueNames, opts.Broker.ConsumerBackend, lo, lvl)
	if err != nil {
		return nil, err
	}

	if opts.JobTracker != nil {
		if tracker, ok := opts.JobTracker.(worker.JobTracker); ok {
			consumer.SetJobTracker(tracker)
			lo.Info("Job tracker injected into worker consumer")
		}
	}

	projectRepo := cached.NewCachedProjectRepository(projects.New(opts.Logger, opts.DB), opts.Cache, cached.DefaultProjectTTL, lo)
	metaEventRepo := meta_events.New(opts.Logger, opts.DB)
	endpointRepo := cached.NewCachedEndpointRepository(endpoints.New(opts.Logger, opts.DB), opts.Cache, cached.DefaultEndpointTTL, lo)
	eventRepo := events.New(opts.Logger, opts.DB)
	eventDeliveryRepo := event_deliveries.New(opts.Logger, opts.DB)
	subRepo := cached.NewCachedSubscriptionRepository(subscriptions.New(opts.Logger, opts.DB), opts.Cache, cached.DefaultSubscriptionTTL, lo)
	configRepo := configuration.New(opts.Logger, opts.DB)
	attemptRepo := delivery_attempts.New(opts.Logger, opts.DB)
	backupJobRepo := backup_jobs.New(opts.Logger, opts.DB)
	filterRepo := cached.NewCachedFilterRepository(filters.New(opts.Logger, opts.DB), opts.Cache, cached.DefaultFilterTTL, lo)
	batchRetryRepo := batch_retries.New(lo, opts.DB)
	ffService := feature_flags.New(opts.Logger, opts.DB)

	rateLimiter := opts.Broker.RateLimiter

	counter := &telemetry.EventsCounter{}
	pb := telemetry.NewposthogBackend()
	defer pb.Close()
	mb := telemetry.NewmixpanelBackend()
	defer mb.Close()

	loadConfiguration, err := configRepo.LoadConfiguration(context.Background())
	if err != nil {
		return nil, fmt.Errorf("failed to initialize configuration: %w", err)
	}

	subscriptionsLoader := loader.NewSubscriptionLoader(subRepo, projectRepo, lo, 0)
	subscriptionsTable := memorystore.NewTable(memorystore.OptionSyncer(subscriptionsLoader))

	opts.setSubscriptionState(subscriptionsLoader, subscriptionsTable)

	err = memorystore.DefaultStore.Register("subscriptions", subscriptionsTable)
	if err != nil {
		return nil, err
	}

	err = subscriptionsLoader.SyncChanges(ctx, subscriptionsTable)
	if err != nil {
		return nil, err
	}

	featureFlag := fflag.NewFFlag(cfg.EnableFeatureFlag)
	newTelemetry := telemetry.NewTelemetry(lo, loadConfiguration,
		telemetry.OptionTracker(counter),
		telemetry.OptionBackend(pb),
		telemetry.OptionBackend(mb))

	caCertTLSCfg, err := config.GetCaCert()
	if err != nil {
		return nil, err
	}

	dispatcher, err := net.NewDispatcher(
		opts.Licenser,
		featureFlag,
		net.LoggerOption(lo),
		net.DetailedTraceOption(true),
		net.ProxyOption(cfg.Server.HTTP.HttpProxy, cfg.Server.HTTP.NoProxy),
		net.AllowListOption(cfg.Dispatcher.AllowList),
		net.BlockListOption(cfg.Dispatcher.BlockList),
		net.TLSConfigOption(cfg.Dispatcher.InsecureSkipVerify, opts.Licenser, caCertTLSCfg),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create new net dispatcher: %w", err)
	}

	// Single source of truth for circuit-breaker enablement: env folded into the
	// instance DB flag, with per-org overrides winning. Shared by the sampler gate,
	// per-delivery enforcement, and dashboard display so they never disagree.
	featureFlagFetcher := ffService
	cbEnablement := cbenablement.NewResolver(featureFlag, featureFlagFetcher, clock.NewRealClock(), lo)

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

	// The manager is always constructed and started. Each sampling tick is gated
	// live by EnabledFuncOption, so toggling the instance flag or an org override
	// takes effect without restarting the worker.
	circuitBreakerManager, err := cb.NewCircuitBreakerManager(
		cb.SkipSleepOption(masterDefaults.SkipSleep),
		cb.MasterConfigOption(masterDefaults),
		cb.ConfigProviderOption(func(projectID string) *cb.CircuitBreakerConfig {
			project, err := projectRepo.FetchProjectByID(ctx, projectID)
			if err != nil {
				lo.Warnf("Failed to fetch project %s for circuit breaker config, using default: %v", projectID, err)
				return &masterDefaults
			}
			if project.Config.CircuitBreaker == nil {
				lo.Warnf("Project %s has no circuit breaker config, using default", projectID)
				return &masterDefaults
			}
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
		cb.StoreOption(opts.Broker.CircuitBreakerStore),
		cb.ClockOption(clock.NewRealClock()),
		cb.LoggerOption(lo),
		cb.EnabledFuncOption(cbEnablement.EnabledAnywhere),
		// Returns true only when the alert was dispatched, so the manager counts an
		// alert that this tick actually produced. Every other exit reports false and
		// leaves the window's one alert unspent.
		cb.NotificationFunctionOption(func(n cb.NotificationType, c cb.CircuitBreakerConfig, b *cb.CircuitBreaker) (bool, error) {
			// This handler only knows how to disable a resource. A type it does
			// not recognise must not fall through to the disable side effect, so
			// it is rejected rather than silently deactivating the endpoint.
			if n != cb.TypeDisableResource {
				return false, fmt.Errorf("unsupported circuit breaker notification type: %s", n)
			}

			endpointId := strings.Split(b.Key, ":")[1]
			project, funcErr := projectRepo.FetchProjectByID(ctx, b.TenantId)
			if funcErr != nil {
				return false, funcErr
			}

			endpoint, funcErr := endpointRepo.FindEndpointByID(ctx, endpointId, b.TenantId)
			if funcErr != nil {
				return false, funcErr
			}

			// Honor per-org enablement (override wins) for the disable side effect,
			// matching the enforcement path. The sampler computes globally, but an
			// org with circuit breaking disabled (e.g. a disabled override while env
			// forces the instance default on) must not have its endpoints auto-disabled.
			if !cbEnablement.EnabledForOrg(ctx, project.OrganisationID) {
				return false, nil
			}

			// Circuit breaker auto-disable requires project.Config.DisableEndpoint,
			// matching per-delivery enforcement (see internal/endpoints/disable).
			if !disable.CircuitBreakerOwnsEndpointDisable(ctx, opts.Licenser, cbEnablement, project) {
				return false, nil
			}

			// Re-applied on every tick the breaker stays tripped, because the
			// endpoint may have been re-activated while it is still failing.
			statusChanged, breakerErr := endpointRepo.UpdateEndpointStatus(ctx, project.UID, endpoint.UID, datastore.InactiveEndpointStatus)
			if breakerErr != nil {
				return false, breakerErr
			}
			if statusChanged {
				b.DisableAlertPending = true
			}

			// Alerts fire on active-to-inactive transitions only, but keep retrying
			// within the window when enqueue failed on the transition tick.
			if b.NotificationsSent > 0 || !b.DisableAlertPending {
				return false, nil
			}

			ownerEmail := ""
			orgRepo := organisations.New(lo, opts.DB)
			if org, err := orgRepo.FetchOrganisationByID(ctx, project.OrganisationID); err == nil {
				if owner, err := users.New(opts.Logger, opts.DB).FindUserByID(ctx, org.OwnerID); err == nil {
					ownerEmail = owner.Email
				}
			}

			sent := EnqueueCircuitBreakerNotifications(ctx, opts.Queue, lo, opts.Licenser, project, endpoint, ownerEmail, b.FailureRate)
			return sent, nil
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create circuit breaker manager: %w", err)
	}

	go circuitBreakerManager.Start(ctx, attemptRepo.GetFailureAndSuccessCounts)

	// Retention is paid-only and partition-based; the license is the single
	// gate (the delete-query retention system and its feature flag were
	// removed). LicensedRetentionPolicy re-reads partition state at job time
	// so `convoy utils partition` activates retention without a worker restart.
	// Until tables are partitioned it deletes nothing and logs the action.
	var ret retention.Retentioner
	if opts.Licenser.RetentionPolicy() && cfg.Retention.Enabled {
		if _, pErr := retention.UnpartitionedTables(ctx, opts.DB); pErr != nil {
			// Fail closed: a lookup failure is not a definitive "unpartitioned"
			// verdict, and boot-time DB reads already abort startup above
			// (LoadConfiguration). Do not guess which retention policy to install.
			return nil, fmt.Errorf("failed to check retention partition state: %w", pErr)
		}

		policyPeriod := strings.TrimSpace(loadConfiguration.GetRetentionPolicyConfig().Period)
		if policyPeriod == "" {
			policyPeriod = cfg.Retention.Period
		}
		policy, _err := time.ParseDuration(policyPeriod)
		if _err != nil {
			return nil, fmt.Errorf("failed to parse retention period: %w", _err)
		}

		ret = retention.NewLicensedRetentionPolicy(opts.DB, lo, policy)
		ret.Start(ctx, time.Minute)
	}

	channels := make(map[string]task.EventChannel)
	defaultCh, broadcastCh, dynamicCh := task.NewDefaultEventChannel(), task.NewBroadcastEventChannel(subscriptionsTable), task.NewDynamicEventChannel()
	channels["default"] = defaultCh
	channels["broadcast"] = broadcastCh
	channels["dynamic"] = dynamicCh

	// Route OAuth2 token exchange through a netjail dispatcher so the outbound
	// request to authentication.oauth2.url is subject to the IP allow/block
	// rules when IpRules is enabled. Without this the token endpoint is an
	// unfiltered outbound request (SSRF bypass). A dedicated dispatcher is used
	// so the token hop always validates TLS, instead of inheriting the webhook
	// insecure_skip_verify setting.
	oauth2Dispatcher, err := net.NewOAuth2Dispatcher(opts.Licenser, featureFlag, lo, cfg, caCertTLSCfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create oauth2 dispatcher: %w", err)
	}

	oauth2TokenService := services.NewOAuth2TokenService(
		opts.Cache,
		lo,
		services.WithOAuth2HTTPClient(oauth2Dispatcher.HTTPClient()),
		services.WithOAuth2Context(oauth2Dispatcher.ContextWithRules),
	)

	locker := opts.Broker.JobLocker

	eventDeliveryProcessorDeps := task.EventDeliveryProcessorDeps{
		EndpointRepo:               endpointRepo,
		EventDeliveryRepo:          eventDeliveryRepo,
		Licenser:                   opts.Licenser,
		ProjectRepo:                projectRepo,
		Queue:                      opts.Queue,
		RateLimiter:                rateLimiter,
		Dispatcher:                 dispatcher,
		AttemptsRepo:               attemptRepo,
		CircuitBreakerManager:      circuitBreakerManager,
		CBEnablement:               cbEnablement,
		FeatureFlag:                featureFlag,
		FeatureFlagFetcher:         featureFlagFetcher,
		EarlyAdopterFeatureFetcher: ffService,
		OAuth2TokenService:         oauth2TokenService,
		Logger:                     lo,
	}

	consumer.RegisterHandlers(convoy.EventProcessor, task.ProcessEventDelivery(eventDeliveryProcessorDeps), newTelemetry)

	eventProcessorDeps := task.EventProcessorDeps{
		EndpointRepo:       endpointRepo,
		EventRepo:          eventRepo,
		ProjectRepo:        projectRepo,
		EventQueue:         opts.Queue,
		TaskErrors:         opts.Broker.TaskErrors,
		SubRepo:            subRepo,
		FilterRepo:         filterRepo,
		Licenser:           opts.Licenser,
		OAuth2TokenService: oauth2TokenService,
		FeatureFlag:        featureFlag,
		FeatureFlagFetcher: ffService,
		Acker:              dynamicEventAcker,
		Logger:             lo,
	}

	consumer.RegisterHandlers(convoy.CreateEventProcessor, task.ProcessEventCreation(eventProcessorDeps), newTelemetry)
	consumer.RegisterHandlers(convoy.RetryEventProcessor, task.ProcessRetryEventDelivery(eventDeliveryProcessorDeps), newTelemetry)
	consumer.RegisterHandlers(convoy.CreateBroadcastEventProcessor, task.ProcessBroadcastEventCreation(broadcastCh, eventProcessorDeps), newTelemetry)
	consumer.RegisterHandlers(convoy.CreateDynamicEventProcessor, task.ProcessDynamicEventCreation(eventProcessorDeps), newTelemetry)

	if opts.Licenser.RetentionPolicy() {
		// RetentionPolicies needs a constructed Retentioner (gated on Retention.Enabled).
		// Registering with a nil ret panics on any stale or manually queued task.
		if ret != nil {
			consumer.RegisterHandlers(convoy.RetentionPolicies, task.RetentionPolicies(locker, ret, lo), nil)
		}
		consumer.RegisterHandlers(convoy.EnqueueBackupJobs, task.EnqueueBackupJobs(configRepo, backupJobRepo, lo), nil)
		consumer.RegisterHandlers(convoy.ProcessBackupJob, task.ProcessBackupJob(configRepo, eventRepo, eventDeliveryRepo, attemptRepo, backupJobRepo, locker, lo), nil)
	}

	// ManualBackupJob is always registered so instance-admin triggers work
	// without a license gate on the handler path. The task still requires DB
	// webhook_archiving.enabled and usable storage before exporting.
	consumer.RegisterHandlers(convoy.ManualBackupJob, task.ManualBackup(configRepo, eventRepo, eventDeliveryRepo, attemptRepo, locker, lo), nil)

	matchSubscriptionsDeps := task.MatchSubscriptionsDeps{
		Channels:                   channels,
		EndpointRepo:               endpointRepo,
		EventRepo:                  eventRepo,
		ProjectRepo:                projectRepo,
		EventDeliveryRepo:          eventDeliveryRepo,
		EventQueue:                 opts.Queue,
		SubRepo:                    subRepo,
		FilterRepo:                 filterRepo,
		Licenser:                   opts.Licenser,
		OAuth2TokenService:         oauth2TokenService,
		FeatureFlag:                featureFlag,
		FeatureFlagFetcher:         ffService,
		EarlyAdopterFeatureFetcher: ffService,
		Acker:                      dynamicEventAcker,
		Logger:                     lo,
	}
	consumer.RegisterHandlers(convoy.MatchEventSubscriptionsProcessor, task.MatchSubscriptionsAndCreateEventDeliveries(matchSubscriptionsDeps), newTelemetry)

	consumer.RegisterHandlers(convoy.MonitorTwitterSources, task.MonitorTwitterSources(opts.DB, opts.Queue, locker, lo), nil)
	consumer.RegisterHandlers(convoy.ExpireSecretsProcessor, task.ExpireSecret(endpointRepo), nil)
	consumer.RegisterHandlers(convoy.DailyAnalytics, task.PushDailyTelemetry(lo, opts.DB, locker), nil)
	consumer.RegisterHandlers(convoy.SnapshotUsage, task.SnapshotUsage(lo, opts.DB, opts.Cache, locker), nil)
	consumer.RegisterHandlers(convoy.RefreshEventDeliveryDailyCounts, task.RefreshEventDeliveryDailyCounts(lo, opts.DB, locker), nil)
	consumer.RegisterHandlers(convoy.RefreshQueueMetricsSnapshot, task.RefreshQueueMetricsSnapshot(lo, opts.DB, locker), nil)
	consumer.RegisterHandlers(convoy.EmailProcessor, task.ProcessEmails(sc), nil)

	// events_search tokenization is legacy FTS copy; unified list search (PDE-1009) reads
	// convoy.events directly and no longer enqueues TokenizeSearch jobs.

	consumer.RegisterHandlers(convoy.NotificationProcessor, task.ProcessNotifications(sc, dispatcher), nil)
	consumer.RegisterHandlers(convoy.MetaEventProcessor, task.ProcessMetaEvent(projectRepo, metaEventRepo, dispatcher, lo), nil)
	consumer.RegisterHandlers(convoy.DeleteArchivedTasksProcessor, task.DeleteArchivedTasks(opts.Queue, locker, lo), nil)

	consumer.RegisterHandlers(convoy.BatchRetryProcessor, task.ProcessBatchRetry(batchRetryRepo, eventDeliveryRepo, opts.Queue, lo), nil)

	bulkOnboardDeps := task.BulkOnboardDeps{
		EndpointRepo:               endpointRepo,
		SubRepo:                    subRepo,
		ProjectRepo:                projectRepo,
		Licenser:                   opts.Licenser,
		FeatureFlag:                featureFlag,
		FeatureFlagFetcher:         ffService,
		EarlyAdopterFeatureFetcher: ffService,
		Logger:                     lo,
	}
	consumer.RegisterHandlers(convoy.BulkOnboardProcessor, task.ProcessBulkOnboard(bulkOnboardDeps), newTelemetry)

	var billingClient billing.Client
	if cfg.UsesOrgBilling() {
		billingClient = billing.NewClient(cfg.Billing)
		consumer.RegisterHandlers(convoy.UpdateOrganisationStatus, task.UpdateOrganisationStatus(opts.DB, billingClient, locker, lo), nil)
	}

	err = metrics.RegisterQueueMetrics(opts.Queue, opts.DB, circuitBreakerManager)
	if err != nil {
		return nil, fmt.Errorf("failed to register queue metrics: %w", err)
	}

	// Optionally start the CDC-based backup collector.
	// CONVOY_CDC_BACKUP_ENABLED selects architecture (CDC vs cron). DB
	// webhook_archiving.enabled gates uploads on every flush so a dashboard
	// disable stops cold-storage export without a worker restart.
	var collector *backup_collector.BackupCollector
	dbArchivingEnabled := loadConfiguration.GetWebhookArchivingConfig().Enabled
	lo.Info(fmt.Sprintf("CDC backup config: cdc=%v, webhook_archiving_db=%v", cfg.WebhookArchiving.CDCEnabled, dbArchivingEnabled))
	if cfg.WebhookArchiving.CDCEnabled {
		usableErr := blobstore.StoragePolicyUsable(loadConfiguration.StoragePolicy)
		if usableErr != nil {
			if dbArchivingEnabled {
				return nil, fmt.Errorf("storage not usable for CDC backup: %w", usableErr)
			}
			// Archiving off: do not block worker boot on cold-storage config.
			lo.Warn(fmt.Sprintf("CDC enabled but storage not usable; skipping collector until storage is fixed and worker restarts: %v", usableErr))
		} else {
			blobStoreClient, blobErr := blobstore.NewBlobStoreClient(loadConfiguration.StoragePolicy, lo)
			if blobErr != nil {
				return nil, fmt.Errorf("failed to create blob store for CDC backup: %w", blobErr)
			}

			flushInterval := exporter.ParseBackupInterval(cfg.WebhookArchiving.Interval)

			// ReplicationDSN connects directly to Postgres (bypassing pgbouncer)
			// for the WAL replication protocol. Falls back to normal DSN if not set.
			replDSN := cfg.WebhookArchiving.ReplicationDSN
			if replDSN == "" {
				replDSN = cfg.Database.BuildDsn()
			}

			collector = backup_collector.NewBackupCollector(opts.DB.GetConn(), replDSN, blobStoreClient, flushInterval, lo, func(ctx context.Context) (bool, error) {
				dbCfg, err := configRepo.LoadConfiguration(ctx)
				if err != nil {
					return false, err
				}
				if !dbCfg.GetWebhookArchivingConfig().Enabled {
					return false, nil
				}
				// Enabled but unusable: error so doFlush skips without discarding,
				// keeping the buffer and LSN until storage is fixed.
				if err := blobstore.StoragePolicyUsable(dbCfg.StoragePolicy); err != nil {
					return false, err
				}
				return true, nil
			})
		}
	}

	return &Worker{
		consumer:        consumer,
		backupCollector: collector,
		logger:          lo,
	}, nil
}

func (w *Worker) Run(ctx context.Context, workerReady chan struct{}) error {
	if err := w.consumer.Start(); err != nil {
		return fmt.Errorf("failed to start consumer: %w", err)
	}
	w.logger.Printf("Starting Convoy Consumer Pool")

	// Start CDC backup collector if enabled
	if w.backupCollector != nil {
		if err := w.backupCollector.Start(ctx); err != nil {
			w.logger.Error(fmt.Sprintf("failed to start backup collector: %v", err))
			// Non-fatal — worker can still process events without CDC backup
		}
	}

	if workerReady != nil {
		close(workerReady)
	}

	<-ctx.Done()
	w.logger.Printf("Context canceled, stopping Convoy Consumer Pool...")

	if w.backupCollector != nil {
		w.backupCollector.Stop(ctx)
	}

	w.consumer.Stop()
	w.logger.Printf("Convoy Consumer Pool stopped")

	return ctx.Err()
}
