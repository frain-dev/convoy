package hooks

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/cache"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/database"
	"github.com/frain-dev/convoy/database/postgres"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/internal/pkg/apm"
	"github.com/frain-dev/convoy/internal/pkg/cli"
	"github.com/frain-dev/convoy/internal/pkg/rdb"
	"github.com/frain-dev/convoy/internal/pkg/searcher"
	"github.com/frain-dev/convoy/limiter"
	"github.com/frain-dev/convoy/pkg/log"
	"github.com/frain-dev/convoy/queue"
	redisQueue "github.com/frain-dev/convoy/queue/redis"
	"github.com/frain-dev/convoy/tracer"
	"github.com/newrelic/go-agent/v3/newrelic"
	"github.com/oklog/ulid/v2"
	"github.com/spf13/cobra"
	"gopkg.in/guregu/null.v4"
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
		var li limiter.RateLimiter
		var q queue.Queuer

		if cfg.Queue.Type == config.RedisQueueProvider {
			redis, err := rdb.NewClient(cfg.Queue.BuildDsn())
			if err != nil {
				return err
			}
			queueNames := map[string]int{
				string(convoy.EventQueue):       3,
				string(convoy.CreateEventQueue): 3,
				string(convoy.SearchIndexQueue): 1,
				string(convoy.ScheduleQueue):    1,
				string(convoy.DefaultQueue):     1,
				string(convoy.StreamQueue):      1,
			}
			opts := queue.QueueOptions{
				Names:             queueNames,
				RedisClient:       redis,
				RedisAddress:      cfg.Queue.BuildDsn(),
				Type:              string(config.RedisQueueProvider),
				PrometheusAddress: cfg.Prometheus.Dsn,
			}
			q = redisQueue.NewQueue(opts)
		}

		lo := log.NewLogger(os.Stdout)

		if cfg.Tracer.Type == config.NewRelicTracerProvider {
			tr, err = tracer.NewTracer(cfg, lo.WithLogger())
			if err != nil {
				return err
			}
		}

		ca, err = cache.NewCache(cfg.Cache)
		if err != nil {
			return err
		}

		li, err = limiter.NewLimiter(cfg.Limiter)
		if err != nil {
			return err
		}

		se, err := searcher.NewSearchClient(cfg)
		if err != nil {
			return err
		}

		postgresDB, err := postgres.NewDB(cfg)
		if err != nil {
			return err
		}

		*db = *postgresDB

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
		app.Limiter = li
		app.Searcher = se

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

	// CONVOY_DB_DSN
	dbDsn, err := cmd.Flags().GetString("db-dsn")
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
		Dsn:      dbDsn,
		Scheme:   dbScheme,
		Host:     dbHost,
		Username: dbUsername,
		Password: dbPassword,
		Database: dbDatabase,
		Port:     dbPort,
	}

	// CONVOY_QUEUE_TYPE
	queueType, err := cmd.Flags().GetString("queue-type")
	if err != nil {
		return nil, err
	}

	// CONVOY_QUEUE_DSN
	queueDsn, err := cmd.Flags().GetString("queue-dsn")
	if err != nil {
		return nil, err
	}

	// CONVOY_QUEUE_SCHEME
	queueScheme, err := cmd.Flags().GetString("queue-scheme")
	if err != nil {
		return nil, err
	}

	// CONVOY_QUEUE_HOST
	queueHost, err := cmd.Flags().GetString("queue-host")
	if err != nil {
		return nil, err
	}

	// CONVOY_QUEUE_USERNAME
	queueUsername, err := cmd.Flags().GetString("queue-username")
	if err != nil {
		return nil, err
	}

	// CONVOY_QUEUE_PASSWORD
	queuePassword, err := cmd.Flags().GetString("queue-password")
	if err != nil {
		return nil, err
	}

	// CONVOY_QUEUE_DATABASE
	queueDatabase, err := cmd.Flags().GetString("queue-database")
	if err != nil {
		return nil, err
	}

	// CONVOY_QUEUE_PORT
	queuePort, err := cmd.Flags().GetInt("queue-port")
	if err != nil {
		return nil, err
	}

	c.Queue = config.QueueConfiguration{
		Type:     config.QueueProvider(queueType),
		Dsn:      queueDsn,
		Scheme:   queueScheme,
		Host:     queueHost,
		Username: queueUsername,
		Password: queuePassword,
		Database: queueDatabase,
		Port:     queuePort,
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

	return rows.Close()
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
	userRepo := postgres.NewUserRepo(a.DB)
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
