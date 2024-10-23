package hooks

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/frain-dev/convoy/internal/pkg/license"
	"github.com/frain-dev/convoy/internal/pkg/license/keygen"

	"github.com/frain-dev/convoy/internal/pkg/limiter"

	"github.com/frain-dev/convoy/util"
	pyro "github.com/grafana/pyroscope-go"

	fflag2 "github.com/frain-dev/convoy/internal/pkg/fflag"

	dbhook "github.com/frain-dev/convoy/database/hooks"
	"github.com/frain-dev/convoy/database/listener"
	"github.com/frain-dev/convoy/queue"
	"github.com/oklog/ulid/v2"
	"gopkg.in/guregu/null.v4"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/cache"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/database"
	"github.com/frain-dev/convoy/database/postgres"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/internal/pkg/cli"
	"github.com/frain-dev/convoy/internal/pkg/rdb"
	"github.com/frain-dev/convoy/internal/pkg/tracer"
	"github.com/frain-dev/convoy/internal/telemetry"
	"github.com/frain-dev/convoy/pkg/log"
	redisQueue "github.com/frain-dev/convoy/queue/redis"
	"github.com/spf13/cobra"
)

func PreRun(app *cli.App, db *postgres.Postgres) func(cmd *cobra.Command, args []string) error {
	return func(cmd *cobra.Command, args []string) error {
		cfgPath, err := cmd.Flags().GetString("config")
		if err != nil {
			return err
		}

		err = config.LoadConfig(cfgPath)
		if err != nil {
			return err
		}

		cfg, err := config.Get()
		if err != nil {
			return err
		}

		// Override with CLI Flags
		cliConfig, err := buildCliConfiguration(cmd)
		if err != nil {
			return err
		}

		if err = config.Override(cliConfig); err != nil {
			return err
		}

		postgresDB, err := postgres.NewDB(cfg)
		if err != nil {
			return err
		}

		*db = *postgresDB

		if _, ok := skipHook[cmd.Use]; ok {
			return nil
		}

		cfg, err = config.Get() // updated
		if err != nil {
			return err
		}

		app.TracerShutdown, err = tracer.Init(cfg.Tracer, cmd.Name())
		if err != nil {
			return err
		}

		var ca cache.Cache
		var q queue.Queuer

		redis, err := rdb.NewClient(cfg.Redis.BuildDsn())
		if err != nil {
			return err
		}
		queueNames := map[string]int{
			string(convoy.EventQueue):         5,
			string(convoy.CreateEventQueue):   2,
			string(convoy.EventWorkflowQueue): 3,
			string(convoy.ScheduleQueue):      1,
			string(convoy.DefaultQueue):       1,
			string(convoy.MetaEventQueue):     1,
		}

		opts := queue.QueueOptions{
			Names:             queueNames,
			RedisClient:       redis,
			RedisAddress:      cfg.Redis.BuildDsn(),
			Type:              string(config.RedisQueueProvider),
			PrometheusAddress: cfg.Prometheus.Dsn,
		}

		if cfg.Pyroscope.EnableProfiling {
			err = enableProfiling(cfg, cmd)
			if err != nil {
				return err
			}
		}

		q = redisQueue.NewQueue(opts)

		lo := log.NewLogger(os.Stdout)

		rd, err := rdb.NewClient(cfg.Redis.BuildDsn())
		if err != nil {
			return err
		}

		ca, err = cache.NewCache(cfg.Redis)
		if err != nil {
			return err
		}
		err = ca.Set(context.Background(), "ping", "pong", 10*time.Second)
		if err != nil {
			return err
		}

		hooks := dbhook.Init()

		// the order matters here
		projectListener := listener.NewProjectListener(q)
		hooks.RegisterHook(datastore.ProjectUpdated, projectListener.AfterUpdate)
		projectRepo := postgres.NewProjectRepo(postgresDB, ca)

		metaEventRepo := postgres.NewMetaEventRepo(postgresDB, ca)
		attemptsRepo := postgres.NewDeliveryAttemptRepo(postgresDB)
		endpointListener := listener.NewEndpointListener(q, projectRepo, metaEventRepo)
		eventDeliveryListener := listener.NewEventDeliveryListener(q, projectRepo, metaEventRepo, attemptsRepo)

		hooks.RegisterHook(datastore.EndpointCreated, endpointListener.AfterCreate)
		hooks.RegisterHook(datastore.EndpointUpdated, endpointListener.AfterUpdate)
		hooks.RegisterHook(datastore.EndpointDeleted, endpointListener.AfterDelete)
		hooks.RegisterHook(datastore.EventDeliveryUpdated, eventDeliveryListener.AfterUpdate)

		if ok := shouldCheckMigration(cmd); ok {
			err = checkPendingMigrations(db)
			if err != nil {
				return err
			}
		}

		app.Redis = rd.Client()
		app.DB = postgresDB
		app.Queue = q
		app.Logger = lo
		app.Cache = ca

		if ok := shouldBootstrap(cmd); ok {
			err = ensureDefaultUser(context.Background(), app)
			if err != nil {
				return err
			}

			dbCfg, err := ensureInstanceConfig(context.Background(), app, cfg)
			if err != nil {
				return err
			}

			t := telemetry.NewTelemetry(lo, dbCfg,
				telemetry.OptionBackend(telemetry.NewposthogBackend()),
				telemetry.OptionBackend(telemetry.NewmixpanelBackend()))

			err = t.Identify(cmd.Context(), dbCfg.UID)
			if err != nil {
				// do nothing?
				return err
			}
		}

		rateLimiter, err := limiter.NewLimiter(cfg)
		if err != nil {
			return err
		}

		app.Rate = rateLimiter

		app.Licenser, err = license.NewLicenser(&license.Config{
			KeyGen: keygen.Config{
				LicenseKey:  cfg.LicenseKey,
				OrgRepo:     postgres.NewOrgRepo(app.DB, app.Cache),
				UserRepo:    postgres.NewUserRepo(app.DB, app.Cache),
				ProjectRepo: projectRepo,
			},
		})
		if err != nil {
			return err
		}

		licenseOverrideCfg(&cfg, app.Licenser)
		if err = config.Override(&cfg); err != nil {
			return err
		}

		// update config singleton with the instance id
		if _, ok := skipConfigLoadCmd[cmd.Use]; !ok {
			configRepo := postgres.NewConfigRepo(app.DB)
			instCfg, err := configRepo.LoadConfiguration(cmd.Context())
			if err != nil {
				log.WithError(err).Error("Failed to load configuration")
			} else {
				cfg.InstanceId = instCfg.UID
				if err = config.Override(&cfg); err != nil {
					return err
				}
			}
		}

		return nil
	}
}

