package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"
	_ "time/tzdata"

	"github.com/frain-dev/convoy/cache"
	"github.com/frain-dev/convoy/logger"
	memqueue "github.com/frain-dev/convoy/queue/memqueue"
	redisqueue "github.com/frain-dev/convoy/queue/redis"
	"github.com/frain-dev/convoy/tracer"
	"github.com/frain-dev/convoy/worker/task"
	"github.com/getsentry/sentry-go"
	"github.com/go-redis/redis/v8"
	"github.com/google/uuid"
	prefixed "github.com/x-cray/logrus-prefixed-formatter"
	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/frain-dev/convoy/util"
	log "github.com/sirupsen/logrus"
	"github.com/vmihailenco/taskq/v3"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/queue"
	"github.com/spf13/cobra"

	"github.com/frain-dev/convoy/datastore/bolt"
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

	cmd := &cobra.Command{
		Use:   "Convoy",
		Short: "Fast & reliable webhooks service",
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			cfgPath, err := cmd.Flags().GetString("config")
			if err != nil {
				return err
			}

			override := new(config.Configuration)

			// override config with cli flags
			redisCliDsn, err := cmd.Flags().GetString("queue")
			if err != nil {
				return err
			}
			override.Queue.Redis.DSN = redisCliDsn

			mongoCliDsn, err := cmd.Flags().GetString("db")
			if err != nil {
				return err
			}
			override.Database.Dsn = mongoCliDsn

			err = config.LoadConfig(cfgPath, override)
			if err != nil {
				return err
			}

			cfg, err := config.Get()
			if err != nil {
				return err
			}

			db, err = NewDB(cfg)
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

			var qFn taskq.Factory
			var rC *redis.Client
			var lo logger.Logger
			var tr tracer.Tracer
			var lS queue.Storage
			var opts queue.QueueOptions
			var ca cache.Cache

			if cfg.Queue.Type == config.RedisQueueProvider {
				rC, qFn, err = redisqueue.NewClient(cfg)
				if err != nil {
					return err
				}
				opts = queue.QueueOptions{
					Type:    "redis",
					Redis:   rC,
					Factory: qFn,
				}
			}

			if cfg.Queue.Type == config.InMemoryQueueProvider {
				lS, qFn, err = memqueue.NewClient(cfg)
				if err != nil {
					return err
				}
				opts = queue.QueueOptions{
					Type:    "in-memory",
					Storage: lS,
					Factory: qFn,
				}
			}

			lo, err = logger.NewLogger(cfg.Logger)
			if err != nil {
				return err
			}

			if cfg.Tracer.Type == config.NewRelicTracerProvider {
				tr, err = tracer.NewTracer(cfg, lo.WithLogger())
				if err != nil {
					return err
				}
			}

			if util.IsStringEmpty(string(cfg.GroupConfig.Signature.Header)) {
				cfg.GroupConfig.Signature.Header = config.DefaultSignatureHeader
				log.Warnf("signature header is blank. setting default %s", config.DefaultSignatureHeader)
			}

			ca, err = cache.NewCache(cfg.Cache)
			if err != nil {
				return err
			}

			app.apiKeyRepo = db.APIRepo()
			app.groupRepo = db.GroupRepo()
			app.eventRepo = db.EventRepo()
			app.applicationRepo = db.AppRepo()
			app.eventDeliveryRepo = db.EventDeliveryRepo()

			app.eventQueue = NewQueue(opts, "EventQueue")
			app.deadLetterQueue = NewQueue(opts, "DeadLetterQueue")
			app.logger = lo
			app.tracer = tr
			app.cache = ca

			return ensureDefaultGroup(context.Background(), cfg, app)

		},
		PersistentPostRunE: func(cmd *cobra.Command, args []string) error {
			defer func() {
				err := app.eventQueue.Close()
				if err != nil {
					log.Errorln("failed to close app queue - ", err)
				}

				err = app.deadLetterQueue.Close()
				if err != nil {
					log.Errorln("failed to close app queue - ", err)
				}
			}()
			err := db.Disconnect(context.Background())
			if err == nil {
				os.Exit(0)
			}
			return err
		},
	}

	var configFile string
	var redisDsn string
	var mongoDsn string

	cmd.PersistentFlags().StringVar(&configFile, "config", "./convoy.json", "Configuration file for convoy")
	cmd.PersistentFlags().StringVar(&redisDsn, "queue", "", "Redis DSN")
	cmd.PersistentFlags().StringVar(&mongoDsn, "db", "", "MongoDB DSN")

	cmd.AddCommand(addVersionCommand())
	cmd.AddCommand(addCreateCommand(app))
	cmd.AddCommand(addGetComamnd(app))
	cmd.AddCommand(addServerCommand(app))
	cmd.AddCommand(addWorkerCommand(app))
	cmd.AddCommand(addQueueCommand(app))
	cmd.AddCommand(addRetryCommand(app))
	if err := cmd.Execute(); err != nil {
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

func ensureDefaultGroup(ctx context.Context, cfg config.Configuration, a *app) error {
	var filter *datastore.GroupFilter
	var groups []*datastore.Group
	var group *datastore.Group
	var err error

	filter = &datastore.GroupFilter{}
	groups, err = a.groupRepo.LoadGroups(ctx, filter)
	if err != nil {
		return fmt.Errorf("failed to load groups - %w", err)
	}

	// return if a group already exists or it's a multi tenant app
	if cfg.MultipleTenants {
		return nil
	}

	if len(groups) > 1 {
		filter = &datastore.GroupFilter{Names: []string{"default-group"}}
		groups, err = a.groupRepo.LoadGroups(ctx, filter)
		if err != nil {
			return fmt.Errorf("failed to load groups - %w", err)
		}
	}

	groupCfg := &datastore.GroupConfig{
		Strategy: datastore.StrategyConfiguration{
			Type: cfg.GroupConfig.Strategy.Type,
			Default: datastore.DefaultStrategyConfiguration{
				IntervalSeconds: cfg.GroupConfig.Strategy.Default.IntervalSeconds,
				RetryLimit:      cfg.GroupConfig.Strategy.Default.RetryLimit,
			},
		},
		Signature: datastore.SignatureConfiguration{
			Header: config.SignatureHeaderProvider(cfg.GroupConfig.Signature.Header),
			Hash:   cfg.GroupConfig.Signature.Hash,
		},
		DisableEndpoint: cfg.GroupConfig.DisableEndpoint,
	}

	if len(groups) == 0 {
		defaultGroup := &datastore.Group{
			UID:            uuid.New().String(),
			Name:           "default-group",
			Config:         groupCfg,
			CreatedAt:      primitive.NewDateTimeFromTime(time.Now()),
			UpdatedAt:      primitive.NewDateTimeFromTime(time.Now()),
			DocumentStatus: datastore.ActiveDocumentStatus,
		}

		err = a.groupRepo.CreateGroup(ctx, defaultGroup)
		if err != nil {
			return fmt.Errorf("failed to create default group - %w", err)
		}

		groups = append(groups, defaultGroup)
	}

	group = groups[0]

	group.Config = groupCfg
	err = a.groupRepo.UpdateGroup(ctx, group)
	if err != nil {
		log.WithError(err).Error("Default group update failed.")
		return err
	}

	taskName := convoy.EventProcessor.SetPrefix(group.Name)
	task.CreateTask(taskName, *group, task.ProcessEventDelivery(a.applicationRepo, a.eventDeliveryRepo, a.groupRepo))

	return nil
}

type app struct {
	apiKeyRepo        datastore.APIKeyRepository
	groupRepo         datastore.GroupRepository
	applicationRepo   datastore.ApplicationRepository
	eventRepo         datastore.EventRepository
	eventDeliveryRepo datastore.EventDeliveryRepository
	eventQueue        queue.Queuer
	deadLetterQueue   queue.Queuer
	logger            logger.Logger
	tracer            tracer.Tracer
	cache             cache.Cache
}

func getCtx() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), time.Second*1)
}

func NewDB(cfg config.Configuration) (datastore.DatabaseClient, error) {
	switch cfg.Database.Type {
	case "mongodb":
		db, err := mongo.New(cfg)
		if err != nil {
			return nil, err
		}
		return db, nil
	case "bolt":
		bolt, err := bolt.New(cfg)
		if err != nil {
			return nil, err
		}
		return bolt, nil
	default:
		return nil, errors.New("invalid database type")
	}
}
