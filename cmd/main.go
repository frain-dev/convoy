package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"
	_ "time/tzdata"

	"github.com/frain-dev/convoy/cache"
	"github.com/frain-dev/convoy/datastore/badger"
	"github.com/frain-dev/convoy/searcher"
	"github.com/go-redis/redis/v8"
	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/frain-dev/convoy/logger"
	memqueue "github.com/frain-dev/convoy/queue/memqueue"
	redisqueue "github.com/frain-dev/convoy/queue/redis"
	"github.com/frain-dev/convoy/tracer"
	"github.com/getsentry/sentry-go"
	prefixed "github.com/x-cray/logrus-prefixed-formatter"

	log "github.com/sirupsen/logrus"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/limiter"
	"github.com/frain-dev/convoy/queue"
	"github.com/spf13/cobra"

	"github.com/frain-dev/convoy/datastore/mongo"
)

func main() {
	log.SetLevel(log.InfoLevel)

	log.SetFormatter(&prefixed.TextFormatter{
		DisableColors:   false,
		TimestampFormat: "2006-01-02 15:04:05",
		FullTimestamp:   true,
		ForceFormatting: true,
	})
	log.SetReportCaller(true)

	err := os.Setenv("TZ", "") // Use UTC by default :)
	if err != nil {
		log.Fatal("failed to set env - ", err)
	}

	app := &app{}
	var db datastore.DatabaseClient

	cli := NewCli(app, db)
	if err := cli.Execute(); err != nil {
		log.Fatal(err)
	}
}

func NewQueue(opts queue.QueueOptions, name string) queue.Queuer {
	optsType := opts.Type
	var convoyQueue queue.Queuer
	switch optsType {
	case "in-memory":
		opts.Name = name
		convoyQueue = memqueue.NewQueue(opts)

	case "redis":
		opts.Name = name
		convoyQueue = redisqueue.NewQueue(opts)
	default:
		log.Errorf("Invalid queue type: %v", optsType)
	}
	return convoyQueue
}

func ensureDefaultUser(ctx context.Context, a *app) error {
	pageable := datastore.Pageable{}

	users, _, err := a.userRepo.LoadUsersPaged(ctx, pageable)

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
		UID:            uuid.NewString(),
		FirstName:      "default",
		LastName:       "default",
		Email:          "superuser@default.com",
		Password:       string(p.Hash),
		CreatedAt:      primitive.NewDateTimeFromTime(time.Now()),
		UpdatedAt:      primitive.NewDateTimeFromTime(time.Now()),
		DocumentStatus: datastore.ActiveDocumentStatus,
	}

	err = a.userRepo.CreateUser(ctx, defaultUser)
	if err != nil {
		return fmt.Errorf("failed to create user - %w", err)
	}

	log.Infof("Created Superuser with username: %s and password: %s", defaultUser.Email, p.Plaintext)

	return nil
}

type app struct {
	apiKeyRepo        datastore.APIKeyRepository
	groupRepo         datastore.GroupRepository
	applicationRepo   datastore.ApplicationRepository
	eventRepo         datastore.EventRepository
	eventDeliveryRepo datastore.EventDeliveryRepository
	orgRepo           datastore.OrganisationRepository
	sourceRepo        datastore.SourceRepository
	userRepo          datastore.UserRepository
	eventQueue        queue.Queuer
	createEventQueue  queue.Queuer
	logger            logger.Logger
	tracer            tracer.Tracer
	cache             cache.Cache
	limiter           limiter.RateLimiter
	searcher          searcher.Searcher
}

func getCtx() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), time.Second*1)
}

func NewDB(cfg config.Configuration) (datastore.DatabaseClient, error) {
	switch cfg.Database.Type {
	case config.MongodbDatabaseProvider:
		db, err := mongo.New(cfg)
		if err != nil {
			return nil, err
		}
		return db, nil
	case config.InMemoryDatabaseProvider:
		bolt, err := badger.New(cfg)
		if err != nil {
			return nil, err
		}
		return bolt, nil
	default:
		return nil, errors.New("invalid database type")
	}
}