func licenseOverrideCfg(cfg *config.Configuration, licenser license.Licenser) {
	if !licenser.ConsumerPoolTuning() {
		cfg.ConsumerPoolSize = config.DefaultConfiguration.ConsumerPoolSize
	}

	if !licenser.IngestRate() {
		cfg.InstanceIngestRate = config.DefaultConfiguration.InstanceIngestRate
		cfg.ApiRateLimit = config.DefaultConfiguration.ApiRateLimit
	}
}

// these commands don't need to load instance config
var skipConfigLoadCmd = map[string]struct{}{
	"bootstrap": {},
}

// commands dont need the hooks
var skipHook = map[string]struct{}{
	// migrate commands
	"up":     {},
	"down":   {},
	"create": {},

	"version": {},
}

func PostRun(app *cli.App, db *postgres.Postgres) func(cmd *cobra.Command, args []string) error {
	return func(cmd *cobra.Command, args []string) error {
		err := db.Close()
		if err == nil {
			os.Exit(0)
		}

		err = app.TracerShutdown(context.Background())
		if err == nil {
			os.Exit(0)
		}
		return err
	}
}

func enableProfiling(cfg config.Configuration, cmd *cobra.Command) error {
	_, err := pyro.Start(pyro.Config{
		ApplicationName: cfg.Pyroscope.ProfileID,
		Tags: map[string]string{
			"cmd": cmd.Use,
		},
		// replace this with the address of pyro server
		ServerAddress: cfg.Pyroscope.URL,

		// you can disable logging by setting this to nil
		// Logger: pyro.StandardLogger,
		UploadRate: time.Second * 5,

		// optionally, if authentication is enabled, specify the API key:
		BasicAuthUser:     cfg.Pyroscope.Username,
		BasicAuthPassword: cfg.Pyroscope.Password,

		// but you can select the ones you want to use:
		ProfileTypes: []pyro.ProfileType{
			pyro.ProfileCPU,
			pyro.ProfileInuseObjects,
			pyro.ProfileAllocObjects,
			pyro.ProfileInuseSpace,
			pyro.ProfileAllocSpace,
			pyro.ProfileGoroutines,
			pyro.ProfileMutexCount,
			pyro.ProfileMutexDuration,
			pyro.ProfileBlockCount,
			pyro.ProfileBlockDuration,
		},
	})
	return err
}

