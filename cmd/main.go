package main

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"
	_ "time/tzdata"

	convoyRedis "github.com/frain-dev/convoy/queue/redis"
	"github.com/frain-dev/convoy/worker/task"
	"github.com/getsentry/sentry-go"
	"github.com/google/uuid"
	prefixed "github.com/x-cray/logrus-prefixed-formatter"
	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/frain-dev/convoy/util"
	"github.com/go-redis/redis/v8"
	log "github.com/sirupsen/logrus"
	"github.com/vmihailenco/taskq/v3"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/queue"
	"github.com/spf13/cobra"
	"go.mongodb.org/mongo-driver/mongo"
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

	var db *mongo.Client

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

			db, err = datastore.New(cfg)
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

			if cfg.Queue.Type == config.RedisQueueProvider {
				rC, qFn, err = convoyRedis.NewClient(cfg)
				if err != nil {
					return err
				}
			}

			if util.IsStringEmpty(string(cfg.GroupConfig.Signature.Header)) {
				cfg.GroupConfig.Signature.Header = config.DefaultSignatureHeader
				log.Warnf("signature header is blank. setting default %s", config.DefaultSignatureHeader)
			}

			u, err := url.Parse(cfg.Database.Dsn)
			if err != nil {
				return err
			}

			dbName := strings.TrimPrefix(u.Path, "/")
			conn := db.Database(dbName, nil)

			app.groupRepo = datastore.NewGroupRepo(conn)
			app.applicationRepo = datastore.NewApplicationRepo(conn)
			app.eventRepo = datastore.NewEventRepository(conn)
			app.eventDeliveryRepo = datastore.NewEventDeliveryRepository(conn)
			app.apiKeyRepo = datastore.NewApiKeyRepo(conn)
			app.eventQueue = convoyRedis.NewQueue(rC, qFn, "EventQueue")
			app.deadLetterQueue = convoyRedis.NewQueue(rC, qFn, "DeadLetterQueue")

			ensureMongoIndices(conn)
			err = ensureDefaultGroup(context.Background(), cfg, app)
			if err != nil {
				return err
			}

			return nil
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

	if err := cmd.Execute(); err != nil {
		log.Fatal(err)
	}
}

func ensureMongoIndices(conn *mongo.Database) {
	datastore.EnsureIndex(conn, datastore.GroupCollection, "uid", true)
	datastore.EnsureIndex(conn, datastore.GroupCollection, "name", true)
	datastore.EnsureIndex(conn, datastore.AppCollections, "uid", true)
	datastore.EnsureIndex(conn, datastore.EventCollection, "uid", true)
	datastore.EnsureIndex(conn, datastore.APIKeyCollection, "mask_id", true)
	datastore.EnsureIndex(conn, datastore.EventCollection, "event_type", false)
}

func ensureDefaultGroup(ctx context.Context, cfg config.Configuration, a *app) error {
	var filter *convoy.GroupFilter
	var groups []*convoy.Group
	var group *convoy.Group
	var err error

	filter = &convoy.GroupFilter{}
	groups, err = a.groupRepo.LoadGroups(ctx, filter)
	if err != nil {
		return fmt.Errorf("failed to load groups - %w", err)
	}

	// return if a group already exists or it's a multi tenant app
	if cfg.MultipleTenants {
		return nil
	}

	if len(groups) > 1 {
		filter = &convoy.GroupFilter{Names: []string{"default-group"}}
		groups, err = a.groupRepo.LoadGroups(ctx, filter)
		if err != nil {
			return fmt.Errorf("failed to load groups - %w", err)
		}
	}

	groupCfg := &convoy.GroupConfig{
		Strategy: convoy.StrategyConfiguration{
			Type: convoy.StrategyProvider(cfg.GroupConfig.Strategy.Type),
			Default: struct {
				IntervalSeconds uint64 `json:"intervalSeconds"`
				RetryLimit      uint64 `json:"retryLimit"`
			}{
				IntervalSeconds: cfg.GroupConfig.Strategy.Default.IntervalSeconds,
				RetryLimit:      cfg.GroupConfig.Strategy.Default.RetryLimit,
			},
		},
		Signature: convoy.SignatureConfiguration{
			Header: convoy.SignatureHeaderProvider(cfg.GroupConfig.Signature.Header),
			Hash:   cfg.GroupConfig.Signature.Hash,
		},
		DisableEndpoint: cfg.GroupConfig.DisableEndpoint,
	}

	if len(groups) == 0 {
		defaultGroup := &convoy.Group{
			UID:            uuid.New().String(),
			Name:           "default-group",
			Config:         groupCfg,
			CreatedAt:      primitive.NewDateTimeFromTime(time.Now()),
			UpdatedAt:      primitive.NewDateTimeFromTime(time.Now()),
			DocumentStatus: convoy.ActiveDocumentStatus,
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
	groupRepo         convoy.GroupRepository
	applicationRepo   convoy.ApplicationRepository
	eventRepo         convoy.EventRepository
	eventDeliveryRepo convoy.EventDeliveryRepository
	apiKeyRepo        convoy.APIKeyRepository
	eventQueue        queue.Queuer
	deadLetterQueue   queue.Queuer
}

func getCtx() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), time.Second*1)
}
