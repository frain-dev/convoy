package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"
	_ "time/tzdata"

	configCmd "github.com/frain-dev/convoy/cmd/config"
	"github.com/frain-dev/convoy/cmd/ingest"
	"github.com/frain-dev/convoy/cmd/migrate"
	"github.com/frain-dev/convoy/cmd/retry"
	"github.com/frain-dev/convoy/cmd/scheduler"
	"github.com/frain-dev/convoy/cmd/server"
	"github.com/frain-dev/convoy/cmd/stream"
	"github.com/frain-dev/convoy/cmd/version"
	"github.com/frain-dev/convoy/cmd/worker"
	"github.com/frain-dev/convoy/database"
	"github.com/frain-dev/convoy/database/postgres"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/pkg/log"
	"github.com/frain-dev/convoy/util"
	"github.com/oklog/ulid/v2"
	"github.com/sirupsen/logrus"
	"gopkg.in/guregu/null.v4"

	"github.com/frain-dev/convoy/cache"
	"github.com/frain-dev/convoy/internal/pkg/apm"
	"github.com/frain-dev/convoy/internal/pkg/cli"
	"github.com/frain-dev/convoy/internal/pkg/rdb"
	"github.com/frain-dev/convoy/internal/pkg/searcher"
	redisqueue "github.com/frain-dev/convoy/queue/redis"
	"github.com/frain-dev/convoy/tracer"
	"github.com/newrelic/go-agent/v3/newrelic"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/limiter"
	"github.com/frain-dev/convoy/queue"
	"github.com/spf13/cobra"
)

func main() {
	slog := logrus.New()
	slog.Out = os.Stdout

	err := os.Setenv("TZ", "") // Use UTC by default :)
	if err != nil {
		slog.Fatal("failed to set env - ", err)
	}

	app := &cli.App{}
	app.Version = convoy.GetVersionFromFS(convoy.F)
	db := &postgres.Postgres{}

	cli := cli.NewCli(app, db)

	var redisDsn string
	var dbDsn string
	var queue string
	var configFile string

	cli.Flags().StringVar(&configFile, "config", "./convoy.json", "Configuration file for convoy")
	cli.Flags().StringVar(&queue, "queue", "", "Queue provider (\"redis\")")
	cli.Flags().StringVar(&dbDsn, "db", "", "Postgres database dsn")
	cli.Flags().StringVar(&redisDsn, "redis", "", "Redis dsn")

	cli.PersistentPreRunE(preRun(app, db))
	cli.PersistentPostRunE(postRun(app, db))

	cli.AddCommand(version.AddVersionCommand())
	cli.AddCommand(server.AddServerCommand(app))
	cli.AddCommand(worker.AddWorkerCommand(app))
	cli.AddCommand(retry.AddRetryCommand(app))
	cli.AddCommand(scheduler.AddSchedulerCommand(app))
	cli.AddCommand(migrate.AddMigrateCommand(app))
	cli.AddCommand(configCmd.AddConfigCommand(app))
	cli.AddCommand(stream.AddStreamCommand(app))
	cli.AddCommand(ingest.AddIngestCommand(app))

	if err := cli.Execute(); err != nil {
		slog.Fatal(err)
	}
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

	config, err := configRepo.LoadConfiguration(ctx)
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

	config.StoragePolicy = storagePolicy
	config.IsSignupEnabled = cfg.Auth.IsSignupEnabled
	config.IsAnalyticsEnabled = cfg.Analytics.IsEnabled
	config.UpdatedAt = time.Now()

	return configRepo.UpdateConfiguration(ctx, config)
}

func preRun(app *cli.App, db *postgres.Postgres) func(cmd *cobra.Command, args []string) error {
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
			rdb, err := rdb.NewClient(cfg.Queue.Redis.Dsn)
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
				RedisClient:       rdb,
				RedisAddress:      cfg.Queue.Redis.Dsn,
				Type:              string(config.RedisQueueProvider),
				PrometheusAddress: cfg.Prometheus.Dsn,
			}
			q = redisqueue.NewQueue(opts)
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

func postRun(app *cli.App, db *postgres.Postgres) func(cmd *cobra.Command, args []string) error {
	return func(cmd *cobra.Command, args []string) error {
		err := db.Close()
		if err == nil {
			os.Exit(0)
		}
		return err
	}
}

func buildCliConfiguration(cmd *cobra.Command) (*config.Configuration, error) {
	c := &config.Configuration{}

	// CONVOY_DB_DSN, CONVOY_DB_TYPE
	dbDsn, err := cmd.Flags().GetString("db")
	if err != nil {
		return nil, err
	}

	if !util.IsStringEmpty(dbDsn) {
		c.Database = config.DatabaseConfiguration{
			Type: config.PostgresDatabaseProvider,
			Dsn:  dbDsn,
		}
	}

	// CONVOY_REDIS_DSN
	redisDsn, err := cmd.Flags().GetString("redis")
	if err != nil {
		return nil, err
	}

	// CONVOY_QUEUE_PROVIDER
	queueDsn, err := cmd.Flags().GetString("queue")
	if err != nil {
		return nil, err
	}

	if !util.IsStringEmpty(queueDsn) {
		c.Queue.Type = config.QueueProvider(queueDsn)
		if queueDsn == "redis" && !util.IsStringEmpty(redisDsn) {
			c.Queue.Redis.Dsn = redisDsn
		}
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
	rows.Close()

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