func ensureInstanceConfig(ctx context.Context, a *cli.App, cfg config.Configuration) (*datastore.Configuration, error) {
	configRepo := postgres.NewConfigRepo(a.DB)

	s3 := datastore.S3Storage{
		Prefix:       null.NewString(cfg.StoragePolicy.S3.Prefix, true),
		Bucket:       null.NewString(cfg.StoragePolicy.S3.Bucket, true),
		AccessKey:    null.NewString(cfg.StoragePolicy.S3.AccessKey, true),
		SecretKey:    null.NewString(cfg.StoragePolicy.S3.SecretKey, true),
		Region:       null.NewString(cfg.StoragePolicy.S3.Region, true),
		SessionToken: null.NewString(cfg.StoragePolicy.S3.SessionToken, true),
		Endpoint:     null.NewString(cfg.StoragePolicy.S3.Endpoint, true),
	}

	onPrem := datastore.OnPremStorage{
		Path: null.NewString(cfg.StoragePolicy.OnPrem.Path, true),
	}

	storagePolicy := &datastore.StoragePolicyConfiguration{
		Type:   datastore.StorageType(cfg.StoragePolicy.Type),
		S3:     &s3,
		OnPrem: &onPrem,
	}

	retentionPolicy := &datastore.RetentionPolicyConfiguration{
		Policy:                   cfg.RetentionPolicy.Policy,
		IsRetentionPolicyEnabled: cfg.RetentionPolicy.IsRetentionPolicyEnabled,
	}

	circuitBreakerConfig := &datastore.CircuitBreakerConfig{
		SampleRate:                  cfg.CircuitBreaker.SampleRate,
		ErrorTimeout:                cfg.CircuitBreaker.ErrorTimeout,
		FailureThreshold:            cfg.CircuitBreaker.FailureThreshold,
		SuccessThreshold:            cfg.CircuitBreaker.SuccessThreshold,
		ObservabilityWindow:         cfg.CircuitBreaker.ObservabilityWindow,
		MinimumRequestCount:         cfg.CircuitBreaker.MinimumRequestCount,
		ConsecutiveFailureThreshold: cfg.CircuitBreaker.ConsecutiveFailureThreshold,
	}

	configuration, err := configRepo.LoadConfiguration(ctx)
	if err != nil {
		if errors.Is(err, datastore.ErrConfigNotFound) {
			a.Logger.Info("Creating Instance Config")
			c := &datastore.Configuration{
				UID:                  ulid.Make().String(),
				StoragePolicy:        storagePolicy,
				IsAnalyticsEnabled:   cfg.Analytics.IsEnabled,
				IsSignupEnabled:      cfg.Auth.IsSignupEnabled,
				RetentionPolicy:      retentionPolicy,
				CircuitBreakerConfig: circuitBreakerConfig,
				CreatedAt:            time.Now(),
				UpdatedAt:            time.Now(),
			}

			return c, configRepo.CreateConfiguration(ctx, c)
		}

		return configuration, err
	}

	configuration.StoragePolicy = storagePolicy
	configuration.IsSignupEnabled = cfg.Auth.IsSignupEnabled
	configuration.IsAnalyticsEnabled = cfg.Analytics.IsEnabled
	configuration.CircuitBreakerConfig = circuitBreakerConfig
	configuration.RetentionPolicy = retentionPolicy
	configuration.UpdatedAt = time.Now()

	return configuration, configRepo.UpdateConfiguration(ctx, configuration)
}

