package main

import (
	"context"
	"fmt"
	"os"
	"time"
	_ "time/tzdata"

	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/frain-dev/convoy/util"
	log "github.com/sirupsen/logrus"
	prefixed "github.com/x-cray/logrus-prefixed-formatter"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/queue"
	"github.com/frain-dev/convoy/queue/redis"
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

			err = config.LoadConfig(cfgPath)
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

			var queuer queue.Queuer

			if cfg.Queue.Type == config.RedisQueueProvider {
				queuer, err = redis.New(cfg)
				if err != nil {
					return err
				}
			}

			if util.IsStringEmpty(string(cfg.Signature.Header)) {
				cfg.Signature.Header = config.DefaultSignatureHeader
				log.Warnf("signature header is blank. setting default %s", config.DefaultSignatureHeader)
			}

			conn := db.Database("convoy", nil)

			app.groupRepo = datastore.NewGroupRepo(conn)
			app.applicationRepo = datastore.NewApplicationRepo(conn)
			app.messageRepo = datastore.NewMessageRepository(conn)
			app.queue = queuer

			ensureMongoIndices(conn)
			err = ensureDefaultGroup(context.Background(), app.groupRepo)
			if err != nil {
				return err
			}

			return nil
		},
		PersistentPostRunE: func(cmd *cobra.Command, args []string) error {
			defer func() {
				err := app.queue.Close()
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

	cmd.PersistentFlags().StringVar(&configFile, "config", "./convoy.json", "Configuration file for convoy")

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

	datastore.EnsureIndex(conn, datastore.AppCollections, "uid", true)

	datastore.EnsureIndex(conn, datastore.MsgCollection, "uid", true)
	datastore.EnsureIndex(conn, datastore.MsgCollection, "event_type", false)
}

func ensureDefaultGroup(ctx context.Context, groupRepo convoy.GroupRepository) error {
	groups, err := groupRepo.LoadGroups(ctx, &convoy.GroupFilter{})
	if err != nil {
		return fmt.Errorf("failed to load groups - %w", err)
	}

	// a group already exists, so return
	if len(groups) != 0 {
		return nil
	}

	defaultGroup := &convoy.Group{
		UID:            uuid.New().String(),
		Name:           "default-group",
		CreatedAt:      primitive.NewDateTimeFromTime(time.Now()),
		UpdatedAt:      primitive.NewDateTimeFromTime(time.Now()),
		DocumentStatus: convoy.ActiveDocumentStatus,
	}

	err = groupRepo.CreateGroup(ctx, defaultGroup)
	if err != nil {
		return fmt.Errorf("failed to create default group - %w", err)
	}
	return nil
}

type app struct {
	groupRepo       convoy.GroupRepository
	applicationRepo convoy.ApplicationRepository
	messageRepo     convoy.MessageRepository
	queue           queue.Queuer
}

func getCtx() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), time.Second*1)
}
