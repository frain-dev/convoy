package hooks

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"time"

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
	"github.com/frain-dev/convoy/internal/pkg/apm"
	"github.com/frain-dev/convoy/internal/pkg/cli"
	"github.com/frain-dev/convoy/internal/pkg/rdb"
	"github.com/frain-dev/convoy/pkg/log"
	redisQueue "github.com/frain-dev/convoy/queue/redis"
	"github.com/frain-dev/convoy/tracer"
	"github.com/newrelic/go-agent/v3/newrelic"
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

		nwCfg := cfg.Tracer.NewRelic
		nRApp, err := newrelic.NewApplication(
			newrelic.ConfigAppName(nwCfg.AppName),
			newrelic.ConfigLicense(nwCfg.LicenseKey),
			newrelic.ConfigDistributedTracerEnabled(nwCfg.DistributedTracerEnabled),
			newrelic.ConfigEnabled(nwCfg.ConfigEnabled),
		)
		if err != nil {
			return err
		}

		apm.SetApplication(nRApp)

		var tr tracer.Tracer
		var ca cache.Cache
		var q queue.Queuer

		redis, err := rdb.NewClient(cfg.Redis.BuildDsn())
		if err != nil {
			return err
		}
		queueNames := map[string]int{
			string(convoy.EventQueue):       3,
			string(convoy.CreateEventQueue): 3,
			string(convoy.ScheduleQueue):    1,
			string(convoy.DefaultQueue):     1,
			string(convoy.MetaEventQueue):   1,
		}

		opts := queue.QueueOptions{
			Names:             queueNames,
			RedisClient:       redis,
			RedisAddress:      cfg.Redis.BuildDsn(),
			Type:              string(config.RedisQueueProvider),
			PrometheusAddress: cfg.Prometheus.Dsn,
		}
		q = redisQueue.NewQueue(opts)

		lo := log.NewLogger(os.Stdout)

		if cfg.Tracer.Type == config.NewRelicTracerProvider {
			tr, err = tracer.NewTracer(cfg, lo.WithLogger())
			if err != nil {
				return err
			}
		}

		ca, err = cache.NewCache(cfg.Redis)
		if err != nil {
			return err
		}

		postgresDB, err := postgres.NewDB(cfg)
		if err != nil {
			return err
		}

		*db = *postgresDB

		hooks := dbhook.Init()

		// the order matters here
		projectListener := listener.NewProjectListener(q)
		hooks.RegisterHook(datastore.ProjectUpdated, projectListener.AfterUpdate)
		projectRepo := postgres.NewProjectRepo(postgresDB, ca)

		metaEventRepo := postgres.NewMetaEventRepo(postgresDB, ca)
		endpointListener := listener.NewEndpointListener(q, projectRepo, metaEventRepo)
		eventDeliveryListener := listener.NewEventDeliveryListener(q, projectRepo, metaEventRepo)

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

		app.DB = postgresDB
		app.Queue = q
		app.Logger = lo
		app.Tracer = tr
		app.Cache = ca

		if ok := shouldBootstrap(cmd); ok {
			err = ensureDefaultUser(context.Background(), app)
			if err != nil {
				return err
			}

			err = ensureInstanceConfig(context.Background(), app, cfg)
			if err != nil {
				return err
			}
		}

		return nil
	}
}

func PostRun(app *cli.App, db *postgres.Postgres) func(cmd *cobra.Command, args []string) error {
	return func(cmd *cobra.Command, args []string) error {
		err := db.Close()
		if err == nil {
			os.Exit(0)
		}
		return err
	}
}

func ensureInstanceConfig(ctx context.Context, a *cli.App, cfg config.Configuration) error {
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

	configuration, err := configRepo.LoadConfiguration(ctx)
	if err != nil {
		if errors.Is(err, datastore.ErrConfigNotFound) {
			a.Logger.Info("Creating Instance Config")
			return configRepo.CreateConfiguration(ctx, &datastore.Configuration{
				UID:                ulid.Make().String(),
				StoragePolicy:      storagePolicy,
				IsAnalyticsEnabled: cfg.Analytics.IsEnabled,
				IsSignupEnabled:    cfg.Auth.IsSignupEnabled,
				CreatedAt:          time.Now(),
				UpdatedAt:          time.Now(),
			})
		}

		return err
	}

	configuration.StoragePolicy = storagePolicy
	configuration.IsSignupEnabled = cfg.Auth.IsSignupEnabled
	configuration.IsAnalyticsEnabled = cfg.Analytics.IsEnabled
	configuration.UpdatedAt = time.Now()

	return configRepo.UpdateConfiguration(ctx, configuration)
}

func buildCliConfiguration(cmd *cobra.Command) (*config.Configuration, error) {
	c := &config.Configuration{}

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

	// Feature flags
	fflag, err := cmd.Flags().GetString("feature-flag")
	if err != nil {
		return nil, err
	}

	switch fflag {
	case config.Experimental:
		c.FeatureFlag = config.ExperimentalFlagLevel
	}

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
	pageable := datastore.Pageable{PerPage: 10, Direction: datastore.Next, NextCursor: datastore.DefaultCursor}
	userRepo := postgres.NewUserRepo(a.DB, a.Cache)
	users, _, err := userRepo.LoadUsersPaged(ctx, pageable)
	if err != nil {
		return fmt.Errorf("failed to load users - %w", err)
	}

	if len(users) > 0 {
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