func buildCliConfiguration(cmd *cobra.Command) (*config.Configuration, error) {
	c := &config.Configuration{}

	// CONVOY_INSTANCE_INGEST_RATE
	instanceIngestRate, err := cmd.Flags().GetInt("instance-ingest-rate")
	if err != nil {
		return nil, err
	}

	c.InstanceIngestRate = instanceIngestRate

	// CONVOY_API_RATE_LIMIT
	apiRateLimit, err := cmd.Flags().GetInt("api-rate-limit")
	if err != nil {
		return nil, err
	}

	c.ApiRateLimit = apiRateLimit

	// CONVOY_LICENSE_KEY
	licenseKey, err := cmd.Flags().GetString("license-key")
	if err != nil {
		return nil, err
	}

	c.LicenseKey = licenseKey

	// CONVOY_DB_TYPE
	dbType, err := cmd.Flags().GetString("db-type")
	if err != nil {
		return nil, err
	}

	// CONVOY_DB_SCHEME
	dbScheme, err := cmd.Flags().GetString("db-scheme")
	if err != nil {
		return nil, err
	}

	// CONVOY_DB_HOST
	dbHost, err := cmd.Flags().GetString("db-host")
	if err != nil {
		return nil, err
	}

	// CONVOY_DB_USERNAME
	dbUsername, err := cmd.Flags().GetString("db-username")
	if err != nil {
		return nil, err
	}

	// CONVOY_DB_PASSWORD
	dbPassword, err := cmd.Flags().GetString("db-password")
	if err != nil {
		return nil, err
	}

	// CONVOY_DB_DATABASE
	dbDatabase, err := cmd.Flags().GetString("db-database")
	if err != nil {
		return nil, err
	}

	// CONVOY_DB_PORT
	dbPort, err := cmd.Flags().GetInt("db-port")
	if err != nil {
		return nil, err
	}

	c.Database = config.DatabaseConfiguration{
		Type:     config.DatabaseProvider(dbType),
		Scheme:   dbScheme,
		Host:     dbHost,
		Username: dbUsername,
		Password: dbPassword,
		Database: dbDatabase,
		Port:     dbPort,
	}

	// CONVOY_REDIS_SCHEME
	redisScheme, err := cmd.Flags().GetString("redis-scheme")
	if err != nil {
		return nil, err
	}

	// CONVOY_REDIS_HOST
	redisHost, err := cmd.Flags().GetString("redis-host")
	if err != nil {
		return nil, err
	}

	// CONVOY_REDIS_USERNAME
	redisUsername, err := cmd.Flags().GetString("redis-username")
	if err != nil {
		return nil, err
	}

	// CONVOY_REDIS_PASSWORD
	redisPassword, err := cmd.Flags().GetString("redis-password")
	if err != nil {
		return nil, err
	}

	// CONVOY_REDIS_DATABASE
	redisDatabase, err := cmd.Flags().GetString("redis-database")
	if err != nil {
		return nil, err
	}

	// CONVOY_REDIS_PORT
	redisPort, err := cmd.Flags().GetInt("redis-port")
	if err != nil {
		return nil, err
	}

	c.Redis = config.RedisConfiguration{
		Scheme:   redisScheme,
		Host:     redisHost,
		Username: redisUsername,
		Password: redisPassword,
		Database: redisDatabase,
		Port:     redisPort,
	}

	// CONVOY_RETENTION_POLICY
	retentionPolicy, err := cmd.Flags().GetString("retention-policy")
	if err != nil {
		return nil, err
	}

	if !util.IsStringEmpty(retentionPolicy) {
		c.RetentionPolicy.Policy = retentionPolicy
	}

	// CONVOY_RETENTION_POLICY_ENABLED
	isretentionPolicyEnabledSet := cmd.Flags().Changed("retention-policy-enabled")
	if isretentionPolicyEnabledSet {
		retentionPolicyEnabled, err := cmd.Flags().GetBool("retention-policy-enabled")
		if err != nil {
			return nil, err
		}

		c.RetentionPolicy.IsRetentionPolicyEnabled = retentionPolicyEnabled
	}

	// CONVOY_ENABLE_FEATURE_FLAG
	fflag, err := cmd.Flags().GetStringSlice("enable-feature-flag")
	if err != nil {
		return nil, err
	}
	c.EnableFeatureFlag = fflag

	// CONVOY_DISPATCHER_BLOCK_LIST
	ipBlockList, err := cmd.Flags().GetStringSlice("ip-block-list")
	if err != nil {
		return nil, err
	}
	if len(ipBlockList) > 0 {
		c.Dispatcher.BlockList = ipBlockList
	}

	// CONVOY_DISPATCHER_ALLOW_LIST
	ipAllowList, err := cmd.Flags().GetStringSlice("ip-allow-list")
	if err != nil {
		return nil, err
	}
	if len(ipAllowList) > 0 {
		c.Dispatcher.AllowList = ipAllowList
	}

	// tracing
	tracingProvider, err := cmd.Flags().GetString("tracer-type")
	if err != nil {
		return nil, err
	}

	c.Tracer = config.TracerConfiguration{
		Type: config.TracerProvider(tracingProvider),
	}

	switch c.Tracer.Type {
	case config.OTelTracerProvider:
		sampleRate, err := cmd.Flags().GetFloat64("otel-sample-rate")
		if err != nil {
			return nil, err
		}

		collectorURL, err := cmd.Flags().GetString("otel-collector-url")
		if err != nil {
			return nil, err
		}

		headerName, err := cmd.Flags().GetString("otel-auth-header-name")
		if err != nil {
			return nil, err
		}

		headerValue, err := cmd.Flags().GetString("otel-auth-header-value")
		if err != nil {
			return nil, err
		}

		insecureSkipVerify, err := cmd.Flags().GetBool("otel-insecure-skip-verify")
		if err != nil {
			return nil, err
		}

		c.Tracer.OTel = config.OTelConfiguration{
			SampleRate:         sampleRate,
			CollectorURL:       collectorURL,
			InsecureSkipVerify: insecureSkipVerify,
			OTelAuth: config.OTelAuthConfiguration{
				HeaderName:  headerName,
				HeaderValue: headerValue,
			},
		}

	case config.SentryTracerProvider:
		dsn, err := cmd.Flags().GetString("sentry-dsn")
		if err != nil {
			return nil, err
		}

		c.Tracer.Sentry = config.SentryConfiguration{
			DSN: dsn,
		}

	}

	flag := fflag2.NewFFlag(c.EnableFeatureFlag)
	c.Metrics = config.MetricsConfiguration{
		IsEnabled: false,
	}

	if flag.CanAccessFeature(fflag2.Prometheus) {
		metricsBackend, err := cmd.Flags().GetString("metrics-backend")
		if err != nil {
			return nil, err
		}

		if !config.IsStringEmpty(metricsBackend) {
			c.Metrics = config.MetricsConfiguration{
				IsEnabled: false,
				Backend:   config.MetricsBackend(metricsBackend),
			}

			switch c.Metrics.Backend {
			case config.PrometheusMetricsProvider:
				sampleTime, err := cmd.Flags().GetUint64("metrics-prometheus-sample-time")
				if err != nil {
					return nil, err
				}

				if sampleTime < 1 {
					return nil, errors.New("metrics-prometheus-sample-time must be non-zero")
				}

				c.Metrics = config.MetricsConfiguration{
					IsEnabled: true,
					Backend:   config.MetricsBackend(metricsBackend),
					Prometheus: config.PrometheusMetricsConfiguration{
						SampleTime: sampleTime,
					},
				}
			}
		} else {
			log.Warn("metrics backend not specified")
		}
	} else {
		log.Info(fflag2.ErrPrometheusMetricsNotEnabled)
	}

	maxRetrySeconds, err := cmd.Flags().GetUint64("max-retry-seconds")
	if err != nil {
		return nil, err
	}

	c.MaxRetrySeconds = maxRetrySeconds

	return c, nil
}

