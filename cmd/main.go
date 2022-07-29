package main

import (
	"context"
	"fmt"
	"os"
	"time"
	_ "time/tzdata"

	"github.com/frain-dev/convoy/notification"
	"github.com/frain-dev/convoy/notification/email"
	"github.com/frain-dev/convoy/notification/noop"

	"github.com/frain-dev/convoy/cache"
	"github.com/frain-dev/convoy/internal/pkg/apm"
	"github.com/frain-dev/convoy/internal/pkg/rdb"
	"github.com/frain-dev/convoy/searcher"
	"github.com/google/uuid"
	"github.com/newrelic/go-agent/v3/newrelic"
	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/frain-dev/convoy/logger"
	redisqueue "github.com/frain-dev/convoy/queue/redis"
	"github.com/frain-dev/convoy/tracer"
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
	db := &mongo.Client{}

	cli := NewCli(app, db)
	if err := cli.Execute(); err != nil {
		log.Fatal(err)
	}
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
	apiKeyRepo              datastore.APIKeyRepository
	groupRepo               datastore.GroupRepository
	applicationRepo         datastore.ApplicationRepository
	deviceRepo              datastore.DeviceRepository
	eventRepo               datastore.EventRepository
	eventDeliveryRepo       datastore.EventDeliveryRepository
	subRepo                 datastore.SubscriptionRepository
	orgRepo                 datastore.OrganisationRepository
	orgMemberRepo           datastore.OrganisationMemberRepository
	orgInviteRepo           datastore.OrganisationInviteRepository
	sourceRepo              datastore.SourceRepository
	userRepo                datastore.UserRepository
	configRepo              datastore.ConfigurationRepository
	emailNotificationSender notification.Sender
	queue                   queue.Queuer
	logger                  logger.Logger
	tracer                  tracer.Tracer
	cache                   cache.Cache
	limiter                 limiter.RateLimiter
	searcher                searcher.Searcher
}

func getCtx() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), time.Second*1)
}

func preRun(app *app, db *mongo.Client) func(cmd *cobra.Command, args []string) error {
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

		db, err = mongo.New(cfg)
		if err != nil {
			return err
		}

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
				string(convoy.PriorityQueue):    5,
				string(convoy.EventQueue):       2,
				string(convoy.CreateEventQueue): 2,
				string(convoy.ScheduleQueue):    1,
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

		em := noop.NewNoopNotificationSender()
		if (cfg.SMTP != config.SMTPConfiguration{}) {
			em, err = email.NewEmailNotificationSender(&cfg.SMTP)
			if err != nil {
				return fmt.Errorf("failed to initialize new email notification sender: %v", err)
			}
		}

		app.subRepo = db.SubRepo()
		app.apiKeyRepo = db.APIRepo()
		app.groupRepo = db.GroupRepo()
		app.eventRepo = db.EventRepo()
		app.applicationRepo = db.AppRepo()
		app.eventDeliveryRepo = db.EventDeliveryRepo()
		app.sourceRepo = db.SourceRepo()
		app.deviceRepo = db.DeviceRepo()
		app.userRepo = db.UserRepo()
		app.configRepo = db.ConfigurationRepo()
		app.orgRepo = db.OrganisationRepo()
		app.orgMemberRepo = db.OrganisationMemberRepo()
		app.orgInviteRepo = db.OrganisationInviteRepo()

		app.queue = q
		app.logger = lo
		app.tracer = tr
		app.cache = ca
		app.limiter = li
		app.searcher = se
		app.emailNotificationSender = em

		return ensureDefaultUser(context.Background(), app)
	}
}

func postRun(app *app, db *mongo.Client) func(cmd *cobra.Command, args []string) error {
	return func(cmd *cobra.Command, args []string) error {
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
	cmd.PersistentFlags().StringVar(&queue, "queue", "", "Queue provider (\"redis\")")
	cmd.PersistentFlags().StringVar(&dbDsn, "db", "", "Database dsn or path to in-memory file")
	cmd.PersistentFlags().StringVar(&redisDsn, "redis", "", "Redis dsn")

	cmd.AddCommand(addVersionCommand())
	cmd.AddCommand(addCreateCommand(app))
	cmd.AddCommand(addGetComamnd(app))
	cmd.AddCommand(addServerCommand(app))
	cmd.AddCommand(addWorkerCommand(app))
	cmd.AddCommand(addRetryCommand(app))
	cmd.AddCommand(addSchedulerCommand(app))
	cmd.AddCommand(addUpgradeCommand(app))
	cmd.AddCommand(addIndexCommand(app))
	cmd.AddCommand(addStreamCommand(app))
	cmd.AddCommand(addLoginCommand())
	cmd.AddCommand(addSwitchCommand())
	cmd.AddCommand(addProjectCommand())
}

type ConvoyCli struct {
	cmd *cobra.Command
}

func NewCli(app *app, db *mongo.Client) ConvoyCli {
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