func preRun(app *app, db datastore.DatabaseClient) func(cmd *cobra.Command, args []string) error {
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

		err = config.OverrideConfigWithCliFlags(cmd, &cfg)
		if err != nil {
			return err
		}

		db, err := NewDB(cfg)
		if err != nil {
			return err
		}

		err = sentry.Init(sentry.ClientOptions{
			Debug:       true,
			Dsn:         cfg.Sentry.Dsn,
			Environment: cfg.Environment,
		})
		if err != nil {
			return err
		}

		defer sentry.Recover()              // recover any panic and report to sentry
		defer sentry.Flush(2 * time.Second) // send any events in sentry before exiting

		sentryHook := convoy.NewSentryHook(convoy.DefaultLevels)
		log.AddHook(sentryHook)

		var rC *redis.Client
		var tr tracer.Tracer
		var opts queue.QueueOptions
		var ca cache.Cache
		var li limiter.RateLimiter

		if cfg.Queue.Type == config.RedisQueueProvider {
			rC, err = redisqueue.NewClient(cfg)
			if err != nil {
				return err
			}
			opts = queue.QueueOptions{
				Type:  "redis",
				Redis: rC,
			}
		}

		if cfg.Queue.Type == config.InMemoryQueueProvider {
			opts = queue.QueueOptions{
				Type: "in-memory",
			}
		}

		lo, err := logger.NewLogger(cfg.Logger)
		if err != nil {
			return err
		}

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

		app.apiKeyRepo = db.APIRepo()
		app.groupRepo = db.GroupRepo()
		app.eventRepo = db.EventRepo()
		app.applicationRepo = db.AppRepo()
		app.eventDeliveryRepo = db.EventDeliveryRepo()
		app.orgRepo = db.OrganisationRepo()
		app.sourceRepo = db.SourceRepo()
		app.userRepo = db.UserRepo()

		app.eventQueue = NewQueue(opts, "EventQueue")
		app.createEventQueue = NewQueue(opts, "CreateEventQueue")

		app.logger = lo
		app.tracer = tr
		app.cache = ca
		app.limiter = li
		app.searcher = se

		return ensureDefaultUser(context.Background(), app)
	}
}

func postRun(app *app, db datastore.DatabaseClient) func(cmd *cobra.Command, args []string) error {
	return func(cmd *cobra.Command, args []string) error {
		defer func() {
			err := app.eventQueue.Stop()
			if err != nil {
				log.Errorln("failed to close app queue - ", err)
			}
		}()
		err := db.Disconnect(context.Background())
		if err == nil {
			os.Exit(0)
		}
		return err
	}
}

func parsePersistentArgs(app *app, cmd *cobra.Command) {
	var redisDsn string
	var dbDsn string
	var queue string
	var configFile string

	cmd.PersistentFlags().StringVar(&configFile, "config", "./convoy.json", "Configuration file for convoy")
	cmd.PersistentFlags().StringVar(&queue, "queue", "", "Queue provider (\"redis\" or \"in-memory\")")
	cmd.PersistentFlags().StringVar(&dbDsn, "db", "", "Database dsn or path to in-memory file")
	cmd.PersistentFlags().StringVar(&redisDsn, "redis", "", "Redis dsn")

	cmd.AddCommand(addVersionCommand())
	cmd.AddCommand(addCreateCommand(app))
	cmd.AddCommand(addGetComamnd(app))
	cmd.AddCommand(addServerCommand(app))
	cmd.AddCommand(addWorkerCommand(app))
	cmd.AddCommand(addQueueCommand(app))
	cmd.AddCommand(addRetryCommand(app))
	cmd.AddCommand(addSchedulerCommand(app))
	cmd.AddCommand(addUpgradeCommand(app))
	cmd.AddCommand(addIndexCommand(app))
}

type ConvoyCli struct {
	cmd *cobra.Command
}

func NewCli(app *app, db datastore.DatabaseClient) ConvoyCli {
	cmd := &cobra.Command{
		Use:     "Convoy",
		Version: convoy.GetVersion(),
		Short:   "Fast & reliable webhooks service",
	}

	cmd.PersistentPreRunE = preRun(app, db)
	cmd.PersistentPostRunE = postRun(app, db)
	parsePersistentArgs(app, cmd)

	return ConvoyCli{cmd: cmd}
}

func (c *ConvoyCli) SetArgs(args []string) {
	c.cmd.SetArgs(args)
}

func (c *ConvoyCli) Execute() error {
	return c.cmd.Execute()
}