func checkPendingMigrations(db database.Database) error {
	p, ok := db.(*postgres.Postgres)
	if !ok {
		return errors.New("failed to open database")
	}

	type ID struct {
		Id string
	}
	counter := map[string]ID{}

	files, err := convoy.MigrationFiles.ReadDir("sql")
	if err != nil {
		return err
	}

	for _, file := range files {
		if !file.IsDir() {
			id := ID{Id: file.Name()}
			counter[id.Id] = id
		}
	}

	rows, err := p.GetDB().Queryx("SELECT id FROM convoy.gorp_migrations")
	if err != nil {
		return err
	}
	defer closeWithError(rows)

	for rows.Next() {
		var id ID

		err = rows.StructScan(&id)
		if err != nil {
			return err
		}

		_, ok := counter[id.Id]
		if ok {
			delete(counter, id.Id)
		}
	}

	if len(counter) > 0 {
		return postgres.ErrPendingMigrationsFound
	}

	return nil
}

func shouldCheckMigration(cmd *cobra.Command) bool {
	if cmd.Annotations == nil {
		return true
	}

	val, ok := cmd.Annotations["CheckMigration"]
	if !ok {
		return true
	}

	if val != "false" {
		return true
	}

	return false
}

func shouldBootstrap(cmd *cobra.Command) bool {
	if cmd.Annotations == nil {
		return true
	}

	val, ok := cmd.Annotations["ShouldBootstrap"]
	if !ok {
		return true
	}

	if val != "false" {
		return true
	}

	return false
}

func ensureDefaultUser(ctx context.Context, a *cli.App) error {
	userRepo := postgres.NewUserRepo(a.DB, a.Cache)
	count, err := userRepo.CountUsers(ctx)
	if err != nil {
		return fmt.Errorf("failed to count users: %v", err)
	}

	if count > 0 {
		return nil
	}

	p := datastore.Password{Plaintext: "default"}
	err = p.GenerateHash()

	if err != nil {
		return err
	}

	defaultUser := &datastore.User{
		UID:           ulid.Make().String(),
		FirstName:     "default",
		LastName:      "default",
		Email:         "superuser@default.com",
		Password:      string(p.Hash),
		EmailVerified: true,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}

	err = userRepo.CreateUser(ctx, defaultUser)
	if err != nil {
		return fmt.Errorf("failed to create user - %w", err)
	}

	a.Logger.Infof("Created Superuser with username: %s and password: %s", defaultUser.Email, p.Plaintext)

	return nil
}

func closeWithError(closer io.Closer) {
	err := closer.Close()
	if err != nil {
		fmt.Printf("%v, an error occurred while closing the client", err)
	}
}
